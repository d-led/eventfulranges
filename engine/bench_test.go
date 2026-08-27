package engine_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/d-led/eventfulranges/engine"
	"github.com/d-led/eventfulranges/interval"
	"github.com/d-led/eventfulranges/op"
	"github.com/d-led/eventfulranges/store/memory"
	"github.com/d-led/eventfulranges/strategy"
)

// overlappingIntervals returns n 1D additions that all overlap the origin, the
// worst case for priority-resolved materialization.
func overlappingIntervals(n int) []op.Op {
	ops := make([]op.Op, n)
	for i := range ops {
		ops[i] = op.Op{
			ID:       fmt.Sprintf("op-%d", i),
			Kind:     op.KindAdd,
			TS:       int64(i),
			Interval: interval.Interval{Start: 0, End: float64(20 + i)},
		}
	}
	return ops
}

// disjointIntervals returns n 1D additions that never overlap, the common
// case: separate edits in separate places.
func disjointIntervals(n int) []op.Op {
	ops := make([]op.Op, n)
	for i := range ops {
		start := float64(i * 10)
		ops[i] = op.Op{
			ID:       fmt.Sprintf("op-%d", i),
			Kind:     op.KindAdd,
			TS:       int64(i),
			Interval: interval.Interval{Start: start, End: start + 5},
		}
	}
	return ops
}

// BenchmarkApplyIncremental measures folding n operations one at a time. The
// root engine materializes eagerly, so this is the full append+fold cost.
func BenchmarkApplyIncremental(b *testing.B) {
	ctx := context.Background()
	for _, s := range []strategy.Strategy{strategy.LWW, strategy.AdditiveWins, strategy.GrowOnly} {
		for _, tc := range []struct {
			name string
			gen  func(int) []op.Op
		}{{"overlap", overlappingIntervals}, {"disjoint", disjointIntervals}} {
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
// materializing once — the realistic total cost.
func BenchmarkApplyThenMaterialize(b *testing.B) {
	ctx := context.Background()
	for _, s := range []strategy.Strategy{strategy.LWW, strategy.AdditiveWins, strategy.GrowOnly} {
		for _, tc := range []struct {
			name string
			gen  func(int) []op.Op
		}{{"overlap", overlappingIntervals}, {"disjoint", disjointIntervals}} {
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
