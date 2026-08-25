package space

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func nan() float64 {
	return math.NaN()
}

func inf() float64 {
	return math.Inf(1)
}

func box2(x0, y0, x1, y1 float64) Box {
	return NewBox([]float64{x0, y0}, []float64{x1, y1})
}

func TestValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		box     Box
		wantErr error
	}{
		{name: "a 1D interval", box: NewBox([]float64{0}, []float64{1})},
		{name: "a 2D rectangle", box: box2(0, 0, 1, 1)},
		{name: "a 3D box", box: NewBox([]float64{0, 0, 0}, []float64{1, 1, 1})},
		{name: "a 4D box", box: NewBox([]float64{0, 0, 0, 0}, []float64{1, 1, 1, 1})},
		{
			name: "a zero-width side is still well-formed",
			box:  box2(1, 0, 1, 1),
		},

		{name: "min longer than max", box: Box{Min: []float64{0, 0}, Max: []float64{1}}, wantErr: ErrDimensions},
		{name: "max longer than min", box: Box{Min: []float64{0}, Max: []float64{1, 1}}, wantErr: ErrDimensions},
		{name: "no coordinates at all", box: Box{}, wantErr: ErrNoDimensions},
		{name: "empty coordinate slices", box: Box{Min: []float64{}, Max: []float64{}}, wantErr: ErrNoDimensions},

		{name: "NaN in the first min coordinate", box: box2(nan(), 0, 1, 1), wantErr: ErrNaN},
		{name: "NaN in the last max coordinate", box: NewBox([]float64{0, 0, 0}, []float64{1, 1, nan()}), wantErr: ErrNaN},

		{name: "positive infinity in a min coordinate", box: box2(inf(), 0, 1, 1), wantErr: ErrInfinite},
		{name: "negative infinity in a max coordinate", box: box2(0, 0, 1, math.Inf(-1)), wantErr: ErrInfinite},

		{name: "inverted first dimension", box: box2(1, 0, 0, 1), wantErr: ErrInverted},
		{name: "inverted middle dimension", box: NewBox([]float64{0, 1, 0}, []float64{1, 0, 1}), wantErr: ErrInverted},
		{name: "inverted last dimension", box: NewBox([]float64{0, 0, 1}, []float64{1, 1, 0}), wantErr: ErrInverted},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.wantErr == nil {
				require.NoError(t, tt.box.Validate())
				return
			}
			require.ErrorIs(t, tt.box.Validate(), tt.wantErr)
		})
	}
}

func TestContains(t *testing.T) {
	t.Parallel()
	b := box2(1, 2, 3, 4)
	require.True(t, b.Contains([]float64{1, 2}), "min corner is included")
	require.True(t, b.Contains([]float64{2.5, 3.5}), "interior point is included")
	require.False(t, b.Contains([]float64{3, 4}), "max corner is excluded (half-open)")
	require.False(t, b.Contains([]float64{0, 3}), "point left of min is excluded")
	require.False(t, b.Contains([]float64{2, 5}), "point above max is excluded")
}

func TestOverlaps(t *testing.T) {
	t.Parallel()
	a := box2(0, 0, 2, 2)
	require.True(t, Overlaps(a, box2(1, 1, 3, 3)), "overlapping boxes share points")
	require.False(t, Overlaps(a, box2(2, 0, 3, 2)), "touching along an edge does not overlap (half-open)")
	require.False(t, Overlaps(a, box2(0, 2, 2, 3)), "touching along the top edge does not overlap")
	require.False(t, Overlaps(a, box2(3, 3, 4, 4)), "disjoint boxes do not overlap")
}

func TestEmpty(t *testing.T) {
	t.Parallel()
	require.True(t, box2(1, 0, 1, 1).Empty(), "zero width makes a box empty")
	require.False(t, box2(0, 0, 1, 1).Empty())
}

func TestDims(t *testing.T) {
	t.Parallel()
	require.Equal(t, 2, box2(0, 0, 1, 1).Dims())
}

