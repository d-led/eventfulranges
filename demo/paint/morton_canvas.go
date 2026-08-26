package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/d-led/eventfulranges"
	"github.com/d-led/eventfulranges/demo/internal/morton"
	"github.com/d-led/eventfulranges/interval"
	"github.com/d-led/eventfulranges/op"
	"github.com/d-led/eventfulranges/store/memory"
	"github.com/d-led/eventfulranges/strategy"
)

// mortonCanvas encodes 2-D cells onto the library's 1-D range CRDT through a
// Morton (Z-order) curve, so a rectangle of cells folds into a few contiguous
// 1-D operations. It is one implementation of Canvas; a native n-D engine
// could replace it without touching the board or the transport.
type mortonCanvas struct {
	set *eventfulranges.RangeSet
}

// newMortonCanvas opens a fresh in-memory, additive-wins canvas. Additive-wins
// makes the painted set the union of every stroke minus the union of every
// erasure, so concurrent edits converge regardless of order.
func newMortonCanvas() *mortonCanvas {
	set, err := eventfulranges.OpenStore(context.Background(), memory.New(), strategy.AdditiveWins)
	if err != nil {
		panic(err) // a fresh in-memory store cannot fail to open
	}
	return &mortonCanvas{set: set}
}

// Paint adds a rectangle of cells to the whiteboard.
func (c *mortonCanvas) Paint(r Rect) ([]Event, error) {
	return c.fold("paint", r)
}

// Erase removes a rectangle of cells from the whiteboard.
func (c *mortonCanvas) Erase(r Rect) ([]Event, error) {
	return c.fold("erase", r)
}

// fold decomposes a rectangle into contiguous Morton ranges and appends each
// as one half-open 1-D operation.
func (c *mortonCanvas) fold(kind string, r Rect) ([]Event, error) {
	ranges, err := morton.Ranges(r.X0, r.Y0, r.X1, r.Y1)
	if err != nil {
		return nil, err
	}
	events := make([]Event, 0, len(ranges))
	for _, mr := range ranges {
		o, err := c.apply(kind, mr)
		if err != nil {
			return nil, err
		}
		events = append(events, Event{
			ID:     o.ID,
			Kind:   o.Kind.String(),
			Data:   intervalData(o.Interval),
			Detail: fmt.Sprintf("[%d,%d)", int64(o.Interval.Start), int64(o.Interval.End)),
		})
	}
	return events, nil
}

// apply appends one half-open Morton range under the given command kind.
func (c *mortonCanvas) apply(kind string, mr morton.Range) (op.Op, error) {
	start, end := float64(mr.Lo), float64(mr.Hi)
	switch kind {
	case "paint":
		return c.set.AddWithBounds(context.Background(), start, end, interval.Closed, interval.Open)
	case "erase":
		return c.set.RemoveWithBounds(context.Background(), start, end, interval.Closed, interval.Open)
	default:
		return op.Op{}, fmt.Errorf("paint: unknown command %q", kind)
	}
}

// intervalData renders a half-open interval as {"a":start,"b":end}, the shape
// the browser materializes.
func intervalData(iv interval.Interval) json.RawMessage {
	raw, _ := json.Marshal(map[string]float64{"a": iv.Start, "b": iv.End})
	return raw
}
