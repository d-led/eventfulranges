package engine_test

import (
	"strconv"

	"pgregory.net/rapid"

	"github.com/d-led/eventfulranges/interval"
	"github.com/d-led/eventfulranges/op"
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
		sbName := rapid.SampledFrom([]string{"closed", "open"}).Draw(t, "sb")
		ebName := rapid.SampledFrom([]string{"closed", "open"}).Draw(t, "eb")
		kindName := rapid.SampledFrom([]string{"add", "remove"}).Draw(t, "kind")
		ts := rapid.IntRange(1, 4).Draw(t, "ts")
		ops[i] = op.Op{
			ID:       strconv.Itoa(i),
			Kind:     kindByName(kindName),
			TS:       int64(ts),
			Interval: iv(start, end, boundByName(sbName), boundByName(ebName)),
		}
	}
	return ops
}

// boundByName maps a readable bound name to its value.
func boundByName(name string) interval.Bound {
	if name == "open" {
		return interval.Open
	}
	return interval.Closed
}

// kindByName maps a readable kind name to its value.
func kindByName(name string) op.Kind {
	if name == "remove" {
		return op.KindRemove
	}
	return op.KindAdd
}
