package space

import "testing"

func FuzzNormalize(f *testing.F) {
	f.Add([]byte{0, 1, 2, 3, 4, 5, 6, 7})
	f.Fuzz(func(t *testing.T, data []byte) {
		boxes := boxesFromBytes(data)
		normalized := Normalize(boxes)
		for _, p := range samplePoints() {
			before, after := Contains(boxes, p), Contains(normalized, p)
			if before != after {
				t.Fatalf("normalize must cover exactly the same points as its input\n"+
					"point %s\n  covered by input: %t\n  covered by normalized: %t\n  input: %s\n  normalized: %s",
					renderPoint(p), before, after, renderBoxes(boxes), renderBoxes(normalized))
			}
		}
	})
}

func FuzzDifference(f *testing.F) {
	f.Add([]byte{0, 1, 2, 3, 4, 5, 6, 7})
	f.Fuzz(func(t *testing.T, data []byte) {
		half := len(data) / 2
		a := boxesFromBytes(data[:half])
		b := boxesFromBytes(data[half:])
		diff := Difference(a, b)
		for _, p := range samplePoints() {
			want := Contains(a, p) && !Contains(b, p)
			got := Contains(diff, p)
			if got != want {
				t.Fatalf("difference must cover a point exactly when a covers it and b does not\n"+
					"point %s\n  covered by difference: %t\n  expected: %t\n  a: %s\n  b: %s\n  difference: %s",
					renderPoint(p), got, want, renderBoxes(a), renderBoxes(b), renderBoxes(diff))
			}
		}
	})
}
