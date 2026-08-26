package space

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPathValidate(t *testing.T) {
	t.Parallel()
	require.NoError(t, NewPath([]float64{0, 0}, []float64{1, 1}).Validate())
	// A zero-length path is a valid point query.
	require.NoError(t, NewPath([]float64{2, 2}, []float64{2, 2}).Validate())

	require.ErrorIs(t, NewPath([]float64{0}, []float64{1, 1}).Validate(), ErrDimensions)
	require.ErrorIs(t, NewPath(nil, nil).Validate(), ErrNoDimensions)
	require.ErrorIs(t, NewPath([]float64{math.NaN()}, []float64{1}).Validate(), ErrNaN)
	require.ErrorIs(t, NewPath([]float64{0}, []float64{math.Inf(1)}).Validate(), ErrInfinite)
}

func TestPathPoint(t *testing.T) {
	t.Parallel()
	p := NewPath([]float64{0, 10}, []float64{4, -10})
	require.Equal(t, []float64{0, 10}, p.Point(0))
	require.Equal(t, []float64{4, -10}, p.Point(1))
	require.Equal(t, []float64{2, 0}, p.Point(0.5))
}

func TestPathSpanDiagonal(t *testing.T) {
	t.Parallel()
	// The main diagonal of a square stays inside the square for its whole run.
	span, ok := NewPath([]float64{0, 0}, []float64{4, 4}).Span(box2(0, 0, 4, 4))
	require.True(t, ok)
	require.Equal(t, Span{Start: 0, End: 1}, span)

	// The diagonal through the inner box enters and leaves at a quarter run.
	span, ok = NewPath([]float64{0, 0}, []float64{4, 4}).Span(box2(1, 1, 3, 3))
	require.True(t, ok)
	require.Equal(t, Span{Start: 0.25, End: 0.75}, span)
}

func TestPathSpanReverseDirection(t *testing.T) {
	t.Parallel()
	// Reversing the path leaves the interior interval unchanged.
	span, ok := NewPath([]float64{4, 4}, []float64{0, 0}).Span(box2(1, 1, 3, 3))
	require.True(t, ok)
	require.Equal(t, Span{Start: 0.25, End: 0.75}, span)
}

func TestPathSpanAxisAligned(t *testing.T) {
	t.Parallel()
	// Along the x-axis, a vertical slice enters immediately and exits halfway.
	span, ok := NewPath([]float64{0, 0}, []float64{4, 0}).Span(box2(0, 0, 2, 2))
	require.True(t, ok)
	require.Equal(t, Span{Start: 0, End: 0.5}, span)

	// A vertical path through the middle of a box spans the whole run.
	span, ok = NewPath([]float64{2, 0}, []float64{2, 4}).Span(box2(0, 0, 4, 4))
	require.True(t, ok)
	require.Equal(t, Span{Start: 0, End: 1}, span)
}

func TestPathSpanDoesNotCrossOutside(t *testing.T) {
	t.Parallel()
	_, ok := NewPath([]float64{0, 0}, []float64{1, 1}).Span(box2(10, 10, 20, 20))
	require.False(t, ok)
}

func TestPathSpanGrazesEdgeOrCorner(t *testing.T) {
	t.Parallel()
	// Running along a box's Max face is outside it (half-open).
	_, ok := NewPath([]float64{1, 0}, []float64{1, 1}).Span(box2(0, 0, 1, 1))
	require.False(t, ok)

	// Touching a corner is a single point, which has no positive length.
	_, ok = NewPath([]float64{1, 1}, []float64{2, 2}).Span(box2(0, 0, 1, 1))
	require.False(t, ok)
}

func TestPathSpanPointPath(t *testing.T) {
	t.Parallel()
	// A stationary point inside the box is inside for the whole parameter run.
	span, ok := NewPath([]float64{2, 2}, []float64{2, 2}).Span(box2(0, 0, 4, 4))
	require.True(t, ok)
	require.Equal(t, Span{Start: 0, End: 1}, span)

	// A stationary point on the Max face is outside.
	_, ok = NewPath([]float64{4, 4}, []float64{4, 4}).Span(box2(0, 0, 4, 4))
	require.False(t, ok)
}

