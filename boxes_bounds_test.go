package eventfulranges

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/d-led/eventfulranges/space"
	"github.com/d-led/eventfulranges/space/store/memory"
	"github.com/d-led/eventfulranges/space/strategy"
)

func TestBoxSetClosedBoundsEndToEnd(t *testing.T) {
	t.Parallel()
	set, err := OpenBoxStore(context.Background(), memory.New(), strategy.AdditiveWins)
	require.NoError(t, err)
	ctx := context.Background()

	// add [0,2] closed, remove (0,2) open → the endpoints {0} and {2} remain.
	_, err = set.AddBounds(ctx, []float64{0}, []float64{2}, []space.Bound{space.Closed}, []space.Bound{space.Closed})
	require.NoError(t, err)
	_, err = set.RemoveBounds(ctx, []float64{0}, []float64{2}, []space.Bound{space.Open}, []space.Bound{space.Open})
	require.NoError(t, err)

	require.Len(t, set.Boxes(), 2)
	require.True(t, set.Contains([]float64{0}))
	require.False(t, set.Contains([]float64{1}))
	require.True(t, set.Contains([]float64{2}))
}
