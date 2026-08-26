package space

import (
	"fmt"
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
