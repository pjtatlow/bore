package tunnel

import (
	"context"
	"fmt"
	"sync"

	"github.com/pjtatlow/bore/internal/config"
	"github.com/pjtatlow/bore/internal/ssh"
)

// Manager manages tunnel lifecycle and SSH connections
type Manager struct {
	mu         sync.RWMutex
	cfg        *config.Config
	tunnels    map[string]Tunnel
	sshClients map[string]*ssh.Client
	sshReader  *config.SSHConfigReader
}

// NewManager creates a new tunnel manager
func NewManager(cfg *config.Config) (*Manager, error) {
	sshReader, err := config.NewSSHConfigReader()
	if err != nil {
		return nil, fmt.Errorf("failed to read SSH config: %w", err)
	}

	return &Manager{
		cfg:        cfg,
		tunnels:    make(map[string]Tunnel),
		sshClients: make(map[string]*ssh.Client),
		sshReader:  sshReader,
	}, nil
}

// StartTunnel starts a tunnel by name
func (m *Manager) StartTunnel(ctx context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already running
	if _, exists := m.tunnels[name]; exists {
		return fmt.Errorf("tunnel '%s' is already running", name)
	}

	// Get tunnel config
	tunnelCfg, ok := m.cfg.GetTunnel(name)
	if !ok {
		return fmt.Errorf("tunnel '%s' not found in config", name)
	}

	// Check for port conflicts
	if err := m.checkPortConflict(tunnelCfg); err != nil {
		return err
	}

	// Get or create SSH client for this host
	client, err := m.getOrCreateSSHClient(ctx, tunnelCfg.Host)
	if err != nil {
		return fmt.Errorf("failed to connect to host '%s': %w", tunnelCfg.Host, err)
	}

	// Create tunnel based on type
	var tunnel Tunnel
	switch tunnelCfg.Type {
	case config.TunnelTypeLocal:
		tunnel = NewLocalTunnel(name, tunnelCfg, client)
	case config.TunnelTypeRemote:
		tunnel = NewRemoteTunnel(name, tunnelCfg, client)
	default:
		return fmt.Errorf("unknown tunnel type: %s", tunnelCfg.Type)
	}

	// Start the tunnel
	if err := tunnel.Start(ctx); err != nil {
		return err
	}

	m.tunnels[name] = tunnel
	return nil
}

// StopTunnel stops a tunnel by name
func (m *Manager) StopTunnel(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	tunnel, exists := m.tunnels[name]
	if !exists {
		return fmt.Errorf("tunnel '%s' is not running", name)
	}

	if err := tunnel.Stop(); err != nil {
		return err
	}

	delete(m.tunnels, name)

	// Clean up unused SSH clients
	m.cleanupUnusedClients()

	return nil
}

// StartGroup starts all tunnels in a group
func (m *Manager) StartGroup(ctx context.Context, groupName string) error {
	tunnelNames, err := m.cfg.GetTunnelsForGroup(groupName)
	if err != nil {
		return err
	}

	// Check for port conflicts before starting any tunnels
	if err := m.checkGroupPortConflicts(tunnelNames); err != nil {
		return err
	}

	// Start all tunnels
	for _, name := range tunnelNames {
		if err := m.StartTunnel(ctx, name); err != nil {
			// Stop any tunnels we started on failure
			for _, startedName := range tunnelNames {
				if startedName == name {
					break
				}
				m.StopTunnel(startedName)
			}
			return fmt.Errorf("failed to start tunnel '%s': %w", name, err)
		}
	}

	return nil
}

// StopGroup stops all tunnels in a group
func (m *Manager) StopGroup(groupName string) error {
	tunnelNames, err := m.cfg.GetTunnelsForGroup(groupName)
	if err != nil {
		return err
	}

	var lastErr error
	for _, name := range tunnelNames {
		if err := m.StopTunnel(name); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// GetTunnelInfo returns info about a specific tunnel
func (m *Manager) GetTunnelInfo(name string) (Info, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tunnel, exists := m.tunnels[name]
	if !exists {
		return Info{}, false
	}

	return tunnel.Info(), true
}

// GetAllTunnelInfo returns info about all running tunnels
func (m *Manager) GetAllTunnelInfo() []Info {
	m.mu.RLock()
	defer m.mu.RUnlock()

	infos := make([]Info, 0, len(m.tunnels))
	for _, tunnel := range m.tunnels {
		infos = append(infos, tunnel.Info())
	}

	return infos
}

// ListRunningTunnels returns names of all running tunnels
func (m *Manager) ListRunningTunnels() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.tunnels))
	for name := range m.tunnels {
		names = append(names, name)
	}

	return names
}

