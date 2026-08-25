package strategy_test

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

// samplePoints returns grid points and midpoints covering every open segment.
func samplePoints() []float64 {
	pts := make([]float64, 0, 15)
	for k := -7; k <= 7; k++ {
		pts = append(pts, float64(k)/2)
	}
	return pts
}

// genOpList draws a list of ops with unique ids on the -3..3 grid.
func genOpList(t *rapid.T) []op.Op {
	n := rapid.IntRange(0, 8).Draw(t, "n")
	ops := make([]op.Op, n)
	for i := range ops {
		start := rapid.IntRange(-3, 3).Draw(t, "start")
		end := rapid.IntRange(start, 3).Draw(t, "end")
		sbName := rapid.SampledFrom([]string{"closed", "open"}).Draw(t, "sb")
		ebName := rapid.SampledFrom([]string{"closed", "open"}).Draw(t, "eb")
		kindName := rapid.SampledFrom([]string{"add", "remove"}).Draw(t, "kind")
		ts := rapid.IntRange(0, 4).Draw(t, "ts")
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

// opsFromBytes derives ops with unique ids deterministically.
func opsFromBytes(data []byte) []op.Op {
	if len(data) < 3 {
		return nil
	}
	n := int(data[0]%8) + 1
	ops := make([]op.Op, 0, n)
	for i := 0; i < n && i*3+3 <= len(data); i++ {
		start := int(data[i*3]%7) - 3
		end := int(data[i*3+1]%7) - 3
		if start > end {
			start, end = end, start
		}
		sb, eb := interval.Closed, interval.Closed
		if data[i*3]&1 == 0 {
			sb = interval.Open
		}
		if data[i*3+1]&1 == 0 {
			eb = interval.Open
		}
		kind := op.KindAdd
		if data[i*3+2]&1 == 0 {
			kind = op.KindRemove
		}
		ops = append(ops, op.Op{
			ID:       strconv.Itoa(i),
			Kind:     kind,
			TS:       int64(data[i*3+2] % 5),
			Interval: iv(start, end, sb, eb),
		})
	}
	return ops
}

// lwwMember reports membership under the LWW oracle.
func lwwMember(ops []op.Op, x float64) bool {
	best := -1
	for i, o := range ops {
		if !o.Interval.Contains(x) {
			continue
		}
		if best == -1 || later(ops[i], ops[best]) {
			best = i
		}
	}
	return best >= 0 && ops[best].Kind == op.KindAdd
}

// fwwMember reports membership under the FWW oracle.
func fwwMember(ops []op.Op, x float64) bool {
	best := -1
	for i, o := range ops {
		if !o.Interval.Contains(x) {
			continue
		}
		if best == -1 || earlier(ops[i], ops[best]) {
			best = i
		}
	}
	return best >= 0 && ops[best].Kind == op.KindAdd
}

// additiveMember reports membership under the additive-wins oracle.
func additiveMember(ops []op.Op, x float64) bool {
	var added, removed bool
	for _, o := range ops {
		if !o.Interval.Contains(x) {
			continue
		}
		if o.Kind == op.KindAdd {
			added = true
		} else {
			removed = true
		}
	}
	return added && !removed
}

func later(a, b op.Op) bool {
	if a.TS != b.TS {
		return a.TS > b.TS
	}
	return a.ID > b.ID
}

func earlier(a, b op.Op) bool {
	if a.TS != b.TS {
		return a.TS < b.TS
	}
	return a.ID < b.ID
}