func TestPathSpanWrongDimension(t *testing.T) {
	t.Parallel()
	_, ok := NewPath([]float64{0}, []float64{1}).Span(box2(0, 0, 4, 4))
	require.False(t, ok)
}

func TestCrossed(t *testing.T) {
	t.Parallel()
	boxes := []Box{box2(0, 0, 2, 2), box2(10, 10, 20, 20)}
	p := NewPath([]float64{0, 0}, []float64{1, 1})

	got := Crossed(boxes, p)
	require.Equal(t, []Box{box2(0, 0, 2, 2)}, got)
}

func TestCrossedIgnoresGrazing(t *testing.T) {
	t.Parallel()
	boxes := []Box{box2(0, 0, 1, 1)}
	p := NewPath([]float64{1, 1}, []float64{2, 2})
	require.Empty(t, Crossed(boxes, p))
}

func TestTraverseInAndOut(t *testing.T) {
	t.Parallel()
	// A 1D path through two ranges with a gap between them: enter, exit into
	// empty space, enter again, exit again.
	boxes := []Box{NewBox([]float64{0}, []float64{10}), NewBox([]float64{20}, []float64{30})}
	p := NewPath([]float64{0}, []float64{40})

	segs := Traverse(boxes, p)
	require.Equal(t, []PathSegment{
		{Start: 0, End: 0.25, Covered: true, Boxes: []Box{NewBox([]float64{0}, []float64{10})}},
		{Start: 0.25, End: 0.5, Covered: false},
		{Start: 0.5, End: 0.75, Covered: true, Boxes: []Box{NewBox([]float64{20}, []float64{30})}},
		{Start: 0.75, End: 1, Covered: false},
	}, segs)
}

func TestTraverseOverlappingBoxes(t *testing.T) {
	t.Parallel()
	// Two overlapping ranges list both boxes on the shared interval.
	boxes := []Box{NewBox([]float64{0}, []float64{10}), NewBox([]float64{5}, []float64{15})}
	p := NewPath([]float64{0}, []float64{20})

	segs := Traverse(boxes, p)
	require.Equal(t, []PathSegment{
		{Start: 0, End: 0.25, Covered: true, Boxes: []Box{NewBox([]float64{0}, []float64{10})}},
		{Start: 0.25, End: 0.5, Covered: true, Boxes: []Box{
			NewBox([]float64{0}, []float64{10}),
			NewBox([]float64{5}, []float64{15}),
		}},
		{Start: 0.5, End: 0.75, Covered: true, Boxes: []Box{NewBox([]float64{5}, []float64{15})}},
		{Start: 0.75, End: 1, Covered: false},
	}, segs)
}

func TestTraverseTouchingBoxesHaveNoGap(t *testing.T) {
	t.Parallel()
	// Half-open boxes touching at a boundary leave no gap in the traversal.
	boxes := []Box{NewBox([]float64{0}, []float64{10}), NewBox([]float64{10}, []float64{20})}
	p := NewPath([]float64{0}, []float64{20})

	segs := Traverse(boxes, p)
	require.Equal(t, []PathSegment{
		{Start: 0, End: 0.5, Covered: true, Boxes: []Box{NewBox([]float64{0}, []float64{10})}},
		{Start: 0.5, End: 1, Covered: true, Boxes: []Box{NewBox([]float64{10}, []float64{20})}},
	}, segs)
}

func TestTraverseEmptyCoverIsOneGap(t *testing.T) {
	t.Parallel()
	p := NewPath([]float64{0, 0}, []float64{1, 1})
	require.Equal(t, []PathSegment{{Start: 0, End: 1, Covered: false}}, Traverse(nil, p))
}

func TestTraverseIsDeterministic(t *testing.T) {
	t.Parallel()
	boxes := []Box{box2(0, 0, 2, 2), box2(1, 1, 3, 3)}
	p := NewPath([]float64{0, 0}, []float64{3, 3})
	require.Equal(t, Traverse(boxes, p), Traverse(boxes, p))
}