// StopAll stops all tunnels
func (m *Manager) StopAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var lastErr error
	for name, tunnel := range m.tunnels {
		if err := tunnel.Stop(); err != nil {
			lastErr = err
		}
		delete(m.tunnels, name)
	}

	// Close all SSH clients
	for name, client := range m.sshClients {
		client.Close()
		delete(m.sshClients, name)
	}

	return lastErr
}

// getOrCreateSSHClient returns an existing SSH client or creates a new one
func (m *Manager) getOrCreateSSHClient(ctx context.Context, hostName string) (*ssh.Client, error) {
	if client, exists := m.sshClients[hostName]; exists {
		if client.IsConnected() {
			return client, nil
		}
		// Client disconnected, remove it
		client.Close()
		delete(m.sshClients, hostName)
	}

	// Resolve host config
	boreHost, _ := m.cfg.GetHost(hostName)
	resolvedHost := config.ResolveHost(hostName, boreHost, m.sshReader)

	// Create new client
	client := ssh.NewClient(resolvedHost, m.cfg)
	if err := client.Connect(ctx); err != nil {
		return nil, err
	}

	m.sshClients[hostName] = client
	return client, nil
}

// checkPortConflict checks if a tunnel's local port conflicts with running tunnels
func (m *Manager) checkPortConflict(tunnelCfg config.Tunnel) error {
	for name, tunnel := range m.tunnels {
		if tunnel.Config().LocalPort == tunnelCfg.LocalPort {
			return fmt.Errorf("port conflict: %d already used by tunnel '%s'",
				tunnelCfg.LocalPort, name)
		}
	}
	return nil
}

// checkGroupPortConflicts checks for port conflicts when enabling a group
func (m *Manager) checkGroupPortConflicts(tunnelNames []string) error {
	// Build map of ports from new tunnels
	newPorts := make(map[int]string)
	for _, name := range tunnelNames {
		// Skip if already running
		if _, running := m.tunnels[name]; running {
			continue
		}

		tunnelCfg, ok := m.cfg.GetTunnel(name)
		if !ok {
			return fmt.Errorf("tunnel '%s' not found", name)
		}

		// Check against running tunnels
		for runningName, tunnel := range m.tunnels {
			if tunnel.Config().LocalPort == tunnelCfg.LocalPort {
				return fmt.Errorf("port conflict: %d already used by running tunnel '%s', cannot enable '%s'",
					tunnelCfg.LocalPort, runningName, name)
			}
		}

		// Check against other tunnels in this group
		if existingName, exists := newPorts[tunnelCfg.LocalPort]; exists {
			return fmt.Errorf("port conflict: %d used by both '%s' and '%s' in this group",
				tunnelCfg.LocalPort, existingName, name)
		}

		newPorts[tunnelCfg.LocalPort] = name
	}

	return nil
}

// cleanupUnusedClients removes SSH clients that have no active tunnels
func (m *Manager) cleanupUnusedClients() {
	usedHosts := make(map[string]bool)
	for _, tunnel := range m.tunnels {
		usedHosts[tunnel.Config().Host] = true
	}

	for hostName, client := range m.sshClients {
		if !usedHosts[hostName] {
			client.Close()
			delete(m.sshClients, hostName)
		}
	}
}

// ReconnectTunnel attempts to reconnect a disconnected tunnel
func (m *Manager) ReconnectTunnel(ctx context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	tunnel, exists := m.tunnels[name]
	if !exists {
		return fmt.Errorf("tunnel '%s' not found", name)
	}

	tunnelCfg := tunnel.Config()

	// Stop the old tunnel
	tunnel.Stop()

	// Get fresh SSH client (reconnect if needed)
	delete(m.sshClients, tunnelCfg.Host)
	client, err := m.getOrCreateSSHClient(ctx, tunnelCfg.Host)
	if err != nil {
		tunnel.SetStatus(StatusError, err)
		return err
	}

	// Create new tunnel
	var newTunnel Tunnel
	switch tunnelCfg.Type {
	case config.TunnelTypeLocal:
		newTunnel = NewLocalTunnel(name, tunnelCfg, client)
	case config.TunnelTypeRemote:
		newTunnel = NewRemoteTunnel(name, tunnelCfg, client)
	}

	// Copy reconnect count
	newTunnel.SetStatus(StatusReconnecting, nil)

	if err := newTunnel.Start(ctx); err != nil {
		newTunnel.SetStatus(StatusError, err)
		m.tunnels[name] = newTunnel
		return err
	}

	m.tunnels[name] = newTunnel
	return nil
}
