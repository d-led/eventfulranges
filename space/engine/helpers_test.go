package engine_test

import (
	"strconv"

	"pgregory.net/rapid"

	"github.com/d-led/eventfulranges/space"
	"github.com/d-led/eventfulranges/space/op"
)

// box builds a 2D integer box on the grid.
func box(x0, y0, x1, y1 int) space.Box {
	return space.NewBox([]float64{float64(x0), float64(y0)}, []float64{float64(x1), float64(y1)})
}

// genOps draws a list of non-empty 2D boxes with unique ids and non-zero
// timestamps.
func genOps(t *rapid.T) []op.Op {
	n := rapid.IntRange(0, 8).Draw(t, "n")
	ops := make([]op.Op, n)
	for i := range ops {
		x0 := rapid.IntRange(-2, 2).Draw(t, "x0")
		x1 := rapid.IntRange(x0+1, 3).Draw(t, "x1")
		y0 := rapid.IntRange(-2, 2).Draw(t, "y0")
		y1 := rapid.IntRange(y0+1, 3).Draw(t, "y1")
		kind := rapid.SampledFrom([]op.Kind{op.KindAdd, op.KindRemove}).Draw(t, "kind")
		ts := rapid.IntRange(1, 4).Draw(t, "ts")
		ops[i] = op.Op{
			ID:   strconv.Itoa(i),
			Kind: kind,
			TS:   int64(ts),
			Box:  box(x0, y0, x1, y1),
		}
	}
	return ops
}

// opsFromBytes derives non-empty 2D boxes deterministically.
func opsFromBytes(data []byte) []op.Op {
	if len(data) < 6 {
		return nil
	}
	n := int(data[0] % 5)
	ops := make([]op.Op, 0, n)
	for i := 0; i < n && i*6+6 <= len(data); i++ {
		b := data[i*6:]
		x0 := int(b[0]%5) - 2
		x1 := x0 + 1 + int(b[1]%3)
		y0 := int(b[2]%5) - 2
		y1 := y0 + 1 + int(b[3]%3)
		kind := op.KindAdd
		if b[4]%2 == 0 {
			kind = op.KindRemove
		}
		ops = append(ops, op.Op{
			ID:   strconv.Itoa(i),
			Kind: kind,
			TS:   int64(b[5]%4) + 1,
			Box:  box(x0, y0, x1, y1),
		})
	}
	return ops
}

// reverseOps returns the operations in reverse order.
func reverseOps(ops []op.Op) []op.Op {
	out := make([]op.Op, len(ops))
	for i, o := range ops {
		out[len(ops)-1-i] = o
	}
	return out
}

// splitCanonicalizer halves every box along its first axis. It is a
// cover-preserving, deterministic canonicalizer used to exercise the engine's
// canonicalization seam: the default cover is replaced by an observably
// different but point-equivalent one.
func splitCanonicalizer(boxes []space.Box) []space.Box {
	out := make([]space.Box, 0, 2*len(boxes))
	for _, b := range boxes {
		if len(b.Min) == 0 || b.Max[0]-b.Min[0] < 2 {
			out = append(out, b)
			continue
		}
		mid := (b.Min[0] + b.Max[0]) / 2
		left := space.NewBox(append([]float64(nil), b.Min...), append([]float64(nil), b.Max...))
		right := space.NewBox(append([]float64(nil), b.Min...), append([]float64(nil), b.Max...))
		left.Max[0] = mid
		right.Min[0] = mid
		out = append(out, left, right)
	}
	return out
}
