package space

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func boundSlice(n int, b Bound) []Bound {
	out := make([]Bound, n)
	for i := range out {
		out[i] = b
	}
	return out
}

func TestNewBoxDefaultsToHalfOpen(t *testing.T) {
	t.Parallel()
	box := NewBox([]float64{0}, []float64{2})
	require.True(t, box.Contains([]float64{0}))
	require.True(t, box.Contains([]float64{1}))
	require.False(t, box.Contains([]float64{2}))
}

func TestClosedBoxContainsEndpoints(t *testing.T) {
	t.Parallel()
	box := NewBoxWithBounds([]float64{0}, []float64{2}, []Bound{Closed}, []Bound{Closed})
	require.True(t, box.Contains([]float64{0}))
	require.True(t, box.Contains([]float64{1}))
	require.True(t, box.Contains([]float64{2}))
	require.False(t, box.Contains([]float64{-0.1}))
	require.False(t, box.Contains([]float64{2.1}))
}

func TestOpenBoxExcludesEndpoints(t *testing.T) {
	t.Parallel()
	box := NewBoxWithBounds([]float64{0}, []float64{2}, []Bound{Open}, []Bound{Open})
	require.False(t, box.Contains([]float64{0}))
	require.True(t, box.Contains([]float64{1}))
	require.False(t, box.Contains([]float64{2}))
}

func TestClosedPointBoxIsNonEmpty(t *testing.T) {
	t.Parallel()
	point := NewBoxWithBounds([]float64{1}, []float64{1}, []Bound{Closed}, []Bound{Closed})
	require.False(t, point.Empty())
	require.True(t, point.Contains([]float64{1}))
	require.NoError(t, point.Validate())

	open := NewBoxWithBounds([]float64{1}, []float64{1}, []Bound{Open}, []Bound{Closed})
	require.True(t, open.Empty())
}

func TestOverlapsRespectsBounds(t *testing.T) {
	t.Parallel()
	a := NewBoxWithBounds([]float64{0}, []float64{1}, []Bound{Closed}, []Bound{Closed})
	b := NewBoxWithBounds([]float64{1}, []float64{2}, []Bound{Closed}, []Bound{Closed})
	require.True(t, Overlaps(a, b), "closed faces share the point 1")

	require.False(t, Overlaps(NewBox([]float64{0}, []float64{1}), NewBox([]float64{1}, []float64{2})),
		"half-open boxes merely touching do not share a point")

	point := NewBoxWithBounds([]float64{0}, []float64{0}, []Bound{Closed}, []Bound{Closed})
	open := NewBoxWithBounds([]float64{0}, []float64{1}, []Bound{Open}, []Bound{Open})
	require.False(t, Overlaps(point, open), "a closed point does not overlap an interval that excludes it")
}

func TestNormalizeSubsumesByBounds(t *testing.T) {
	t.Parallel()
	closed := NewBoxWithBounds([]float64{0}, []float64{2}, []Bound{Closed}, []Bound{Closed})
	halfOpen := NewBox([]float64{0}, []float64{2}) // [0,2)
	got := Normalize([]Box{halfOpen, closed})
	require.Len(t, got, 1)
	require.True(t, got[0].Contains([]float64{2}), "the closed box must subsume the half-open one")
}

func TestDifferenceClosedMinusOpenLeavesEndpoints(t *testing.T) {
	t.Parallel()
	p := NewBoxWithBounds([]float64{0}, []float64{2}, []Bound{Closed}, []Bound{Closed})
	q := NewBoxWithBounds([]float64{0}, []float64{2}, []Bound{Open}, []Bound{Open})
	got := Difference([]Box{p}, []Box{q})
	require.Len(t, got, 2) // {0} and {2}
	require.True(t, Contains(got, []float64{0}))
	require.False(t, Contains(got, []float64{1}))
	require.True(t, Contains(got, []float64{2}))
}

func TestDifferenceClosedMinusClosedHalf(t *testing.T) {
	t.Parallel()
	p := NewBoxWithBounds([]float64{0}, []float64{2}, []Bound{Closed}, []Bound{Closed})
	q := NewBoxWithBounds([]float64{0}, []float64{1}, []Bound{Closed}, []Bound{Closed})
	got := Difference([]Box{p}, []Box{q})
	require.Len(t, got, 1) // (1,2]
	require.False(t, Contains(got, []float64{1}))
	require.True(t, Contains(got, []float64{1.5}))
	require.True(t, Contains(got, []float64{2}))
}

func TestUnionOfTouchingClosed(t *testing.T) {
	t.Parallel()
	a := NewBoxWithBounds([]float64{0}, []float64{1}, []Bound{Closed}, []Bound{Closed})
	b := NewBoxWithBounds([]float64{1}, []float64{2}, []Bound{Closed}, []Bound{Closed})
	got := Union([]Box{a}, []Box{b})
	// Union is a cover, not a partition: neither box subsumes the other, so
	// both survive; the shared closed face is still covered exactly once by
	// membership (merge is MergeAdjacent's job).
	require.Len(t, got, 2)
	require.True(t, Contains(got, []float64{1}))
	require.True(t, Contains(got, []float64{0.5}))
	require.True(t, Contains(got, []float64{1.5}))
}

func TestMergeAdjacentRespectsBounds(t *testing.T) {
	t.Parallel()
	a := NewBoxWithBounds([]float64{0}, []float64{1}, []Bound{Closed}, []Bound{Closed})
	b := NewBoxWithBounds([]float64{1}, []float64{2}, []Bound{Closed}, []Bound{Closed})
	merged := MergeAdjacent([]Box{a, b})
	require.Len(t, merged, 1)
	require.True(t, merged[0].Contains([]float64{0}))
	require.True(t, merged[0].Contains([]float64{2}))

	// a pinhole (both shared faces open) prevents merging
	c := NewBoxWithBounds([]float64{0}, []float64{1}, []Bound{Closed}, []Bound{Open})
	d := NewBoxWithBounds([]float64{1}, []float64{2}, []Bound{Open}, []Bound{Closed})
	require.Len(t, MergeAdjacent([]Box{c, d}), 2)
}

