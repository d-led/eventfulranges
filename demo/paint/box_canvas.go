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

// newBoxCanvas opens a fresh in-memory, additive-wins canvas. Additive-wins
// makes the painted set the union of every stroke minus the union of every
// erasure, so concurrent edits converge regardless of order.
func newBoxCanvas() *boxCanvas {
	set, err := eventfulranges.OpenBoxStore(context.Background(), memory.New(), sstrategy.AdditiveWins,
		eventfulranges.WithBoxSnapshotEvery(0))
	if err != nil {
		panic(err) // a fresh in-memory store cannot fail to open
	}
	return &boxCanvas{set: set}
}

// Paint adds a rectangle of cells to the whiteboard, carrying meta on the
// underlying box.
func (c *boxCanvas) Paint(r Rect, meta json.RawMessage) ([]Event, error) {
	return c.fold("paint", r, meta)
}

// Erase removes a rectangle of cells from the whiteboard, carrying meta on the
// underlying box.
func (c *boxCanvas) Erase(r Rect, meta json.RawMessage) ([]Event, error) {
	return c.fold("erase", r, meta)
}

// fold appends one half-open box operation and reports the event it produced.
func (c *boxCanvas) fold(kind string, r Rect, meta json.RawMessage) ([]Event, error) {
	min := []float64{r.X0, r.Y0}
	max := []float64{r.X1, r.Y1}
	o, err := c.apply(kind, min, max, meta)
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

// apply appends one box under the given command kind, carrying meta.
func (c *boxCanvas) apply(kind string, min, max []float64, meta json.RawMessage) (sop.Op, error) {
	switch kind {
	case "paint":
		return c.set.AddWithMeta(context.Background(), min, max, meta)
	case "erase":
		return c.set.RemoveWithMeta(context.Background(), min, max, meta)
	default:
		return sop.Op{}, fmt.Errorf("paint: unknown command %q", kind)
	}
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
