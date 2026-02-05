package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if !cfg.Defaults.Reconnect.Enabled {
		t.Error("expected reconnect to be enabled by default")
	}
	if cfg.Defaults.Reconnect.MaxBackoff != 30*time.Second {
		t.Errorf("expected max backoff 30s, got %v", cfg.Defaults.Reconnect.MaxBackoff)
	}
	if cfg.Defaults.Reconnect.InitialBackoff != 1*time.Second {
		t.Errorf("expected initial backoff 1s, got %v", cfg.Defaults.Reconnect.InitialBackoff)
	}
	if cfg.Defaults.Reconnect.Multiplier != 2.0 {
		t.Errorf("expected multiplier 2.0, got %v", cfg.Defaults.Reconnect.Multiplier)
	}
	if cfg.Defaults.KeepAlive.Interval != 30*time.Second {
		t.Errorf("expected keepalive interval 30s, got %v", cfg.Defaults.KeepAlive.Interval)
	}
}

func TestLoadFrom(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	content := `
defaults:
  reconnect:
    enabled: true
    max_backoff: 60s
    initial_backoff: 2s
    multiplier: 1.5
  keep_alive:
    interval: 15s

hosts:
  bastion:
    hostname: bastion.example.com
    user: admin
    port: 2222
    identity_file: ~/.ssh/id_ed25519

tunnels:
  web-app:
    type: local
    host: bastion
    local_port: 8080
    remote_host: localhost
    remote_port: 80

  api-server:
    type: remote
    host: bastion
    local_port: 3000
    remote_port: 9000

groups:
  development:
    description: "Dev tunnels"
    tunnels: [web-app, api-server]
`

	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := LoadFrom(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Check defaults
	if cfg.Defaults.Reconnect.MaxBackoff != 60*time.Second {
		t.Errorf("expected max backoff 60s, got %v", cfg.Defaults.Reconnect.MaxBackoff)
	}
	if cfg.Defaults.Reconnect.InitialBackoff != 2*time.Second {
		t.Errorf("expected initial backoff 2s, got %v", cfg.Defaults.Reconnect.InitialBackoff)
	}
	if cfg.Defaults.Reconnect.Multiplier != 1.5 {
		t.Errorf("expected multiplier 1.5, got %v", cfg.Defaults.Reconnect.Multiplier)
	}

	// Check hosts
	if len(cfg.Hosts) != 1 {
		t.Errorf("expected 1 host, got %d", len(cfg.Hosts))
	}
	bastion, ok := cfg.Hosts["bastion"]
	if !ok {
		t.Fatal("expected bastion host")
	}
	if bastion.Hostname != "bastion.example.com" {
		t.Errorf("expected hostname bastion.example.com, got %s", bastion.Hostname)
	}
	if bastion.Port != 2222 {
		t.Errorf("expected port 2222, got %d", bastion.Port)
	}

	// Check tunnels
	if len(cfg.Tunnels) != 2 {
		t.Errorf("expected 2 tunnels, got %d", len(cfg.Tunnels))
	}
	webApp, ok := cfg.Tunnels["web-app"]
	if !ok {
		t.Fatal("expected web-app tunnel")
	}
	if webApp.Type != TunnelTypeLocal {
		t.Errorf("expected type local, got %s", webApp.Type)
	}
	if webApp.LocalPort != 8080 {
		t.Errorf("expected local port 8080, got %d", webApp.LocalPort)
	}

	// Check groups
	if len(cfg.Groups) != 1 {
		t.Errorf("expected 1 group, got %d", len(cfg.Groups))
	}
	dev, ok := cfg.Groups["development"]
	if !ok {
		t.Fatal("expected development group")
	}
	if len(dev.Tunnels) != 2 {
		t.Errorf("expected 2 tunnels in group, got %d", len(dev.Tunnels))
	}
}

func TestLoadFromNonExistent(t *testing.T) {
	cfg, err := LoadFrom("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("expected no error for non-existent file, got %v", err)
	}

	// Should return default config
	if !cfg.Defaults.Reconnect.Enabled {
		t.Error("expected default config")
	}
}

func TestGetTunnel(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Tunnels["test"] = Tunnel{
		Type:       TunnelTypeLocal,
		Host:       "bastion",
		LocalPort:  8080,
		RemotePort: 80,
	}

	tunnel, ok := cfg.GetTunnel("test")
	if !ok {
		t.Fatal("expected tunnel to be found")
	}
	if tunnel.LocalPort != 8080 {
		t.Errorf("expected local port 8080, got %d", tunnel.LocalPort)
	}

	_, ok = cfg.GetTunnel("nonexistent")
	if ok {
		t.Error("expected tunnel not to be found")
	}
}

func TestGetTunnelsForGroup(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Groups["dev"] = Group{
		Description: "Dev",
		Tunnels:     []string{"tunnel1", "tunnel2"},
	}

	tunnels, err := cfg.GetTunnelsForGroup("dev")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tunnels) != 2 {
		t.Errorf("expected 2 tunnels, got %d", len(tunnels))
	}

	_, err = cfg.GetTunnelsForGroup("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent group")
	}
}
