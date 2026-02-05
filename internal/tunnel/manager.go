package tunnel

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pjtatlow/bore/internal/config"
	"github.com/pjtatlow/bore/internal/ssh"
)

// Manager manages tunnel lifecycle and SSH connections
type Manager struct {
	mu          sync.RWMutex
	tunnels     map[string]Tunnel
	tunnelHosts map[string]string // tracks which host each tunnel is connected through
	sshClients  map[string]*ssh.Client
	sshReader   *config.SSHConfigReader
}

// NewManager creates a new tunnel manager
func NewManager() (*Manager, error) {
	sshReader, err := config.NewSSHConfigReader()
	if err != nil {
		return nil, fmt.Errorf("failed to read SSH config: %w", err)
	}

	return &Manager{
		tunnels:     make(map[string]Tunnel),
		tunnelHosts: make(map[string]string),
		sshClients:  make(map[string]*ssh.Client),
		sshReader:   sshReader,
	}, nil
}

// StartTunnel starts a tunnel by name using the specified host
func (m *Manager) StartTunnel(ctx context.Context, name, host string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// If already running on same host, nothing to do
	if _, exists := m.tunnels[name]; exists {
		if m.tunnelHosts[name] == host {
			return nil
		}
		// Running on different host - stop it first to switch hosts
		if tunnel, ok := m.tunnels[name]; ok {
			tunnel.Stop()
			delete(m.tunnels, name)
			delete(m.tunnelHosts, name)
			m.cleanupUnusedClients()
		}
	}

	// Load config fresh
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Get tunnel config
	tunnelCfg, ok := cfg.GetTunnel(name)
	if !ok {
		return fmt.Errorf("tunnel '%s' not found in config", name)
	}

	// Check for port conflicts
	if err := m.checkPortConflict(tunnelCfg); err != nil {
		return err
	}

	// Get or create SSH client for this host
	client, err := m.getOrCreateSSHClient(ctx, host)
	if err != nil {
		return fmt.Errorf("failed to connect to host '%s': %w", host, err)
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
	m.tunnelHosts[name] = host
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
	delete(m.tunnelHosts, name)

	// Clean up unused SSH clients
	m.cleanupUnusedClients()

	return nil
}

// StartGroup starts all tunnels in a group using the specified host
func (m *Manager) StartGroup(ctx context.Context, groupName, host string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	tunnelNames, err := cfg.GetTunnelsForGroup(groupName)
	if err != nil {
		return err
	}

	// Check for port conflicts before starting any tunnels
	if err := m.checkGroupPortConflicts(tunnelNames, cfg); err != nil {
		return err
	}

	// Start all tunnels
	var started []string
	for _, name := range tunnelNames {
		if err := m.StartTunnel(ctx, name, host); err != nil {
			// Stop any tunnels we started on failure
			for _, startedName := range started {
				m.StopTunnel(startedName)
			}
			return fmt.Errorf("failed to start tunnel '%s': %w", name, err)
		}
		started = append(started, name)
	}

	return nil
}

// StopGroup stops all tunnels in a group
func (m *Manager) StopGroup(groupName string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	tunnelNames, err := cfg.GetTunnelsForGroup(groupName)
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

// GetTunnelHost returns the host a tunnel is connected through
func (m *Manager) GetTunnelHost(name string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.tunnelHosts[name]
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
		delete(m.tunnelHosts, name)
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

	// Load config fresh
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Resolve host config
	boreHost, _ := cfg.GetHost(hostName)
	resolvedHost := config.ResolveHost(hostName, boreHost, m.sshReader)

	// Create new client
	client := ssh.NewClient(resolvedHost, cfg)
	if err := client.Connect(ctx); err != nil {
		return nil, err
	}

	// Set up disconnect callback to update tunnel statuses
	client.SetOnDisconnect(func(err error) {
		m.onSSHDisconnect(hostName, err)
	})

	m.sshClients[hostName] = client
	return client, nil
}

// onSSHDisconnect handles SSH connection loss by updating all affected tunnels
func (m *Manager) onSSHDisconnect(hostName string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Mark all tunnels using this host as errored
	for name, tunnel := range m.tunnels {
		if m.tunnelHosts[name] == hostName {
			tunnel.SetStatus(StatusError, fmt.Errorf("SSH connection lost: %w", err))
		}
	}

	// Remove the disconnected client from cache
	if client, exists := m.sshClients[hostName]; exists {
		client.Close()
		delete(m.sshClients, hostName)
	}
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
func (m *Manager) checkGroupPortConflicts(tunnelNames []string, cfg *config.Config) error {
	// Build map of ports from new tunnels
	newPorts := make(map[int]string)
	for _, name := range tunnelNames {
		// Skip if already running
		if _, running := m.tunnels[name]; running {
			continue
		}

		tunnelCfg, ok := cfg.GetTunnel(name)
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
	for _, host := range m.tunnelHosts {
		usedHosts[host] = true
	}

	for hostName, client := range m.sshClients {
		if !usedHosts[hostName] {
			client.Close()
			delete(m.sshClients, hostName)
		}
	}
}

// CheckHealth performs a health check on all SSH connections concurrently and updates tunnel statuses
func (m *Manager) CheckHealth() {
	m.mu.RLock()
	// Get list of hosts and clients to check
	clients := make(map[string]*ssh.Client)
	for host, client := range m.sshClients {
		clients[host] = client
	}
	m.mu.RUnlock()

	if len(clients) == 0 {
		return
	}

	// Check all hosts concurrently with a 5 second timeout
	var wg sync.WaitGroup
	for _, client := range clients {
		wg.Add(1)
		go func(c *ssh.Client) {
			defer wg.Done()
			c.CheckHealth(5 * time.Second)
		}(client)
	}
	wg.Wait()
}

// ReconnectTunnel attempts to reconnect a disconnected tunnel
func (m *Manager) ReconnectTunnel(ctx context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	tunnel, exists := m.tunnels[name]
	if !exists {
		return fmt.Errorf("tunnel '%s' not found", name)
	}

	host, hasHost := m.tunnelHosts[name]
	if !hasHost {
		return fmt.Errorf("tunnel '%s' has no associated host", name)
	}

	tunnelCfg := tunnel.Config()

	// Stop the old tunnel
	tunnel.Stop()

	// Get fresh SSH client (reconnect if needed)
	delete(m.sshClients, host)
	client, err := m.getOrCreateSSHClient(ctx, host)
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
