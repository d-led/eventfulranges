package rtree

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/d-led/eventfulranges/space"
)

// The split axis and the canonical order are internal contracts: Search's
// output order and the shape of the tree must not depend on the order the boxes
// were handed to Build. These tests pin the tie-breaks that make that true, and
// the face defaults the total order compares.

func TestWidestAxisUsesExtremesFromAnyBox(t *testing.T) {
	boxes := []space.Box{
		space.NewBox([]float64{0, 0}, []float64{1, 1}),
		space.NewBox([]float64{10, 0}, []float64{20, 2}),
	}

	require.Equal(t, 0, widestAxis(boxes), "axis 0's extremes come from two different boxes")
}

func TestWidestAxisKeepsTheEarlierAxisOnATie(t *testing.T) {
	boxes := []space.Box{space.NewBox([]float64{0, 0}, []float64{4, 4})}

	require.Equal(t, 0, widestAxis(boxes))
}

func TestWidestAxisComparesSpanNotOffset(t *testing.T) {
	// Axis 0 is centred on the origin, so adding the corners instead of
	// subtracting them would pick axis 1.
	boxes := []space.Box{space.NewBox([]float64{-10, 0}, []float64{10, 15})}

	require.Equal(t, 0, widestAxis(boxes))
}

func TestLessOnOrdersByLowerThenUpperCorner(t *testing.T) {
	lower := space.NewBox([]float64{0, 5}, []float64{1, 6})
	higherUpper := space.NewBox([]float64{0, 5}, []float64{1, 7})

	require.True(t, lessOn(lower, higherUpper, 1), "the lower corner ties, so the upper corner decides")
	require.False(t, lessOn(higherUpper, lower, 1))
	require.False(t, lessOn(lower, lower, 1), "a box is not less than itself")
}

func TestLessBoxBreaksTiesOnBoundsThenMetadata(t *testing.T) {
	halfOpen := space.NewBox([]float64{0, 0}, []float64{1, 1})
	closed := space.NewBoxWithBounds(
		[]float64{0, 0}, []float64{1, 1},
		[]space.Bound{space.Closed, space.Closed},
		[]space.Bound{space.Closed, space.Closed},
	)

	require.NotEqual(t, lessBox(halfOpen, closed), lessBox(closed, halfOpen),
		"equal geometry with different faces is still ordered")

	metaA := space.NewBox([]float64{0, 0}, []float64{1, 1})
	metaA.Meta = json.RawMessage(`{"k":"a"}`)
	metaB := space.NewBox([]float64{0, 0}, []float64{1, 1})
	metaB.Meta = json.RawMessage(`{"k":"b"}`)

	require.True(t, lessBox(metaA, metaB), "metadata breaks a full tie, ascending by bytes")
	require.False(t, lessBox(metaB, metaA))
	require.False(t, lessBox(metaA, metaA), "a box is not less than itself")
}

func TestSearchReturnsBoxesInCanonicalOrder(t *testing.T) {
	query := space.NewBox([]float64{0, 0}, []float64{10, 10})
	first := space.NewBox([]float64{1, 1}, []float64{2, 2})
	second := space.NewBox([]float64{1, 1}, []float64{3, 3})
	third := space.NewBox([]float64{2, 0}, []float64{3, 1})

	forward, err := Build([]space.Box{first, second, third})
	require.NoError(t, err)
	backward, err := Build([]space.Box{third, second, first})
	require.NoError(t, err)

	require.Equal(t, []space.Box{first, second, third}, forward.Search(query))
	require.Equal(t, []space.Box{first, second, third}, backward.Search(query),
		"build order must not leak into the result")
}

func TestMbrCoversEveryChildWithClosedFaces(t *testing.T) {
	children := []space.Box{
		space.NewBox([]float64{0, 0}, []float64{1, 1}),
		space.NewBox([]float64{5, -1}, []float64{6, 4}),
	}

	mbr := mbrOf(children)

	require.Equal(t, []float64{0, -1}, mbr.Min, "a later child owns an extreme on each axis")
	require.Equal(t, []float64{6, 4}, mbr.Max)
	for i := range mbr.Min {
		require.Equal(t, space.Closed, mbr.MinBound[i], "a closed face must not escape its parent")
		require.Equal(t, space.Closed, mbr.MaxBound[i])
	}
}

func TestWidestAxisFindsExtremesInLaterBoxes(t *testing.T) {
	// A later box owns the extreme on the widest axis, and the mutation that
	// stops updating the running lo/hi picks a different axis.
	minFromLater := []space.Box{
		space.NewBox([]float64{10, 0}, []float64{11, 1}),
		space.NewBox([]float64{0, 0}, []float64{12, 9}),
	}
	require.Equal(t, 0, widestAxis(minFromLater))

	maxFromLater := []space.Box{
		space.NewBox([]float64{0, 0}, []float64{1, 1}),
		space.NewBox([]float64{0, 10}, []float64{1, 20}),
	}
	require.Equal(t, 1, widestAxis(maxFromLater))
}

func TestWidestAxisPicksALaterWiderAxis(t *testing.T) {
	// Axis 1 is the strict widest, so dropping the update would leave axis 0.
	boxes := []space.Box{space.NewBox([]float64{0, 0}, []float64{1, 5})}

	require.Equal(t, 1, widestAxis(boxes))
}

func TestLessOnOrdersByLowerCornerFirst(t *testing.T) {
	lower := space.NewBox([]float64{0, 5}, []float64{9, 9})
	higher := space.NewBox([]float64{0, 7}, []float64{1, 1})

	require.True(t, lessOn(lower, higher, 1), "the lower corner decides before the upper corner")
	require.False(t, lessOn(higher, lower, 1))
}

func TestLessOnFallsBackToTheCanonicalOrder(t *testing.T) {
	halfOpen := space.NewBox([]float64{0, 0}, []float64{1, 1})
	closed := space.NewBoxWithBounds(
		[]float64{0, 0}, []float64{1, 1},
		[]space.Bound{space.Closed, space.Closed},
		[]space.Bound{space.Closed, space.Closed},
	)

	require.Equal(t, lessBox(halfOpen, closed), lessOn(halfOpen, closed, 0),
		"corners equal on this axis defers to the canonical order")
	require.Equal(t, lessBox(closed, halfOpen), lessOn(closed, halfOpen, 0),
		"and in the other direction too, where the fallback is the stricter one")
}

func TestLeavesHoldUpToCapacityBoxes(t *testing.T) {
	boxes := make([]space.Box, 0, capacity+1)
	for i := range capacity + 1 {
		boxes = append(boxes, space.NewBox([]float64{float64(i), 0}, []float64{float64(i) + 1, 1}))
	}

	exact, err := Build(boxes[:capacity])
	require.NoError(t, err)
	require.True(t, exact.root.leaf, "a leaf holds up to capacity boxes")

	over, err := Build(boxes)
	require.NoError(t, err)
	require.False(t, over.root.leaf, "one box beyond capacity forces a split")
}
