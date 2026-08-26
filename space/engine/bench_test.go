package engine_test

import (
	"context"
	"fmt"
	"math/rand"
	"testing"

	"github.com/d-led/eventfulranges/space"
	"github.com/d-led/eventfulranges/space/engine"
	"github.com/d-led/eventfulranges/space/op"
	"github.com/d-led/eventfulranges/space/store/memory"
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

// BenchmarkApplyIncremental measures the total cost of folding n operations
// one at a time. LWW and AdditiveWinsLWW materialize the whole history after
// every apply, so their total cost grows super-linearly with n.
func BenchmarkApplyIncremental(b *testing.B) {
	ctx := context.Background()
	for _, s := range []strategy.Strategy{strategy.LWW, strategy.AdditiveWins, strategy.AdditiveWinsLWW} {
		for _, n := range []int{50, 200, 500} {
			ops := benchOps(n)
			b.Run(fmt.Sprintf("%s/n=%d", s, n), func(b *testing.B) {
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					e, err := engine.Open(ctx, memory.New(), s)
					if err != nil {
						b.Fatal(err)
					}
					for _, o := range ops {
						_ = e.Apply(ctx, o)
					}
				}
			})
		}
	}
}
