package engine_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/d-led/eventfulranges/space"
	"github.com/d-led/eventfulranges/space/engine"
	"github.com/d-led/eventfulranges/space/op"
	"github.com/d-led/eventfulranges/space/store/memory"
	"github.com/d-led/eventfulranges/space/strategy"
)

// overlappingOps returns n 2D additions that heavily overlap, the worst case
// for priority-resolved materialization.
func overlappingOps(n int) []op.Op {
	ops := make([]op.Op, n)
	for i := range ops {
		ops[i] = op.Op{
			ID:   fmt.Sprintf("op-%d", i),
			Kind: op.KindAdd,
			TS:   int64(i),
			Box:  space.NewBox([]float64{0, 0}, []float64{float64(20 + i), float64(20 + i)}),
		}
	}
	return ops
}

// disjointOps returns n 2D additions that never overlap, the common case for a
// whiteboard: separate strokes in separate places.
func disjointOps(n int) []op.Op {
	ops := make([]op.Op, n)
	for i := range ops {
		x := float64((i % 100) * 10)
		y := float64((i / 100) * 10)
		ops[i] = op.Op{
			ID:   fmt.Sprintf("op-%d", i),
			Kind: op.KindAdd,
			TS:   int64(i),
			Box:  space.NewBox([]float64{x, y}, []float64{x + 5, y + 5}),
		}
	}
	return ops
}

// BenchmarkApplyIncremental measures folding n operations one at a time. The
// engine defers materialization, so this is the cost of the append path alone.
func BenchmarkApplyIncremental(b *testing.B) {
	ctx := context.Background()
	for _, s := range []strategy.Strategy{strategy.LWW, strategy.AdditiveWins, strategy.AdditiveWinsLWW} {
		for _, tc := range []struct {
			name string
			gen  func(int) []op.Op
		}{{"overlap", overlappingOps}, {"disjoint", disjointOps}} {
			for _, n := range []int{100, 1000} {
				ops := tc.gen(n)
				b.Run(fmt.Sprintf("%s/%s/n=%d", s, tc.name, n), func(b *testing.B) {
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						e, err := engine.Open(ctx, memory.New(), s, engine.WithSnapshotEvery(0))
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
}

// BenchmarkApplyThenMaterialize measures folding n operations and then
// materializing once — the realistic total cost under lazy materialization.
func BenchmarkApplyThenMaterialize(b *testing.B) {
	ctx := context.Background()
	for _, s := range []strategy.Strategy{strategy.LWW, strategy.AdditiveWins, strategy.AdditiveWinsLWW} {
		for _, tc := range []struct {
			name string
			gen  func(int) []op.Op
		}{{"overlap", overlappingOps}, {"disjoint", disjointOps}} {
			for _, n := range []int{100, 1000} {
				ops := tc.gen(n)
				b.Run(fmt.Sprintf("%s/%s/n=%d", s, tc.name, n), func(b *testing.B) {
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						e, err := engine.Open(ctx, memory.New(), s, engine.WithSnapshotEvery(0))
						if err != nil {
							b.Fatal(err)
						}
						for _, o := range ops {
							_ = e.Apply(ctx, o)
						}
						_ = e.Materialize()
					}
				})
			}
		}
	}
}
