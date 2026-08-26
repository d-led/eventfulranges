// Package engine applies box operations to an append-only event log and
// materializes the converged n-dimensional view under a chosen
// conflict-resolution strategy. It is the n-dimensional counterpart of the
// root engine package.
package engine

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/d-led/eventfulranges/clock"
	"github.com/d-led/eventfulranges/space"
	"github.com/d-led/eventfulranges/space/op"
	"github.com/d-led/eventfulranges/space/store"
	"github.com/d-led/eventfulranges/space/strategy"
)

// Config holds the engine options.
type Config struct {
	Clock         clock.Clock
	SnapshotEvery int
	Canonicalize  space.Canonicalizer
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

// WithCanonicalizer sets the final canonicalization applied to every
// materialized view. The default (nil) keeps the library's canonical cover
// unchanged.
func WithCanonicalizer(c space.Canonicalizer) Option {
	return func(cfg *Config) { cfg.Canonicalize = c }
}

// Engine is a concurrency-safe CRDT over n-dimensional boxes backed by an
// append-only event log.
type Engine struct {
	mu            sync.RWMutex
	store         store.Log
	strategy      strategy.Strategy
	clock         clock.Clock
	snapshotEvery int

	ops          map[string]op.Op
	view         []space.Box
	adds         []space.Box
	removes      []space.Box
	version      int64
	since        int
	dims         int
	canonicalize space.Canonicalizer
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
		canonicalize:  cfg.Canonicalize,
		ops:           make(map[string]op.Op),
		dims:          -1,
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

// Materialize returns a copy of the current canonical box cover.
func (e *Engine) Materialize() []space.Box {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return append([]space.Box(nil), e.view...)
}

// Contains reports whether the point belongs to the materialized set.
func (e *Engine) Contains(p []float64) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return space.Contains(e.view, p)
}

// Overlaps reports whether any materialized box shares a point with b.
func (e *Engine) Overlaps(b space.Box) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return space.OverlapsSet(e.view, b)
}

// Ops returns the known operations sorted by ID for deterministic exchange.
func (e *Engine) Ops() []op.Op {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]op.Op, 0, len(e.ops))
	for _, o := range e.ops {
		out = append(out, o)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
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
	if err := e.ensureDims(ops); err != nil {
		return err
	}
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

// ensureDims rejects operations whose dimensionality differs from the set.
func (e *Engine) ensureDims(ops []op.Op) error {
	for i := range ops {
		d := ops[i].Box.Dims()
		if e.dims == -1 {
			e.dims = d
		} else if d != e.dims {
			return fmt.Errorf("engine: box has %d dimensions but the set has %d", d, e.dims)
		}
	}
	return nil
}

// fold records the appended operations in the local state.
func (e *Engine) fold(ctx context.Context, fresh []op.Op) error {
	for _, o := range fresh {
		e.clock.Observe(o.TS)
		e.ops[o.ID] = o
		e.applyToView(o)
	}
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
		e.applyToView(o)
	}
	e.version = version
	e.deriveDims()
	return nil
}

// applyToView folds a single operation into the cached view, segments and
// add/remove sets.
func (e *Engine) applyToView(o op.Op) {
	switch e.strategy {
	case strategy.LWW, strategy.FWW:
		e.setView(strategy.Materialize(e.strategy, e.opsList()))
	case strategy.AdditiveWins:
		if o.Kind == op.KindAdd {
			e.adds = space.Union(e.adds, []space.Box{o.Box})
		} else {
			e.removes = space.Union(e.removes, []space.Box{o.Box})
		}
		e.setView(space.Difference(e.adds, e.removes))
	case strategy.GrowOnly:
		if o.Kind == op.KindAdd {
			e.adds = space.Union(e.adds, []space.Box{o.Box})
			e.setView(e.adds)
		}
	}
}

// setView canonicalizes the freshly materialized cover and caches it.
func (e *Engine) setView(boxes []space.Box) {
	if e.canonicalize != nil {
		boxes = e.canonicalize(boxes)
	}
	e.view = boxes
}

// materializeAll rebuilds the view, segments and add/remove sets from the full
// op set.
func (e *Engine) materializeAll() {
	ops := e.opsList()
	switch e.strategy {
	case strategy.LWW, strategy.FWW:
		e.setView(strategy.Materialize(e.strategy, ops))
	case strategy.AdditiveWins:
		e.adds = boxesOf(ops, op.KindAdd)
		e.removes = boxesOf(ops, op.KindRemove)
		e.setView(space.Difference(e.adds, e.removes))
	case strategy.GrowOnly:
		e.adds = boxesOf(ops, op.KindAdd)
		e.setView(e.adds)
	}
}

// deriveDims recomputes the set's dimension from the known operations.
func (e *Engine) deriveDims() {
	e.dims = -1
	for _, o := range e.ops {
		e.dims = o.Box.Dims()
		return
	}
}

// boxesOf returns the normalized boxes of the operations of one kind.
func boxesOf(ops []op.Op, kind op.Kind) []space.Box {
	boxes := make([]space.Box, 0, len(ops))
	for _, o := range ops {
		if o.Kind == kind {
			boxes = append(boxes, o.Box)
		}
	}
	return space.Normalize(boxes)
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
