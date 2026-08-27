// Package clock provides timestamps that order range operations across
// replicas.
package clock

import (
	"sync/atomic"
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
	last atomic.Int64
}

// Tick returns the next hybrid timestamp.
func (h *Hybrid) Tick() int64 {
	for {
		prev := h.last.Load()
		ts := time.Now().UnixNano()
		if ts <= prev {
			ts = prev + 1
		}
		if h.last.CompareAndSwap(prev, ts) {
			return ts
		}
	}
}

// Observe advances the clock past ts.
func (h *Hybrid) Observe(ts int64) {
	advancePast(&h.last, ts)
}

// Lamport is a pure logical clock, useful for deterministic tests.
type Lamport struct {
	last atomic.Int64
}

// NewLamport returns a Lamport clock starting at zero.
func NewLamport() *Lamport {
	return &Lamport{}
}

// Tick returns the next logical timestamp.
func (l *Lamport) Tick() int64 {
	return l.last.Add(1)
}

// Observe advances the clock past ts.
func (l *Lamport) Observe(ts int64) {
	advancePast(&l.last, ts)
}

// advancePast moves c forward to at least ts, if it is behind.
func advancePast(c *atomic.Int64, ts int64) {
	for {
		prev := c.Load()
		if ts <= prev {
			return
		}
		if c.CompareAndSwap(prev, ts) {
			return
		}
	}
}
