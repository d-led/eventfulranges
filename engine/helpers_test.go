package engine_test

import (
	"strconv"

	"pgregory.net/rapid"

	"gitub.com/d-led/eventfulranges/interval"
	"gitub.com/d-led/eventfulranges/op"
)

// iv builds an interval from grid endpoints.
func iv(start, end int, sb, eb interval.Bound) interval.Interval {
	if start == end {
		sb, eb = interval.Closed, interval.Closed
	}
	return interval.Interval{Start: float64(start), End: float64(end), StartBound: sb, EndBound: eb}
}

// genOps draws a list of ops with unique ids and non-zero timestamps on the
// -3..3 grid.
func genOps(t *rapid.T) []op.Op {
	n := rapid.IntRange(0, 8).Draw(t, "n")
	ops := make([]op.Op, n)
	for i := range ops {
		start := rapid.IntRange(-3, 3).Draw(t, "start")
		end := rapid.IntRange(start, 3).Draw(t, "end")
		sb := rapid.SampledFrom([]interval.Bound{interval.Closed, interval.Open}).Draw(t, "sb")
		eb := rapid.SampledFrom([]interval.Bound{interval.Closed, interval.Open}).Draw(t, "eb")
		kind := rapid.SampledFrom([]op.Kind{op.KindAdd, op.KindRemove}).Draw(t, "kind")
		ts := rapid.IntRange(1, 4).Draw(t, "ts")
		ops[i] = op.Op{
			ID:       strconv.Itoa(i),
			Kind:     kind,
			TS:       int64(ts),
			Interval: iv(start, end, sb, eb),
		}
	}
	return ops
}
