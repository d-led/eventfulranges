package engine_test

import (
	"context"
	"testing"

	"pgregory.net/rapid"

	"gitub.com/d-led/eventfulranges/engine"
	"gitub.com/d-led/eventfulranges/interval"
	"gitub.com/d-led/eventfulranges/store/memory"
	"gitub.com/d-led/eventfulranges/strategy"
)

func TestPropertyEngineMatchesStrategy(t *testing.T) {
	t.Parallel()
	strategies := []strategy.Strategy{strategy.LWW, strategy.FWW, strategy.AdditiveWins, strategy.GrowOnly}
	for _, s := range strategies {
		t.Run(s.String(), func(t *testing.T) {
			t.Parallel()
			rapid.Check(t, func(t *rapid.T) {
				ops := genOps(t)
				e, err := engine.Open(context.Background(), memory.New(), s)
				if err != nil {
					t.Fatal(err)
				}
				if err := e.ApplyAll(context.Background(), ops); err != nil {
					t.Fatal(err)
				}
				want := strategy.Materialize(s, ops)
				if !interval.Equal(want, e.Materialize()) {
					t.Fatalf("%v mismatch: want %v got %v (ops %v)", s, want, e.Materialize(), ops)
				}
			})
		})
	}
}

func TestPropertyConvergence(t *testing.T) {
	t.Parallel()
	strategies := []strategy.Strategy{strategy.LWW, strategy.FWW, strategy.AdditiveWins, strategy.GrowOnly}
	for _, s := range strategies {
		t.Run(s.String(), func(t *testing.T) {
			t.Parallel()
			rapid.Check(t, func(t *rapid.T) {
				ops := genOps(t)
				perm := rapid.Permutation(ops).Draw(t, "perm")
				half := len(perm) / 2

				ctx := context.Background()
				a, err := engine.Open(ctx, memory.New(), s)
				if err != nil {
					t.Fatal(err)
				}
				b, err := engine.Open(ctx, memory.New(), s)
				if err != nil {
					t.Fatal(err)
				}
				if err := a.ApplyAll(ctx, perm[:half]); err != nil {
					t.Fatal(err)
				}
				if err := b.ApplyAll(ctx, perm[half:]); err != nil {
					t.Fatal(err)
				}
				if err := a.ApplyAll(ctx, b.Ops()); err != nil {
					t.Fatal(err)
				}
				if err := b.ApplyAll(ctx, a.Ops()); err != nil {
					t.Fatal(err)
				}
				if !interval.Equal(a.Materialize(), b.Materialize()) {
					t.Fatalf("%v did not converge: %v vs %v (ops %v)", s, a.Materialize(), b.Materialize(), ops)
				}
			})
		})
	}
}
