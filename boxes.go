package eventfulranges

import (
	"context"

	"github.com/d-led/eventfulranges/clock"
	"github.com/d-led/eventfulranges/space"
	sengine "github.com/d-led/eventfulranges/space/engine"
	sop "github.com/d-led/eventfulranges/space/op"
	sstore "github.com/d-led/eventfulranges/space/store"
	"github.com/d-led/eventfulranges/space/store/jsonl"
	sstrategy "github.com/d-led/eventfulranges/space/strategy"
)

// BoxOption customizes a BoxSet.
type BoxOption = sengine.Option

// WithBoxClock sets the clock used to stamp new box operations.
func WithBoxClock(c clock.Clock) BoxOption {
	return sengine.WithClock(c)
}

// WithBoxSnapshotEvery snapshots the view every n new operations; 0 disables
// automatic snapshots.
func WithBoxSnapshotEvery(n int) BoxOption {
	return sengine.WithSnapshotEvery(n)
}

// BoxSet is the high-level n-dimensional API over an n-D engine.
type BoxSet struct {
	engine *sengine.Engine
}

// OpenBoxes opens a JSON Lines-backed box set stored under dir.
func OpenBoxes(ctx context.Context, dir string, s sstrategy.Strategy, opts ...BoxOption) (*BoxSet, error) {
	st, err := jsonl.Open(dir)
	if err != nil {
		return nil, err
	}
	return OpenBoxStore(ctx, st, s, opts...)
}

// OpenBoxStore opens a box set backed by the given event log.
func OpenBoxStore(ctx context.Context, st sstore.Log, s sstrategy.Strategy, opts ...BoxOption) (*BoxSet, error) {
	e, err := sengine.Open(ctx, st, s, opts...)
	if err != nil {
		return nil, err
	}
	return &BoxSet{engine: e}, nil
}

// Apply applies a single operation.
func (b *BoxSet) Apply(ctx context.Context, o sop.Op) error {
	return b.engine.Apply(ctx, o)
}

// ApplyAll applies a batch of operations, ignoring duplicates by ID.
func (b *BoxSet) ApplyAll(ctx context.Context, ops []sop.Op) error {
	return b.engine.ApplyAll(ctx, ops)
}

// Add applies an addition over the half-open box [min, max) and returns the
// applied op.
func (b *BoxSet) Add(ctx context.Context, min, max []float64) (sop.Op, error) {
	o := sop.Add(min, max)
	return o, b.engine.Apply(ctx, o)
}

// Remove applies a removal over the half-open box [min, max) and returns the
// applied op.
func (b *BoxSet) Remove(ctx context.Context, min, max []float64) (sop.Op, error) {
	o := sop.Remove(min, max)
	return o, b.engine.Apply(ctx, o)
}

// Boxes returns the current canonical box cover.
func (b *BoxSet) Boxes() []space.Box {
	return b.engine.Materialize()
}

// Contains reports whether the point belongs to the materialized set.
func (b *BoxSet) Contains(p []float64) bool {
	return b.engine.Contains(p)
}

// Overlaps reports whether any materialized box shares a point with box.
func (b *BoxSet) Overlaps(box space.Box) bool {
	return b.engine.Overlaps(box)
}

// Ops returns the known operations for anti-entropy exchange.
func (b *BoxSet) Ops() []sop.Op {
	return b.engine.Ops()
}

// Snapshot persists the current materialized view.
func (b *BoxSet) Snapshot(ctx context.Context) error {
	return b.engine.Snapshot(ctx)
}
