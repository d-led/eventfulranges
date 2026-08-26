package space

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestConnectedComponents(t *testing.T) {
	t.Parallel()
	t.Run("disjoint boxes are separate", func(t *testing.T) {
		t.Parallel()
		comps := ConnectedComponents([]Box{box2(0, 0, 2, 2), box2(3, 0, 5, 2)})
		require.Len(t, comps, 2)
	})
	t.Run("overlapping boxes are one", func(t *testing.T) {
		t.Parallel()
		comps := ConnectedComponents([]Box{box2(0, 0, 2, 2), box2(1, 1, 3, 3)})
		require.Len(t, comps, 1)
		require.Len(t, comps[0], 2)
	})
	t.Run("face-touching boxes are one", func(t *testing.T) {
		t.Parallel()
		comps := ConnectedComponents([]Box{box2(0, 0, 2, 4), box2(2, 0, 4, 3)})
		require.Len(t, comps, 1)
	})
	t.Run("corner-touching boxes are separate", func(t *testing.T) {
		t.Parallel()
		comps := ConnectedComponents([]Box{box2(0, 0, 2, 2), box2(2, 2, 4, 4)})
		require.Len(t, comps, 2)
	})
	t.Run("a chain collapses into one", func(t *testing.T) {
		t.Parallel()
		comps := ConnectedComponents([]Box{box2(0, 0, 2, 2), box2(2, 0, 4, 2), box2(4, 0, 6, 2)})
		require.Len(t, comps, 1)
		require.Len(t, comps[0], 3)
	})
	t.Run("empty input has no components", func(t *testing.T) {
		t.Parallel()
		require.Empty(t, ConnectedComponents(nil))
	})
}

func TestConnected(t *testing.T) {
	t.Parallel()
	t.Run("dims mismatch is not connected", func(t *testing.T) {
		t.Parallel()
		require.False(t, connected(box2(0, 0, 1, 1), NewBox([]float64{0}, []float64{1})))
	})
	t.Run("gap is not connected", func(t *testing.T) {
		t.Parallel()
		require.False(t, connected(box2(0, 0, 1, 1), box2(3, 0, 4, 1)))
	})
	t.Run("corner touch is not connected", func(t *testing.T) {
		t.Parallel()
		require.False(t, connected(box2(0, 0, 2, 2), box2(2, 2, 4, 4)))
	})
	t.Run("overlap is connected", func(t *testing.T) {
		t.Parallel()
		require.True(t, connected(box2(0, 0, 2, 2), box2(1, 1, 3, 3)))
	})
	t.Run("face touch is connected", func(t *testing.T) {
		t.Parallel()
		require.True(t, connected(box2(0, 0, 2, 4), box2(2, 0, 4, 3)))
	})
}

func TestPropertyConnectedComponentsPartition(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		raw := rapid.SliceOfN(genBox(t), 0, 8).Draw(t, "raw")
		comps := ConnectedComponents(raw)
		total := 0
		for _, c := range comps {
			if len(c) == 0 {
				t.Fatalf("empty component in %v", renderBoxes(raw))
			}
			total += len(c)
		}
		if total != len(Normalize(raw)) {
			t.Fatalf("components must partition the normalized boxes: got %d boxes, want %d\n  raw: %s",
				total, len(Normalize(raw)), renderBoxes(raw))
		}
	})
}

func FuzzConnectedComponents(f *testing.F) {
	f.Add([]byte{0, 1, 2, 3, 4, 5, 6, 7})
	f.Fuzz(func(t *testing.T, data []byte) {
		boxes := boxesFromBytes(data)
		comps := ConnectedComponents(boxes)
		total := 0
		for _, c := range comps {
			total += len(c)
		}
		if total != len(Normalize(boxes)) {
			t.Fatalf("components must partition the normalized boxes: got %d boxes, want %d", total, len(Normalize(boxes)))
		}
		for i := 0; i < len(comps); i++ {
			for j := i + 1; j < len(comps); j++ {
				for _, a := range comps[i] {
					for _, b := range comps[j] {
						if connected(a, b) {
							t.Fatalf("boxes in different components are connected: %v and %v", a, b)
						}
					}
				}
			}
		}
	})
}

func ExampleConnectedComponents() {
	busy := []Box{
		NewBox([]float64{0, 0}, []float64{2, 4}),
		NewBox([]float64{2, 0}, []float64{4, 4}),
		NewBox([]float64{5, 0}, []float64{7, 2}),
	}
	fmt.Println(len(ConnectedComponents(busy)), "contiguous regions")
	// Output: 2 contiguous regions
}
