package space

import "sort"

// Normalize returns a canonical cover of the boxes: empty boxes and boxes
// subsumed by another are dropped, and the survivors are sorted by their
// lower corner. Coverage is preserved exactly. All boxes must share the same
// dimensionality.
func Normalize(boxes []Box) []Box {
	kept := make([]Box, 0, len(boxes))
	for _, b := range boxes {
		if b.Empty() {
			continue
		}
		for i := len(kept) - 1; i >= 0; i-- {
			if subsumes(b, kept[i]) {
				kept = append(kept[:i], kept[i+1:]...)
			}
		}
		if subsumedByAny(kept, b) {
			continue
		}
		kept = append(kept, b)
	}
	sortBoxes(kept)
	return kept
}

// Union returns the canonical cover of the boxes of both sets.
func Union(a, b []Box) []Box {
	return Normalize(append(append([]Box(nil), a...), b...))
}

// Difference returns the canonical cover of every point in a that is not in
// b. The result is exact: a point belongs to it if and only if it belongs to
// a and not to b.
func Difference(a, b []Box) []Box {
	result := Normalize(a)
	for _, q := range Normalize(b) {
		var next []Box
		for _, p := range result {
			next = append(next, subtractBox(p, q)...)
		}
		result = Normalize(next)
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

func subsumedByAny(set []Box, b Box) bool {
	for _, o := range set {
		if subsumes(o, b) {
			return true
		}
	}
	return false
}

func cloneBox(b Box) Box {
	return Box{Min: append([]float64(nil), b.Min...), Max: append([]float64(nil), b.Max...)}
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
		return lessBox(boxes[i], boxes[j])
	})
}

func lessBox(a, b Box) bool {
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
