package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/d-led/eventfulranges"
	"github.com/d-led/eventfulranges/space"
	sop "github.com/d-led/eventfulranges/space/op"
	"github.com/d-led/eventfulranges/space/store/memory"
	sstrategy "github.com/d-led/eventfulranges/space/strategy"
)

// boxCanvas folds 2-D cell rectangles straight into the library's n-dimensional
// range CRDT: each stroke is one half-open box operation. It is the native
// Canvas implementation, the one the Morton adapter was built to be replaced
// by.
type boxCanvas struct {
	set *eventfulranges.BoxSet
}

// newBoxCanvas opens a fresh in-memory canvas. Last-write-wins resolves every
// point to the most recent operation covering it: a later stroke paints over
// an earlier erasure, and a later erasure clears an earlier stroke.
func newBoxCanvas() *boxCanvas {
	set, err := eventfulranges.OpenBoxStore(context.Background(), memory.New(), sstrategy.LWW,
		eventfulranges.WithBoxSnapshotEvery(0))
	if err != nil {
		panic(err) // a fresh in-memory store cannot fail to open
	}
	return &boxCanvas{set: set}
}

// Paint adds a rectangle of cells to the whiteboard, carrying meta on the
// underlying box.
func (c *boxCanvas) Paint(id string, r Rect, meta json.RawMessage) ([]Event, error) {
	return c.fold(cmdPaint, id, r, meta)
}

// Erase removes a rectangle of cells from the whiteboard, carrying meta on the
// underlying box.
func (c *boxCanvas) Erase(id string, r Rect, meta json.RawMessage) ([]Event, error) {
	return c.fold(cmdErase, id, r, meta)
}

// Retract cancels the operation named ref, undoing that one edit. id, when
// non-empty, names the retraction itself, so the issuing client can
// acknowledge it by ID like any other operation.
func (c *boxCanvas) Retract(id, ref string) ([]Event, error) {
	target, err := c.set.RetractWithID(context.Background(), id, ref)
	if err != nil {
		return nil, err
	}
	return []Event{{
		ID:     target.ID,
		Kind:   target.Kind.String(),
		Ref:    ref,
		Data:   boxData(target.Box),
		Detail: fmt.Sprintf("undo %s", ref),
	}}, nil
}

// fold appends one half-open box operation and reports the event it produced.
func (c *boxCanvas) fold(kind, id string, r Rect, meta json.RawMessage) ([]Event, error) {
	min := []float64{r.X0, r.Y0}
	max := []float64{r.X1, r.Y1}
	o, err := c.apply(kind, id, min, max, meta)
	if err != nil {
		return nil, err
	}
	return []Event{{
		ID:     o.ID,
		Kind:   o.Kind.String(),
		Data:   boxData(o.Box),
		Detail: boxDetail(o.Box),
	}}, nil
}

// apply appends one box under the given command kind, carrying meta. When id
// is non-empty it overrides the generated operation ID, so the client can
// retract this exact edit later.
func (c *boxCanvas) apply(kind, id string, min, max []float64, meta json.RawMessage) (sop.Op, error) {
	var o sop.Op
	switch kind {
	case cmdPaint:
		o = sop.Add(min, max)
	case cmdErase:
		o = sop.Remove(min, max)
	default:
		return sop.Op{}, fmt.Errorf("paint: unknown command %q", kind)
	}
	if id != "" {
		o.ID = id
	}
	if meta != nil {
		o.Box = o.Box.WithMeta(meta)
	}
	if err := c.set.Apply(context.Background(), o); err != nil {
		return sop.Op{}, err
	}
	return o, nil
}

// boxData renders a box as {"min":[x0,y0],"max":[x1,y1]}, the shape the browser
// materializes.
func boxData(b space.Box) json.RawMessage {
	raw, _ := json.Marshal(map[string][]float64{"min": b.Min, "max": b.Max})
	return raw
}

// boxDetail renders a box for the activity log.
func boxDetail(b space.Box) string {
	return fmt.Sprintf("[%g,%g) x [%g,%g)", b.Min[0], b.Max[0], b.Min[1], b.Max[1])
}
