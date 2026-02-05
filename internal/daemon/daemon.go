package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pjtatlow/bore/internal/config"
	"github.com/pjtatlow/bore/internal/ipc"
	"github.com/pjtatlow/bore/internal/reconnect"
	"github.com/pjtatlow/bore/internal/state"
	"github.com/pjtatlow/bore/internal/tunnel"
)

// Daemon is the main daemon process
type Daemon struct {
	manager        *tunnel.Manager
	server         *Server
	state          *state.State
	networkMonitor *reconnect.Monitor
	ctx            context.Context
	cancel         context.CancelFunc
	logger         *log.Logger
}

// New creates a new daemon instance
func New() (*Daemon, error) {
	manager, err := tunnel.NewManager()
	if err != nil {
		return nil, fmt.Errorf("failed to create tunnel manager: %w", err)
	}

	st, err := state.NewState()
	if err != nil {
		return nil, fmt.Errorf("failed to create state: %w", err)
	}

	// Set up logging
	logPath, err := ipc.LogPath()
	if err != nil {
		return nil, err
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}
	logger := log.New(logFile, "", log.LstdFlags)

	d := &Daemon{
		manager:        manager,
		state:          st,
		networkMonitor: reconnect.NewMonitor(),
		logger:         logger,
	}

	server, err := NewServer(d)
	if err != nil {
		return nil, fmt.Errorf("failed to create server: %w", err)
	}
	d.server = server

	return d, nil
}

// Run starts the daemon main loop
func (d *Daemon) Run() error {
	d.ctx, d.cancel = context.WithCancel(context.Background())

	// Write PID file
	if err := WritePID(); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}
	defer RemovePID()

	// Start IPC server
	if err := d.server.Start(d.ctx); err != nil {
		return fmt.Errorf("failed to start IPC server: %w", err)
	}
	defer d.server.Stop()

	// Start network monitor
	if err := d.networkMonitor.Start(d.ctx); err != nil {
		d.logger.Printf("Warning: failed to start network monitor: %v", err)
	}
	d.networkMonitor.SetOnChange(d.onNetworkChange)

	// Restore previous state
	if err := d.restoreState(); err != nil {
		d.logger.Printf("Warning: failed to restore state: %v", err)
	}

	d.logger.Printf("Daemon started (PID %d)", os.Getpid())

	// Handle signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Wait for either signal or context cancellation
	select {
	case <-sigCh:
		d.logger.Printf("Shutdown signal received")
	case <-d.ctx.Done():
		d.logger.Printf("Shutdown requested via IPC")
	}

	return d.shutdown()
}

// shutdown performs a graceful shutdown
func (d *Daemon) shutdown() error {
	d.cancel()

	// Save state before stopping tunnels
	if err := d.state.Save(); err != nil {
		d.logger.Printf("Warning: failed to save state: %v", err)
	}

	// Stop all tunnels
	if err := d.manager.StopAll(); err != nil {
		d.logger.Printf("Warning: error stopping tunnels: %v", err)
	}

	d.networkMonitor.Stop()
	d.logger.Printf("Daemon stopped")

	return nil
}

// restoreState restores tunnels from saved state
func (d *Daemon) restoreState() error {
	if err := d.state.Load(); err != nil {
		return err
	}

	// Restore groups first (they may contain tunnels)
	for _, gs := range d.state.GetActiveGroups() {
		if err := d.manager.StartGroup(d.ctx, gs.Name, gs.Host); err != nil {
			d.logger.Printf("Failed to restore group '%s': %v", gs.Name, err)
		} else {
			d.logger.Printf("Restored group '%s' via host '%s'", gs.Name, gs.Host)
		}
	}

	// Restore individual tunnels
	for _, ts := range d.state.GetActiveTunnels() {
		if err := d.manager.StartTunnel(d.ctx, ts.Name, ts.Host); err != nil {
			d.logger.Printf("Failed to restore tunnel '%s': %v", ts.Name, err)
		} else {
			d.logger.Printf("Restored tunnel '%s' via host '%s'", ts.Name, ts.Host)
		}
	}

	return nil
}

