package main

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/d-led/eventfulranges/space"
)

func TestBoxCanvasPaintAddsOneBox(t *testing.T) {
	t.Parallel()
	c := newBoxCanvas()

	events, err := c.Paint("", Rect{0, 0, 4, 4}, nil)
	require.NoError(t, err)
	require.Len(t, events, 1, "one rectangle is one box event")

	require.True(t, c.set.Contains([]float64{0.5, 0.5}))
	require.True(t, c.set.Contains([]float64{3.5, 3.5}))
	require.False(t, c.set.Contains([]float64{4, 0}), "boxes are half-open")
}

func TestBoxCanvasEraseCarvesFrame(t *testing.T) {
	t.Parallel()
	c := newBoxCanvas()

	_, err := c.Paint("", Rect{0, 0, 4, 4}, nil)
	require.NoError(t, err)
	_, err = c.Erase("", Rect{1, 1, 3, 3}, nil)
	require.NoError(t, err)

	boxes := c.set.Boxes()
	require.Len(t, boxes, 4, "a square minus a square is a four-piece frame")
	require.False(t, space.Contains(boxes, []float64{2, 2}), "the hole is erased")
	require.True(t, space.Contains(boxes, []float64{0.5, 2}), "the frame survives")
}

func TestBoxCanvasLastWriteWins(t *testing.T) {
	t.Parallel()

	// A later erase clears the overlapped part of an earlier stroke.
	paintFirst := newBoxCanvas()
	_, err := paintFirst.Paint("", Rect{0, 0, 4, 4}, nil)
	require.NoError(t, err)
	_, err = paintFirst.Erase("", Rect{1, 1, 3, 3}, nil)
	require.NoError(t, err)
	require.False(t, space.Contains(paintFirst.set.Boxes(), []float64{2, 2}),
		"a later erase clears the middle")

	// A later stroke repaints over an earlier erasure.
	eraseFirst := newBoxCanvas()
	_, err = eraseFirst.Erase("", Rect{1, 1, 3, 3}, nil)
	require.NoError(t, err)
	_, err = eraseFirst.Paint("", Rect{0, 0, 4, 4}, nil)
	require.NoError(t, err)
	require.True(t, space.Contains(eraseFirst.set.Boxes(), []float64{2, 2}),
		"a later stroke repaints the erased hole")
}

func TestBoxCanvasRejectsEmptyBox(t *testing.T) {
	t.Parallel()
	c := newBoxCanvas()
	_, err := c.Paint("", Rect{1, 1, 1, 2}, nil)
	require.Error(t, err)
}

func TestBoxCanvasPaintsSubdividedCell(t *testing.T) {
	t.Parallel()
	c := newBoxCanvas()

	events, err := c.Paint("", Rect{0, 0, 0.5, 0.5}, nil)
	require.NoError(t, err)
	require.Len(t, events, 1, "one subdivided cell is one box event")

	require.True(t, c.set.Contains([]float64{0.25, 0.25}))
	require.False(t, c.set.Contains([]float64{0.5, 0.5}), "the cell is half-open")
}

func TestBoxCanvasResolvesColorPerPoint(t *testing.T) {
	t.Parallel()
	c := newBoxCanvas()

	_, err := c.Paint("", Rect{0, 0, 4, 4}, json.RawMessage(`{"color":"#ff0000"}`))
	require.NoError(t, err)
	_, err = c.Paint("", Rect{2, 2, 6, 6}, json.RawMessage(`{"color":"#0000ff"}`))
	require.NoError(t, err)

	colorAt := func(p []float64) string {
		for _, b := range c.set.Boxes() {
			if b.Contains(p) {
				return string(b.Meta)
			}
		}
		return ""
	}
	// The later blue stroke wins the overlap; red survives where blue never
	// reached.
	require.JSONEq(t, `{"color":"#0000ff"}`, colorAt([]float64{3, 3}))
	require.JSONEq(t, `{"color":"#0000ff"}`, colorAt([]float64{5, 5}))
	require.JSONEq(t, `{"color":"#ff0000"}`, colorAt([]float64{1, 1}))
}

func TestBoxCanvasRetractRestoresErasedCells(t *testing.T) {
	t.Parallel()
	c := newBoxCanvas()

	_, err := c.Paint("p", Rect{0, 0, 4, 4}, nil)
	require.NoError(t, err)
	_, err = c.Erase("e", Rect{1, 1, 3, 3}, nil)
	require.NoError(t, err)
	require.False(t, c.set.Contains([]float64{2, 2}), "erased first")

	events, err := c.Retract("undo-e", "e")
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, "retract", events[0].Kind)
	require.Equal(t, "e", events[0].Ref)
	require.Equal(t, "undo-e", events[0].ID)

	require.True(t, c.set.Contains([]float64{2, 2}), "undoing the erase restores the hole")
}

func TestBoxCanvasRetractRemovesOnlyThatPaint(t *testing.T) {
	t.Parallel()
	c := newBoxCanvas()

	_, err := c.Paint("alice", Rect{0, 0, 4, 4}, nil)
	require.NoError(t, err)
	_, err = c.Paint("bob", Rect{2, 2, 6, 6}, nil)
	require.NoError(t, err)
	_, err = c.Retract("", "bob")
	require.NoError(t, err)

	require.True(t, c.set.Contains([]float64{1, 1}), "alice's paint survives")
	require.False(t, c.set.Contains([]float64{5, 5}), "the retracted paint is gone")
}
