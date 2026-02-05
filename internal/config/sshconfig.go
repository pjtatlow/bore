package config

import (
	"os"
	"path/filepath"
	"strconv"

	"github.com/kevinburke/ssh_config"
)

// SSHConfigReader reads host information from ~/.ssh/config
type SSHConfigReader struct {
	cfg *ssh_config.Config
}

// NewSSHConfigReader creates a new reader for ~/.ssh/config
func NewSSHConfigReader() (*SSHConfigReader, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	configPath := filepath.Join(home, ".ssh", "config")
	f, err := os.Open(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// No SSH config is fine, return empty reader
			return &SSHConfigReader{cfg: &ssh_config.Config{}}, nil
		}
		return nil, err
	}
	defer f.Close()

	cfg, err := ssh_config.Decode(f)
	if err != nil {
		return nil, err
	}

	return &SSHConfigReader{cfg: cfg}, nil
}

// GetHostname returns the actual hostname for an alias
func (r *SSHConfigReader) GetHostname(alias string) string {
	hostname, _ := r.cfg.Get(alias, "HostName")
	if hostname == "" {
		return alias
	}
	return hostname
}

// GetUser returns the user for a host
func (r *SSHConfigReader) GetUser(alias string) string {
	user, _ := r.cfg.Get(alias, "User")
	return user
}

// GetPort returns the port for a host
func (r *SSHConfigReader) GetPort(alias string) int {
	portStr, _ := r.cfg.Get(alias, "Port")
	if portStr == "" {
		return 22
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 22
	}
	return port
}

// GetIdentityFile returns the identity file for a host
func (r *SSHConfigReader) GetIdentityFile(alias string) string {
	identityFile, _ := r.cfg.Get(alias, "IdentityFile")
	return expandPath(identityFile)
}

// GetProxyJump returns the proxy jump host for a host
func (r *SSHConfigReader) GetProxyJump(alias string) string {
	proxyJump, _ := r.cfg.Get(alias, "ProxyJump")
	return proxyJump
}

// ResolveHost combines bore config and SSH config to get full host details
func ResolveHost(hostName string, boreHost Host, sshReader *SSHConfigReader) Host {
	resolved := Host{
		Hostname:     boreHost.Hostname,
		User:         boreHost.User,
		Port:         boreHost.Port,
		IdentityFile: boreHost.IdentityFile,
		ProxyJump:    boreHost.ProxyJump,
	}

	// Fill in missing values from SSH config
	if resolved.Hostname == "" {
		resolved.Hostname = sshReader.GetHostname(hostName)
	}
	if resolved.User == "" {
		resolved.User = sshReader.GetUser(hostName)
	}
	if resolved.Port == 0 {
		resolved.Port = sshReader.GetPort(hostName)
	}
	if resolved.IdentityFile == "" {
		resolved.IdentityFile = sshReader.GetIdentityFile(hostName)
	}
	if resolved.ProxyJump == "" {
		resolved.ProxyJump = sshReader.GetProxyJump(hostName)
	}

	// Apply defaults
	if resolved.Hostname == "" {
		resolved.Hostname = hostName
	}
	if resolved.Port == 0 {
		resolved.Port = 22
	}

	// Expand identity file path
	resolved.IdentityFile = expandPath(resolved.IdentityFile)

	return resolved
}

// expandPath expands ~ to the home directory
func expandPath(path string) string {
	if path == "" {
		return ""
	}
	if len(path) >= 2 && path[:2] == "~/" {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}
