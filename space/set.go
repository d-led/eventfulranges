package space

import (
	"encoding/json"
	"sort"

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

// DifferenceMerged is Difference with a custom metadata join.
func DifferenceMerged(a, b []Box, merge meta.Merge) []Box {
	result := NormalizeMerged(a, merge)
	for _, q := range NormalizeMerged(b, merge) {
		var next []Box
		for _, p := range result {
			next = append(next, subtractBox(p, q)...)
		}
		result = NormalizeMerged(next, merge)
	}
	return result
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

// subtractBox returns the disjoint boxes covering p without q.
func subtractBox(p, q Box) []Box {
	if !Overlaps(p, q) {
		return []Box{cloneBox(p)}
	}
	for d := range p.Min {
		lo := max(p.Min[d], q.Min[d])
		hi := min(p.Max[d], q.Max[d])
		if lo == p.Min[d] && hi == p.Max[d] {
			continue // q covers p fully in this dimension; try the next
		}
		return subtractAlongDimension(p, q, d, lo, hi)
	}
	// q covers p in every dimension: nothing remains.
	return nil
}

// subtractAlongDimension slices p around q's span in dimension d, then
// subtracts q from the middle part that still overlaps it.
func subtractAlongDimension(p, q Box, d int, lo, hi float64) []Box {
	out := sliceOutside(p, q, d)
	mid := cloneBox(p)
	mid.Min[d] = lo
	mid.Max[d] = hi
	return append(out, subtractBox(mid, q)...)
}

// sliceOutside returns the parts of p that lie strictly outside q in
// dimension d.
func sliceOutside(p, q Box, d int) []Box {
	var out []Box
	if q.Min[d] > p.Min[d] {
		left := cloneBox(p)
		left.Max[d] = q.Min[d]
		out = append(out, left)
	}
	if q.Max[d] < p.Max[d] {
		right := cloneBox(p)
		right.Min[d] = q.Max[d]
		out = append(out, right)
	}
	return out
}

func cloneBox(b Box) Box {
	return Box{
		Min:  append([]float64(nil), b.Min...),
		Max:  append([]float64(nil), b.Max...),
		Meta: append(json.RawMessage(nil), b.Meta...),
	}
}

func equalBox(a, b Box) bool {
	for i := range a.Min {
		if a.Min[i] != b.Min[i] || a.Max[i] != b.Max[i] {
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
// corner first, then by upper corner, comparing coordinate by coordinate.
func Less(a, b Box) bool {
	for i := range a.Min {
		if a.Min[i] != b.Min[i] {
			return a.Min[i] < b.Min[i]
		}
		if a.Max[i] != b.Max[i] {
			return a.Max[i] < b.Max[i]
		}
	}
	return false
}
