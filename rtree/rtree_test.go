package rtree

import (
	"bytes"
	"math"
	"sort"
	"testing"

	"pgregory.net/rapid"

	"github.com/d-led/eventfulranges/space"
)

// genBox draws a 2D box with integer corners in [lo, hi], half-open by default
// and occasionally fully closed so both bound conventions are exercised.
func genBox(lo, hi int) *rapid.Generator[space.Box] {
	return rapid.Custom(func(t *rapid.T) space.Box {
		x0 := rapid.IntRange(lo, hi).Draw(t, "x0")
		x1 := rapid.IntRange(x0, hi).Draw(t, "x1")
		y0 := rapid.IntRange(lo, hi).Draw(t, "y0")
		y1 := rapid.IntRange(y0, hi).Draw(t, "y1")
		if rapid.Bool().Draw(t, "closed") {
			return space.NewBoxWithBounds(
				[]float64{float64(x0), float64(y0)},
				[]float64{float64(x1), float64(y1)},
				[]space.Bound{space.Closed, space.Closed},
				[]space.Bound{space.Closed, space.Closed},
			)
		}
		return space.NewBox([]float64{float64(x0), float64(y0)}, []float64{float64(x1), float64(y1)})
	})
}

// reference answers the query with a linear scan, the oracle the tree must
// match exactly.
func reference(boxes []space.Box, q space.Box) []space.Box {
	var out []space.Box
	for _, b := range boxes {
		if !b.Empty() && space.Overlaps(b, q) {
			out = append(out, b)
		}
	}
	sort.Slice(out, func(i, j int) bool { return lessBox(out[i], out[j]) })
	return out
}

func boxEqual(a, b space.Box) bool {
	if len(a.Min) != len(b.Min) || len(a.Max) != len(b.Max) {
		return false
	}
	for i := range a.Min {
		if a.Min[i] != b.Min[i] || a.Max[i] != b.Max[i] {
			return false
		}
		if loBound(a, i) != loBound(b, i) || hiBound(a, i) != hiBound(b, i) {
			return false
		}
	}
	return bytes.Equal(a.Meta, b.Meta)
}

func requireSameBoxes(t failer, got, want []space.Box) {
	if len(got) != len(want) {
		t.Fatalf("search returned %d boxes, want %d\ngot:  %v\nwant: %v", len(got), len(want), got, want)
	}
	for i := range got {
		if !boxEqual(got[i], want[i]) {
			t.Fatalf("result %d differs\ngot:  %v\nwant: %v", i, got[i], want[i])
		}
	}
}

// failer is the reporting surface both *testing.T and *rapid.T share, so the
// oracle comparison works in plain tests and property tests alike.
type failer interface {
	Fatalf(format string, args ...any)
}

func TestBuildEmpty(t *testing.T) {
	t.Parallel()
	tree, err := Build(nil)
	if err != nil {
		t.Fatalf("Build(nil): %v", err)
	}
	if tree.Len() != 0 || tree.Dims() != 0 {
		t.Fatalf("empty tree: Len=%d Dims=%d", tree.Len(), tree.Dims())
	}
	if got := tree.Search(space.NewBox([]float64{0}, []float64{1})); got != nil {
		t.Fatalf("search on empty tree: got %v, want nil", got)
	}
}

func TestBuildSkipsEmptyAndValidates(t *testing.T) {
	t.Parallel()

	// A degenerate point [2,2) is empty and must be dropped, not an error.
	point := space.NewBox([]float64{2, 2}, []float64{2, 2})
	ok := space.NewBox([]float64{0, 0}, []float64{1, 1})
	tree, err := Build([]space.Box{point, ok})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if tree.Len() != 1 {
		t.Fatalf("empty box must be dropped, Len=%d", tree.Len())
	}

	if _, err := Build([]space.Box{
		space.NewBox([]float64{0}, []float64{1}),
		space.NewBox([]float64{0, 0}, []float64{1, 1}),
	}); err == nil {
		t.Fatal("mixed dimensionality must fail")
	}

	nan := space.NewBox([]float64{math.NaN(), 0}, []float64{1, 1})
	if _, err := Build([]space.Box{nan}); err == nil {
		t.Fatal("NaN coordinate must fail validation")
	}
}

func TestSearchSingle(t *testing.T) {
	t.Parallel()
	tree, err := Build([]space.Box{space.NewBox([]float64{0, 0}, []float64{2, 2})})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	requireSameBoxes(t, tree.Search(space.NewBox([]float64{1, 1}, []float64{3, 3})), []space.Box{
		space.NewBox([]float64{0, 0}, []float64{2, 2}),
	})
	requireSameBoxes(t, tree.Search(space.NewBox([]float64{3, 3}, []float64{4, 4})), nil)
	requireSameBoxes(t, tree.Search(space.NewBox([]float64{0}, []float64{1})), nil) // dims mismatch
}

func TestSearchClosedFaces(t *testing.T) {
	t.Parallel()
	closed := space.NewBoxWithBounds(
		[]float64{0}, []float64{1},
		[]space.Bound{space.Closed}, []space.Bound{space.Closed},
	)
	tree, err := Build([]space.Box{closed})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	// [1,2) shares the point 1 with the closed box [0,1]: a closed face is
	// included, so the search must not be pruned away.
	q := space.NewBox([]float64{1}, []float64{2})
	requireSameBoxes(t, tree.Search(q), []space.Box{closed})

	halfOpen := space.NewBox([]float64{0}, []float64{1}) // [0,1) excludes 1
	tree2, err := Build([]space.Box{halfOpen})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	requireSameBoxes(t, tree2.Search(q), nil)
}

