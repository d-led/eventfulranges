package strategy_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/d-led/eventfulranges/space"
	"github.com/d-led/eventfulranges/space/strategy"
)

// requireCanonical asserts the cover is sorted, non-empty and free of
// subsumed boxes.
func requireCanonical(t *testing.T, set []space.Box) {
	t.Helper()
	for _, b := range set {
		require.False(t, b.Empty(), "empty box in canonical set: %v", b)
	}
	for i := 1; i < len(set); i++ {
		if !space.Less(set[i-1], set[i]) {
			t.Fatalf("not sorted: %v then %v", set[i-1], set[i])
		}
	}
}

func FuzzLWW(f *testing.F) {
	f.Add([]byte("eventfulranges"))
	f.Fuzz(func(t *testing.T, data []byte) {
		ops := opsFromBytes(data)
		got := strategy.Materialize(strategy.LWW, ops)
		requireCanonical(t, got)
		for _, p := range samplePoints() {
			if space.Contains(got, p) != lwwMember(ops, p) {
				t.Fatalf("lww mismatch at %v: %v -> %v", p, ops, got)
			}
		}
	})
}

func FuzzFWW(f *testing.F) {
	f.Add([]byte("eventfulranges"))
	f.Fuzz(func(t *testing.T, data []byte) {
		ops := opsFromBytes(data)
		got := strategy.Materialize(strategy.FWW, ops)
		requireCanonical(t, got)
		for _, p := range samplePoints() {
			if space.Contains(got, p) != fwwMember(ops, p) {
				t.Fatalf("fww mismatch at %v: %v -> %v", p, ops, got)
			}
		}
	})
}

func FuzzAdditiveWins(f *testing.F) {
	f.Add([]byte("eventfulranges"))
	f.Fuzz(func(t *testing.T, data []byte) {
		ops := opsFromBytes(data)
		got := strategy.Materialize(strategy.AdditiveWins, ops)
		requireCanonical(t, got)
		for _, p := range samplePoints() {
			if space.Contains(got, p) != additiveMember(ops, p) {
				t.Fatalf("additive mismatch at %v: %v -> %v", p, ops, got)
			}
		}
	})
}

func FuzzGrowOnly(f *testing.F) {
	f.Add([]byte("eventfulranges"))
	f.Fuzz(func(t *testing.T, data []byte) {
		ops := opsFromBytes(data)
		got := strategy.Materialize(strategy.GrowOnly, ops)
		requireCanonical(t, got)
		for _, p := range samplePoints() {
			if space.Contains(got, p) != growMember(ops, p) {
				t.Fatalf("grow mismatch at %v: %v -> %v", p, ops, got)
			}
		}
	})
}
