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

	t.Run("valid", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, box2(0, 0, 1, 1).Validate())
	})
	t.Run("dimension mismatch", func(t *testing.T) {
		t.Parallel()
		b := Box{Min: []float64{0, 0}, Max: []float64{1}}
		require.ErrorIs(t, b.Validate(), ErrDimensions)
	})
	t.Run("no dimensions", func(t *testing.T) {
		t.Parallel()
		b := Box{Min: nil, Max: nil}
		require.ErrorIs(t, b.Validate(), ErrNoDimensions)
	})
	t.Run("nan", func(t *testing.T) {
		t.Parallel()
		b := box2(0, 0, 1, 1)
		b.Max[1] = nan()
		require.ErrorIs(t, b.Validate(), ErrNaN)
	})
	t.Run("infinite", func(t *testing.T) {
		t.Parallel()
		b := box2(0, 0, 1, 1)
		b.Min[0] = inf()
		require.ErrorIs(t, b.Validate(), ErrInfinite)
	})
	t.Run("inverted", func(t *testing.T) {
		t.Parallel()
		b := box2(1, 0, 0, 1)
		require.ErrorIs(t, b.Validate(), ErrInverted)
	})
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
