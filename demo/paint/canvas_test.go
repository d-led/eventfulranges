package main

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/d-led/eventfulranges/demo/internal/morton"
)

func TestMortonCanvasPaintRectBecomesOneEvent(t *testing.T) {
	t.Parallel()
	c := newMortonCanvas()

	events, err := c.Paint(Rect{0, 0, 4, 4})
	require.NoError(t, err)
	require.Len(t, events, 1, "an aligned 4x4 block is one Morton range, hence one event")

	corner, _ := morton.Encode(0, 0)
	require.True(t, c.set.Contains(float64(corner)))
	opposite, _ := morton.Encode(3, 3)
	require.True(t, c.set.Contains(float64(opposite)))
	outside, _ := morton.Encode(4, 0)
	require.False(t, c.set.Contains(float64(outside)))
}

func TestMortonCanvasEraseCarvesHole(t *testing.T) {
	t.Parallel()
	c := newMortonCanvas()

	_, err := c.Paint(Rect{0, 0, 4, 4})
	require.NoError(t, err)
	events, err := c.Erase(Rect{1, 1, 3, 3})
	require.NoError(t, err)
	require.NotEmpty(t, events)

	corner, _ := morton.Encode(0, 0)
	require.True(t, c.set.Contains(float64(corner)), "the frame survives")
	hole, _ := morton.Encode(1, 1)
	require.False(t, c.set.Contains(float64(hole)), "the hole is erased")
}

func TestMortonCanvasConvergesRegardlessOfOrder(t *testing.T) {
	t.Parallel()
	first := newMortonCanvas()
	second := newMortonCanvas()

	_, err := first.Paint(Rect{0, 0, 4, 4})
	require.NoError(t, err)
	_, err = first.Erase(Rect{1, 1, 3, 3})
	require.NoError(t, err)

	_, err = second.Erase(Rect{1, 1, 3, 3})
	require.NoError(t, err)
	_, err = second.Paint(Rect{0, 0, 4, 4})
	require.NoError(t, err)

	require.Equal(t, first.set.Ranges(), second.set.Ranges())
}

func TestMortonCanvasRejectsOutOfRange(t *testing.T) {
	t.Parallel()
	c := newMortonCanvas()
	_, err := c.Paint(Rect{morton.Limit, 0, morton.Limit + 1, 1})
	require.ErrorIs(t, err, morton.ErrOutOfRange)
}
