package engine_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/d-led/eventfulranges/space/engine"
	"github.com/d-led/eventfulranges/space/op"
	"github.com/d-led/eventfulranges/space/store/memory"
	"github.com/d-led/eventfulranges/space/strategy"
)

func TestRetractRebuildsView(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	e, err := engine.Open(ctx, memory.New(), strategy.AdditiveWins)
	require.NoError(t, err)

	require.NoError(t, e.Apply(ctx, op.Op{ID: "a", Kind: op.KindAdd, Box: box(0, 0, 4, 4)}))
	require.NoError(t, e.Apply(ctx, op.Op{ID: "e", Kind: op.KindRemove, Box: box(1, 1, 3, 3)}))
	require.False(t, e.Contains([]float64{2, 2}), "erased first")

	require.NoError(t, e.Apply(ctx, op.Op{ID: "r", Kind: op.KindRetract, Ref: "e", Box: box(1, 1, 3, 3)}))
	require.True(t, e.Contains([]float64{2, 2}), "undoing the erase restores the hole")
}

func TestOpLookup(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	e, err := engine.Open(ctx, memory.New(), strategy.LWW)
	require.NoError(t, err)

	o := op.Op{ID: "a", Kind: op.KindAdd, Box: box(0, 0, 2, 2)}
	require.NoError(t, e.Apply(ctx, o))

	got, ok := e.Op("a")
	require.True(t, ok)
	require.Equal(t, "a", got.ID)
	require.Equal(t, op.KindAdd, got.Kind)
	require.Equal(t, box(0, 0, 2, 2), got.Box)

	_, ok = e.Op("missing")
	require.False(t, ok)
}
