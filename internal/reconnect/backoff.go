package reconnect

import (
	"math/rand"
	"time"
)

// Backoff implements exponential backoff with jitter
type Backoff struct {
	initial    time.Duration
	max        time.Duration
	multiplier float64
	current    time.Duration
}

// NewBackoff creates a new Backoff instance
func NewBackoff(initial, max time.Duration, multiplier float64) *Backoff {
	return &Backoff{
		initial:    initial,
		max:        max,
		multiplier: multiplier,
		current:    initial,
	}
}

// Next returns the next backoff duration and advances the backoff
func (b *Backoff) Next() time.Duration {
	duration := b.current

	// Add jitter (0-25%)
	jitter := time.Duration(float64(duration) * 0.25 * rand.Float64())
	duration += jitter

	// Advance to next interval
	b.current = time.Duration(float64(b.current) * b.multiplier)
	if b.current > b.max {
		b.current = b.max
	}

	return duration
}

// Reset resets the backoff to initial value
func (b *Backoff) Reset() {
	b.current = b.initial
}

// Current returns the current backoff duration without advancing
func (b *Backoff) Current() time.Duration {
	return b.current
}
