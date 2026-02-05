package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the main configuration structure
type Config struct {
	Defaults Defaults          `yaml:"defaults"`
	Hosts    map[string]Host   `yaml:"hosts"`
	Tunnels  map[string]Tunnel `yaml:"tunnels"`
	Groups   map[string]Group  `yaml:"groups"`
}

// Defaults contains default settings for reconnection and keepalive
type Defaults struct {
	Reconnect ReconnectConfig `yaml:"reconnect"`
	KeepAlive KeepAliveConfig `yaml:"keep_alive"`
}

// ReconnectConfig controls automatic reconnection behavior
type ReconnectConfig struct {
	Enabled        bool          `yaml:"enabled"`
	MaxBackoff     time.Duration `yaml:"max_backoff"`
	InitialBackoff time.Duration `yaml:"initial_backoff"`
	Multiplier     float64       `yaml:"multiplier"`
}

// KeepAliveConfig controls SSH keepalive settings
type KeepAliveConfig struct {
	Interval time.Duration `yaml:"interval"`
}

// Host represents an SSH host configuration
type Host struct {
	Hostname     string `yaml:"hostname"`
	User         string `yaml:"user"`
	Port         int    `yaml:"port"`
	IdentityFile string `yaml:"identity_file"`
	ProxyJump    string `yaml:"proxy_jump"`
}

// Tunnel represents a single tunnel configuration
type Tunnel struct {
	Type       TunnelType `yaml:"type"`
	Host       string     `yaml:"host"`
	LocalHost  string     `yaml:"local_host"`
	LocalPort  int        `yaml:"local_port"`
	RemoteHost string     `yaml:"remote_host"`
	RemotePort int        `yaml:"remote_port"`
}

// TunnelType indicates whether the tunnel is local or remote forwarding
type TunnelType string

const (
	TunnelTypeLocal  TunnelType = "local"
	TunnelTypeRemote TunnelType = "remote"
)

// Group represents a named collection of tunnels
type Group struct {
	Description string   `yaml:"description"`
	Tunnels     []string `yaml:"tunnels"`
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Defaults: Defaults{
			Reconnect: ReconnectConfig{
				Enabled:        true,
				MaxBackoff:     30 * time.Second,
				InitialBackoff: 1 * time.Second,
				Multiplier:     2.0,
			},
			KeepAlive: KeepAliveConfig{
				Interval: 30 * time.Second,
			},
		},
		Hosts:   make(map[string]Host),
		Tunnels: make(map[string]Tunnel),
		Groups:  make(map[string]Group),
	}
}

// ConfigDir returns the path to the bore configuration directory
func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, ".bore"), nil
}

// ConfigPath returns the path to the config file
func ConfigPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

// Load reads and parses the configuration file
func Load() (*Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}
	return LoadFrom(path)
}

// LoadFrom reads and parses configuration from a specific path
func LoadFrom(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply defaults to tunnels
	for name, t := range cfg.Tunnels {
		if t.LocalHost == "" {
			t.LocalHost = "localhost"
		}
		if t.RemoteHost == "" {
			t.RemoteHost = "localhost"
		}
		cfg.Tunnels[name] = t
	}

	return cfg, nil
}

// Save writes the configuration to disk
func (c *Config) Save() error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}
	return c.SaveTo(path)
}

// SaveTo writes the configuration to a specific path
func (c *Config) SaveTo(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// GetTunnel returns a tunnel by name
func (c *Config) GetTunnel(name string) (Tunnel, bool) {
	t, ok := c.Tunnels[name]
	return t, ok
}

// GetHost returns a host by name
func (c *Config) GetHost(name string) (Host, bool) {
	h, ok := c.Hosts[name]
	return h, ok
}

// GetGroup returns a group by name
func (c *Config) GetGroup(name string) (Group, bool) {
	g, ok := c.Groups[name]
	return g, ok
}

// GetTunnelsForGroup returns all tunnel names for a group
func (c *Config) GetTunnelsForGroup(groupName string) ([]string, error) {
	group, ok := c.Groups[groupName]
	if !ok {
		return nil, fmt.Errorf("group not found: %s", groupName)
	}
	return group.Tunnels, nil
}
