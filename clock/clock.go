// Package clock provides timestamps that order range operations across
// replicas.
package clock

import (
	"sync"
	"time"
)

// Clock assigns timestamps to new operations and observes remote ones.
type Clock interface {
	// Tick returns a strictly increasing timestamp for a new local op.
	Tick() int64
	// Observe advances the clock past a timestamp seen from another replica.
	Observe(ts int64)
}

// Hybrid mixes the wall clock with a monotonic counter so that consecutive
// ticks never collide and remote timestamps are always taken into account.
// The zero value is ready to use.
type Hybrid struct {
	mu   sync.Mutex
	last int64
}

// Tick returns the next hybrid timestamp.
func (h *Hybrid) Tick() int64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	ts := time.Now().UnixNano()
	if ts <= h.last {
		ts = h.last + 1
	}
	h.last = ts
	return ts
}

// Observe advances the clock past ts.
func (h *Hybrid) Observe(ts int64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if ts > h.last {
		h.last = ts
	}
}

// Lamport is a pure logical clock, useful for deterministic tests.
type Lamport struct {
	mu   sync.Mutex
	last int64
}

// NewLamport returns a Lamport clock starting at zero.
func NewLamport() *Lamport {
	return &Lamport{}
}

// Tick returns the next logical timestamp.
func (l *Lamport) Tick() int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.last++
	return l.last
}

// Observe advances the clock past ts.
func (l *Lamport) Observe(ts int64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if ts > l.last {
		l.last = ts
	}
}
