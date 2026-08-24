package strategy_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"gitub.com/d-led/eventfulranges/interval"
	"gitub.com/d-led/eventfulranges/op"
	"gitub.com/d-led/eventfulranges/strategy"
)

// requireCanonicalSet asserts sorted, non-overlapping, valid intervals.
func requireCanonicalSet(t *testing.T, set []interval.Interval) {
	t.Helper()
	for _, iv := range set {
		require.NoError(t, iv.Validate())
	}
	for i := 1; i < len(set); i++ {
		if set[i-1].Overlaps(set[i]) || !interval.Less(set[i-1], set[i]) {
			t.Fatalf("not canonical: %v then %v in %v", set[i-1], set[i], set)
		}
	}
}

func FuzzLWW(f *testing.F) {
	f.Add([]byte("eventfulranges"))
	f.Fuzz(func(t *testing.T, data []byte) {
		ops := opsFromBytes(data)
		got := strategy.Materialize(strategy.LWW, ops)
		requireCanonicalSet(t, got)
		reversed := reverse(ops)
		if !interval.Equal(got, strategy.Materialize(strategy.LWW, reversed)) {
			t.Fatalf("lww not order independent: %v", ops)
		}
		for _, x := range samplePoints() {
			if interval.Contains(got, x) != lwwMember(ops, x) {
				t.Fatalf("lww mismatch at %v: %v -> %v", x, ops, got)
			}
		}
	})
}

func FuzzFWW(f *testing.F) {
	f.Add([]byte("eventfulranges"))
	f.Fuzz(func(t *testing.T, data []byte) {
		ops := opsFromBytes(data)
		got := strategy.Materialize(strategy.FWW, ops)
		requireCanonicalSet(t, got)
		if !interval.Equal(got, strategy.Materialize(strategy.FWW, reverse(ops))) {
			t.Fatalf("fww not order independent: %v", ops)
		}
		for _, x := range samplePoints() {
			if interval.Contains(got, x) != fwwMember(ops, x) {
				t.Fatalf("fww mismatch at %v: %v -> %v", x, ops, got)
			}
		}
	})
}

func FuzzAdditive(f *testing.F) {
	f.Add([]byte("eventfulranges"))
	f.Fuzz(func(t *testing.T, data []byte) {
		ops := opsFromBytes(data)
		got := strategy.Materialize(strategy.AdditiveWins, ops)
		requireCanonicalSet(t, got)
		for _, x := range samplePoints() {
			if interval.Contains(got, x) != additiveMember(ops, x) {
				t.Fatalf("additive mismatch at %v: %v -> %v", x, ops, got)
			}
		}
	})
}

func reverse(ops []op.Op) []op.Op {
	out := make([]op.Op, len(ops))
	for i, o := range ops {
		out[len(ops)-1-i] = o
	}
	return out
}