func Test2DClosedMinusOpenKeepsBoundary(t *testing.T) {
	t.Parallel()
	add := NewBoxWithBounds([]float64{0, 0}, []float64{2, 2}, boundSlice(2, Closed), boundSlice(2, Closed))
	rem := NewBoxWithBounds([]float64{0, 0}, []float64{2, 2}, boundSlice(2, Open), boundSlice(2, Open))
	got := Difference([]Box{add}, []Box{rem})
	require.False(t, Contains(got, []float64{1, 1}), "interior removed")
	require.True(t, Contains(got, []float64{0, 1}), "min face remains")
	require.True(t, Contains(got, []float64{2, 1}), "max face remains")
	require.True(t, Contains(got, []float64{1, 0}), "min face remains")
	require.True(t, Contains(got, []float64{1, 2}), "max face remains")
}

func TestPathSpanClosedFace(t *testing.T) {
	t.Parallel()
	closed := NewBoxWithBounds([]float64{0}, []float64{2}, []Bound{Closed}, []Bound{Closed})
	point := NewPath([]float64{2}, []float64{2})
	span, ok := point.Span(closed)
	require.True(t, ok)
	require.Equal(t, Span{Start: 0, End: 1}, span)

	open := NewBoxWithBounds([]float64{0}, []float64{2}, []Bound{Open}, []Bound{Open})
	_, ok = point.Span(open)
	require.False(t, ok, "the point 2 is outside the open interval")
}

func TestBoxBoundsJSONRoundTrip(t *testing.T) {
	t.Parallel()
	box := NewBoxWithBounds([]float64{0, 0}, []float64{1, 1}, []Bound{Open, Closed}, []Bound{Closed, Open})
	data, err := json.Marshal(box)
	require.NoError(t, err)
	var back Box
	require.NoError(t, json.Unmarshal(data, &back))
	require.Equal(t, box, back)

	half := NewBox([]float64{0}, []float64{1})
	data, err = json.Marshal(half)
	require.NoError(t, err)
	require.NotContains(t, string(data), "bound", "half-open boxes stay in their compact form")
	var backHalf Box
	require.NoError(t, json.Unmarshal(data, &backHalf))
	require.True(t, backHalf.Contains([]float64{0}))
	require.False(t, backHalf.Contains([]float64{1}))
}

func TestValidateBoundsLength(t *testing.T) {
	t.Parallel()
	badMin := Box{Min: []float64{0, 0}, Max: []float64{1, 1}, MinBound: []Bound{Closed}}
	require.ErrorIs(t, badMin.Validate(), ErrBounds)
	badMax := Box{Min: []float64{0, 0}, Max: []float64{1, 1}, MaxBound: []Bound{Open}}
	require.ErrorIs(t, badMax.Validate(), ErrBounds)
}

func TestEmptyInverted(t *testing.T) {
	t.Parallel()
	require.True(t, NewBox([]float64{2}, []float64{1}).Empty())
}

func TestStringBounds(t *testing.T) {
	t.Parallel()
	closed := NewBoxWithBounds([]float64{0}, []float64{2}, []Bound{Closed}, []Bound{Closed})
	require.Equal(t, "[0 2]", closed.String())
	open := NewBoxWithBounds([]float64{0}, []float64{2}, []Bound{Open}, []Bound{Open})
	require.Equal(t, "(0 2)", open.String())
}

func TestEqualDistinguishesBounds(t *testing.T) {
	t.Parallel()
	a := NewBoxWithBounds([]float64{0}, []float64{2}, []Bound{Closed}, []Bound{Closed})
	b := NewBoxWithBounds([]float64{0}, []float64{2}, []Bound{Closed}, []Bound{Open})
	require.False(t, Equal([]Box{a}, []Box{b}))
}

func TestLessOrdersByBounds(t *testing.T) {
	t.Parallel()
	halfOpen := NewBoxWithBounds([]float64{0}, []float64{2}, []Bound{Closed}, []Bound{Open})
	closed := NewBoxWithBounds([]float64{0}, []float64{2}, []Bound{Closed}, []Bound{Closed})
	require.True(t, Less(closed, halfOpen), "closed max sorts before open max")
	require.False(t, Less(halfOpen, closed))

	openMin := NewBoxWithBounds([]float64{0}, []float64{2}, []Bound{Open}, []Bound{Open})
	require.True(t, Less(halfOpen, openMin), "closed min sorts before open min")
}

func TestMergePairOtherOrder(t *testing.T) {
	t.Parallel()
	a := NewBoxWithBounds([]float64{1}, []float64{2}, []Bound{Closed}, []Bound{Closed})
	b := NewBoxWithBounds([]float64{0}, []float64{1}, []Bound{Closed}, []Bound{Closed})
	merged, ok := mergePair(a, b)
	require.True(t, ok)
	require.True(t, merged.Contains([]float64{0}))
	require.True(t, merged.Contains([]float64{2}))
}

func TestMergePairSameSpanDifferentBounds(t *testing.T) {
	t.Parallel()
	a := NewBoxWithBounds([]float64{0}, []float64{1}, []Bound{Closed}, []Bound{Closed})
	b := NewBoxWithBounds([]float64{0}, []float64{1}, []Bound{Closed}, []Bound{Open})
	_, ok := mergePair(a, b)
	require.False(t, ok, "same span with different faces cannot merge")
}
