// Package eventfulranges is an event-sourced CRDT for real-valued ranges.
//
// Operations (add or remove a range) are appended to an append-only log and
// merged under a conflict-resolution strategy such as last-write-wins or
// additive-wins. The default backend is a JSON Lines file, which needs no
// network; a KurrentDB backend is available behind the kurrent build tag.
package eventfulranges

import (
	"context"

	"gitub.com/d-led/eventfulranges/clock"
	"gitub.com/d-led/eventfulranges/engine"
	"gitub.com/d-led/eventfulranges/interval"
	"gitub.com/d-led/eventfulranges/op"
	"gitub.com/d-led/eventfulranges/store"
	"gitub.com/d-led/eventfulranges/store/jsonl"
	"gitub.com/d-led/eventfulranges/strategy"
)

// Option customizes a RangeSet.
type Option = engine.Option

// WithClock sets the clock used to stamp new operations.
func WithClock(c clock.Clock) Option {
	return engine.WithClock(c)
}

// WithSnapshotEvery snapshots the view every n new operations; 0 disables
// automatic snapshots.
func WithSnapshotEvery(n int) Option {
	return engine.WithSnapshotEvery(n)
}

// RangeSet is the high-level API over an engine.
type RangeSet struct {
	engine *engine.Engine
}

// Open opens a JSON Lines-backed range set stored under dir.
func Open(ctx context.Context, dir string, s strategy.Strategy, opts ...Option) (*RangeSet, error) {
	st, err := jsonl.Open(dir)
	if err != nil {
		return nil, err
	}
	return OpenStore(ctx, st, s, opts...)
}

// OpenStore opens a range set backed by the given event log. Backends that
// also implement store.Snapshotter get snapshotting for free; stream-only
// backends simply skip it.
func OpenStore(ctx context.Context, st store.Log, s strategy.Strategy, opts ...Option) (*RangeSet, error) {
	e, err := engine.Open(ctx, st, s, opts...)
	if err != nil {
		return nil, err
	}
	return &RangeSet{engine: e}, nil
}

// Apply applies a single operation.
func (r *RangeSet) Apply(ctx context.Context, o op.Op) error {
	return r.engine.Apply(ctx, o)
}

// ApplyAll applies a batch of operations, ignoring duplicates by ID. It is the
// workhorse of anti-entropy: a replica can hand another replica's Ops() result
// straight back to ApplyAll.
func (r *RangeSet) ApplyAll(ctx context.Context, ops []op.Op) error {
	return r.engine.ApplyAll(ctx, ops)
}

// Add applies a closed addition [start, end] and returns the applied op.
func (r *RangeSet) Add(ctx context.Context, start, end float64) (op.Op, error) {
	o := op.Add(start, end)
	return o, r.engine.Apply(ctx, o)
}

// AddWithBounds applies an addition with explicit boundary inclusivity.
func (r *RangeSet) AddWithBounds(ctx context.Context, start, end float64, sb, eb interval.Bound) (op.Op, error) {
	o := op.AddWithBounds(start, end, sb, eb)
	return o, r.engine.Apply(ctx, o)
}

// Remove applies a closed removal [start, end] and returns the applied op.
func (r *RangeSet) Remove(ctx context.Context, start, end float64) (op.Op, error) {
	o := op.Remove(start, end)
	return o, r.engine.Apply(ctx, o)
}

// RemoveWithBounds applies a removal with explicit boundary inclusivity.
func (r *RangeSet) RemoveWithBounds(ctx context.Context, start, end float64, sb, eb interval.Bound) (op.Op, error) {
	o := op.RemoveWithBounds(start, end, sb, eb)
	return o, r.engine.Apply(ctx, o)
}

// Ranges returns the current canonical interval view.
func (r *RangeSet) Ranges() []interval.Interval {
	return r.engine.Materialize()
}

// Contains reports whether x belongs to the materialized set.
func (r *RangeSet) Contains(x float64) bool {
	return r.engine.Contains(x)
}

// Overlaps reports whether any materialized interval shares a point with iv.
func (r *RangeSet) Overlaps(iv interval.Interval) bool {
	return r.engine.Overlaps(iv)
}

// Ops returns the known operations for anti-entropy exchange.
func (r *RangeSet) Ops() []op.Op {
	return r.engine.Ops()
}

// Snapshot persists the current materialized view.
func (r *RangeSet) Snapshot(ctx context.Context) error {
	return r.engine.Snapshot(ctx)
}
