package engine_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/d-led/eventfulranges/meta"
	"github.com/d-led/eventfulranges/space/engine"
	"github.com/d-led/eventfulranges/space/op"
	"github.com/d-led/eventfulranges/space/store/memory"
	"github.com/d-led/eventfulranges/space/strategy"
)

func TestMetaMergeConvergesRegardlessOfOrder(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	red := json.RawMessage(`{"color":"#ff0000"}`)
	alice := json.RawMessage(`{"author":"alice"}`)

	first, err := engine.Open(ctx, memory.New(), strategy.AdditiveWins)
	require.NoError(t, err)
	second, err := engine.Open(ctx, memory.New(), strategy.AdditiveWins)
	require.NoError(t, err)

	require.NoError(t, first.Apply(ctx, op.Op{ID: "a", Kind: op.KindAdd, Box: box(0, 0, 4, 4).WithMeta(red)}))
	require.NoError(t, first.Apply(ctx, op.Op{ID: "b", Kind: op.KindAdd, Box: box(0, 0, 8, 8).WithMeta(alice)}))

	require.NoError(t, second.Apply(ctx, op.Op{ID: "b", Kind: op.KindAdd, Box: box(0, 0, 8, 8).WithMeta(alice)}))
	require.NoError(t, second.Apply(ctx, op.Op{ID: "a", Kind: op.KindAdd, Box: box(0, 0, 4, 4).WithMeta(red)}))

	require.Equal(t, first.Materialize(), second.Materialize(), "replicas must converge, metadata included")
	require.JSONEq(t, `{"author":"alice","color":"#ff0000"}`, string(first.Materialize()[0].Meta))
}

func TestMetaMergeIsPluggable(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	constant := meta.Merge(func(_, _ json.RawMessage) json.RawMessage {
		return json.RawMessage(`{"merged":true}`)
	})

	e, err := engine.Open(ctx, memory.New(), strategy.AdditiveWins, engine.WithMetaMerge(constant))
	require.NoError(t, err)

	require.NoError(t, e.Apply(ctx, op.Op{ID: "a", Kind: op.KindAdd, Box: box(0, 0, 4, 4).WithMeta(json.RawMessage(`{"color":"#ff0000"}`))}))
	require.NoError(t, e.Apply(ctx, op.Op{ID: "b", Kind: op.KindAdd, Box: box(0, 0, 8, 8).WithMeta(json.RawMessage(`{"author":"alice"}`))}))

	boxes := e.Materialize()
	require.Len(t, boxes, 1)
	require.JSONEq(t, `{"merged":true}`, string(boxes[0].Meta), "the custom join decides the merged metadata")
}

func TestAdditiveWinsLWWResolvesColorAndSnapshots(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := memory.New()

	e, err := engine.Open(ctx, st, strategy.AdditiveWinsLWW, engine.WithSnapshotEvery(2))
	require.NoError(t, err)

	require.NoError(t, e.Apply(ctx, op.Op{ID: "a", Kind: op.KindAdd, Box: box(0, 0, 4, 4).WithMeta(json.RawMessage(`{"color":"#ff0000"}`))}))
	require.NoError(t, e.Apply(ctx, op.Op{ID: "b", Kind: op.KindAdd, Box: box(2, 2, 6, 6).WithMeta(json.RawMessage(`{"color":"#0000ff"}`))}))

	metaAt := func(p []float64) string {
		for _, b := range e.Materialize() {
			if b.Contains(p) {
				return string(b.Meta)
			}
		}
		return ""
	}
	require.JSONEq(t, `{"color":"#0000ff"}`, metaAt([]float64{3, 3}), "the later stroke wins the overlap")
	require.JSONEq(t, `{"color":"#ff0000"}`, metaAt([]float64{1, 1}), "red survives where blue never reached")

	// Snapshot and reopen: the colored partition and its timestamps survive.
	require.NoError(t, e.Snapshot(ctx))
	reopened, err := engine.Open(ctx, st, strategy.AdditiveWinsLWW, engine.WithSnapshotEvery(2))
	require.NoError(t, err)
	require.Equal(t, e.Materialize(), reopened.Materialize())
}
