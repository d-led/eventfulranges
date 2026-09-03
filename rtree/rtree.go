// Package rtree provides a bulk-loaded R-tree over axis-aligned boxes.
//
// A Tree is an ephemeral, read-only spatial index: it is built once from a set
// of boxes — for example a shared board's materialized view — and then
// queried. It carries no identity, no events and no mutation, so it is safe to
// share across goroutines and safe to rebuild and discard at will, exactly
// like the demo's other display-oriented views of the canonical cover. Build
// is a pure function of its input, so two boards that converged to the same
// boxes build the same tree; there is nothing to lock and no race to win.
//
// Construction uses a median split, a well-known static packing for R-trees:
// each internal node splits its boxes on the axis with the widest span, at the
// median, so the tree is balanced. Every node stores the smallest box that
// covers its children, and Search descends a node only when that box overlaps
// the query, turning the O(n) scan of a box set into O(log n + k) in the
// typical case.
package rtree

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/d-led/eventfulranges/space"
)

// capacity is the maximum number of boxes in a leaf. Larger values make a
// shallower tree with more per-node work; smaller values the opposite.
const capacity = 8

// node is one node of the tree. A leaf stores boxes; an internal node stores
// children and an mbr covering them all.
type node struct {
	mbr      space.Box
	boxes    []space.Box // leaf only
	children []*node     // internal only
	leaf     bool
}

// Tree is a static, read-only R-tree. The zero value is an empty tree.
type Tree struct {
	root *node
	n    int
	dims int
}

// Build bulk-loads an R-tree over boxes. Empty boxes are dropped and the
// survivors are copied, so later mutation of the input does not affect the
// tree. Every surviving box must be valid and of one shared dimensionality.
func Build(boxes []space.Box) (*Tree, error) {
	return build(boxes, true)
}

// BuildRef is Build without copying the boxes: the tree stores the caller's
// boxes, so the input must not be mutated while the tree is queried. It is
// the memory-conscious form for callers that can guarantee the boxes outlive
// the tree unchanged, and drops the per-box copy that dominates the index's
// footprint. Search still returns copies, so results stay safe to mutate.
func BuildRef(boxes []space.Box) (*Tree, error) {
	return build(boxes, false)
}

// build bulk-loads a tree, deep-copying the boxes only when copyBoxes is set.
func build(boxes []space.Box, copyBoxes bool) (*Tree, error) {
	dims := 0
	kept := make([]space.Box, 0, len(boxes))
	for _, b := range boxes {
		if b.Empty() {
			continue
		}
		if err := b.Validate(); err != nil {
			return nil, err
		}
		if dims == 0 {
			dims = len(b.Min)
		} else if len(b.Min) != dims {
			return nil, fmt.Errorf("rtree: box has %d dimensions, expected %d", len(b.Min), dims)
		}
		if copyBoxes {
			b = cloneBox(b)
		}
		kept = append(kept, b)
	}
	if len(kept) == 0 {
		return &Tree{}, nil
	}
	return &Tree{root: buildNode(kept), n: len(kept), dims: dims}, nil
}

// Len reports how many boxes the tree indexes.
func (t *Tree) Len() int { return t.n }

// Dims reports the dimensionality of the indexed boxes, or 0 when empty.
func (t *Tree) Dims() int { return t.dims }

// Search returns every indexed box that shares a point with q, in canonical
// order. The boxes are copies, so callers cannot mutate the tree through
// them. An empty tree, an empty query, or a query of a different
// dimensionality matches nothing.
func (t *Tree) Search(q space.Box) []space.Box {
	refs := t.searchInto(q, nil)
	if len(refs) == 0 {
		return nil
	}
	sort.Slice(refs, func(i, j int) bool { return lessBox(refs[i], refs[j]) })
	out := make([]space.Box, len(refs))
	for i, b := range refs {
		out[i] = cloneBox(b)
	}
	return out
}

// SearchRef is Search without the copy and the canonical sort: it returns the
// matching boxes in traversal order, as references into the tree. It is the
// fast path for callers that only read the results and do not care about their
// order. The order is deterministic for a given tree, but not sorted, and the
// returned boxes must not be mutated.
func (t *Tree) SearchRef(q space.Box) []space.Box {
	return t.searchInto(q, nil)
}

// searchInto appends the boxes overlapping q, as references, in traversal
// order, to dst.
func (t *Tree) searchInto(q space.Box, dst []space.Box) []space.Box {
	if t.root == nil || q.Empty() || len(q.Min) != t.dims {
		return dst
	}
	searchNode(t.root, q, &dst)
	return dst
}

