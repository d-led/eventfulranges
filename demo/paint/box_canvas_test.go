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

	events, err := c.Paint(Rect{0, 0, 4, 4}, nil)
	require.NoError(t, err)
	require.Len(t, events, 1, "one rectangle is one box event")

	require.True(t, c.set.Contains([]float64{0.5, 0.5}))
	require.True(t, c.set.Contains([]float64{3.5, 3.5}))
	require.False(t, c.set.Contains([]float64{4, 0}), "boxes are half-open")
}

func TestBoxCanvasEraseCarvesFrame(t *testing.T) {
	t.Parallel()
	c := newBoxCanvas()

	_, err := c.Paint(Rect{0, 0, 4, 4}, nil)
	require.NoError(t, err)
	_, err = c.Erase(Rect{1, 1, 3, 3}, nil)
	require.NoError(t, err)

	boxes := c.set.Boxes()
	require.Len(t, boxes, 4, "a square minus a square is a four-piece frame")
	require.False(t, space.Contains(boxes, []float64{2, 2}), "the hole is erased")
	require.True(t, space.Contains(boxes, []float64{0.5, 2}), "the frame survives")
}

func TestBoxCanvasConvergesRegardlessOfOrder(t *testing.T) {
	t.Parallel()
	first := newBoxCanvas()
	second := newBoxCanvas()

	_, err := first.Paint(Rect{0, 0, 4, 4}, nil)
	require.NoError(t, err)
	_, err = first.Erase(Rect{1, 1, 3, 3}, nil)
	require.NoError(t, err)

	_, err = second.Erase(Rect{1, 1, 3, 3}, nil)
	require.NoError(t, err)
	_, err = second.Paint(Rect{0, 0, 4, 4}, nil)
	require.NoError(t, err)

	require.True(t, space.Equal(first.set.Boxes(), second.set.Boxes()))
}

func TestBoxCanvasRejectsEmptyBox(t *testing.T) {
	t.Parallel()
	c := newBoxCanvas()
	_, err := c.Paint(Rect{1, 1, 1, 2}, nil)
	require.Error(t, err)
}

func TestBoxCanvasPaintsSubdividedCell(t *testing.T) {
	t.Parallel()
	c := newBoxCanvas()

	events, err := c.Paint(Rect{0, 0, 0.5, 0.5}, nil)
	require.NoError(t, err)
	require.Len(t, events, 1, "one subdivided cell is one box event")

	require.True(t, c.set.Contains([]float64{0.25, 0.25}))
	require.False(t, c.set.Contains([]float64{0.5, 0.5}), "the cell is half-open")
}

func TestBoxCanvasStoresColorMetadata(t *testing.T) {
	t.Parallel()
	c := newBoxCanvas()

	_, err := c.Paint(Rect{0, 0, 4, 4}, json.RawMessage(`{"color":"#ff0000"}`))
	require.NoError(t, err)
	_, err = c.Paint(Rect{0, 0, 8, 8}, json.RawMessage(`{"author":"alice"}`))
	require.NoError(t, err)

	boxes := c.set.Boxes()
	require.Len(t, boxes, 1, "the larger stroke subsumes the smaller")
	require.JSONEq(t, `{"color":"#ff0000","author":"alice"}`, string(boxes[0].Meta))
}
