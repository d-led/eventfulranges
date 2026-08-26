package space

import "sort"

// ConnectedComponents groups boxes into maximal sets whose union is connected.
// Two boxes belong to the same component when they overlap or touch along a
// face of positive measure — adjacent in exactly one dimension and overlapping
// in every other dimension. Boxes within a component and the components
// themselves are returned in canonical order. All boxes must share the same
// dimensionality.
func ConnectedComponents(boxes []Box) [][]Box {
	boxes = Normalize(boxes)
	if len(boxes) == 0 {
		return nil
	}
	ds := newDisjointSet(len(boxes))
	for i := 0; i < len(boxes); i++ {
		for j := i + 1; j < len(boxes); j++ {
			if connected(boxes[i], boxes[j]) {
				ds.union(i, j)
			}
		}
	}
	return ds.groups(boxes)
}

// connected reports whether two boxes overlap or touch along a face.
func connected(a, b Box) bool {
	if len(a.Min) != len(b.Min) {
		return false
	}
	if Overlaps(a, b) {
		return true
	}
	for d := range a.Min {
		if a.Max[d] == b.Min[d] || b.Max[d] == a.Min[d] {
			if touchAlongFace(a, b, d) {
				return true
			}
		}
	}
	return false
}

// touchAlongFace reports whether a and b overlap with positive measure in
// every dimension other than d, where d is a touching dimension.
func touchAlongFace(a, b Box, d int) bool {
	for k := range a.Min {
		if k == d {
			continue
		}
		if a.Max[k] <= b.Min[k] || b.Max[k] <= a.Min[k] {
			return false
		}
	}
	return true
}

// disjointSet is a path-compressing union-find over box indices.
type disjointSet struct {
	parent []int
}

func newDisjointSet(n int) *disjointSet {
	parent := make([]int, n)
	for i := range parent {
		parent[i] = i
	}
	return &disjointSet{parent: parent}
}

func (d *disjointSet) find(x int) int {
	if d.parent[x] != x {
		d.parent[x] = d.find(d.parent[x])
	}
	return d.parent[x]
}

func (d *disjointSet) union(a, b int) {
	ra, rb := d.find(a), d.find(b)
	if ra != rb {
		d.parent[rb] = ra
	}
}

// groups returns the boxes of each component, ordered canonically.
func (d *disjointSet) groups(boxes []Box) [][]Box {
	groups := make(map[int][]int, len(boxes))
	for i := range boxes {
		groups[d.find(i)] = append(groups[d.find(i)], i)
	}
	roots := make([]int, 0, len(groups))
	for r := range groups {
		roots = append(roots, r)
	}
	sort.Ints(roots)
	out := make([][]Box, 0, len(roots))
	for _, r := range roots {
		comp := make([]Box, 0, len(groups[r]))
		for _, idx := range groups[r] {
			comp = append(comp, boxes[idx])
		}
		out = append(out, comp)
	}
	return out
}
