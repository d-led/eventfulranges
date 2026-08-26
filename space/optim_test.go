package space

import (
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestDifferenceSortedMatchesDifference(t *testing.T) {
	t.Parallel()
	a := []Box{box2(0, 0, 10, 10), box2(20, 0, 30, 10)}
	b := []Box{box2(2, 2, 5, 5), box2(22, 1, 26, 9), box2(2, 2, 5, 5)}
	require.Equal(t, Difference(a, b), DifferenceSorted(a, Normalize(b)))
}

func TestInsertNormalizedMatchesNormalize(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		set  []Box
		box  Box
	}{
		{"disjoint", []Box{box2(0, 0, 4, 4)}, box2(10, 0, 14, 4)},
		{"subsumes all", []Box{box2(0, 0, 4, 4), box2(10, 0, 14, 4)}, box2(0, 0, 20, 4)},
		{"subsumed by set", []Box{box2(0, 0, 10, 10)}, box2(2, 2, 3, 3)},
		{"partial overlap", []Box{box2(0, 0, 4, 4)}, box2(2, 0, 6, 4)},
		{"empty box", []Box{box2(0, 0, 4, 4)}, box2(1, 1, 1, 2)},
		{"empty set", nil, box2(0, 0, 4, 4)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			set := Normalize(tc.set)
			want := Normalize(append(append([]Box(nil), set...), tc.box))
			require.Equal(t, want, InsertNormalized(set, tc.box))
		})
	}
}

func TestPropertyDifferenceSorted(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		a := rapid.SliceOfN(genBox(t), 0, 6).Draw(t, "a")
		b := rapid.SliceOfN(genBox(t), 0, 6).Draw(t, "b")
		require.Equal(t, Difference(a, b), DifferenceSorted(a, Normalize(b)))
	})
}

func TestPropertyInsertNormalized(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		set := rapid.SliceOfN(genBox(t), 0, 6).Draw(t, "set")
		box := genBox(t).Draw(t, "box")
		norm := Normalize(set)
		want := Normalize(append(append([]Box(nil), norm...), box))
		require.Equal(t, want, InsertNormalized(norm, box))
	})
}
