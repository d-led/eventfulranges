package interval_test

import (
	"testing"

	"pgregory.net/rapid"

	"gitub.com/d-led/eventfulranges/interval"
)

func TestPropertyNormalize(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		raw := rapid.SliceOfN(genValidInterval(t), 0, 6).Draw(t, "raw")
		normalized := interval.Normalize(raw)
		requireCanonical(t, normalized)
		for _, x := range samplePoints() {
			if interval.Contains(normalized, x) != anyContains(raw, x) {
				t.Fatalf("normalize changed membership at %v: %v -> %v", x, raw, normalized)
			}
		}
	})
}

func TestPropertySubtract(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		iv := genValidInterval(t).Draw(t, "iv")
		cut := genValidInterval(t).Draw(t, "cut")
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

func TestPropertyUnion(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		a := rapid.SliceOfN(genValidInterval(t), 0, 4).Draw(t, "a")
		b := rapid.SliceOfN(genValidInterval(t), 0, 4).Draw(t, "b")
		union := interval.Union(a, b)
		requireCanonical(t, union)
		for _, x := range samplePoints() {
			want := anyContains(a, x) || anyContains(b, x)
			if interval.Contains(union, x) != want {
				t.Fatalf("union wrong at %v: %v | %v = %v", x, a, b, union)
			}
		}
	})
}

func TestPropertyDifference(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		a := rapid.SliceOfN(genValidInterval(t), 0, 4).Draw(t, "a")
		b := rapid.SliceOfN(genValidInterval(t), 0, 4).Draw(t, "b")
		diff := interval.Difference(a, b)
		requireCanonical(t, diff)
		for _, x := range samplePoints() {
			want := anyContains(a, x) && !anyContains(b, x)
			if interval.Contains(diff, x) != want {
				t.Fatalf("difference wrong at %v: %v - %v = %v", x, a, b, diff)
			}
		}
	})
}
