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
