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
	switch e.strategy {
	case strategy.LWW, strategy.FWW:
		e.segments = snap.Segments
		for _, o := range delta {
			e.segments = strategy.CombineSegments(e.strategy, e.segments, strategy.Segments(e.strategy, []op.Op{o}))
		}
		e.view = strategy.ToIntervals(e.segments)
	case strategy.AdditiveWins:
		e.adds = snap.Adds
		e.removes = snap.Removes
		e.view = interval.Difference(e.adds, e.removes)
		for _, o := range delta {
			e.applyToView(o)
		}
	case strategy.GrowOnly:
		e.adds = snap.Intervals
		e.view = e.adds
		for _, o := range delta {
			e.applyToView(o)
		}
	}
}

// snapshot writes the current view and version to the store. Backends without
// snapshot support skip it.
func (e *Engine) snapshot(ctx context.Context) error {
	sn, ok := e.store.(store.Snapshotter)
	if !ok {
		return nil
	}
	snap := Snapshot{
		Strategy:  e.strategy,
		Version:   e.version,
		Intervals: e.view,
	}
	switch e.strategy {
	case strategy.LWW, strategy.FWW:
		snap.Segments = e.segments
	case strategy.AdditiveWins:
		snap.Adds = e.adds
		snap.Removes = e.removes
	}
	// The strategy is validated at Open, so Marshal cannot fail.
	data, _ := json.Marshal(snap)
	if err := sn.SaveSnapshot(ctx, data, e.version); err != nil {
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
