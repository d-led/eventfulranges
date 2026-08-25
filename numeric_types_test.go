package eventfulranges_test

import (
	"context"
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"gitub.com/d-led/eventfulranges"
	"gitub.com/d-led/eventfulranges/store/memory"
	"gitub.com/d-led/eventfulranges/strategy"
)

// Coordinates are float64. Integers are carried exactly while they fit into a
// float64's 53-bit mantissa; fractional values are stored and compared
// verbatim. There is no arbitrary-precision (math/big) coordinate type — see
// TestIntegerPrecisionCeilingIsTwoPow53 for the exactness boundary.

func numericSet(t *testing.T) *eventfulranges.RangeSet {
	t.Helper()
	set, err := eventfulranges.OpenStore(context.Background(), memory.New(), strategy.LWW)
	require.NoError(t, err)
	return set
}

func TestIntegerEndpointsAreExact(t *testing.T) {
	t.Parallel()
	set := numericSet(t)
	ctx := context.Background()

	_, err := set.Add(ctx, 0, 10) // [0,10]
	require.NoError(t, err)
	_, err = set.Add(ctx, -100, -90) // [-100,-90]
	require.NoError(t, err)
	_, err = set.Add(ctx, 1<<40, (1<<40)+7) // [2^40, 2^40+7], still exact
	require.NoError(t, err)

	require.True(t, set.Contains(5), "interior of [0,10]")
	require.True(t, set.Contains(-95), "interior of [-100,-90]")
	require.True(t, set.Contains((1<<40)+3), "interior of [2^40, 2^40+7]")
	require.False(t, set.Contains(10.5), "past the [0,10] endpoint")
}

func TestFractionalEndpointsAreUsedVerbatim(t *testing.T) {
	t.Parallel()
	set := numericSet(t)
	ctx := context.Background()

	_, err := set.Add(ctx, 0.5, 2.25)
	require.NoError(t, err)
	_, err = set.Add(ctx, math.Pi, math.Pi+0.5)
	require.NoError(t, err)

	require.True(t, set.Contains(1.0), "interior of [0.5, 2.25]")
	require.True(t, set.Contains(math.Pi), "the stored math.Pi value itself")
	require.True(t, set.Contains(math.Pi+0.25), "between math.Pi and math.Pi+0.5")
	require.False(t, set.Contains(2.5), "past the [0.5, 2.25] endpoint")
}

func TestIntegerPrecisionCeilingIsTwoPow53(t *testing.T) {
	t.Parallel()
	set := numericSet(t)
	ctx := context.Background()

	// float64 holds a 53-bit mantissa, so above 2^53 consecutive integers no
	// longer round-trip: 2^53 + 1 is stored as 2^53. That is the boundary at
	// which a caller would need arbitrary precision, which this library does
	// not provide.
	require.Equal(t, float64(1<<53), float64((1<<53)+1))

	// A range "above" the ceiling collapses onto the point [2^53, 2^53].
	_, err := set.Add(ctx, float64(1<<53), float64((1<<53)+1))
	require.NoError(t, err)

	require.True(t, set.Contains(float64(1<<53)))
	require.False(t, set.Contains(float64((1<<53)+2)), "2^53+2 is representable and lies outside")
}