// buildNode recursively packs boxes into a balanced tree.
func buildNode(boxes []space.Box) *node {
	if len(boxes) <= capacity {
		return &node{leaf: true, boxes: boxes, mbr: mbrOf(boxes)}
	}
	axis := widestAxis(boxes)
	sort.Slice(boxes, func(i, j int) bool { return lessOn(boxes[i], boxes[j], axis) })
	mid := len(boxes) / 2
	left := buildNode(boxes[:mid])
	right := buildNode(boxes[mid:])
	return &node{
		leaf:     false,
		children: []*node{left, right},
		mbr:      mbrOf([]space.Box{left.mbr, right.mbr}),
	}
}

// searchNode descends only into nodes whose mbr overlaps the query.
func searchNode(n *node, q space.Box, out *[]space.Box) {
	if !space.Overlaps(n.mbr, q) {
		return
	}
	if n.leaf {
		for _, b := range n.boxes {
			if space.Overlaps(b, q) {
				*out = append(*out, b)
			}
		}
		return
	}
	for _, c := range n.children {
		searchNode(c, q, out)
	}
}

// widestAxis returns the axis along which boxes span the largest range, so the
// split separates the most spread-out data. Ties keep the earlier axis.
func widestAxis(boxes []space.Box) int {
	axis, width := 0, 0.0
	for d := range boxes[0].Min {
		lo, hi := boxes[0].Min[d], boxes[0].Max[d]
		for _, b := range boxes[1:] {
			if b.Min[d] < lo {
				lo = b.Min[d]
			}
			if b.Max[d] > hi {
				hi = b.Max[d]
			}
		}
		if hi-lo > width {
			axis, width = d, hi-lo
		}
	}
	return axis
}

// lessOn orders boxes along one axis for the median split, falling back to the
// canonical order so the sort is deterministic even on ties.
func lessOn(a, b space.Box, axis int) bool {
	if a.Min[axis] != b.Min[axis] {
		return a.Min[axis] < b.Min[axis]
	}
	if a.Max[axis] != b.Max[axis] {
		return a.Max[axis] < b.Max[axis]
	}
	return lessBox(a, b)
}

// lessBox is a strict total order over boxes: lower corner, then upper corner,
// then face bounds, then metadata. Search sorts its result with it so the
// output is canonical regardless of the tree's construction order.
func lessBox(a, b space.Box) bool {
	for d := range a.Min {
		if a.Min[d] != b.Min[d] {
			return a.Min[d] < b.Min[d]
		}
	}
	for d := range a.Max {
		if a.Max[d] != b.Max[d] {
			return a.Max[d] < b.Max[d]
		}
	}
	for d := range a.Min {
		if la, lb := loBound(a, d), loBound(b, d); la != lb {
			return la < lb
		}
		if ha, hb := hiBound(a, d), hiBound(b, d); ha != hb {
			return ha < hb
		}
	}
	return bytes.Compare(a.Meta, b.Meta) < 0
}

// mbrOf returns the smallest box covering boxes, closed on every face. A child
// with a closed face must not escape its parent's bound, or Search could
// wrongly prune the child; a fully closed bound is always a safe superset, at
// worst causing a little extra descent.
func mbrOf(boxes []space.Box) space.Box {
	mn := append([]float64(nil), boxes[0].Min...)
	mx := append([]float64(nil), boxes[0].Max...)
	for _, b := range boxes[1:] {
		for d := range mn {
			if b.Min[d] < mn[d] {
				mn[d] = b.Min[d]
			}
			if b.Max[d] > mx[d] {
				mx[d] = b.Max[d]
			}
		}
	}
	loB := make([]space.Bound, len(mn))
	hiB := make([]space.Bound, len(mn))
	for i := range loB {
		loB[i], hiB[i] = space.Closed, space.Closed
	}
	return space.NewBoxWithBounds(mn, mx, loB, hiB)
}

// cloneBox deep-copies a box so the tree owns its geometry.
func cloneBox(b space.Box) space.Box {
	return space.Box{
		Min:      append([]float64(nil), b.Min...),
		Max:      append([]float64(nil), b.Max...),
		MinBound: append([]space.Bound(nil), b.MinBound...),
		MaxBound: append([]space.Bound(nil), b.MaxBound...),
		Meta:     append([]byte(nil), b.Meta...),
	}
}

// loBound and hiBound mirror space.Box's defaulting: a nil bound slice means
// Min closed, Max open. The tree needs them only for its total order.
func loBound(b space.Box, i int) space.Bound {
	if i < len(b.MinBound) {
		return b.MinBound[i]
	}
	return space.Closed
}

func hiBound(b space.Box, i int) space.Bound {
	if i < len(b.MaxBound) {
		return b.MaxBound[i]
	}
	return space.Open
}