func TestBoxOrderingHelpers(t *testing.T) {
	t.Parallel()
	a := box2(0, 0, 1, 1)
	require.True(t, lessBox(a, box2(0, 0, 1, 2)), "same min, smaller max sorts first")
	require.False(t, lessBox(a, a), "equal boxes are not strictly ordered")
	require.True(t, equalBox(a, a))
	require.False(t, equalBox(a, box2(0, 0, 1, 2)), "max differs")
	require.False(t, equalBox(a, box2(1, 0, 1, 1)), "min differs")
}

func TestString(t *testing.T) {
	t.Parallel()
	require.Equal(t, "[0 2) x [1 3)", box2(0, 1, 2, 3).String())
}

func TestNormalize(t *testing.T) {
	t.Parallel()
	t.Run("drops empty", func(t *testing.T) {
		t.Parallel()
		got := Normalize([]Box{box2(0, 0, 0, 1)})
		require.Empty(t, got)
	})
	t.Run("drops subsumed either order", func(t *testing.T) {
		t.Parallel()
		big := box2(0, 0, 4, 4)
		small := box2(1, 1, 2, 2)
		want := []Box{big}
		require.Equal(t, want, Normalize([]Box{small, big}))
		require.Equal(t, want, Normalize([]Box{big, small}))
	})
	t.Run("keeps partial overlaps", func(t *testing.T) {
		t.Parallel()
		a := box2(0, 0, 2, 2)
		b := box2(1, 1, 3, 3)
		got := Normalize([]Box{a, b})
		require.Len(t, got, 2)
	})
}

func TestUnion(t *testing.T) {
	t.Parallel()
	a := []Box{box2(0, 0, 2, 2)}
	b := []Box{box2(1, 1, 3, 3), box2(1.5, 1.5, 2, 2)}
	union := Union(a, b)
	require.Len(t, union, 2, "subsumed box must be dropped")
	require.Contains(t, union, box2(0, 0, 2, 2))
	require.Contains(t, union, box2(1, 1, 3, 3))
}

func TestDifference(t *testing.T) {
	t.Parallel()
	t.Run("disjoint leaves operand intact", func(t *testing.T) {
		t.Parallel()
		a := []Box{box2(0, 0, 2, 2)}
		require.Equal(t, a, Difference(a, []Box{box2(5, 5, 6, 6)}))
	})
	t.Run("fully covered box vanishes", func(t *testing.T) {
		t.Parallel()
		a := []Box{box2(1, 1, 2, 2)}
		require.Empty(t, Difference(a, []Box{box2(0, 0, 4, 4)}))
	})
	t.Run("edge clip produces two slabs", func(t *testing.T) {
		t.Parallel()
		a := []Box{box2(0, 0, 4, 4)}
		cut := []Box{box2(1, -1, 3, 5)}
		got := Difference(a, cut)
		require.Equal(t, []Box{box2(0, 0, 1, 4), box2(3, 0, 4, 4)}, got)
	})
	t.Run("center hole produces four slabs", func(t *testing.T) {
		t.Parallel()
		a := []Box{box2(0, 0, 4, 4)}
		cut := []Box{box2(1, 1, 3, 3)}
		got := Difference(a, cut)
		require.Len(t, got, 4)
	})
}

func TestEqual(t *testing.T) {
	t.Parallel()
	a := []Box{box2(0, 0, 2, 2), box2(1, 1, 3, 3)}
	b := []Box{box2(1, 1, 3, 3), box2(0, 0, 2, 2)}
	require.True(t, Equal(a, b), "order must not matter")
	require.False(t, Equal(a, []Box{box2(0, 0, 2, 2)}), "different cardinality")
	require.False(t, Equal(a, []Box{box2(0, 0, 2, 2), box2(1, 1, 3, 4)}), "same cardinality, different coverage")
}

func TestSetQueries(t *testing.T) {
	t.Parallel()
	set := []Box{box2(0, 0, 2, 2), box2(5, 5, 6, 6)}
	require.True(t, Contains(set, []float64{1, 1}))
	require.True(t, Contains(set, []float64{5, 5}))
	require.False(t, Contains(set, []float64{3, 3}))
	require.True(t, OverlapsSet(set, box2(1, 1, 7, 7)))
	require.False(t, OverlapsSet(set, box2(3, 3, 4, 4)))
}
