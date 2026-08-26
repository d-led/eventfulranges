package space

import (
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// genPath draws a 2D path with integer endpoints on the -3..3 grid.
func genPath(t *rapid.T) *rapid.Generator[Path] {
	return rapid.Custom(func(t *rapid.T) Path {
		return NewPath(
			[]float64{
				float64(rapid.IntRange(-3, 3).Draw(t, "fx")),
				float64(rapid.IntRange(-3, 3).Draw(t, "fy")),
			},
			[]float64{
				float64(rapid.IntRange(-3, 3).Draw(t, "tx")),
				float64(rapid.IntRange(-3, 3).Draw(t, "ty")),
			},
		)
	})
}

// genericTs samples the path parameter at dyadic points that never coincide
// with an integer-coordinate box boundary: (2k+1)/2048 has denominator a
// power of two, while any boundary of integer boxes and paths has denominator
// at most 6. Every membership comparison is therefore exact.
func genericTs() []float64 {
	ts := make([]float64, 0, 1024)
	for k := 0; k < 1024; k++ {
		ts = append(ts, float64(2*k+1)/2048)
	}
	return ts
}

func containsBox(boxes []Box, b Box) bool {
	for _, x := range boxes {
		if equalBox(x, b) {
			return true
		}
	}
	return false
}

func TestPropertySpanAgreesWithMembership(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		b := genBox(t).Draw(t, "box")
		p := genPath(t).Draw(t, "path")
		require.NoError(t, p.Validate())

		span, ok := p.Span(b)
		require.Equal(t, !span.Empty(), ok, "ok must mean positive length: %+v", span)

		for _, tv := range genericTs() {
			inside := b.Contains(p.Point(tv))
			want := ok && tv >= span.Start && tv < span.End
			if inside != want {
				t.Fatalf("span disagrees with membership\n"+
					"box  %v\n"+
					"path %v -> %v\n"+
					"t    %v\n"+
					"  inside: %t\n"+
					"  span:   %+v (ok=%t)",
					b, p.From, p.To, tv, inside, span, ok)
			}
		}
	})
}

func TestPropertyTraversePartitions(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		raw := rapid.SliceOfN(genBox(t), 0, 6).Draw(t, "raw")
		p := genPath(t).Draw(t, "path")
		segs := Traverse(raw, p)
		requireSegmentsTile(t, segs)
		requireBoxesListedConsistently(t, raw, p, segs)
		requireCrossedMatchesSegments(t, raw, p, segs)
	})
}

// requireSegmentsTile asserts the segments tile [0, 1] exactly: no overlaps,
// no gaps, and each segment's coverage agrees with its box list.
func requireSegmentsTile(t *rapid.T, segs []PathSegment) {
	t.Helper()
	if len(segs) == 0 {
		t.Fatal("traversal must never be empty")
	}
	if segs[0].Start != 0 || segs[len(segs)-1].End != 1 {
		t.Fatalf("traversal must tile [0,1]: first=%+v last=%+v", segs[0], segs[len(segs)-1])
	}
	for i, s := range segs {
		if s.Start >= s.End {
			t.Fatalf("segment %d has no positive length: %+v", i, s)
		}
		if s.Covered != (len(s.Boxes) > 0) {
			t.Fatalf("segment %d: covered and boxes must agree: %+v", i, s)
		}
		if i > 0 && segs[i-1].End != s.Start {
			t.Fatalf("segment %d must abut its predecessor: %+v then %+v", i, segs[i-1], s)
		}
	}
}

// requireBoxesListedConsistently asserts a crossed box is listed in exactly the
// segments overlapping its span — a box can appear in several abutting
// segments when another box's boundary splits its span.
func requireBoxesListedConsistently(t *rapid.T, raw []Box, p Path, segs []PathSegment) {
	t.Helper()
	for _, b := range Normalize(raw) {
		span, ok := p.Span(b)
		if !ok {
			continue
		}
		for _, s := range segs {
			overlaps := s.Start < span.End && s.End > span.Start
			listed := containsBox(s.Boxes, b)
			if overlaps != listed {
				t.Fatalf("box %v span %+v vs segment %+v: listed=%t overlaps=%t", b, span, s, listed, overlaps)
			}
			if listed && !s.Covered {
				t.Fatalf("box %v listed in a gap segment", b)
			}
		}
	}
}

// requireCrossedMatchesSegments asserts Crossed is exactly the set of boxes
// listed by the covered segments, with duplicates collapsed.
func requireCrossedMatchesSegments(t *rapid.T, raw []Box, p Path, segs []PathSegment) {
	t.Helper()
	crossed := Crossed(raw, p)
	var seen []Box
	for _, s := range segs {
		for _, b := range s.Boxes {
			if !containsBox(seen, b) {
				seen = append(seen, b)
			}
		}
	}
	sortBoxes(crossed)
	sortBoxes(seen)
	if len(crossed) != len(seen) {
		t.Fatalf("crossed: %s seen: %s", renderBoxes(crossed), renderBoxes(seen))
	}
	for i := range crossed {
		if !equalBox(crossed[i], seen[i]) {
			t.Fatalf("crossed and seen disagree at %d: %v vs %v", i, crossed[i], seen[i])
		}
	}
}
