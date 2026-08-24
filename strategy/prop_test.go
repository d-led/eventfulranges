package strategy_test

import (
	"testing"

	"pgregory.net/rapid"

	"gitub.com/d-led/eventfulranges/interval"
	"gitub.com/d-led/eventfulranges/strategy"
)

// TestPropertyMatchesBiogoOracle checks that the CRDT materialization equals a
// sequential replay of the same operations onto a biogo interval tree.
func TestPropertyMatchesBiogoOracle(t *testing.T) {
	t.Parallel()
	strategies := []strategy.Strategy{strategy.LWW, strategy.FWW, strategy.AdditiveWins, strategy.GrowOnly}
	for _, s := range strategies {
		t.Run(s.String(), func(t *testing.T) {
			t.Parallel()
			rapid.Check(t, func(t *rapid.T) {
				ops := genOpList(t)
				oracle := newOracle(s)
				oracle.applyAll(ops)
				want := interval.Normalize(oracle.ranges())
				got := strategy.Materialize(s, ops)
				if !interval.Equal(want, got) {
					t.Fatalf("%v mismatch\n  want %v\n  got  %v\n  ops %v", s, want, got, ops)
				}
			})
		})
	}
}

func TestPropertyOrderIndependence(t *testing.T) {
	t.Parallel()
	strategies := []strategy.Strategy{strategy.LWW, strategy.FWW, strategy.AdditiveWins, strategy.GrowOnly}
	rapid.Check(t, func(t *rapid.T) {
		ops := genOpList(t)
		shuffled := rapid.Permutation(ops).Draw(t, "perm")
		for _, s := range strategies {
			want := strategy.Materialize(s, ops)
			got := strategy.Materialize(s, shuffled)
			if !interval.Equal(want, got) {
				t.Fatalf("%v not order independent: %v vs %v", s, want, got)
			}
		}
	})
}
