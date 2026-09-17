package space

import (
	"fmt"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestMergeAdjacent(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   []Box
		want []Box
	}{
		{
			name: "adjacent boxes merge along x",
			in:   []Box{box2(0, 0, 2, 4), box2(2, 0, 4, 4)},
			want: []Box{box2(0, 0, 4, 4)},
		},
		{
			name: "adjacent boxes merge along y",
			in:   []Box{box2(0, 0, 4, 2), box2(0, 2, 4, 4)},
			want: []Box{box2(0, 0, 4, 4)},
		},
		{
			name: "a gap is not merged",
			in:   []Box{box2(0, 0, 2, 2), box2(3, 0, 5, 2)},
			want: []Box{box2(0, 0, 2, 2), box2(3, 0, 5, 2)},
		},
		{
			name: "partial overlap is not merged",
			in:   []Box{box2(0, 0, 2, 2), box2(1, 0, 3, 2)},
			want: []Box{box2(0, 0, 2, 2), box2(1, 0, 3, 2)},
		},
		{
			name: "a cross is not mergeable",
			in:   []Box{box2(0, 1, 4, 3), box2(1, 0, 3, 4)},
			want: []Box{box2(0, 1, 4, 3), box2(1, 0, 3, 4)},
		},
		{
			name: "a row of three collapses to one",
			in:   []Box{box2(0, 0, 2, 2), box2(2, 0, 4, 2), box2(4, 0, 6, 2)},
			want: []Box{box2(0, 0, 6, 2)},
		},
		{
			name: "a staircase collapses to one",
			in: []Box{
				box2(0, 0, 2, 2), // bottom-left
				box2(2, 0, 4, 4), // right, taller
				box2(0, 2, 2, 4), // top-left
			},
			want: []Box{box2(0, 0, 4, 4)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, MergeAdjacent(tt.in))
		})
	}
}

func TestMergePair(t *testing.T) {
	t.Parallel()
	t.Run("identical boxes do not merge", func(t *testing.T) {
		t.Parallel()
		a := box2(0, 0, 2, 2)
		_, ok := mergePair(a, a)
		require.False(t, ok)
	})
	t.Run("mismatched dimensions do not merge", func(t *testing.T) {
		t.Parallel()
		_, ok := mergePair(box2(0, 0, 1, 1), NewBox([]float64{0}, []float64{1}))
		require.False(t, ok)
	})
	t.Run("differing in two dimensions do not merge", func(t *testing.T) {
		t.Parallel()
		_, ok := mergePair(box2(0, 0, 1, 1), box2(1, 1, 2, 2))
		require.False(t, ok)
	})
	t.Run("merge is order-independent", func(t *testing.T) {
		t.Parallel()
		got, ok := mergePair(box2(2, 0, 4, 2), box2(0, 0, 2, 2))
		require.True(t, ok)
		require.Equal(t, box2(0, 0, 4, 2), got)
	})
}

func TestChain(t *testing.T) {
	t.Parallel()
	t.Run("normalize then merge", func(t *testing.T) {
		t.Parallel()
		compact := Chain(Normalize, MergeAdjacent)
		got := compact([]Box{box2(2, 0, 4, 4), box2(0, 0, 2, 4)})
		require.Equal(t, []Box{box2(0, 0, 4, 4)}, got)
	})
	t.Run("skips nil canonicalizers", func(t *testing.T) {
		t.Parallel()
		got := Chain(nil, Normalize, nil)([]Box{box2(0, 0, 1, 1)})
		require.Equal(t, []Box{box2(0, 0, 1, 1)}, got)
	})
}

func TestPropertyMergeAdjacentPreservesCoverage(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		raw := rapid.SliceOfN(genBox(t), 0, 6).Draw(t, "raw")
		merged := MergeAdjacent(raw)
		requireCanonical(t, merged)
		for _, p := range samplePoints() {
			if Contains(merged, p) != Contains(raw, p) {
				t.Fatalf("merge-adjacent must cover the same points\n"+
					"point %s\n  raw:     %s\n  merged:  %s",
					renderPoint(p), renderBoxes(raw), renderBoxes(merged))
			}
		}
	})
}

func TestPropertyMergeAdjacentIsIdempotent(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		raw := rapid.SliceOfN(genBox(t), 0, 6).Draw(t, "raw")
		once := MergeAdjacent(raw)
		twice := MergeAdjacent(once)
		if !Equal(once, twice) {
			t.Fatalf("merge-adjacent must be idempotent\n  raw:   %s\n  once:  %s\n  twice: %s",
				renderBoxes(raw), renderBoxes(once), renderBoxes(twice))
		}
	})
}

// referenceMergeAdjacent is the definition of MergeAdjacent written the
// obvious way: test every pair, merge the first mergeable one, re-normalize
// the whole cover, repeat. It is far too slow for the cell grids the
// visualizer merges, which is why MergeAdjacent uses an index — and why the
// tests below hold the two to the same answer.
func referenceMergeAdjacent(boxes []Box) []Box {
	boxes = Normalize(boxes)
	for {
		merged, changed := referenceMergeFirstPair(boxes)
		if !changed {
			return boxes
		}
		boxes = merged
	}
}

