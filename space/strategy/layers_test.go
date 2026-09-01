package strategy_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/d-led/eventfulranges/space/op"
	"github.com/d-led/eventfulranges/space/strategy"
)

func TestLayersKeepsBigSquareWhole(t *testing.T) {
	t.Parallel()
	red := box(0, 0, 10, 10).WithMeta(json.RawMessage(`{"color":"#ff0000"}`))
	blue := box(4, 4, 6, 6).WithMeta(json.RawMessage(`{"color":"#0000ff"}`))
	got := strategy.Layers([]op.Op{
		{ID: "a", Kind: op.KindAdd, TS: 1, Box: red},
		{ID: "b", Kind: op.KindAdd, TS: 2, Box: blue},
	})
	require.Len(t, got, 2)
	require.Equal(t, op.KindAdd, got[0].Kind)
	require.Equal(t, red, got[0].Box, "the big square stays one whole box")
	require.Equal(t, blue, got[1].Box, "the pixel layers on top, in paint order")
}

func TestLayersCullsCoveredOps(t *testing.T) {
	t.Parallel()
	got := strategy.Layers([]op.Op{
		{ID: "a", Kind: op.KindAdd, TS: 1, Box: box(0, 0, 2, 2)},
		{ID: "b", Kind: op.KindAdd, TS: 2, Box: box(4, 4, 6, 6)},
		{ID: "c", Kind: op.KindAdd, TS: 3, Box: box(0, 0, 10, 10)},
	})
	require.Len(t, got, 1, "the two earlier squares are fully covered")
	require.Equal(t, box(0, 0, 10, 10), got[0].Box)
}

func TestLayersKeepsEraseAsLayer(t *testing.T) {
	t.Parallel()
	got := strategy.Layers([]op.Op{
		{ID: "a", Kind: op.KindAdd, TS: 1, Box: box(0, 0, 10, 10)},
		{ID: "b", Kind: op.KindRemove, TS: 2, Box: box(4, 4, 6, 6)},
	})
	require.Len(t, got, 2)
	require.Equal(t, op.KindAdd, got[0].Kind)
	require.Equal(t, box(0, 0, 10, 10), got[0].Box)
	require.Equal(t, op.KindRemove, got[1].Kind)
	require.Equal(t, box(4, 4, 6, 6), got[1].Box)
}

func TestLayersCullsEraseRepaintedOver(t *testing.T) {
	t.Parallel()
	got := strategy.Layers([]op.Op{
		{ID: "a", Kind: op.KindAdd, TS: 1, Box: box(0, 0, 10, 10)},
		{ID: "b", Kind: op.KindRemove, TS: 2, Box: box(4, 4, 6, 6)},
		{ID: "c", Kind: op.KindAdd, TS: 3, Box: box(4, 4, 6, 6)},
	})
	require.Equal(t, []strategy.Layer{
		{Box: box(0, 0, 10, 10), Kind: op.KindAdd},
		{Box: box(4, 4, 6, 6), Kind: op.KindAdd},
	}, got, "the erase is invisible under the repaint")
}

func TestLayersCullsFullyErasedLayer(t *testing.T) {
	t.Parallel()
	got := strategy.Layers([]op.Op{
		{ID: "a", Kind: op.KindAdd, TS: 1, Box: box(0, 0, 10, 10)},
		{ID: "b", Kind: op.KindRemove, TS: 2, Box: box(0, 0, 10, 10)},
	})
	require.Len(t, got, 1, "the painted square is fully erased")
	require.Equal(t, op.KindRemove, got[0].Kind)
}

func TestLayersResolvesRetractions(t *testing.T) {
	t.Parallel()
	got := strategy.Layers([]op.Op{
		{ID: "a", Kind: op.KindAdd, TS: 1, Box: box(0, 0, 10, 10)},
		{ID: "r", Kind: op.KindRetract, TS: 2, Ref: "a", Box: box(0, 0, 10, 10)},
	})
	require.Empty(t, got, "a retracted op leaves nothing to paint")
}

func TestLayersEmptyInput(t *testing.T) {
	t.Parallel()
	require.Empty(t, strategy.Layers(nil))
	require.Empty(t, strategy.Layers([]op.Op{}))
}

func TestLayersOrderIsBottomToTop(t *testing.T) {
	t.Parallel()
	got := strategy.Layers([]op.Op{
		{ID: "a", Kind: op.KindAdd, TS: 1, Box: box(0, 0, 4, 4)},
		{ID: "b", Kind: op.KindAdd, TS: 2, Box: box(4, 0, 8, 4)},
	})
	require.Len(t, got, 2)
	require.Equal(t, box(0, 0, 4, 4), got[0].Box)
	require.Equal(t, box(4, 0, 8, 4), got[1].Box)
}