// onNetworkChange handles network status changes
func (d *Daemon) onNetworkChange(status reconnect.NetworkStatus) {
	if status == reconnect.NetworkAvailable {
		d.logger.Printf("Network restored, reconnecting tunnels...")
		d.reconnectAllTunnels()
	} else {
		d.logger.Printf("Network unavailable")
	}
}

// reconnectAllTunnels attempts to reconnect all tunnels
func (d *Daemon) reconnectAllTunnels() {
	for _, name := range d.manager.ListRunningTunnels() {
		info, ok := d.manager.GetTunnelInfo(name)
		if !ok {
			continue
		}

		if info.Status == tunnel.StatusError || info.Status == tunnel.StatusReconnecting {
			d.reconnectTunnelWithBackoff(name)
		}
	}
}

// reconnectTunnelWithBackoff attempts to reconnect a tunnel with exponential backoff
func (d *Daemon) reconnectTunnelWithBackoff(name string) {
	cfg, err := config.Load()
	if err != nil {
		d.logger.Printf("Failed to load config for reconnect: %v", err)
		return
	}

	backoff := reconnect.NewBackoff(
		cfg.Defaults.Reconnect.InitialBackoff,
		cfg.Defaults.Reconnect.MaxBackoff,
		cfg.Defaults.Reconnect.Multiplier,
	)

	go func() {
		for {
			select {
			case <-d.ctx.Done():
				return
			default:
			}

			// Wait for network if unavailable
			if !d.networkMonitor.IsAvailable() {
				d.networkMonitor.WaitForNetwork(d.ctx)
				backoff.Reset()
			}

			err := d.manager.ReconnectTunnel(d.ctx, name)
			if err == nil {
				d.logger.Printf("Reconnected tunnel '%s'", name)
				return
			}

			d.logger.Printf("Failed to reconnect tunnel '%s': %v", name, err)

			wait := backoff.Next()
			d.logger.Printf("Retrying tunnel '%s' in %v", name, wait)

			select {
			case <-d.ctx.Done():
				return
			case <-time.After(wait):
			}
		}
	}()
}

// HandleRequest implements RequestHandler
func (d *Daemon) HandleRequest(req ipc.Request) ipc.Response {
	switch req.Type {
	case ipc.ReqPing:
		return ipc.Response{Success: true}

	case ipc.ReqStatus:
		return d.handleStatus()

	case ipc.ReqStop:
		go func() {
			time.Sleep(100 * time.Millisecond)
			d.cancel()
		}()
		return ipc.Response{Success: true}

	case ipc.ReqTunnelUp:
		return d.handleTunnelUp(req.Data)

	case ipc.ReqTunnelDown:
		return d.handleTunnelDown(req.Data)

	case ipc.ReqGroupEnable:
		return d.handleGroupEnable(req.Data)

	case ipc.ReqGroupDisable:
		return d.handleGroupDisable(req.Data)

	default:
		return ipc.Response{
			Success: false,
			Error:   fmt.Sprintf("unknown request type: %s", req.Type),
		}
	}
}

