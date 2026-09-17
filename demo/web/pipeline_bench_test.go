//go:build !js

package main

import (
	"math/rand"
	"testing"

	"github.com/d-led/eventfulranges/meta"
	"github.com/d-led/eventfulranges/space"
)

// These benchmarks replay the workload that made the deployed 3D
// partition-merge page take seconds per operation, so that the cost of each
// stage of the pipeline stays visible: the built-in example, then random edits
// on top of it, of both kinds.

// benchOp is one operation of a benchmark workload.
type benchOp struct {
	remove bool
	lo, hi []float64
}

// deployedWorkload reconstructs the shape of a real session: a 3x3x3 block of
// unit cubes with the center carved out (the built-in example), followed by
// random adds and removes inside the same bounds.
func deployedWorkload(dims, randomOps int, seed int64) []benchOp {
	const grid = 3.0
	ops := make([]benchOp, 0, 27+1+randomOps)
	for x := 0.0; x < grid; x++ {
		for y := 0.0; y < grid; y++ {
			for z := 0.0; z < grid; z++ {
				ops = append(ops, benchOp{lo: []float64{x, y, z}, hi: []float64{x + 1, y + 1, z + 1}})
			}
		}
	}
	center := make([]float64, dims)
	corner := make([]float64, dims)
	for d := range center {
		center[d] = 1
		corner[d] = 2
	}
	ops = append(ops, benchOp{remove: true, lo: center, hi: corner})

	rng := rand.New(rand.NewSource(seed))
	for range randomOps {
		lo := make([]float64, dims)
		hi := make([]float64, dims)
		for d := range lo {
			a := rng.Float64() * grid
			b := rng.Float64() * grid
			lo[d] = min(a, b)
			hi[d] = max(a, b) + 0.01
		}
		ops = append(ops, benchOp{remove: rng.Intn(2) == 0, lo: lo, hi: hi})
	}
	return ops
}

// benchHubApply measures one fold, canonicalizer included, for the given
// compaction mode. The reported time is for the whole workload, so comparing
// the modes shows what each one costs a viewer who is watching it run.
func benchHubApply(b *testing.B, mode string, dims, randomOps int) {
	ops := deployedWorkload(dims, randomOps, 7)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		h := newHubMode(mode)
		if err := h.applyClientOp("bench", clientOp{Kind: "dims", Dims: dims}); err != nil {
			b.Fatal(err)
		}
		b.StartTimer()
		for _, o := range ops {
			kind := opAdd
			if o.remove {
				kind = opRemove
			}
			if _, err := h.record("bench", kind, o.lo, o.hi); err != nil {
				b.Fatal(err)
			}
		}
	}
}

func BenchmarkHubApply3DCanonical(b *testing.B)      { benchHubApply(b, compactCanonical, 3, 8) }
func BenchmarkHubApply3DMerge(b *testing.B)          { benchHubApply(b, compactMerge, 3, 8) }
func BenchmarkHubApply3DPartition(b *testing.B)      { benchHubApply(b, compactPartition, 3, 8) }
func BenchmarkHubApply3DPartitionMerge(b *testing.B) { benchHubApply(b, compactPartitionMerge, 3, 8) }

// workloadCover is the cover the AdditiveWins engine holds at the end of the
// workload: the union of the additions minus the union of the removals. It is
// what a canonicalizer is handed after every operation.
func workloadCover(dims, randomOps int, seed int64) []space.Box {
	var adds, removes []space.Box
	for _, o := range deployedWorkload(dims, randomOps, seed) {
		box := space.NewBox(o.lo, o.hi)
		if o.remove {
			removes = space.UnionMerged(removes, []space.Box{box}, meta.Union)
			continue
		}
		adds = space.UnionMerged(adds, []space.Box{box}, meta.Union)
	}
	return space.DifferenceSortedMerged(adds, removes, meta.Union)
}

// benchCanonicalizer measures one canonicalizer over that cover, so the cost of
// the compaction on its own is visible next to the cost of the fold that wraps
// it.
func benchCanonicalizer(b *testing.B, c space.Canonicalizer, partitionFirst bool) {
	boxes := workloadCover(3, 8, 7)
	if partitionFirst {
		boxes = Partition(boxes)
	}
	b.ReportAllocs()
	b.ReportMetric(float64(len(boxes)), "boxes")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c(boxes)
	}
}

func BenchmarkPartitionWorkloadCover(b *testing.B) {
	benchCanonicalizer(b, Partition, false)
}

func BenchmarkMergeAdjacentWorkloadCells(b *testing.B) {
	benchCanonicalizer(b, space.MergeAdjacent, true)
}

func BenchmarkPartitionMergeWorkloadCover(b *testing.B) {
	benchCanonicalizer(b, space.Chain(Partition, space.MergeAdjacent), false)
}
