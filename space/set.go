package space

import (
	"encoding/json"
	"sort"

	"github.com/d-led/eventfulranges/interval"
	"github.com/d-led/eventfulranges/meta"
)

// Normalize returns a canonical cover of the boxes: empty boxes and boxes
// subsumed by another are dropped, and the survivors are sorted by their
// lower corner. Coverage is preserved exactly. Metadata of a dropped box is
// merged into the box that subsumes it under the default union join.
func Normalize(boxes []Box) []Box {
	return NormalizeMerged(boxes, meta.Union)
}

// NormalizeMerged is Normalize with a custom metadata join. When a box is
// dropped because another subsumes it, its metadata is folded into the
// survivor using merge.
func NormalizeMerged(boxes []Box, merge meta.Merge) []Box {
	kept := make([]Box, 0, len(boxes))
	for _, b := range boxes {
		if b.Empty() {
			continue
		}
		subsumed := false
		for i := len(kept) - 1; i >= 0; i-- {
			switch {
			case subsumes(b, kept[i]):
				b.Meta = merge(b.Meta, kept[i].Meta)
				kept = append(kept[:i], kept[i+1:]...)
			case subsumes(kept[i], b):
				kept[i].Meta = merge(kept[i].Meta, b.Meta)
				subsumed = true
			}
		}
		if subsumed {
			continue
		}
		kept = append(kept, b)
	}
	sortBoxes(kept)
	return kept
}

// Union returns the canonical cover of the boxes of both sets.
func Union(a, b []Box) []Box {
	return UnionMerged(a, b, meta.Union)
}

// UnionMerged is Union with a custom metadata join.
func UnionMerged(a, b []Box, merge meta.Merge) []Box {
	return NormalizeMerged(append(append([]Box(nil), a...), b...), merge)
}

// Difference returns the canonical cover of every point in a that is not in
// b. The result is exact: a point belongs to it if and only if it belongs to
// a and not to b. Pieces carved out of a keep a's metadata.
func Difference(a, b []Box) []Box {
	return DifferenceMerged(a, b, meta.Union)
}

// DifferenceSorted is Difference with b already normalized (sorted and
// subsumption-free), so it skips re-normalizing b.
func DifferenceSorted(a, b []Box) []Box {
	return DifferenceSortedMerged(a, b, meta.Union)
}

// DifferenceMerged is Difference with a custom metadata join.
func DifferenceMerged(a, b []Box, merge meta.Merge) []Box {
	return DifferenceSortedMerged(a, NormalizeMerged(b, merge), merge)
}

// DifferenceSortedMerged is DifferenceMerged with b already normalized.
func DifferenceSortedMerged(a, b []Box, merge meta.Merge) []Box {
	result := NormalizeMerged(a, merge)
	for _, q := range b {
		var next []Box
		for _, p := range result {
			next = append(next, subtractBox(p, q)...)
		}
		result = NormalizeMerged(next, merge)
	}
	return result
}

// InsertNormalized returns the normalized union of a sorted, subsumption-free
// set and one box, without re-normalizing the whole set. It is the incremental
// form of Normalize(append(set, box)): boxes the new box subsumes are dropped,
// and the new box is inserted in order unless an existing box subsumes it.
// Metadata is not joined, because the geometry callers use it for does not read
// the metadata of the set.
func InsertNormalized(set []Box, box Box) []Box {
	if box.Empty() {
		return set
	}
	kept := make([]Box, 0, len(set)+1)
	subsumed := false
	for _, b := range set {
		switch {
		case subsumes(box, b):
			// box covers b: drop b.
		case subsumes(b, box):
			// b covers box: drop box, keep b.
			subsumed = true
			kept = append(kept, b)
		default:
			kept = append(kept, b)
		}
	}
	if !subsumed {
		kept = insertSortedBox(kept, box)
	}
	return kept
}

// insertSortedBox inserts b into a box-sorted slice, keeping the order.
func insertSortedBox(set []Box, b Box) []Box {
	i := sort.Search(len(set), func(i int) bool { return !Less(set[i], b) })
	set = append(set, Box{})
	copy(set[i+1:], set[i:])
	set[i] = b
	return set
}

// Contains reports whether any box contains the point p.
func Contains(set []Box, p []float64) bool {
	for _, b := range set {
		if b.Contains(p) {
			return true
		}
	}
	return false
}

// OverlapsSet reports whether any box shares a point with q.
func OverlapsSet(set []Box, q Box) bool {
	for _, b := range set {
		if Overlaps(b, q) {
			return true
		}
	}
	return false
}

// Equal reports whether two sets cover exactly the same points.
func Equal(a, b []Box) bool {
	x, y := Normalize(a), Normalize(b)
	if len(x) != len(y) {
		return false
	}
	for i := range x {
		if !equalBox(x[i], y[i]) {
			return false
		}
	}
	return true
}

// subtractBox returns the disjoint boxes covering p without q. It sweeps one
// axis at a time, keeping a worklist of pieces still inside q on the axes
// already swept: on each axis a piece is split into the parts outside q's span
// (final) and the part inside it (deferred to the next axis). The sweep is a
// bounded loop over the dimensions, never recursive.
func subtractBox(p, q Box) []Box {
	if !Overlaps(p, q) {
		return []Box{cloneBox(p)}
	}
	dims := len(p.Min)
	pending := []Box{p}
	var result []Box
	for d := 0; d < dims; d++ {
		next := make([]Box, 0, len(pending))
		for _, b := range pending {
			bLo, bHi := b.Min[d], b.Max[d]
			bLoB, bHiB := b.loBound(d), b.hiBound(d)
			qLo, qHi := q.Min[d], q.Max[d]
			qLoB, qHiB := q.loBound(d), q.hiBound(d)

			// b's span on this axis is p's span, which overlaps q (Overlaps
			// held), so either q covers it or they partially overlap.
			if intervalContains(qLo, qHi, qLoB, qHiB, bLo, bHi, bLoB, bHiB) {
				next = append(next, b)
				continue
			}
			for _, piece := range intervalSubtract(bLo, qLo, bHi, qHi, bLoB, qLoB, bHiB, qHiB) {
				result = append(result, clipToInterval(b, d, piece))
			}
			next = append(next, clipToInterval(b, d, intervalIntersection(bLo, qLo, bHi, qHi, bLoB, qLoB, bHiB, qHiB)))
		}
		pending = next
	}
	return result
}