func TestSearchDeepTreeMatchesLinearScan(t *testing.T) {
	t.Parallel()
	// Enough boxes to force several levels of median splits, with duplicates
	// and closed faces to exercise the tie-breaks.
	var boxes []space.Box
	for x := -3; x <= 3; x++ {
		for y := -3; y <= 3; y++ {
			boxes = append(boxes, space.NewBox(
				[]float64{float64(x), float64(y)},
				[]float64{float64(x) + 1, float64(y) + 1},
			))
		}
	}
	boxes = append(boxes, space.NewBoxWithBounds(
		[]float64{-3, -3}, []float64{3, 3},
		[]space.Bound{space.Closed, space.Closed}, []space.Bound{space.Closed, space.Closed},
	))
	tree, err := Build(boxes)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if tree.Len() != len(boxes) {
		t.Fatalf("Len=%d, want %d", tree.Len(), len(boxes))
	}
	for _, q := range []space.Box{
		space.NewBox([]float64{0, 0}, []float64{1, 1}),
		space.NewBox([]float64{-4, -4}, []float64{-3, -3}),
		space.NewBox([]float64{4, 4}, []float64{5, 5}),
		space.NewBox([]float64{-4, -4}, []float64{4, 4}),
		space.NewBoxWithBounds(
			[]float64{-3}, []float64{3},
			[]space.Bound{space.Closed}, []space.Bound{space.Closed},
		),
	} {
		// A 1D query must match nothing; the rest must match the oracle.
		if q.Dims() == 1 {
			requireSameBoxes(t, tree.Search(q), nil)
			continue
		}
		requireSameBoxes(t, tree.Search(q), reference(boxes, q))
	}
}

func TestBuildCopiesInputAndResult(t *testing.T) {
	t.Parallel()
	b := space.NewBox([]float64{0}, []float64{1})
	tree, err := Build([]space.Box{b})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	b.Min[0] = 99 // mutate the caller's box after building
	got := tree.Search(space.NewBox([]float64{0.5}, []float64{1.5}))
	if len(got) != 1 || got[0].Min[0] != 0 {
		t.Fatalf("tree must own its geometry, got %v", got)
	}
	got[0].Min[0] = 42 // mutate a returned box
	again := tree.Search(space.NewBox([]float64{0.5}, []float64{1.5}))
	if again[0].Min[0] != 0 {
		t.Fatalf("returned boxes must be copies, got %v", again)
	}
}

func TestWidestAxisPrefersFirstOnTie(t *testing.T) {
	t.Parallel()
	boxes := []space.Box{
		space.NewBox([]float64{0, 0}, []float64{1, 1}),
		space.NewBox([]float64{1, 1}, []float64{2, 2}),
	}
	if got := widestAxis(boxes); got != 0 {
		t.Fatalf("widestAxis on tied axes = %d, want 0", got)
	}
}

func TestLessBoxIsATotalOrder(t *testing.T) {
	t.Parallel()
	a := space.NewBoxWithBounds(
		[]float64{0, 0}, []float64{1, 1},
		[]space.Bound{space.Closed, space.Closed}, []space.Bound{space.Closed, space.Closed},
	).WithMeta([]byte(`{"k":"a"}`))
	b := space.NewBox([]float64{0, 0}, []float64{1, 1}).WithMeta([]byte(`{"k":"a"}`))
	c := space.NewBox([]float64{0, 0}, []float64{1, 1}).WithMeta([]byte(`{"k":"b"}`))
	d := space.NewBox([]float64{0, 0}, []float64{2, 1}).WithMeta([]byte(`{"k":"a"}`))

	// Distinct coordinates, bounds, upper corners and metadata must all order.
	if !lessBox(b, d) { // same lower corner, d reaches further on the first axis
		t.Fatal("upper corner must order boxes")
	}
	if !lessBox(a, b) { // a closed face sorts before an open one at the same corner
		t.Fatal("face bounds must order boxes")
	}
	if !lessBox(b, c) { // metadata breaks the final tie
		t.Fatal("metadata must order boxes")
	}
	if lessBox(a, a) {
		t.Fatal("a box must not be less than itself")
	}

	// A differing lower face must order too: [0,1) has a closed lower face,
	// (0,1] an open one, at the same corner.
	e := space.NewBox([]float64{0}, []float64{1})
	f := space.NewBoxWithBounds(
		[]float64{0}, []float64{1},
		[]space.Bound{space.Open}, []space.Bound{space.Closed},
	)
	if !lessBox(e, f) {
		t.Fatal("lower face bound must order boxes")
	}
}

func TestPropertySearchMatchesLinearScan(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		boxes := rapid.SliceOfN(genBox(-3, 3), 0, 40).Draw(t, "boxes")
		q := genBox(-4, 4).Draw(t, "query")
		tree, err := Build(boxes)
		if err != nil {
			t.Fatalf("Build: %v", err)
		}
		requireSameBoxes(t, tree.Search(q), reference(boxes, q))
	})
}
