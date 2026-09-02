package engine_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/d-led/eventfulranges/space"
	"github.com/d-led/eventfulranges/space/engine"
	"github.com/d-led/eventfulranges/space/op"
	"github.com/d-led/eventfulranges/space/store/memory"
	"github.com/d-led/eventfulranges/space/strategy"
)

func bounds(n int, b space.Bound) []space.Bound {
	out := make([]space.Bound, n)
	for i := range out {
		out[i] = b
	}
	return out
}

func TestClosedBoundsEndToEnd1D(t *testing.T) {
	t.Parallel()
	e, err := engine.Open(context.Background(), memory.New(), strategy.AdditiveWins)
	require.NoError(t, err)
	ctx := context.Background()

	// add [0,2] closed, remove (0,2) open → the endpoints {0} and {2} remain.
	require.NoError(t, e.Apply(ctx, op.AddBounds([]float64{0}, []float64{2}, bounds(1, space.Closed), bounds(1, space.Closed))))
	require.NoError(t, e.Apply(ctx, op.RemoveBounds([]float64{0}, []float64{2}, bounds(1, space.Open), bounds(1, space.Open))))

	require.Len(t, e.Materialize(), 2)
	require.True(t, e.Contains([]float64{0}))
	require.True(t, e.Contains([]float64{2}))
	require.False(t, e.Contains([]float64{1}))
}

func TestClosedBoundsEndToEnd4D(t *testing.T) {
	t.Parallel()
	e, err := engine.Open(context.Background(), memory.New(), strategy.AdditiveWins)
	require.NoError(t, err)
	ctx := context.Background()

	require.NoError(t, e.Apply(ctx, op.AddBounds(
		[]float64{0, 0, 0, 0}, []float64{2, 2, 2, 2},
		bounds(4, space.Closed), bounds(4, space.Closed))))
	require.NoError(t, e.Apply(ctx, op.RemoveBounds(
		[]float64{0, 0, 0, 0}, []float64{2, 2, 2, 2},
		bounds(4, space.Open), bounds(4, space.Open))))

	require.False(t, e.Contains([]float64{1, 1, 1, 1}), "the open interior is removed")
	require.True(t, e.Contains([]float64{0, 1, 1, 1}), "the closed min face remains")
	require.True(t, e.Contains([]float64{2, 1, 1, 1}), "the closed max face remains")
}
