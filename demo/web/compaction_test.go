//go:build !js

package main

import (
	"fmt"
	"testing"

	"github.com/d-led/eventfulranges/meta"
	"github.com/d-led/eventfulranges/space"

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

// latestWinsCover is the cover the log describes when the latest operation wins
// at every point: fold the operations in order, adding and cutting as they
// come. It is deliberately independent of the hub's strategy, so a switch back
// to a union-minus-union rule — where a removal is forever — shows up here.
func latestWinsCover(t *testing.T, ops []clientOp) []space.Box {
	t.Helper()
	var cover []space.Box
	for _, op := range ops {
		box := space.NewBox(op.Min, op.Max)
		if op.Kind == string(opRemove) {
			cover = space.DifferenceSortedMerged(cover, []space.Box{box}, meta.Union)
			continue
		}
		cover = space.UnionMerged(cover, []space.Box{box}, meta.Union)
	}
	return cover
}

// TestEveryCompactionModeMaterializesWhatTheLogSays pins two things at once:
// folding the same log twice gives the same view, because a canonicalizer must
// not depend on map or heap iteration order, and that view covers exactly the
// points the latest operation left in. The mode decides how the cover is cut,
// never which points are in it — mixedOps paints over a hole, so a strategy
// where removals win would fail this.
func TestEveryCompactionModeMaterializesWhatTheLogSays(t *testing.T) {
	t.Parallel()
	ops := mixedOps()
	want := latestWinsCover(t, ops)
	for _, mode := range compactionModes {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			first := foldOps(t, mode, ops)
			second := foldOps(t, mode, ops)

			require.Equal(t, first.Boxes, second.Boxes, "the same log twice is the same view")
			requireSameCoverage(t, want, first.Boxes)
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

// TestConsolidationLeavesNothingMergeable is the promise the merge modes make:
// consolidate everything that can be consolidated. Feeding the result back
// through the merger must therefore change nothing — what is left is genuinely
// unmergeable, and only then is a large count acceptable.
func TestConsolidationLeavesNothingMergeable(t *testing.T) {
	t.Parallel()
	ops := mixedOps()
	for _, mode := range []string{compactMerge, compactPartitionMerge} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			got := foldOps(t, mode, ops)
			require.Len(t, space.MergeAdjacent(got.Boxes), len(got.Boxes),
				"every mergeable pair must already have been merged")
		})
	}
}

// TestPartitionModesReportTheCellsTheyConsolidated covers the number the view
// shows: a partition counts its own cells, and the merge that follows reports
// the same cells it started from and fewer ranges.
func TestPartitionModesReportTheCellsTheyConsolidated(t *testing.T) {
	t.Parallel()
	t.Run("overlaps are split, then consolidated", func(t *testing.T) {
		t.Parallel()
		ops := mixedOps()
		partition := foldOps(t, compactPartition, ops)
		merged := foldOps(t, compactPartitionMerge, ops)

		require.Equal(t, len(partition.Boxes), partition.Cells,
			"a partition consolidates nothing, so it reports its own size")
		require.Greater(t, partition.Cells, len(foldOps(t, compactCanonical, ops).Boxes),
			"the edits overlap, so a partition holds more boxes than the cover")
		require.Equal(t, partition.Cells, merged.Cells,
			"the merge starts from the cells the partition produced")
		require.Less(t, len(merged.Boxes), partition.Cells,
			"the merge consolidates, so its ranges are fewer than the cells")
	})
	t.Run("a disjoint cover partitions into itself", func(t *testing.T) {
		t.Parallel()
		// The built-in example is disjoint tiles with the middle carved out, so
		// there is nothing to split: the cells are the tiles.
		require.Equal(t, 26, foldOps(t, compactPartition, exampleOps(3)).Cells)
	})
}

// TestModesWithoutPartitionReportNoCellCount keeps the readout honest: a
// canonical or merge cover never went through a partition, so it reports no
// cell count rather than a misleading zero.
func TestModesWithoutPartitionReportNoCellCount(t *testing.T) {
	t.Parallel()
	require.Zero(t, foldOps(t, compactCanonical, exampleOps(3)).Cells)
	require.Zero(t, foldOps(t, compactMerge, exampleOps(3)).Cells)
}

// TestCompactionModesReportThemselves keeps the panel honest: the view says
// which mode produced it, so the label the viewer reads matches the boxes they
// see.
func TestCompactionModesReportThemselves(t *testing.T) {
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
