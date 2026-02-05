package reconnect

import (
	"testing"
	"time"
)

func TestBackoff(t *testing.T) {
	initial := 1 * time.Second
	max := 10 * time.Second
	multiplier := 2.0

	b := NewBackoff(initial, max, multiplier)

	// First call should be around initial (with jitter)
	d1 := b.Next()
	if d1 < initial || d1 > initial+initial/4 {
		t.Errorf("first backoff %v out of expected range [%v, %v]", d1, initial, initial+initial/4)
	}

	// Second call should be around 2s
	d2 := b.Next()
	expected2 := 2 * time.Second
	if d2 < expected2 || d2 > expected2+expected2/4 {
		t.Errorf("second backoff %v out of expected range [%v, %v]", d2, expected2, expected2+expected2/4)
	}

	// Third call should be around 4s
	d3 := b.Next()
	expected3 := 4 * time.Second
	if d3 < expected3 || d3 > expected3+expected3/4 {
		t.Errorf("third backoff %v out of expected range [%v, %v]", d3, expected3, expected3+expected3/4)
	}

	// Fourth call should be around 8s
	d4 := b.Next()
	expected4 := 8 * time.Second
	if d4 < expected4 || d4 > expected4+expected4/4 {
		t.Errorf("fourth backoff %v out of expected range [%v, %v]", d4, expected4, expected4+expected4/4)
	}

	// Fifth call should be capped at max (10s)
	d5 := b.Next()
	if d5 < max || d5 > max+max/4 {
		t.Errorf("fifth backoff %v out of expected range [%v, %v]", d5, max, max+max/4)
	}
}

func TestBackoffReset(t *testing.T) {
	b := NewBackoff(1*time.Second, 10*time.Second, 2.0)

	// Advance backoff
	b.Next()
	b.Next()

	// Current should be 4s
	if b.Current() != 4*time.Second {
		t.Errorf("expected current 4s, got %v", b.Current())
	}

	// Reset should bring it back to initial
	b.Reset()
	if b.Current() != 1*time.Second {
		t.Errorf("expected current 1s after reset, got %v", b.Current())
	}
}

func TestBackoffCapping(t *testing.T) {
	b := NewBackoff(1*time.Second, 5*time.Second, 3.0)

	// After a few iterations, should be capped at max
	for i := 0; i < 10; i++ {
		b.Next()
	}

	// Current should be at max
	if b.Current() != 5*time.Second {
		t.Errorf("expected current to be capped at 5s, got %v", b.Current())
	}
}