func (d *Daemon) handleStatus() ipc.Response {
	// Check health of all SSH connections before reporting status
	d.manager.CheckHealth()

	tunnelInfos := d.manager.GetAllTunnelInfo()
	tunnelStatuses := make([]ipc.TunnelStatus, 0, len(tunnelInfos))

	for _, info := range tunnelInfos {
		uptime := ""
		if info.Stats.Uptime > 0 {
			uptime = info.Stats.Uptime.Truncate(time.Second).String()
		}
		tunnelStatuses = append(tunnelStatuses, ipc.TunnelStatus{
			Name:           info.Name,
			Type:           string(info.Config.Type),
			Host:           d.manager.GetTunnelHost(info.Name),
			LocalPort:      info.Config.LocalPort,
			RemoteHost:     info.Config.RemoteHost,
			RemotePort:     info.Config.RemotePort,
			Status:         info.Status,
			Error:          info.Error,
			BytesSent:      info.Stats.BytesSent,
			BytesReceived:  info.Stats.BytesReceived,
			Connections:    info.Stats.Connections,
			ReconnectCount: info.ReconnectCount,
			Uptime:         uptime,
		})
	}

	// Build group statuses
	runningTunnels := make(map[string]bool)
	for _, name := range d.manager.ListRunningTunnels() {
		runningTunnels[name] = true
	}

	cfg, _ := config.Load()
	groupStatuses := make([]ipc.GroupStatus, 0)
	for name, group := range cfg.Groups {
		enabled := true
		for _, tunnelName := range group.Tunnels {
			if !runningTunnels[tunnelName] {
				enabled = false
				break
			}
		}
		groupStatuses = append(groupStatuses, ipc.GroupStatus{
			Name:        name,
			Description: group.Description,
			Enabled:     enabled,
			Tunnels:     group.Tunnels,
		})
	}

	networkStatus := "unknown"
	switch d.networkMonitor.Status() {
	case reconnect.NetworkAvailable:
		networkStatus = "available"
	case reconnect.NetworkUnavailable:
		networkStatus = "unavailable"
	}

	status := ipc.StatusResponse{
		Running: true,
		PID:     os.Getpid(),
		Uptime:  d.state.Uptime().Truncate(time.Second).String(),
		Tunnels: tunnelStatuses,
		Groups:  groupStatuses,
		Network: ipc.NetworkStatusInfo{Status: networkStatus},
	}

	return ipc.Response{Success: true, Data: status}
}

func (d *Daemon) handleTunnelUp(data interface{}) ipc.Response {
	var req ipc.TunnelRequest
	if err := decodeData(data, &req); err != nil {
		return ipc.Response{Success: false, Error: err.Error()}
	}

	if req.Host == "" {
		return ipc.Response{Success: false, Error: "host is required"}
	}

	if err := d.manager.StartTunnel(d.ctx, req.Name, req.Host); err != nil {
		return ipc.Response{Success: false, Error: err.Error()}
	}

	d.state.AddTunnel(req.Name, req.Host)
	d.state.Save()
	d.logger.Printf("Started tunnel '%s' via host '%s'", req.Name, req.Host)

	return ipc.Response{Success: true}
}

func (d *Daemon) handleTunnelDown(data interface{}) ipc.Response {
	var req ipc.TunnelRequest
	if err := decodeData(data, &req); err != nil {
		return ipc.Response{Success: false, Error: err.Error()}
	}

	if err := d.manager.StopTunnel(req.Name); err != nil {
		return ipc.Response{Success: false, Error: err.Error()}
	}

	d.state.RemoveTunnel(req.Name)
	d.state.Save()
	d.logger.Printf("Stopped tunnel '%s'", req.Name)

	return ipc.Response{Success: true}
}

func (d *Daemon) handleGroupEnable(data interface{}) ipc.Response {
	var req ipc.GroupRequest
	if err := decodeData(data, &req); err != nil {
		return ipc.Response{Success: false, Error: err.Error()}
	}

	if req.Host == "" {
		return ipc.Response{Success: false, Error: "host is required"}
	}

	if err := d.manager.StartGroup(d.ctx, req.Name, req.Host); err != nil {
		return ipc.Response{Success: false, Error: err.Error()}
	}

	d.state.AddGroup(req.Name, req.Host)
	d.state.Save()
	d.logger.Printf("Enabled group '%s' via host '%s'", req.Name, req.Host)

	return ipc.Response{Success: true}
}

func (d *Daemon) handleGroupDisable(data interface{}) ipc.Response {
	var req ipc.GroupRequest
	if err := decodeData(data, &req); err != nil {
		return ipc.Response{Success: false, Error: err.Error()}
	}

	if err := d.manager.StopGroup(req.Name); err != nil {
		return ipc.Response{Success: false, Error: err.Error()}
	}

	d.state.RemoveGroup(req.Name)
	d.state.Save()
	d.logger.Printf("Disabled group '%s'", req.Name)

	return ipc.Response{Success: true}
}

func decodeData(data interface{}, target interface{}) error {
	if data == nil {
		return fmt.Errorf("missing request data")
	}

	// Re-encode and decode to handle type conversion
	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(bytes, target)
}
