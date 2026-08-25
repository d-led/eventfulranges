package strategy_test

import (
	"testing"

	"pgregory.net/rapid"

	"github.com/d-led/eventfulranges/space"
	"github.com/d-led/eventfulranges/space/strategy"
)

func TestPropertyMatchesPointOracle(t *testing.T) {
	t.Parallel()
	strategies := []strategy.Strategy{strategy.LWW, strategy.FWW, strategy.AdditiveWins, strategy.GrowOnly}
	for _, s := range strategies {
		t.Run(s.String(), func(t *testing.T) {
			t.Parallel()
			rapid.Check(t, func(t *rapid.T) {
				ops := genOps(t)
				got := strategy.Materialize(s, ops)
				for _, p := range samplePoints() {
					if space.Contains(got, p) != member(s, ops, p) {
						t.Fatalf("%v mismatch at %v\n  ops: %v\n  got: %v", s, p, ops, got)
					}
				}
			})
		})
	}
}

func TestPropertyOrderIndependence(t *testing.T) {
	t.Parallel()
	strategies := []strategy.Strategy{strategy.LWW, strategy.FWW, strategy.AdditiveWins, strategy.GrowOnly}
	for _, s := range strategies {
		t.Run(s.String(), func(t *testing.T) {
			t.Parallel()
			rapid.Check(t, func(t *rapid.T) {
				ops := genOps(t)
				perm := rapid.Permutation(ops).Draw(t, "perm")
				want := strategy.Materialize(s, ops)
				got := strategy.Materialize(s, perm)
				if !space.Equal(want, got) {
					t.Fatalf("%v not order independent\n  ops:  %v\n  want: %v\n  got:  %v", s, ops, want, got)
				}
			})
		})
	}
}
