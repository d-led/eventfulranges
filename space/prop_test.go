package space

import (
	"testing"

	"pgregory.net/rapid"
)

func TestPropertyNormalize(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		raw := rapid.SliceOfN(genBox(t), 0, 6).Draw(t, "raw")
		normalized := Normalize(raw)
		requireCanonical(t, normalized)
		for _, p := range samplePoints() {
			before, after := Contains(raw, p), Contains(normalized, p)
			if before != after {
				t.Fatalf("normalize must cover exactly the same points as its input\n"+
					"point %s\n"+
					"  covered by input:      %t\n"+
					"  covered by normalized: %t\n"+
					"  input:      %s\n"+
					"  normalized: %s",
					renderPoint(p), before, after, renderBoxes(raw), renderBoxes(normalized))
			}
		}
	})
}

func TestPropertyUnion(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		a := rapid.SliceOfN(genBox(t), 0, 4).Draw(t, "a")
		b := rapid.SliceOfN(genBox(t), 0, 4).Draw(t, "b")
		union := Union(a, b)
		requireCanonical(t, union)
		for _, p := range samplePoints() {
			want := Contains(a, p) || Contains(b, p)
			got := Contains(union, p)
			if got != want {
				t.Fatalf("union must cover a point exactly when either operand covers it\n"+
					"point %s\n"+
					"  covered by union: %t\n"+
					"  expected:         %t\n"+
					"  a:     %s\n"+
					"  b:     %s\n"+
					"  union: %s",
					renderPoint(p), got, want, renderBoxes(a), renderBoxes(b), renderBoxes(union))
			}
		}
	})
}

func TestPropertyDifference(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		a := rapid.SliceOfN(genBox(t), 0, 4).Draw(t, "a")
		b := rapid.SliceOfN(genBox(t), 0, 4).Draw(t, "b")
		diff := Difference(a, b)
		requireCanonical(t, diff)
		for _, p := range samplePoints() {
			want := Contains(a, p) && !Contains(b, p)
			got := Contains(diff, p)
			if got != want {
				t.Fatalf("difference must cover a point exactly when a covers it and b does not\n"+
					"point %s\n"+
					"  covered by difference: %t\n"+
					"  expected:              %t\n"+
					"  a:          %s\n"+
					"  b:          %s\n"+
					"  difference: %s",
					renderPoint(p), got, want, renderBoxes(a), renderBoxes(b), renderBoxes(diff))
			}
		}
	})
}
