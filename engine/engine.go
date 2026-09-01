// Package engine applies range operations to an append-only event log and
// materializes the converged view under a chosen conflict-resolution
// strategy.
package engine

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/d-led/eventfulranges/clock"
	"github.com/d-led/eventfulranges/interval"
	"github.com/d-led/eventfulranges/op"
	"github.com/d-led/eventfulranges/store"
	"github.com/d-led/eventfulranges/strategy"
)

// Config holds the engine options.
type Config struct {
	Clock         clock.Clock
	SnapshotEvery int
}

// Option customizes an engine.
type Option func(*Config)

// WithClock sets the clock used to stamp new operations.
func WithClock(c clock.Clock) Option {
	return func(cfg *Config) { cfg.Clock = c }
}

// WithSnapshotEvery snapshots the view every n new operations; 0 disables
// automatic snapshots.
func WithSnapshotEvery(n int) Option {
	return func(cfg *Config) { cfg.SnapshotEvery = n }
}

// Engine is a concurrency-safe CRDT over real-valued ranges backed by an
// append-only event log.
type Engine struct {
	mu            sync.RWMutex
	store         store.Log
	strategy      strategy.Strategy
	clock         clock.Clock
	snapshotEvery int

	ops      map[string]op.Op
	view     []interval.Interval
	segments []strategy.Segment
	adds     []interval.Interval
	removes  []interval.Interval
	version  int64
	since    int
	dirty    bool
}

// Open loads an existing log (and snapshot, when compatible) and returns a
// ready engine.
func Open(ctx context.Context, st store.Log, s strategy.Strategy, opts ...Option) (*Engine, error) {
	if !isDefined(s) {
		return nil, fmt.Errorf("engine: %w", strategy.ErrUnknownStrategy)
	}
	cfg := Config{Clock: &clock.Hybrid{}, SnapshotEvery: 100}
	for _, opt := range opts {
		opt(&cfg)
	}
	e := &Engine{
		store:         st,
		strategy:      s,
		clock:         cfg.Clock,
		snapshotEvery: cfg.SnapshotEvery,
		ops:           make(map[string]op.Op),
	}
	if err := e.reload(ctx); err != nil {
		return nil, err
	}
	return e, nil
}

// Apply applies a single operation, ignoring duplicates by ID.
func (e *Engine) Apply(ctx context.Context, o op.Op) error {
	return e.apply(ctx, []op.Op{o})
}

// ApplyAll applies a batch of operations, ignoring duplicates by ID.
func (e *Engine) ApplyAll(ctx context.Context, ops []op.Op) error {
	if len(ops) == 0 {
		return nil
	}
	return e.apply(ctx, ops)
}

// Materialize returns a copy of the current canonical interval view.
func (e *Engine) Materialize() []interval.Interval {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.ensureView()
	return append([]interval.Interval(nil), e.view...)
}

// Contains reports whether x belongs to the materialized set.
func (e *Engine) Contains(x float64) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.ensureView()
	return interval.Contains(e.view, x)
}

// Overlaps reports whether any materialized interval shares a point with iv.
func (e *Engine) Overlaps(iv interval.Interval) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.ensureView()
	return interval.Overlaps(e.view, iv)
}

// Ops returns the known operations sorted by ID for deterministic exchange.
func (e *Engine) Ops() []op.Op {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.opsList()
}

// Snapshot persists the current materialized view and log version.
func (e *Engine) Snapshot(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.snapshot(ctx)
}

func (e *Engine) apply(ctx context.Context, ops []op.Op) error {
	if err := validateOps(ops); err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.applyLocked(ctx, ops)
}

// applyLocked retries appends that conflict with concurrent writers.
func (e *Engine) applyLocked(ctx context.Context, ops []op.Op) error {
	for attempt := 0; ; attempt++ {
		fresh := e.unseen(ops)
		if len(fresh) == 0 {
			return nil
		}
		e.stamp(fresh)
		err := e.store.Append(ctx, e.version, fresh)
		if err != nil {
			if !errors.Is(err, store.ErrVersionConflict) || attempt >= 3 {
				return err
			}
			if cerr := e.catchUp(ctx); cerr != nil {
				return cerr
			}
			continue
		}
		return e.fold(ctx, fresh)
	}
}

// stamp assigns timestamps to operations that do not carry one yet.
func (e *Engine) stamp(fresh []op.Op) {
	for i := range fresh {
		if fresh[i].TS == 0 {
			fresh[i].TS = e.clock.Tick()
		}
	}
}

// fold records the appended operations in the local state and marks the view
// dirty, so it is rebuilt once when next requested rather than per operation.
func (e *Engine) fold(ctx context.Context, fresh []op.Op) error {
	for _, o := range fresh {
		e.clock.Observe(o.TS)
		e.ops[o.ID] = o
	}
	e.dirty = true
	e.version += int64(len(fresh))
	e.since += len(fresh)
	if e.snapshotEvery > 0 && e.since >= e.snapshotEvery {
		return e.snapshot(ctx)
	}
	return nil
}

// validateOps reports the first operation that is not well-formed.
func validateOps(ops []op.Op) error {
	for i := range ops {
		if err := ops[i].Validate(); err != nil {
			return fmt.Errorf("engine: op %q: %w", ops[i].ID, err)
		}
	}
	return nil
}

// unseen returns the operations whose IDs are not yet known.
func (e *Engine) unseen(ops []op.Op) []op.Op {
	var fresh []op.Op
	for _, o := range ops {
		if _, ok := e.ops[o.ID]; !ok {
			fresh = append(fresh, o)
		}
	}
	return fresh
}

// catchUp reads operations appended by other writers since the last known
// version and merges them into the local state.
func (e *Engine) catchUp(ctx context.Context) error {
	ops, version, err := e.store.Read(ctx, e.version)
	if err != nil {
		return err
	}
	for _, o := range ops {
		e.clock.Observe(o.TS)
		e.ops[o.ID] = o
	}
	e.dirty = true
	e.version = version
	return nil
}

// ensureView rebuilds the cached view when a deferred materialization is
// pending.
func (e *Engine) ensureView() {
	if e.dirty {
		e.materializeAll()
		e.dirty = false
	}
}

// materializeAll rebuilds the view, segments and add/remove sets from the full
// op set.
func (e *Engine) materializeAll() {
	ops := e.opsList()
	switch e.strategy {
	case strategy.LWW, strategy.FWW:
		e.segments = strategy.Segments(e.strategy, ops)
		e.view = strategy.ToIntervals(e.segments)
	case strategy.AdditiveWins:
		e.adds = strategy.IntervalsOf(ops, op.KindAdd)
		e.removes = strategy.IntervalsOf(ops, op.KindRemove)
		e.view = interval.Difference(e.adds, e.removes)
	case strategy.GrowOnly:
		e.adds = strategy.IntervalsOf(ops, op.KindAdd)
		e.view = e.adds
	}
}

// opsList returns the known operations sorted by ID.
func (e *Engine) opsList() []op.Op {
	out := make([]op.Op, 0, len(e.ops))
	for _, o := range e.ops {
		out = append(out, o)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func isDefined(s strategy.Strategy) bool {
	return s == strategy.LWW || s == strategy.FWW ||
		s == strategy.AdditiveWins || s == strategy.GrowOnly
}
