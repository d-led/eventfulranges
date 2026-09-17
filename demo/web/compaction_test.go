//go:build !js

package main

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// compactionModes is every option the New-session dialog offers. They differ in
// what they do to the cover — keep it, join touching boxes, split overlaps, or
// both — but never in which points are covered.
var compactionModes = []string{
	compactCanonical,
	compactMerge,
	compactPartition,
	compactPartitionMerge,
}

// mixedOps is the visualizer's built-in example followed by the kind of edits a
// viewer makes on top of it: an add and two removes that cut across the tiles
// and overlap each other, so every mode has something to split and to join.
func mixedOps() []clientOp {
	return append(exampleOps(3),
		clientOp{Kind: "remove", Min: []float64{0.5, 0.5, 0.5}, Max: []float64{2.5, 1.5, 2.5}},
		clientOp{Kind: "add", Min: []float64{1.2, 1.2, 1.2}, Max: []float64{2.8, 2.8, 2.8}},
		clientOp{Kind: "remove", Min: []float64{0, 0, 0}, Max: []float64{3, 0.4, 3}},
	)
}

// foldOps folds the operations into a fresh hub in the given mode and returns
// the resulting view.
func foldOps(t *testing.T, mode string, ops []clientOp) view {
	t.Helper()
	h := newHubMode(mode)
	for _, op := range ops {
		_, err := h.apply(opKind(op.Kind), op.Min, op.Max)
		require.NoError(t, err, "applying %s %v -> %v", op.Kind, op.Min, op.Max)
	}
	return h.snapshot()
}

// TestEveryCompactionModeCoversTheSamePoints is the promise all four modes
// share: however they rearrange the boxes, a point is in the result exactly
// when it is in the canonical cover. Anything else would be a compaction that
// changes the model.
func TestEveryCompactionModeCoversTheSamePoints(t *testing.T) {
	t.Parallel()
	ops := mixedOps()
	canonical := foldOps(t, compactCanonical, ops)
	require.NotEmpty(t, canonical.Boxes, "the workload must cover something")

	for _, mode := range compactionModes {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			got := foldOps(t, mode, ops)
			requireSameCoverage(t, canonical.Boxes, got.Boxes)
		})
	}
}

// TestEveryCompactionModeConvergesRegardlessOfArrivalOrder replays the same
// operations backwards. A replica must reach the same view whichever order the
// operations arrive in — the compaction included, which is why a canonicalizer
// has to be deterministic.
func TestEveryCompactionModeConvergesRegardlessOfArrivalOrder(t *testing.T) {
	t.Parallel()
	ops := mixedOps()
	for _, mode := range compactionModes {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			forward := foldOps(t, mode, ops)

			reversed := make([]clientOp, len(ops))
			for i, op := range ops {
				reversed[len(ops)-1-i] = op
			}
			backward := foldOps(t, mode, reversed)

			require.Equal(t, forward.Boxes, backward.Boxes, "arrival order must not change the view")
			require.Equal(t, forward.Adds, backward.Adds)
			require.Equal(t, forward.Removes, backward.Removes)
		})
	}
}

// TestPartitionModesAreDisjoint pins the point of the partition: the boxes a
// viewer sees must not share a point, so translucent boxes are never drawn
// twice over the same place.
func TestPartitionModesAreDisjoint(t *testing.T) {
	t.Parallel()
	ops := mixedOps()
	for _, mode := range []string{compactPartition, compactPartitionMerge} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			got := foldOps(t, mode, ops)
			requireDisjoint(t, got.Boxes)
		})
	}
}

// TestMergingModesNeverAddBoxes pins what the merge-half of a mode is allowed
// to do: joining touching boxes can only ever reduce their number. If a merge
// mode ended up with more boxes than the same cover without it, the
// canonicalizer would be doing something other than compacting.
func TestMergingModesNeverAddBoxes(t *testing.T) {
	t.Parallel()
	ops := mixedOps()
	canonical := foldOps(t, compactCanonical, ops)
	merge := foldOps(t, compactMerge, ops)
	partition := foldOps(t, compactPartition, ops)
	partitionMerge := foldOps(t, compactPartitionMerge, ops)

	require.LessOrEqual(t, len(merge.Boxes), len(canonical.Boxes), "merge only joins boxes")
	require.LessOrEqual(t, len(partitionMerge.Boxes), len(partition.Boxes), "the merge half only joins boxes")
}

// TestReplayingTheExampleIsIdempotent covers the "Load example" button: it
// sends the same operations again, and the view must not change, because
// additions union and removals union.
func TestReplayingTheExampleIsIdempotent(t *testing.T) {
	t.Parallel()
	ops := exampleOps(3)
	for _, mode := range compactionModes {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			once := foldOps(t, mode, ops)
			twice := foldOps(t, mode, append(append([]clientOp(nil), ops...), ops...))
			require.Equal(t, once.Boxes, twice.Boxes)
		})
	}
}

// TestEveryCompactionModeReportsItself keeps the panel honest: the view says
// which mode produced it, so the label the viewer reads matches the boxes they
// see.
func TestEveryCompactionModeReportsItself(t *testing.T) {
	t.Parallel()
	for _, mode := range compactionModes {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, mode, foldOps(t, mode, exampleOps(2)).Compact)
		})
	}
}

// TestUnknownCompactionFallsBackToCanonical guards the URL parameter: an
// unrecognized ?compact= value must not invent a fifth mode.
func TestUnknownCompactionFallsBackToCanonical(t *testing.T) {
	t.Parallel()
	require.Equal(t, compactCanonical, foldOps(t, "nonsense", exampleOps(2)).Compact)
}

// TestCompactionModesAgreeOnTheExampleInEveryDimension repeats the shared
// coverage promise over the built-in example in every dimension the visualizer
// draws, including the 1D case whose carving differs.
func TestCompactionModesAgreeOnTheExampleInEveryDimension(t *testing.T) {
	t.Parallel()
	for dims := 1; dims <= maxDims; dims++ {
		t.Run(fmt.Sprintf("%dD", dims), func(t *testing.T) {
			t.Parallel()
			ops := exampleOps(dims)
			canonical := foldOps(t, compactCanonical, ops)
			for _, mode := range compactionModes {
				got := foldOps(t, mode, ops)
				requireSameCoverage(t, canonical.Boxes, got.Boxes)
				require.Equal(t, dims, got.Dims)
			}
		})
	}
}
