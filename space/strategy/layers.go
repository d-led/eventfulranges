package strategy

import (
	"sort"

	"github.com/d-led/eventfulranges/space"
	"github.com/d-led/eventfulranges/space/op"
)

// Layer is one entry of a layered front: a box painted in order, later layers
// over earlier ones. Kind says whether the layer paints (KindAdd) or erases
// (KindRemove, which paints the background over whatever lies beneath it).
type Layer struct {
	Box  space.Box `json:"box"`
	Kind op.Kind   `json:"kind"`
}

// Layers projects the operations into a culled, layered front under
// last-write-wins priority: the effective operations in bottom-to-top paint
// order, each box kept whole, with every box fully covered by a higher layer
// removed.
//
// It is the painter's-algorithm counterpart of Materialize. Materialize
// returns a non-overlapping cover, so a small stroke inside a big box carves
// the big box into strips; the front instead keeps the big box whole and
// layers the stroke on top. A Remove layer erases by painting the background,
// and a later Add layer repaints over an earlier erase exactly as a later
// stroke repaints over an earlier one.
func Layers(ops []op.Op) []Layer {
	effective := Effective(ops)
	sort.Slice(effective, func(i, j int) bool {
		return better(LWW, winnerOf(effective[i]), winnerOf(effective[j]))
	})
	var covered []space.Box
	layers := make([]Layer, 0, len(effective))
	for _, o := range effective {
		free := space.DifferenceSorted([]space.Box{o.Box}, covered)
		covered = space.InsertNormalized(covered, o.Box)
		if len(free) == 0 {
			continue // fully covered by a higher layer: cull
		}
		layers = append(layers, Layer{Box: o.Box, Kind: o.Kind})
	}
	// The sweep visits higher layers first; flip to bottom-to-top so a
	// consumer paints in slice order.
	for i, j := 0, len(layers)-1; i < j; i, j = i+1, j-1 {
		layers[i], layers[j] = layers[j], layers[i]
	}
	return layers
}

// winnerOf lifts an operation to the priority tuple the sweep orders by.
func winnerOf(o op.Op) Winner {
	return Winner{TS: o.TS, ID: o.ID, Kind: o.Kind}
}
