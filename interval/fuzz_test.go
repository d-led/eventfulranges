package interval_test

import (
	"testing"

	"gitub.com/d-led/eventfulranges/interval"
)

func FuzzNormalize(f *testing.F) {
	f.Add([]byte("eventfulranges"))
	f.Fuzz(func(t *testing.T, data []byte) {
		raw := intervalsFromBytes(data)
		normalized := interval.Normalize(raw)
		requireCanonical(t, normalized)
		for _, x := range samplePoints() {
			if interval.Contains(normalized, x) != anyContains(raw, x) {
				t.Fatalf("normalize changed membership at %v: %v -> %v", x, raw, normalized)
			}
		}
	})
}

func FuzzSubtract(f *testing.F) {
	f.Add([]byte("eventfulranges"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) < 4 {
			return
		}
		ivs := intervalsFromBytes(data[:len(data)/2])
		cuts := intervalsFromBytes(data[len(data)/2:])
		if len(ivs) == 0 || len(cuts) == 0 {
			return
		}
		iv, cut := ivs[0], cuts[0]
		pieces := iv.Subtract(cut)
		requireCanonical(t, pieces)
		for _, x := range samplePoints() {
			want := iv.Contains(x) && !cut.Contains(x)
			got := 0
			for _, p := range pieces {
				if p.Contains(x) {
					got++
				}
			}
			if got > 1 {
				t.Fatalf("subtract pieces overlap at %v: %v", x, pieces)
			}
			if (got == 1) != want {
				t.Fatalf("subtract wrong at %v: %v - %v = %v", x, iv, cut, pieces)
			}
		}
	})
}

func FuzzUnionDifference(f *testing.F) {
	f.Add([]byte("eventfulranges"))
	f.Fuzz(func(t *testing.T, data []byte) {
		half := len(data) / 2
		a := intervalsFromBytes(data[:half])
		b := intervalsFromBytes(data[half:])
		union := interval.Union(a, b)
		diff := interval.Difference(a, b)
		requireCanonical(t, union)
		requireCanonical(t, diff)
		for _, x := range samplePoints() {
			if interval.Contains(union, x) != (anyContains(a, x) || anyContains(b, x)) {
				t.Fatalf("union wrong at %v: %v | %v = %v", x, a, b, union)
			}
			if interval.Contains(diff, x) != (anyContains(a, x) && !anyContains(b, x)) {
				t.Fatalf("difference wrong at %v: %v - %v = %v", x, a, b, diff)
			}
		}
	})
}
