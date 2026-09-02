//go:build !js

package main

import (
	"testing"

	"github.com/d-led/eventfulranges/space"
	"github.com/stretchr/testify/require"
)

func box2d(loX, loY, hiX, hiY float64) space.Box {
	return space.NewBox([]float64{loX, loY}, []float64{hiX, hiY})
}

func TestPartitionKeepsDisjointInput(t *testing.T) {
	t.Parallel()
	in := []space.Box{box2d(0, 0, 1, 1), box2d(2, 2, 3, 3)}
	out := Partition(in)
	require.Len(t, out, 2)
	requireDisjoint(t, out)
	requireSameCoverage(t, in, out)
}

func TestPartitionSplitsCornerOverlap(t *testing.T) {
	t.Parallel()
	in := []space.Box{box2d(0, 0, 2, 2), box2d(1, 1, 3, 3)}
	out := Partition(in)
	require.Len(t, out, 5, "an L-shaped union splits into five rectangles")
	requireDisjoint(t, out)
	requireSameCoverage(t, in, out)
}

func TestPartitionSplitsFullFaceOverlap(t *testing.T) {
	t.Parallel()
	in := []space.Box{box2d(0, 0, 4, 4), box2d(2, 0, 6, 4)}
	out := Partition(in)
	require.Len(t, out, 3, "a full-face overlap splits into three abutting slabs")
	requireDisjoint(t, out)
	requireSameCoverage(t, in, out)
}

func TestPartitionGeneralizesTo3D(t *testing.T) {
	t.Parallel()
	a := space.NewBox([]float64{0, 0, 0}, []float64{2, 2, 2})
	b := space.NewBox([]float64{1, 1, 1}, []float64{3, 3, 3})
	out := Partition([]space.Box{a, b})
	require.Len(t, out, 7, "two overlapping cubes split into seven boxes")
	requireDisjoint(t, out)
	requireSameCoverage(t, []space.Box{a, b}, out)
}

func TestPartitionGeneralizesTo4D(t *testing.T) {
	t.Parallel()
	a := space.NewBox([]float64{0, 0, 0, 0}, []float64{2, 2, 2, 2})
	b := space.NewBox([]float64{1, 1, 1, 1}, []float64{3, 3, 3, 3})
	out := Partition([]space.Box{a, b})
	require.Len(t, out, 9, "two overlapping 4D boxes split into nine boxes")
	requireDisjoint(t, out)
	requireSameCoverage(t, []space.Box{a, b}, out)
}

func TestPartitionIsIdempotent(t *testing.T) {
	t.Parallel()
	in := []space.Box{box2d(0, 0, 2, 2), box2d(1, 1, 3, 3), box2d(2, 2, 4, 4)}
	once := Partition(in)
	require.Equal(t, once, Partition(once))
}

func TestHubPartitionModeReturnsDisjointBoxes(t *testing.T) {
	t.Parallel()
	h := newHubMode(compactPartition)
	_, err := h.apply(opAdd, []float64{0, 0}, []float64{2, 2})
	require.NoError(t, err)
	_, err = h.apply(opAdd, []float64{1, 1}, []float64{3, 3})
	require.NoError(t, err)
	v := h.snapshot()
	require.Equal(t, compactPartition, v.Compact)
	requireDisjoint(t, v.Boxes)
}

// requireDisjoint asserts every box is valid and no two boxes share a point.
func requireDisjoint(t *testing.T, boxes []space.Box) {
	t.Helper()
	for i := range boxes {
		require.NoError(t, boxes[i].Validate(), "box %d is invalid", i)
		for j := i + 1; j < len(boxes); j++ {
			require.False(t, space.Overlaps(boxes[i], boxes[j]),
				"boxes %d and %d overlap: %v vs %v", i, j, boxes[i], boxes[j])
		}
	}
}

// requireSameCoverage samples a fine grid and asserts the two covers agree on
// every point, proving the partition is cover-preserving.
func requireSameCoverage(t *testing.T, in, out []space.Box) {
	t.Helper()
	dims := len(in[0].Min)
	lo := make([]float64, dims)
	hi := make([]float64, dims)
	for i := range dims {
		lo[i], hi[i] = in[0].Min[i], in[0].Max[i]
	}
	for _, b := range in {
		for i := range dims {
			if b.Min[i] < lo[i] {
				lo[i] = b.Min[i]
			}
			if b.Max[i] > hi[i] {
				hi[i] = b.Max[i]
			}
		}
	}
	var visit func(prefix []float64, d int)
	visit = func(prefix []float64, d int) {
		if d == dims {
			p := append([]float64(nil), prefix...)
			require.Equal(t, space.Contains(in, p), space.Contains(out, p), "point %v", p)
			return
		}
		for v := lo[d] - 0.5; v < hi[d]+0.5; v += 0.25 {
			visit(append(prefix, v), d+1)
		}
	}
	visit(nil, 0)
}
