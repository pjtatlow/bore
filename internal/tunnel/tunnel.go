package tunnel

import (
	"context"
	"time"

	"github.com/pjtatlow/bore/internal/config"
)

// Status represents the current state of a tunnel
type Status string

const (
	StatusStopped      Status = "stopped"
	StatusConnecting   Status = "connecting"
	StatusConnected    Status = "connected"
	StatusReconnecting Status = "reconnecting"
	StatusError        Status = "error"
)

// Info contains runtime information about a tunnel
type Info struct {
	Name           string
	Config         config.Tunnel
	Status         Status
	Error          string
	Stats          StatsSnapshot
	ReconnectCount int
	LastConnected  time.Time
	LastError      time.Time
}

// Tunnel represents an SSH tunnel (local or remote forwarding)
type Tunnel interface {
	// Name returns the tunnel name
	Name() string

	// Config returns the tunnel configuration
	Config() config.Tunnel

	// Start begins the tunnel forwarding
	Start(ctx context.Context) error

	// Stop stops the tunnel
	Stop() error

	// Status returns the current tunnel status
	Status() Status

	// Info returns detailed tunnel information
	Info() Info

	// SetStatus updates the tunnel status
	SetStatus(status Status, err error)
}

// baseTunnel contains common tunnel functionality
type baseTunnel struct {
	name           string
	config         config.Tunnel
	status         Status
	lastError      error
	stats          *Stats
	reconnectCount int
	lastConnected  time.Time
	lastErrorTime  time.Time
}

func newBaseTunnel(name string, cfg config.Tunnel) *baseTunnel {
	return &baseTunnel{
		name:   name,
		config: cfg,
		status: StatusStopped,
		stats:  NewStats(),
	}
}

func (t *baseTunnel) Name() string {
	return t.name
}

func (t *baseTunnel) Config() config.Tunnel {
	return t.config
}

func (t *baseTunnel) Status() Status {
	return t.status
}

func (t *baseTunnel) SetStatus(status Status, err error) {
	t.status = status
	if err != nil {
		t.lastError = err
		t.lastErrorTime = time.Now()
	}
	if status == StatusConnected {
		t.lastConnected = time.Now()
	}
	if status == StatusReconnecting {
		t.reconnectCount++
	}
}

func (t *baseTunnel) Info() Info {
	errMsg := ""
	if t.lastError != nil {
		errMsg = t.lastError.Error()
	}
	return Info{
		Name:           t.name,
		Config:         t.config,
		Status:         t.status,
		Error:          errMsg,
		Stats:          t.stats.Snapshot(),
		ReconnectCount: t.reconnectCount,
		LastConnected:  t.lastConnected,
		LastError:      t.lastErrorTime,
	}
}

