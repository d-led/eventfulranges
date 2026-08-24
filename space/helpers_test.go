package space

import (
	"strconv"
	"strings"

	"pgregory.net/rapid"
)

// genBox draws a 2D box with integer corners on the -2..2 grid.
func genBox(t *rapid.T) *rapid.Generator[Box] {
	return rapid.Custom(func(t *rapid.T) Box {
		x0 := rapid.IntRange(-2, 2).Draw(t, "x0")
		x1 := rapid.IntRange(x0, 2).Draw(t, "x1")
		y0 := rapid.IntRange(-2, 2).Draw(t, "y0")
		y1 := rapid.IntRange(y0, 2).Draw(t, "y1")
		return box2(float64(x0), float64(y0), float64(x1), float64(y1))
	})
}

// samplePoints returns a dense 2D grid that catches boundary errors.
func samplePoints() [][]float64 {
	var pts [][]float64
	for x := -4.0; x <= 4.0; x += 0.5 {
		for y := -4.0; y <= 4.0; y += 0.5 {
			pts = append(pts, []float64{x, y})
		}
	}
	return pts
}

// boxesFromBytes derives 2D integer boxes deterministically.
func boxesFromBytes(data []byte) []Box {
	if len(data) < 4 {
		return nil
	}
	n := int(data[0] % 4)
	out := make([]Box, 0, n)
	for i := 0; i < n && i*4+4 <= len(data); i++ {
		x0 := int(data[i*4]%5) - 2
		x1 := int(data[i*4+1]%5) - 2
		y0 := int(data[i*4+2]%5) - 2
		y1 := int(data[i*4+3]%5) - 2
		if x0 > x1 {
			x0, x1 = x1, x0
		}
		if y0 > y1 {
			y0, y1 = y1, y0
		}
		out = append(out, box2(float64(x0), float64(y0), float64(x1), float64(y1)))
	}
	return out
}

// renderBoxes formats a set of boxes in decimal, e.g. "[0 2) x [1 3), [5 6) x [7 8)".
func renderBoxes(boxes []Box) string {
	if len(boxes) == 0 {
		return "(empty)"
	}
	parts := make([]string, len(boxes))
	for i, b := range boxes {
		parts[i] = b.String()
	}
	return strings.Join(parts, ", ")
}

// renderPoint formats a point in decimal, e.g. "(1, 2.5)".
func renderPoint(p []float64) string {
	parts := make([]string, len(p))
	for i, v := range p {
		parts[i] = strconv.FormatFloat(v, 'f', -1, 64)
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

// requireCanonical asserts that the set is sorted, non-empty and free of
// subsumed boxes.
func requireCanonical(t *rapid.T, set []Box) {
	t.Helper()
	for _, b := range set {
		if b.Empty() {
			t.Fatalf("empty box in canonical set: %v", b)
		}
	}
	for i := 1; i < len(set); i++ {
		if !lessBox(set[i-1], set[i]) {
			t.Fatalf("not sorted: %v then %v", set[i-1], set[i])
		}
	}
	for i := range set {
		for j := range set {
			if i != j && subsumes(set[j], set[i]) {
				t.Fatalf("subsumed box in canonical set: %v inside %v", set[i], set[j])
			}
		}
	}
}
