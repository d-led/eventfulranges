package rtree

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/d-led/eventfulranges/space"
)

// sink keeps benchmark results observable so the compiler cannot discard the
// work under test.
var sink int

// randomBoxes draws n non-overlapping-ish 2D boxes on a square canvas of the
// given side length. Boxes are small relative to the canvas, like the strokes
// of the paint demo, so the set is sparse and a query touches few of them.
func randomBoxes(n int, seed int64, side float64) []space.Box {
	r := rand.New(rand.NewSource(seed))
	boxes := make([]space.Box, n)
	for i := range boxes {
		x := r.Float64() * side
		y := r.Float64() * side
		w := 1 + r.Float64()*64
		h := 1 + r.Float64()*64
		boxes[i] = space.NewBox([]float64{x, y}, []float64{x + w, y + h})
	}
	return boxes
}

// --- memory accounting -----------------------------------------------------
// The helpers below sum the bytes the tree owns — backing arrays and node
// structs — so two configurations compare deterministically, independent of
// the allocator's reuse. They measure the retained footprint, not the
// transient allocation churn (BenchmarkBuild's B/op covers that).

func boxBytes(b space.Box) int {
	return len(b.Min)*8 + len(b.Max)*8 + len(b.MinBound) + len(b.MaxBound) + len(b.Meta)
}

func boxBytesOf(boxes []space.Box) int {
	total := 0
	for _, b := range boxes {
		total += boxBytes(b)
	}
	return total
}

func mbrBytes(b space.Box) int {
	return len(b.Min)*8 + len(b.Max)*8 + len(b.MinBound) + len(b.MaxBound)
}

func nodeBytes(n *node) int {
	total := mbrBytes(n.mbr)
	if n.leaf {
		for _, b := range n.boxes {
			total += boxBytes(b)
		}
		return total
	}
	for _, c := range n.children {
		total += nodeBytes(c)
	}
	return total
}

func treeBytes(t *Tree) int {
	if t == nil || t.root == nil {
		return 0
	}
	return nodeBytes(t.root)
}

// overheadBytes is the bytes the tree allocates for its own structure — every
// node's MBR — excluding the box data, which BuildRef shares with the caller.
func overheadBytes(t *Tree) int {
	if t == nil || t.root == nil {
		return 0
	}
	var sum func(*node) int
	sum = func(n *node) int {
		total := mbrBytes(n.mbr)
		for _, c := range n.children {
			total += sum(c)
		}
		return total
	}
	return sum(t.root)
}

func nodeCount(t *Tree) int {
	if t == nil || t.root == nil {
		return 0
	}
	var count func(*node) int
	count = func(n *node) int {
		if n.leaf {
			return 1
		}
		c := 1
		for _, ch := range n.children {
			c += count(ch)
		}
		return c
	}
	return count(t.root)
}

// BenchmarkBuildRef measures the allocation churn of a no-copy build, for the
// memory comparison against Build (whose B/op is BenchmarkBuild).
func BenchmarkBuildRef(b *testing.B) {
	const side = 1 << 20
	for _, n := range []int{1_000, 10_000, 100_000} {
		boxes := randomBoxes(n, 7, side)
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				tree, err := BuildRef(boxes)
				if err != nil {
					b.Fatal(err)
				}
				sink += tree.Len()
			}
		})
	}
}

// BenchmarkBuildMemory reports the retained footprint: the input's box data,
// a copying tree that owns everything, and a referencing tree that owns only
// its node structure.
func BenchmarkBuildMemory(b *testing.B) {
	const side = 1 << 20
	for _, n := range []int{1_000, 10_000, 100_000} {
		boxes := randomBoxes(n, 7, side)
		copied, err := Build(boxes)
		if err != nil {
			b.Fatal(err)
		}
		referenced, err := BuildRef(boxes)
		if err != nil {
			b.Fatal(err)
		}
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			b.ReportMetric(float64(boxBytesOf(boxes)), "B/input-boxes")
			b.ReportMetric(float64(treeBytes(copied)), "B/owned-copy")
			b.ReportMetric(float64(overheadBytes(referenced)), "B/owned-ref")
			b.ReportMetric(float64(nodeCount(referenced)), "nodes")
			for i := 0; i < b.N; i++ {
				sink += copied.Len() + referenced.Len()
			}
		})
	}
}

// countOverlaps is the current production path: a linear scan.
func countOverlaps(boxes []space.Box, q space.Box) int {
	count := 0
	for _, b := range boxes {
		if space.Overlaps(b, q) {
			count++
		}
	}
	return count
}

// BenchmarkBuild measures the rebuild cost an ephemeral index pays each time
// the board changes.
func BenchmarkBuild(b *testing.B) {
	const side = 1 << 20
	for _, n := range []int{1_000, 10_000, 100_000} {
		boxes := randomBoxes(n, 7, side)
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				tree, err := Build(boxes)
				if err != nil {
					b.Fatal(err)
				}
				sink += tree.Len()
			}
		})
	}
}

// BenchmarkSearchOverlap compares a viewport overlap query ("which boxes are
// inside this region") between the linear scan and the tree, across set sizes
// and two query selectivities: a small viewport and a query covering the whole
// canvas (the worst case, where the index is pure overhead).
func BenchmarkSearchOverlap(b *testing.B) {
	const side = 1 << 20
	for _, n := range []int{1_000, 10_000, 100_000} {
		boxes := randomBoxes(n, 11, side)
		tree, err := Build(boxes)
		if err != nil {
			b.Fatal(err)
		}
		for _, qs := range []float64{256, 8192, 2 * side} {
			q := space.NewBox([]float64{side / 2, side / 2}, []float64{side/2 + qs, side/2 + qs})
			label := fmt.Sprintf("n=%d/q=%g", n, qs)
			b.Run("linear/"+label, func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					sink += countOverlaps(boxes, q)
				}
			})
			b.Run("rtree/"+label, func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					sink += len(tree.Search(q))
				}
			})
			b.Run("rtree-ref/"+label, func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					sink += len(tree.SearchRef(q))
				}
			})
		}
	}
}

// BenchmarkSearchPoint compares a point-in-box query ("what is under the
// cursor"), the other half of a "places of interest" feature.
func BenchmarkSearchPoint(b *testing.B) {
	const side = 1 << 20
	for _, n := range []int{1_000, 10_000, 100_000} {
		boxes := randomBoxes(n, 13, side)
		tree, err := Build(boxes)
		if err != nil {
			b.Fatal(err)
		}
		p := []float64{side / 2, side / 2}
		point := space.NewBoxWithBounds(p, p,
			[]space.Bound{space.Closed, space.Closed},
			[]space.Bound{space.Closed, space.Closed},
		)
		label := fmt.Sprintf("n=%d", n)
		b.Run("linear/"+label, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if space.Contains(boxes, p) {
					sink++
				}
			}
		})
		b.Run("rtree/"+label, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				sink += len(tree.Search(point))
			}
		})
	}
}
