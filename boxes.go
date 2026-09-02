package eventfulranges

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/d-led/eventfulranges/clock"
	"github.com/d-led/eventfulranges/meta"
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

// WithBoxCanonicalizer sets the final canonicalization applied to every
// materialized view.
func WithBoxCanonicalizer(c space.Canonicalizer) BoxOption {
	return sengine.WithCanonicalizer(c)
}

// WithBoxMetaMerge sets the join used when boxes carrying metadata merge under
// AdditiveWins or GrowOnly. The default is the top-level key union in the meta
// package.
func WithBoxMetaMerge(m meta.Merge) BoxOption {
	return sengine.WithMetaMerge(m)
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

// Add applies an addition over the half-open box [lo, hi) and returns the
// applied op.
func (b *BoxSet) Add(ctx context.Context, lo, hi []float64) (sop.Op, error) {
	o := sop.Add(lo, hi)
	return o, b.engine.Apply(ctx, o)
}

// AddWithMeta applies an addition over the half-open box [lo, hi) carrying
// JSON-object metadata, and returns the applied op.
func (b *BoxSet) AddWithMeta(ctx context.Context, lo, hi []float64, m json.RawMessage) (sop.Op, error) {
	o := sop.Add(lo, hi)
	o.Box = o.Box.WithMeta(m)
	return o, b.engine.Apply(ctx, o)
}

// Remove applies a removal over the half-open box [lo, hi) and returns the
// applied op.
func (b *BoxSet) Remove(ctx context.Context, lo, hi []float64) (sop.Op, error) {
	o := sop.Remove(lo, hi)
	return o, b.engine.Apply(ctx, o)
}

// AddBounds applies an addition over the box [lo, hi] with explicit per-face
// inclusivity and returns the applied op.
func (b *BoxSet) AddBounds(ctx context.Context, lo, hi []float64, loB, hiB []space.Bound) (sop.Op, error) {
	o := sop.AddBounds(lo, hi, loB, hiB)
	return o, b.engine.Apply(ctx, o)
}

// RemoveBounds applies a removal over the box [lo, hi] with explicit per-face
// inclusivity and returns the applied op.
func (b *BoxSet) RemoveBounds(ctx context.Context, lo, hi []float64, loB, hiB []space.Bound) (sop.Op, error) {
	o := sop.RemoveBounds(lo, hi, loB, hiB)
	return o, b.engine.Apply(ctx, o)
}

// RemoveWithMeta applies a removal over the half-open box [lo, hi) carrying
// JSON-object metadata, and returns the applied op.
func (b *BoxSet) RemoveWithMeta(ctx context.Context, lo, hi []float64, m json.RawMessage) (sop.Op, error) {
	o := sop.Remove(lo, hi)
	o.Box = o.Box.WithMeta(m)
	return o, b.engine.Apply(ctx, o)
}

// Retract cancels the operation named refID, undoing that one edit without
// touching any other operation. Retraction is single-level: a Retract cannot
// itself be retracted.
func (b *BoxSet) Retract(ctx context.Context, refID string) (sop.Op, error) {
	return b.RetractWithID(ctx, "", refID)
}

// RetractWithID cancels the operation named refID exactly like Retract, but
// records the retraction itself under id when it is non-empty. A caller that
// assigns its own operation identifiers can therefore acknowledge the
// retraction by ID, just like any other operation.
func (b *BoxSet) RetractWithID(ctx context.Context, id, refID string) (sop.Op, error) {
	target, ok := b.engine.Op(refID)
	if !ok {
		return sop.Op{}, fmt.Errorf("box set: retract unknown operation %q", refID)
	}
	if target.Kind == sop.KindRetract {
		return sop.Op{}, fmt.Errorf("box set: cannot retract a retraction %q", refID)
	}
	o := sop.Retract(refID, target.Box)
	if id != "" {
		o.ID = id
	}
	return o, b.engine.Apply(ctx, o)
}

// Boxes returns the current canonical box cover.
func (b *BoxSet) Boxes() []space.Box {
	return b.engine.Materialize()
}

// Layers returns the culled, layered front of the known operations under
// last-write-wins priority, in bottom-to-top paint order. It is the
// painter's-algorithm view: a later stroke layers over an earlier one instead
// of carving it into strips, so a big box stays whole even when smaller
// strokes land inside it. See space/strategy.Layers.
func (b *BoxSet) Layers() []sstrategy.Layer {
	return b.engine.Layers()
}

// Contains reports whether the point belongs to the materialized set.
func (b *BoxSet) Contains(p []float64) bool {
	return b.engine.Contains(p)
}

// Overlaps reports whether any materialized box shares a point with box.
func (b *BoxSet) Overlaps(box space.Box) bool {
	return b.engine.Overlaps(box)
}

// Crossed returns the materialized boxes the path crosses with positive
// length, in canonical cover order.
func (b *BoxSet) Crossed(p space.Path) []space.Box {
	return space.Crossed(b.Boxes(), p)
}

// Traverse partitions the path's parameter interval into covered and gap
// segments as it enters and leaves the materialized set.
func (b *BoxSet) Traverse(p space.Path) []space.PathSegment {
	return space.Traverse(b.Boxes(), p)
}

// Ops returns the known operations for anti-entropy exchange.
func (b *BoxSet) Ops() []sop.Op {
	return b.engine.Ops()
}

// Snapshot persists the current materialized view.
func (b *BoxSet) Snapshot(ctx context.Context) error {
	return b.engine.Snapshot(ctx)
}

// Compact rewrites the store as a snapshot of the current view, collapsing the
// stream to its smallest form.
func (b *BoxSet) Compact(ctx context.Context) error {
	return b.engine.Compact(ctx)
}
