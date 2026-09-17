package space

import "testing"

// BenchmarkMergeAdjacent runs the indexed merger next to the plain pair-scanning
// definition of it (referenceMergeAdjacent, in canonical_test.go) over the same
// cover — roughly the size the visualizer's partition hands it on a 3D session.
// The reference is cubic, which is why it is only a test oracle and a benchmark,
// never the implementation.
func BenchmarkMergeAdjacent(b *testing.B) {
	cells := benchmarkGrid(32)
	b.ReportMetric(float64(len(cells)), "cells")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MergeAdjacent(cells)
	}
}

func BenchmarkMergeAdjacentReference(b *testing.B) {
	cells := benchmarkGrid(32)
	b.ReportMetric(float64(len(cells)), "cells")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		referenceMergeAdjacent(cells)
	}
}

// benchmarkGrid returns the cells of an n x n unit grid over [0,n)^2 with a
// staircase bite taken out of one corner: enough shared faces for long merge
// chains, and a shape that cannot collapse into a single box.
func benchmarkGrid(n int) []Box {
	cells := make([]Box, 0, n*n)
	for i := range n {
		for j := range n {
			if i+j < n/2 {
				continue // the bite: this part of the grid is not covered
			}
			cells = append(cells, box2(float64(i), float64(j), float64(i+1), float64(j+1)))
		}
	}
	return cells
}
