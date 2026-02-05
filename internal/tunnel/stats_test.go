package tunnel

import (
	"testing"
	"time"
)

func TestStats(t *testing.T) {
	stats := NewStats()

	// Initial values should be zero
	snapshot := stats.Snapshot()
	if snapshot.BytesSent != 0 {
		t.Errorf("expected 0 bytes sent, got %d", snapshot.BytesSent)
	}
	if snapshot.BytesReceived != 0 {
		t.Errorf("expected 0 bytes received, got %d", snapshot.BytesReceived)
	}
	if snapshot.Connections != 0 {
		t.Errorf("expected 0 connections, got %d", snapshot.Connections)
	}

	// Add some data
	stats.AddSent(100)
	stats.AddReceived(200)
	stats.IncrementConnections()
	stats.IncrementConnections()

	snapshot = stats.Snapshot()
	if snapshot.BytesSent != 100 {
		t.Errorf("expected 100 bytes sent, got %d", snapshot.BytesSent)
	}
	if snapshot.BytesReceived != 200 {
		t.Errorf("expected 200 bytes received, got %d", snapshot.BytesReceived)
	}
	if snapshot.Connections != 2 {
		t.Errorf("expected 2 connections, got %d", snapshot.Connections)
	}
	if snapshot.TotalBytes() != 300 {
		t.Errorf("expected 300 total bytes, got %d", snapshot.TotalBytes())
	}
}

func TestStatsUptime(t *testing.T) {
	stats := NewStats()

	// Wait a bit
	time.Sleep(10 * time.Millisecond)

	snapshot := stats.Snapshot()
	if snapshot.Uptime < 10*time.Millisecond {
		t.Errorf("expected uptime >= 10ms, got %v", snapshot.Uptime)
	}
}

func TestStatsLastActivity(t *testing.T) {
	stats := NewStats()

	// Initially should be zero
	snapshot := stats.Snapshot()
	if !snapshot.LastActivity.IsZero() {
		t.Error("expected zero last activity initially")
	}

	// After activity, should be set
	stats.AddSent(1)
	snapshot = stats.Snapshot()
	if snapshot.LastActivity.IsZero() {
		t.Error("expected non-zero last activity after send")
	}
}
