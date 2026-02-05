package tunnel

import (
	"sync/atomic"
	"time"
)

// Stats tracks tunnel traffic statistics
type Stats struct {
	BytesSent     atomic.Int64
	BytesReceived atomic.Int64
	Connections   atomic.Int64
	StartTime     time.Time
	LastActivity  atomic.Int64 // Unix timestamp
}

// NewStats creates a new Stats instance
func NewStats() *Stats {
	return &Stats{
		StartTime: time.Now(),
	}
}

// AddSent adds to the bytes sent counter
func (s *Stats) AddSent(n int64) {
	s.BytesSent.Add(n)
	s.LastActivity.Store(time.Now().Unix())
}

// AddReceived adds to the bytes received counter
func (s *Stats) AddReceived(n int64) {
	s.BytesReceived.Add(n)
	s.LastActivity.Store(time.Now().Unix())
}

// IncrementConnections increments the connection counter
func (s *Stats) IncrementConnections() {
	s.Connections.Add(1)
}

// Snapshot returns a snapshot of the current stats
func (s *Stats) Snapshot() StatsSnapshot {
	lastActivity := s.LastActivity.Load()
	var lastActivityTime time.Time
	if lastActivity > 0 {
		lastActivityTime = time.Unix(lastActivity, 0)
	}

	return StatsSnapshot{
		BytesSent:     s.BytesSent.Load(),
		BytesReceived: s.BytesReceived.Load(),
		Connections:   s.Connections.Load(),
		StartTime:     s.StartTime,
		LastActivity:  lastActivityTime,
		Uptime:        time.Since(s.StartTime),
	}
}

// StatsSnapshot is an immutable snapshot of stats
type StatsSnapshot struct {
	BytesSent     int64
	BytesReceived int64
	Connections   int64
	StartTime     time.Time
	LastActivity  time.Time
	Uptime        time.Duration
}

// TotalBytes returns total bytes transferred
func (s StatsSnapshot) TotalBytes() int64 {
	return s.BytesSent + s.BytesReceived
}
