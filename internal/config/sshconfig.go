package config

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"

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
	content, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// No SSH config is fine, return empty reader
			return &SSHConfigReader{cfg: &ssh_config.Config{}}, nil
		}
		return nil, err
	}

	// Filter out Match blocks which aren't supported by the ssh_config library
	filtered := filterMatchBlocks(content)

	cfg, err := ssh_config.Decode(bytes.NewReader(filtered))
	if err != nil {
		// If parsing still fails, return an empty config
		return &SSHConfigReader{cfg: &ssh_config.Config{}}, nil
	}

	return &SSHConfigReader{cfg: cfg}, nil
}

// filterMatchBlocks removes Match directive blocks from SSH config content.
// The ssh_config library doesn't support Match directives, so we strip them
// out along with their associated settings (until the next Host or Match line).
func filterMatchBlocks(content []byte) []byte {
	var result bytes.Buffer
	scanner := bufio.NewScanner(bytes.NewReader(content))
	inMatchBlock := false

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		lowerTrimmed := strings.ToLower(trimmed)

		// Check if this is a Host or Match directive
		if strings.HasPrefix(lowerTrimmed, "host ") || strings.HasPrefix(lowerTrimmed, "host\t") {
			inMatchBlock = false
		} else if strings.HasPrefix(lowerTrimmed, "match ") || strings.HasPrefix(lowerTrimmed, "match\t") {
			inMatchBlock = true
			continue // Skip the Match line itself
		}

		if !inMatchBlock {
			result.WriteString(line)
			result.WriteString("\n")
		}
	}

	return result.Bytes()
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
