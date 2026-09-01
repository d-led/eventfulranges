package engine

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/d-led/eventfulranges/interval"
	"github.com/d-led/eventfulranges/op"
	"github.com/d-led/eventfulranges/store"
	"github.com/d-led/eventfulranges/strategy"
)

// Snapshot is the materialized view together with the log version it covers.
type Snapshot struct {
	Strategy  strategy.Strategy   `json:"strategy"`
	Version   int64               `json:"version"`
	Intervals []interval.Interval `json:"intervals"`
	Segments  []strategy.Segment  `json:"segments,omitempty"`
	Adds      []interval.Interval `json:"adds,omitempty"`
	Removes   []interval.Interval `json:"removes,omitempty"`
	Ops       []op.Op             `json:"ops,omitempty"`
}

// reload loads the snapshot (when compatible) and replays the tail of the
// log to rebuild the in-memory state.
func (e *Engine) reload(ctx context.Context) error {
	snap, err := e.loadSnapshot(ctx)
	if err != nil && !errors.Is(err, store.ErrSnapshotNotFound) {
		return err
	}
	from := int64(0)
	if snap != nil && snap.Strategy == e.strategy {
		from = snap.Version
	}
	ops, version, err := e.store.Read(ctx, from)
	if err != nil {
		return err
	}
	e.version = version
	for _, o := range ops {
		e.clock.Observe(o.TS)
		e.ops[o.ID] = o
	}
	if snap != nil && snap.Strategy == e.strategy {
		e.restore(snap, ops)
	} else {
		e.materializeAll()
	}
	return nil
}

// restore rebuilds the view from the snapshot plus the replayed tail.
func (e *Engine) restore(snap *Snapshot, delta []op.Op) {
	// Fold the snapshot's full op history back in so compaction keeps Ops()
	// complete for anti-entropy exchange.
	for _, o := range snap.Ops {
		e.clock.Observe(o.TS)
		e.ops[o.ID] = o
	}
	switch e.strategy {
	case strategy.LWW, strategy.FWW:
		e.segments = snap.Segments
		for _, o := range delta {
			e.segments = strategy.CombineSegments(e.strategy, e.segments, strategy.Segments(e.strategy, []op.Op{o}))
		}
		e.view = strategy.ToIntervals(e.segments)
	case strategy.AdditiveWins, strategy.GrowOnly:
		// The snapshot's full op history is already folded back into e.ops, so
		// a lazy rebuild reproduces the view exactly; there is no need to
		// re-difference the delta here.
		e.dirty = true
	}
}

// buildSnapshot assembles the current view and log version into a Snapshot.
func (e *Engine) buildSnapshot() Snapshot {
	snap := Snapshot{
		Strategy:  e.strategy,
		Version:   e.version,
		Intervals: e.view,
		Ops:       e.opsList(),
	}
	switch e.strategy {
	case strategy.LWW, strategy.FWW:
		snap.Segments = e.segments
	case strategy.AdditiveWins:
		snap.Adds = e.adds
		snap.Removes = e.removes
	}
	return snap
}

// snapshot writes the current view and version to the store. Backends without
// snapshot support skip it.
func (e *Engine) snapshot(ctx context.Context) error {
	e.ensureView()
	sn, ok := e.store.(store.Snapshotter)
	if !ok {
		return nil
	}
	// The strategy is validated at Open, so Marshal cannot fail.
	data, _ := json.Marshal(e.buildSnapshot())
	if err := sn.SaveSnapshot(ctx, data, e.version); err != nil {
		return err
	}
	e.since = 0
	return nil
}

// Compact rewrites the store as a snapshot of the current view plus any
// operations it does not cover, collapsing the stream to its smallest form.
func (e *Engine) Compact(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.ensureView()
	compactor, ok := e.store.(store.Compactor)
	if !ok {
		return store.ErrCompactionUnsupported
	}
	data, _ := json.Marshal(e.buildSnapshot())
	if err := compactor.Compact(ctx, data, e.version); err != nil {
		return err
	}
	e.since = 0
	return nil
}

// loadSnapshot reads and decodes the latest snapshot, if any.
func (e *Engine) loadSnapshot(ctx context.Context) (*Snapshot, error) {
	sn, ok := e.store.(store.Snapshotter)
	if !ok {
		return nil, store.ErrSnapshotNotFound
	}
	data, version, err := sn.LoadSnapshot(ctx)
	if errors.Is(err, store.ErrSnapshotNotFound) {
		return nil, store.ErrSnapshotNotFound
	}
	if err != nil {
		return nil, err
	}
	snap := &Snapshot{}
	if err := json.Unmarshal(data, snap); err != nil {
		return nil, err
	}
	snap.Version = version
	return snap, nil
}
