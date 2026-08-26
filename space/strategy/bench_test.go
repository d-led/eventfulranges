package strategy_test

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/d-led/eventfulranges/space"
	"github.com/d-led/eventfulranges/space/op"
	"github.com/d-led/eventfulranges/space/strategy"
)

// benchOps returns n overlapping 2D additions with deterministic pseudo-random
// corners and sequential timestamps.
func benchOps(n int) []op.Op {
	rng := rand.New(rand.NewSource(42))
	ops := make([]op.Op, n)
	for i := range ops {
		x0 := rng.Intn(100)
		y0 := rng.Intn(100)
		x1 := x0 + 1 + rng.Intn(20)
		y1 := y0 + 1 + rng.Intn(20)
		ops[i] = op.Op{
			ID:   fmt.Sprintf("op-%d", i),
			Kind: op.KindAdd,
			TS:   int64(i),
			Box:  space.NewBox([]float64{float64(x0), float64(y0)}, []float64{float64(x1), float64(y1)}),
		}
	}
	return ops
}

var sink []space.Box

// BenchmarkMaterialize measures one full materialization over an increasing
// operation set. LWW and AdditiveWinsLWW partition the winners, so this is the
// cost of a single view rebuild.
func BenchmarkMaterialize(b *testing.B) {
	for _, s := range []strategy.Strategy{strategy.LWW, strategy.AdditiveWins, strategy.AdditiveWinsLWW} {
		for _, n := range []int{100, 1000, 5000} {
			ops := benchOps(n)
			b.Run(fmt.Sprintf("%s/n=%d", s, n), func(b *testing.B) {
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					sink = strategy.Materialize(s, ops)
				}
				b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "materializes/s")
			})
		}
	}
}
