package config

import (
	"strings"
	"testing"
	"time"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name:    "default config is valid",
			config:  DefaultConfig(),
			wantErr: false,
		},
		{
			name: "invalid multiplier",
			config: &Config{
				Defaults: Defaults{
					Reconnect: ReconnectConfig{
						Multiplier:     0,
						InitialBackoff: 1 * time.Second,
						MaxBackoff:     30 * time.Second,
					},
				},
			},
			wantErr: true,
			errMsg:  "multiplier",
		},
		{
			name: "negative initial backoff",
			config: &Config{
				Defaults: Defaults{
					Reconnect: ReconnectConfig{
						Multiplier:     2.0,
						InitialBackoff: -1 * time.Second,
						MaxBackoff:     30 * time.Second,
					},
				},
			},
			wantErr: true,
			errMsg:  "initial_backoff",
		},
		{
			name: "max backoff less than initial",
			config: &Config{
				Defaults: Defaults{
					Reconnect: ReconnectConfig{
						Multiplier:     2.0,
						InitialBackoff: 30 * time.Second,
						MaxBackoff:     10 * time.Second,
					},
				},
			},
			wantErr: true,
			errMsg:  "max_backoff",
		},
		{
			name: "tunnel with invalid type",
			config: &Config{
				Defaults: Defaults{
					Reconnect: ReconnectConfig{
						Multiplier:     2.0,
						InitialBackoff: 1 * time.Second,
						MaxBackoff:     30 * time.Second,
					},
				},
				Tunnels: map[string]Tunnel{
					"test": {
						Type:       "invalid",
						Host:       "bastion",
						LocalPort:  8080,
						RemotePort: 80,
					},
				},
			},
			wantErr: true,
			errMsg:  "type",
		},
		{
			name: "tunnel without host is valid (host specified at runtime)",
			config: &Config{
				Defaults: Defaults{
					Reconnect: ReconnectConfig{
						Multiplier:     2.0,
						InitialBackoff: 1 * time.Second,
						MaxBackoff:     30 * time.Second,
					},
				},
				Tunnels: map[string]Tunnel{
					"test": {
						Type:       TunnelTypeLocal,
						LocalPort:  8080,
						RemotePort: 80,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "tunnel with invalid port",
			config: &Config{
				Defaults: Defaults{
					Reconnect: ReconnectConfig{
						Multiplier:     2.0,
						InitialBackoff: 1 * time.Second,
						MaxBackoff:     30 * time.Second,
					},
				},
				Tunnels: map[string]Tunnel{
					"test": {
						Type:       TunnelTypeLocal,
						Host:       "bastion",
						LocalPort:  0,
						RemotePort: 80,
					},
				},
			},
			wantErr: true,
			errMsg:  "local_port",
		},
		{
			name: "group with unknown tunnel",
			config: &Config{
				Defaults: Defaults{
					Reconnect: ReconnectConfig{
						Multiplier:     2.0,
						InitialBackoff: 1 * time.Second,
						MaxBackoff:     30 * time.Second,
					},
				},
				Tunnels: map[string]Tunnel{},
				Groups: map[string]Group{
					"dev": {
						Description: "Dev",
						Tunnels:     []string{"nonexistent"},
					},
				},
			},
			wantErr: true,
			errMsg:  "unknown tunnel",
		},
		{
			name: "group with empty tunnels",
			config: &Config{
				Defaults: Defaults{
					Reconnect: ReconnectConfig{
						Multiplier:     2.0,
						InitialBackoff: 1 * time.Second,
						MaxBackoff:     30 * time.Second,
					},
				},
				Groups: map[string]Group{
					"dev": {
						Description: "Dev",
						Tunnels:     []string{},
					},
				},
			},
			wantErr: true,
			errMsg:  "at least one tunnel",
		},
		{
			name: "valid config with tunnels and groups",
			config: &Config{
				Defaults: Defaults{
					Reconnect: ReconnectConfig{
						Multiplier:     2.0,
						InitialBackoff: 1 * time.Second,
						MaxBackoff:     30 * time.Second,
					},
				},
				Tunnels: map[string]Tunnel{
					"web": {
						Type:       TunnelTypeLocal,
						Host:       "bastion",
						LocalPort:  8080,
						RemotePort: 80,
					},
				},
				Groups: map[string]Group{
					"dev": {
						Description: "Dev",
						Tunnels:     []string{"web"},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got none")
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestCheckPortConflicts(t *testing.T) {
	tests := []struct {
		name    string
		active  map[string]Tunnel
		new     map[string]Tunnel
		wantErr bool
	}{
		{
			name:    "no conflicts with empty active",
			active:  map[string]Tunnel{},
			new:     map[string]Tunnel{"new": {LocalPort: 8080}},
			wantErr: false,
		},
		{
			name:    "no conflicts with different ports",
			active:  map[string]Tunnel{"existing": {LocalPort: 8080}},
			new:     map[string]Tunnel{"new": {LocalPort: 9090}},
			wantErr: false,
		},
		{
			name:    "conflict on same port",
			active:  map[string]Tunnel{"existing": {LocalPort: 8080}},
			new:     map[string]Tunnel{"new": {LocalPort: 8080}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckPortConflicts(tt.active, tt.new)
			if tt.wantErr && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
