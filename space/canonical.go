package space

// Canonicalizer canonicalizes a cover of boxes into a deterministic
// representation that preserves the covered point set exactly. It is the
// compaction seam for the n-dimensional engine: the library's default keeps
// partially-overlapping boxes, while a custom Canonicalizer may subdivide
// them, apply simulation of simplicity to break boundary ties, or index them
// spatially.
//
// A Canonicalizer must be cover-preserving — a point is covered by the output
// exactly when it is covered by the input — and deterministic: the same input
// yields the same output. Both are required so that replicas which have seen
// the same operations converge to the same canonicalized view.
type Canonicalizer func([]Box) []Box

// Chain composes canonicalizers, applying them left to right. Nil
// canonicalizers are skipped, so a pipeline can be assembled without
// branching.
func Chain(cs ...Canonicalizer) Canonicalizer {
	return func(boxes []Box) []Box {
		for _, c := range cs {
			if c != nil {
				boxes = c(boxes)
			}
		}
		return boxes
	}
}

// MergeAdjacent returns a cover in which every pair of boxes that touch along
// a full edge and agree in every other dimension has been merged into one
// box. It is a deterministic, cover-preserving greedy fixpoint — not a
// provably-minimal rectangle cover — so it is safe to use as a Canonicalizer.
//
// The greedy takes the first mergeable pair in canonical cover order and
// repeats. The merger in merge_index.go picks that same pair through an index
// of candidates instead of testing every pair, so this returns the cover the
// straightforward definition returns, at a cost that stays near linear instead
// of cubic. That matters because a partition of a few dozen overlapping boxes
// is thousands of cells, each of them a box.
func MergeAdjacent(boxes []Box) []Box {
	boxes = Normalize(boxes)
	if len(boxes) == 0 {
		return boxes
	}
	m := newMerger(boxes)
	for {
		a, b, ok := m.firstMergeablePair()
		if !ok {
			return m.result()
		}
		m.merge(a, b)
	}
}

// mergePair merges two boxes that differ in exactly one dimension where they
// touch edge-to-edge, reporting whether such a merge is possible. It assumes
// the boxes are well-formed and of the same dimensionality.
func mergePair(a, b Box) (Box, bool) {
	if len(a.Min) != len(b.Min) {
		return Box{}, false
	}
	diff, ok := singleDiffAxis(a, b)
	if !ok {
		return Box{}, false
	}
	lo, hi := a, b
	switch {
	case b.Max[diff] == a.Min[diff]:
		lo, hi = b, a
	case a.Max[diff] != b.Min[diff]:
		return Box{}, false // gap or overlap: only touching boxes merge
	}
	// The shared face becomes interior in the merged box. If both sides
	// exclude it, the union has a pinhole there and a single box cannot
	// represent it; require at least one side closed.
	if lo.hiBound(diff) == Open && hi.loBound(diff) == Open {
		return Box{}, false
	}
	return Box{
		Min:      append([]float64(nil), lo.Min...),
		Max:      append([]float64(nil), hi.Max...),
		MinBound: append([]Bound(nil), lo.MinBound...),
		MaxBound: append([]Bound(nil), hi.MaxBound...),
	}, true
}

// singleDiffAxis finds the one axis where a and b differ, reporting whether
// they differ in exactly one axis and agree (including face inclusivity) on
// every other axis.
func singleDiffAxis(a, b Box) (int, bool) {
	diff := -1
	for d := range a.Min {
		if a.Min[d] == b.Min[d] && a.Max[d] == b.Max[d] {
			if a.loBound(d) != b.loBound(d) || a.hiBound(d) != b.hiBound(d) {
				return -1, false // same span but different faces
			}
			continue
		}
		if diff != -1 {
			return -1, false // differ in more than one dimension
		}
		diff = d
	}
	return diff, diff != -1
}
