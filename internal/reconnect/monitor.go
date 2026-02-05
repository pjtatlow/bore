package reconnect

import (
	"context"
	"net"
	"runtime"
	"sync"
	"time"

	"github.com/iamcalledrob/netstatus"
)

// NetworkStatus represents the current network availability
type NetworkStatus int

const (
	NetworkUnknown NetworkStatus = iota
	NetworkAvailable
	NetworkUnavailable
)

// Monitor watches for network status changes
type Monitor struct {
	mu        sync.RWMutex
	status    NetworkStatus
	onChange  func(NetworkStatus)
	stopCh    chan struct{}
	stopOnce  sync.Once
	useNative bool
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewMonitor creates a new network monitor
func NewMonitor() *Monitor {
	return &Monitor{
		status:    NetworkUnknown,
		useNative: runtime.GOOS == "darwin" || runtime.GOOS == "windows",
	}
}

// Start begins monitoring network status
func (m *Monitor) Start(ctx context.Context) error {
	m.ctx, m.cancel = context.WithCancel(ctx)
	m.stopCh = make(chan struct{})

	if m.useNative {
		return m.startNative()
	}
	return m.startFallback()
}

// startNative uses netstatus for macOS/Windows
func (m *Monitor) startNative() error {
	monitor := netstatus.StartMonitor(m.ctx)

	// Register callback for status changes
	monitor.OnChange(func(status netstatus.Status) {
		m.mu.Lock()
		var newStatus NetworkStatus
		if status.Available {
			newStatus = NetworkAvailable
		} else {
			newStatus = NetworkUnavailable
		}
		changed := m.status != newStatus
		m.status = newStatus
		callback := m.onChange
		m.mu.Unlock()

		if changed && callback != nil {
			callback(newStatus)
		}
	})

	return nil
}

// startFallback uses DNS polling for Linux
func (m *Monitor) startFallback() error {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		// Check immediately
		m.checkNetwork()

		for {
			select {
			case <-m.ctx.Done():
				return
			case <-m.stopCh:
				return
			case <-ticker.C:
				m.checkNetwork()
			}
		}
	}()

	return nil
}

// checkNetwork checks network availability via DNS
func (m *Monitor) checkNetwork() {
	_, err := net.LookupHost("dns.google")

	m.mu.Lock()
	var newStatus NetworkStatus
	if err == nil {
		newStatus = NetworkAvailable
	} else {
		newStatus = NetworkUnavailable
	}
	changed := m.status != newStatus
	m.status = newStatus
	callback := m.onChange
	m.mu.Unlock()

	if changed && callback != nil {
		callback(newStatus)
	}
}

// Stop stops the monitor
func (m *Monitor) Stop() {
	m.stopOnce.Do(func() {
		if m.cancel != nil {
			m.cancel()
		}
		close(m.stopCh)
	})
}

// Status returns the current network status
func (m *Monitor) Status() NetworkStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}

// SetOnChange sets the callback for network status changes
func (m *Monitor) SetOnChange(fn func(NetworkStatus)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onChange = fn
}

// IsAvailable returns true if network is available
func (m *Monitor) IsAvailable() bool {
	return m.Status() == NetworkAvailable
}

// WaitForNetwork blocks until network is available or context is cancelled
func (m *Monitor) WaitForNetwork(ctx context.Context) error {
	if m.IsAvailable() {
		return nil
	}

	ch := make(chan struct{}, 1)
	m.mu.Lock()
	oldCallback := m.onChange
	m.onChange = func(status NetworkStatus) {
		if status == NetworkAvailable {
			select {
			case ch <- struct{}{}:
			default:
			}
		}
		if oldCallback != nil {
			oldCallback(status)
		}
	}
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.onChange = oldCallback
		m.mu.Unlock()
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-ch:
		return nil
	}
}
