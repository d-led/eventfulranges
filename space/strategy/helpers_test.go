package strategy_test

import (
	"strconv"

	"pgregory.net/rapid"

	"github.com/d-led/eventfulranges/space"
	"github.com/d-led/eventfulranges/space/op"
	"github.com/d-led/eventfulranges/space/strategy"
)

// box builds a 2D integer box on the grid.
func box(x0, y0, x1, y1 int) space.Box {
	return space.NewBox([]float64{float64(x0), float64(y0)}, []float64{float64(x1), float64(y1)})
}

// samplePoints returns a dense 2D grid that catches boundary errors.
func samplePoints() [][]float64 {
	var pts [][]float64
	for x := -4.0; x <= 4.0; x += 0.5 {
		for y := -4.0; y <= 4.0; y += 0.5 {
			pts = append(pts, []float64{x, y})
		}
	}
	return pts
}

// genOps draws a list of non-empty 2D boxes with unique ids and small
// timestamps, so ties exercise the id tiebreaker.
func genOps(t *rapid.T) []op.Op {
	n := rapid.IntRange(0, 8).Draw(t, "n")
	ops := make([]op.Op, n)
	for i := range ops {
		x0 := rapid.IntRange(-2, 2).Draw(t, "x0")
		x1 := rapid.IntRange(x0+1, 3).Draw(t, "x1")
		y0 := rapid.IntRange(-2, 2).Draw(t, "y0")
		y1 := rapid.IntRange(y0+1, 3).Draw(t, "y1")
		kind := rapid.SampledFrom([]op.Kind{op.KindAdd, op.KindRemove}).Draw(t, "kind")
		ts := rapid.IntRange(0, 3).Draw(t, "ts")
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
			TS:   int64(b[5] % 4),
			Box:  box(x0, y0, x1, y1),
		})
	}
	return ops
}

// member reports point membership under the chosen strategy, the independent
// oracle used by the property and fuzz tests.
func member(s strategy.Strategy, ops []op.Op, p []float64) bool {
	switch s {
	case strategy.LWW:
		return lwwMember(ops, p)
	case strategy.FWW:
		return fwwMember(ops, p)
	case strategy.AdditiveWins:
		return additiveMember(ops, p)
	case strategy.GrowOnly:
		return growMember(ops, p)
	}
	return false
}

// lwwMember reports membership under the LWW oracle: the highest-priority
// covering operation decides.
func lwwMember(ops []op.Op, p []float64) bool {
	best := -1
	for i, o := range ops {
		if !o.Box.Contains(p) {
			continue
		}
		if best == -1 || later(ops[i], ops[best]) {
			best = i
		}
	}
	return best >= 0 && ops[best].Kind == op.KindAdd
}

// fwwMember reports membership under the FWW oracle: the lowest-priority
// covering operation decides.
func fwwMember(ops []op.Op, p []float64) bool {
	best := -1
	for i, o := range ops {
		if !o.Box.Contains(p) {
			continue
		}
		if best == -1 || earlier(ops[i], ops[best]) {
			best = i
		}
	}
	return best >= 0 && ops[best].Kind == op.KindAdd
}

// additiveMember reports membership under the additive-wins oracle.
func additiveMember(ops []op.Op, p []float64) bool {
	var added, removed bool
	for _, o := range ops {
		if !o.Box.Contains(p) {
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

// growMember reports membership under the grow-only oracle.
func growMember(ops []op.Op, p []float64) bool {
	for _, o := range ops {
		if o.Kind == op.KindAdd && o.Box.Contains(p) {
			return true
		}
	}
	return false
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
