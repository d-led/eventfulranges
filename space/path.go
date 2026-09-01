package space

import (
	"math"
	"sort"
)

// Path is a straight segment through n dimensions, parameterized by t in
// [0, 1]: P(t) = From + t·(To − From). It is the query shape for region
// queries: it lets a caller ask which boxes a segment crosses, in order,
// including the empty gaps between them.
//
// Intersections use the same half-open convention as Box: a point on a box's
// Max face belongs to the neighboring box, not this one. A path that grazes
// a face or runs along an edge therefore resolves deterministically to the
// adjacent box, without simulation of simplicity.
type Path struct {
	From []float64 `json:"from"`
	To   []float64 `json:"to"`
}

// NewPath builds a path from the given endpoints. Both slices are copied, so
// the caller keeps ownership of the originals.
func NewPath(from, to []float64) Path {
	return Path{From: append([]float64(nil), from...), To: append([]float64(nil), to...)}
}

// Dims returns the number of dimensions of the path.
func (p Path) Dims() int {
	return len(p.From)
}

// Validate reports whether the path is well-formed. A zero-length path (From
// equal to To) is valid and behaves as a point query.
func (p Path) Validate() error {
	if len(p.From) != len(p.To) {
		return ErrDimensions
	}
	if len(p.From) == 0 {
		return ErrNoDimensions
	}
	for i := range p.From {
		for _, v := range []float64{p.From[i], p.To[i]} {
			switch {
			case math.IsNaN(v):
				return ErrNaN
			case math.IsInf(v, 0):
				return ErrInfinite
			}
		}
	}
	return nil
}

// Point returns P(t): the position along the path at parameter t.
func (p Path) Point(t float64) []float64 {
	pt := make([]float64, len(p.From))
	for i := range p.From {
		pt[i] = p.From[i] + t*(p.To[i]-p.From[i])
	}
	return pt
}

// Span is a half-open interval [Start, End) of the path's parameter t.
type Span struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

// Empty reports whether the span has no positive length.
func (s Span) Empty() bool {
	return s.Start >= s.End
}

// Span returns the interval of t for which the path lies inside b, clipped to
// [0, 1]. ok is false when the path never lies inside b with positive length:
// a path that merely grazes a face, runs along an edge, or has a mismatched
// dimension does not intersect.
//
// Each dimension contributes a half-open t-interval and the result is their
// intersection, computed with the slab method. The arithmetic here is the one
// place the box algebra touches floating-point division; endpoints are still
// only ever compared, never combined.
func (p Path) Span(b Box) (Span, bool) {
	if len(b.Min) != len(p.From) {
		return Span{}, false
	}
	start, end := 0.0, 1.0
	for i := range b.Min {
		entry, exit, ok := entryExit(p.From[i], p.To[i], b.Min[i], b.Max[i])
		if !ok {
			return Span{}, false
		}
		if entry > start {
			start = entry
		}
		if exit < end {
			end = exit
		}
	}
	if start >= end {
		return Span{}, false
	}
	return Span{Start: start, End: end}, true
}

// entryExit returns the half-open t-interval [entry, exit) where the path
// coordinate from → to lies inside the box coordinate [lo, hi). ok is false
// when a constant coordinate lies outside the half-open interval; a constant
// coordinate inside contributes the full [0, 1) range, imposing no
// constraint on t.
func entryExit(from, to, lo, hi float64) (entry, exit float64, ok bool) {
	d := to - from
	if d == 0 {
		return 0, 1, from >= lo && from < hi
	}
	if d > 0 {
		return (lo - from) / d, (hi - from) / d, true // enters at lo, exits at hi
	}
	return (hi - from) / d, (lo - from) / d, true // enters at hi, exits at lo
}

// Crossed returns the boxes the path crosses with positive length, in
// canonical cover order. The input is canonicalized first, so empty and
// subsumed boxes are dropped and the answer reflects the cover rather than
// the raw list. A box grazed along a face or edge is not crossed.
func Crossed(boxes []Box, p Path) []Box {
	boxes = Normalize(boxes)
	out := make([]Box, 0, len(boxes))
	for _, b := range boxes {
		if _, ok := p.Span(b); ok {
			out = append(out, b)
		}
	}
	return out
}

// PathSegment is one contiguous piece of a traversal: a t-interval of
// constant coverage. Covered segments list the boxes covering them; gap
// segments are empty space with no boxes.
type PathSegment struct {
	Start   float64 `json:"start"`
	End     float64 `json:"end"`
	Covered bool    `json:"covered"`
	Boxes   []Box   `json:"boxes,omitempty"`
}

// Traverse partitions the path's parameter interval [0, 1] into maximal
// segments of constant coverage, in the order the path passes through them.
// Covered and gap segments alternate as the path enters and leaves ranges:
// entered a box, exited into empty space, entered the next, and so on.
//
// The input is canonicalized first, so the answer reflects the cover. The
// result is deterministic and tiles [0, 1] exactly: the first segment starts
// at 0, the last ends at 1, and each segment's End is the next segment's
// Start.
func Traverse(boxes []Box, p Path) []PathSegment {
	crossings := collectCrossings(Normalize(boxes), p)
	if len(crossings) == 0 {
		return []PathSegment{{Start: 0, End: 1, Covered: false}}
	}

	// Every span boundary is a breakpoint; coverage is constant between
	// consecutive breakpoints, so each open sub-interval belongs to exactly
	// the boxes whose span contains it.
	breakpoints := uniqueBreakpoints(crossings)
	segs := make([]PathSegment, 0, len(breakpoints)-1)
	for i := 0; i < len(breakpoints)-1; i++ {
		start, end := breakpoints[i], breakpoints[i+1]
		covering := boxesCovering(crossings, start, end)
		if len(covering) == 0 {
			segs = append(segs, PathSegment{Start: start, End: end, Covered: false})
			continue
		}
		segs = append(segs, PathSegment{Start: start, End: end, Covered: true, Boxes: covering})
	}
	return segs
}

// pathCrossing is one box's half-open t-interval along the path.
type pathCrossing struct {
	box  Box
	span Span
}

// collectCrossings returns the crossing of every box the path enters with
// positive length.
func collectCrossings(boxes []Box, p Path) []pathCrossing {
	crossings := make([]pathCrossing, 0, len(boxes))
	for _, b := range boxes {
		if s, ok := p.Span(b); ok {
			crossings = append(crossings, pathCrossing{box: b, span: s})
		}
	}
	return crossings
}

// uniqueBreakpoints returns the sorted, deduplicated boundaries of every
// crossing, bookended by the path's parameter range [0, 1].
func uniqueBreakpoints(crossings []pathCrossing) []float64 {
	breakpoints := make([]float64, 0, 2*len(crossings)+2)
	breakpoints = append(breakpoints, 0, 1)
	for _, c := range crossings {
		breakpoints = append(breakpoints, c.span.Start, c.span.End)
	}
	sort.Float64s(breakpoints)
	unique := breakpoints[:1]
	for _, t := range breakpoints[1:] {
		if t != unique[len(unique)-1] {
			unique = append(unique, t)
		}
	}
	return unique
}

// boxesCovering returns the boxes whose span contains the whole sub-interval
// [start, end), in canonical order.
func boxesCovering(crossings []pathCrossing, start, end float64) []Box {
	var covering []Box
	for _, c := range crossings {
		if c.span.Start <= start && c.span.End >= end {
			covering = append(covering, c.box)
		}
	}
	sortBoxes(covering)
	return covering
}