// intervalSubtract returns the disjoint 1-D intervals covering [alo, ahi] minus
// [blo, bhi], each carrying its endpoint inclusivity. The caller guarantees the
// two intervals overlap (share a point).
func intervalSubtract(alo, blo, ahi, bhi float64, aloB, bloB, ahiB, bhiB Bound) []interval.Interval {
	var out []interval.Interval
	if alo < blo {
		out = append(out, interval.Interval{Start: alo, End: blo, StartBound: aloB, EndBound: invertBound(bloB)})
	} else if alo == blo && aloB == Closed && bloB == Open {
		out = append(out, interval.Interval{Start: alo, End: alo, StartBound: Closed, EndBound: Closed})
	}
	if ahi > bhi {
		out = append(out, interval.Interval{Start: bhi, End: ahi, StartBound: invertBound(bhiB), EndBound: ahiB})
	} else if ahi == bhi && ahiB == Closed && bhiB == Open {
		out = append(out, interval.Interval{Start: ahi, End: ahi, StartBound: Closed, EndBound: Closed})
	}
	return out
}

// intervalIntersection returns a∩b. The caller guarantees the two intervals
// overlap (share a point), so the result is non-empty.
func intervalIntersection(alo, blo, ahi, bhi float64, aloB, bloB, ahiB, bhiB Bound) interval.Interval {
	lo, loB := alo, aloB
	if blo > lo {
		lo, loB = blo, bloB
	} else if blo == lo && (aloB == Open || bloB == Open) {
		loB = Open
	}
	hi, hiB := ahi, ahiB
	if bhi < hi {
		hi, hiB = bhi, bhiB
	} else if bhi == hi && (ahiB == Open || bhiB == Open) {
		hiB = Open
	}
	return interval.Interval{Start: lo, End: hi, StartBound: loB, EndBound: hiB}
}

// invertBound flips Closed to Open and Open to Closed.
func invertBound(b Bound) Bound {
	if b == Closed {
		return Open
	}
	return Closed
}

// clipToInterval returns a copy of b with axis d clipped to iv, materializing
// the bound slices so the cut faces carry iv's inclusivity. Redundant default
// bounds are dropped back to nil, keeping half-open boxes in their compact
// canonical form.
func clipToInterval(b Box, d int, iv interval.Interval) Box {
	c := cloneBox(b)
	if c.MinBound == nil {
		c.MinBound = make([]Bound, len(c.Min))
		for i := range c.MinBound {
			c.MinBound[i] = Closed
		}
	}
	if c.MaxBound == nil {
		c.MaxBound = make([]Bound, len(c.Max))
		for i := range c.MaxBound {
			c.MaxBound[i] = Open
		}
	}
	c.Min[d] = iv.Start
	c.Max[d] = iv.End
	c.MinBound[d] = iv.StartBound
	c.MaxBound[d] = iv.EndBound
	if allClosed(c.MinBound) {
		c.MinBound = nil
	}
	if allOpen(c.MaxBound) {
		c.MaxBound = nil
	}
	return c
}

// allClosed reports whether every bound is Closed.
func allClosed(bounds []Bound) bool {
	for _, b := range bounds {
		if b != Closed {
			return false
		}
	}
	return true
}

// allOpen reports whether every bound is Open.
func allOpen(bounds []Bound) bool {
	for _, b := range bounds {
		if b != Open {
			return false
		}
	}
	return true
}

func cloneBox(b Box) Box {
	return Box{
		Min:      append([]float64(nil), b.Min...),
		Max:      append([]float64(nil), b.Max...),
		MinBound: append([]Bound(nil), b.MinBound...),
		MaxBound: append([]Bound(nil), b.MaxBound...),
		Meta:     append(json.RawMessage(nil), b.Meta...),
	}
}

func equalBox(a, b Box) bool {
	for i := range a.Min {
		if a.Min[i] != b.Min[i] || a.Max[i] != b.Max[i] {
			return false
		}
		if a.loBound(i) != b.loBound(i) || a.hiBound(i) != b.hiBound(i) {
			return false
		}
	}
	return true
}

func sortBoxes(boxes []Box) {
	sort.Slice(boxes, func(i, j int) bool {
		return Less(boxes[i], boxes[j])
	})
}

// Less reports whether a sorts before b in canonical cover order: by lower
// corner first, then by upper corner, then by face inclusivity, comparing
// coordinate by coordinate.
func Less(a, b Box) bool {
	for i := range a.Min {
		if a.Min[i] != b.Min[i] {
			return a.Min[i] < b.Min[i]
		}
		if a.Max[i] != b.Max[i] {
			return a.Max[i] < b.Max[i]
		}
		if a.loBound(i) != b.loBound(i) {
			return a.loBound(i) < b.loBound(i)
		}
		if a.hiBound(i) != b.hiBound(i) {
			return a.hiBound(i) < b.hiBound(i)
		}
	}
	return false
}
