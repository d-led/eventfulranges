package engine

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/d-led/eventfulranges/space"
	"github.com/d-led/eventfulranges/space/op"
	"github.com/d-led/eventfulranges/space/store"
	"github.com/d-led/eventfulranges/space/strategy"
)

// Snapshot is the materialized view together with the log version it covers.
// For LWW/FWW the full operation history is persisted too, because the n-D
// winner partition cannot be reconstructed from a box cover alone.
type Snapshot struct {
	Strategy strategy.Strategy `json:"strategy"`
	Version  int64             `json:"version"`
	Boxes    []space.Box       `json:"boxes"`
	Adds     []space.Box       `json:"adds,omitempty"`
	Removes  []space.Box       `json:"removes,omitempty"`
	Ops      []op.Op           `json:"ops,omitempty"`
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
	e.deriveDims()
	return nil
}

// restore rebuilds the view from the snapshot plus the replayed tail.
func (e *Engine) restore(snap *Snapshot, delta []op.Op) {
	switch e.strategy {
	case strategy.LWW, strategy.FWW, strategy.AdditiveWinsLWW:
		for _, o := range snap.Ops {
			e.clock.Observe(o.TS)
			e.ops[o.ID] = o
		}
		e.materializeAll()
	case strategy.AdditiveWins:
		e.adds = snap.Adds
		e.removes = snap.Removes
		e.setView(space.DifferenceMerged(e.adds, e.removes, e.metaMerge))
		for _, o := range delta {
			e.applyToView(o)
		}
	case strategy.GrowOnly:
		e.adds = snap.Boxes
		e.setView(e.adds)
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
		Strategy: e.strategy,
		Version:  e.version,
		Boxes:    e.view,
	}
	switch e.strategy {
	case strategy.LWW, strategy.FWW, strategy.AdditiveWinsLWW:
		snap.Ops = e.opsList()
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
