// Partition is a space.Canonicalizer for the visualizer that turns any cover
// of axis-aligned boxes into a disjoint partition: boxes that are pairwise
// non-overlapping and whose union is exactly the input union. Every partial
// overlap is split along the boxes' faces, so each covered point is
// materialized by exactly one rectangular box.
//
// It generalizes to any dimension by recursing one axis at a time. The unique
// coordinates along the first axis slice the cover into slabs; each slab is
// clipped and partitioned again in the remaining axes, bottoming out at one
// cell per covered region. The half-open [lo, hi) convention is the
// tie-breaker: every point falls in exactly one slab per axis, so coincident
// faces never double-count. That determinism is the simulation of simplicity —
// boundaries are resolved by the interval convention itself, with no numeric
// perturbation needed (and no point-in-polygon test, which axis-aligned boxes
// never require).
//
// This lives in the demo rather than the library: it is a display-oriented
// view, not the engine's canonical cover, and its box count can exceed the
// minimal cover for inputs with many nested overlaps.
package main

import (
	"sort"

	"github.com/d-led/eventfulranges/space"
)

func Partition(boxes []space.Box) []space.Box {
	dims := 0
	for _, b := range boxes {
		if !b.Empty() {
			dims = b.Dims()
			break
		}
	}
	if dims == 0 {
		return nil
	}
	return partitionAxis(boxes, 0, dims)
}

// partitionAxis splits the boxes along every face on the given axis, then
// recurses into the remaining axes for each covered slab.
func partitionAxis(boxes []space.Box, axis, dims int) []space.Box {
	if len(boxes) == 0 {
		return nil
	}
	coords := slabCoordinates(boxes, axis)
	var out []space.Box
	for i := 0; i+1 < len(coords); i++ {
		lo, hi := coords[i], coords[i+1]
		var slab []space.Box
		for _, b := range boxes {
			if b.Min[axis] <= lo && hi <= b.Max[axis] {
				slab = append(slab, clip(b, axis, lo, hi))
			}
		}
		if len(slab) == 0 {
			continue
		}
		if axis == dims-1 {
			// Every box in the slab is the same cell now (clipped on all
			// axes); emit it once, regardless of how many covered it.
			out = append(out, slab[0])
		} else {
			out = append(out, partitionAxis(slab, axis+1, dims)...)
		}
	}
	return out
}

// slabCoordinates returns the sorted, de-duplicated face coordinates along one
// axis, taken from every box.
func slabCoordinates(boxes []space.Box, axis int) []float64 {
	coords := make([]float64, 0, 2*len(boxes))
	for _, b := range boxes {
		if b.Empty() {
			continue
		}
		coords = append(coords, b.Min[axis], b.Max[axis])
	}
	sort.Float64s(coords)
	out := coords[:0]
	for _, c := range coords {
		if len(out) == 0 || out[len(out)-1] != c {
			out = append(out, c)
		}
	}
	return out
}

// clip returns a copy of b with the given axis fixed to [lo, hi).
func clip(b space.Box, axis int, lo, hi float64) space.Box {
	c := space.Box{
		Min:  append([]float64(nil), b.Min...),
		Max:  append([]float64(nil), b.Max...),
		Meta: append([]byte(nil), b.Meta...),
	}
	c.Min[axis] = lo
	c.Max[axis] = hi
	return c
}