// referenceMergeFirstPair merges the first mergeable pair in sorted order,
// reporting whether any merge happened.
func referenceMergeFirstPair(boxes []Box) ([]Box, bool) {
	for i := 0; i < len(boxes); i++ {
		for j := i + 1; j < len(boxes); j++ {
			merged, ok := mergePair(boxes[i], boxes[j])
			if !ok {
				continue
			}
			out := make([]Box, 0, len(boxes)-1)
			out = append(out, boxes[:i]...)
			out = append(out, boxes[i+1:j]...)
			out = append(out, boxes[j+1:]...)
			out = append(out, merged)
			return Normalize(out), true
		}
	}
	return boxes, false
}

func TestMergeAdjacentMatchesReference(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   []Box
	}{
		{"empty", nil},
		{"a single box", []Box{box2(0, 0, 2, 2)}},
		{"a row and a column", []Box{box2(0, 0, 2, 2), box2(2, 0, 4, 2), box2(0, 2, 2, 4), box2(2, 2, 4, 4)}},
		{"a staircase with an obstruction", []Box{
			box2(0, 0, 2, 2), box2(2, 0, 4, 4), box2(0, 2, 2, 4), box2(4, 0, 6, 2),
		}},
		{"a finger of three", []Box{box2(0, 0, 1, 1), box2(1, 0, 2, 1), box2(2, 0, 3, 1), box2(1, 1, 2, 2)}},
		{"touching only at a corner", []Box{box2(0, 0, 1, 1), box2(1, 1, 2, 2)}},
		{"duplicates collapse", []Box{box2(0, 0, 2, 2), box2(0, 0, 2, 2)}},
		{"a covered box is absorbed", []Box{box2(0, 0, 4, 4), box2(1, 1, 2, 2)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, referenceMergeAdjacent(tt.in), MergeAdjacent(tt.in))
		})
	}
}

func TestPropertyMergeAdjacentMatchesReference(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		raw := rapid.SliceOfN(genBox(t), 0, 12).Draw(t, "raw")
		want := referenceMergeAdjacent(raw)
		got := MergeAdjacent(raw)
		if !Equal(want, got) {
			t.Fatalf("the indexed merge must return what the plain greedy returns\n"+
				"raw:  %s\n  plain:   %s\n  indexed: %s",
				renderBoxes(raw), renderBoxes(want), renderBoxes(got))
		}
		require.Equal(t, want, got)
	})
}

func TestPropertyMergeAdjacentMatchesReferenceOnGrids(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		cells := gridCells(t, 4)
		if len(cells) < 2 {
			return // nothing to merge, nothing to compare
		}
		want := referenceMergeAdjacent(cells)
		got := MergeAdjacent(cells)
		require.Equal(t, want, got,
			"the grids MergeAdjacent is actually fed must match too\n  cells: %s", renderBoxes(cells))
	})
}

// gridCells returns the cells of a random grid covering [0,4)^2, keeping each
// one with a coin toss. It is the shape of cover MergeAdjacent is really fed —
// a partition hands it cells that share cross-sections and touch along faces —
// which is where the candidate index has to prove itself against the pair scan.
func gridCells(t *rapid.T, extent int) []Box {
	xs := gridCuts(t, extent)
	ys := gridCuts(t, extent)
	var cells []Box
	for i := 0; i+1 < len(xs); i++ {
		for j := 0; j+1 < len(ys); j++ {
			if rapid.Bool().Draw(t, "keep the cell") {
				cells = append(cells, box2(xs[i], ys[j], xs[i+1], ys[j+1]))
			}
		}
	}
	return cells
}

// gridCuts returns the distinct cut lines of a random grid over [0,extent].
func gridCuts(t *rapid.T, extent int) []float64 {
	cuts := []float64{0}
	for range extent - 1 {
		cuts = append(cuts, rapid.Float64Range(0, float64(extent)).Draw(t, "cut"))
	}
	cuts = append(cuts, float64(extent))
	slices.Sort(cuts)
	return slices.Compact(cuts)
}

func FuzzMergeAdjacent(f *testing.F) {
	f.Add([]byte{0, 1, 2, 3, 4, 5, 6, 7})
	f.Fuzz(func(t *testing.T, data []byte) {
		boxes := boxesFromBytes(data)
		merged := MergeAdjacent(boxes)
		for _, p := range samplePoints() {
			if Contains(merged, p) != Contains(boxes, p) {
				t.Fatalf("merge-adjacent must cover the same points\npoint %s\n  raw:    %s\n  merged: %s",
					renderPoint(p), renderBoxes(boxes), renderBoxes(merged))
			}
		}
	})
}

func ExampleMergeAdjacent() {
	cover := []Box{
		NewBox([]float64{0, 0}, []float64{2, 4}),
		NewBox([]float64{2, 0}, []float64{4, 4}),
	}
	fmt.Println(MergeAdjacent(cover))
	// Output: [[0 4) x [0 4)]
}

func ExampleChain() {
	cover := []Box{
		NewBox([]float64{2, 0}, []float64{4, 4}),
		NewBox([]float64{0, 0}, []float64{2, 4}),
	}
	compact := Chain(Normalize, MergeAdjacent)
	fmt.Println(compact(cover))
	// Output: [[0 4) x [0 4)]
}
