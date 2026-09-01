package engine_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/d-led/eventfulranges/clock"
	"github.com/d-led/eventfulranges/space"
	"github.com/d-led/eventfulranges/space/engine"
	"github.com/d-led/eventfulranges/space/op"
	"github.com/d-led/eventfulranges/space/store/jsonl"
	"github.com/d-led/eventfulranges/space/store/memory"
	"github.com/d-led/eventfulranges/space/strategy"
)

func TestOpenEmpty(t *testing.T) {
	t.Parallel()
	e, err := engine.Open(context.Background(), memory.New(), strategy.LWW)
	require.NoError(t, err)
	require.Empty(t, e.Materialize())
	require.Empty(t, e.Ops())
}

func TestLayers(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	e, err := engine.Open(ctx, memory.New(), strategy.LWW)
	require.NoError(t, err)

	require.NoError(t, e.Apply(ctx, op.Op{ID: "a", Kind: op.KindAdd, TS: 1, Box: box(0, 0, 10, 10)}))
	require.NoError(t, e.Apply(ctx, op.Op{ID: "b", Kind: op.KindAdd, TS: 2, Box: box(4, 4, 6, 6)}))

	require.Equal(t, []strategy.Layer{
		{Box: box(0, 0, 10, 10), Kind: op.KindAdd},
		{Box: box(4, 4, 6, 6), Kind: op.KindAdd},
	}, e.Layers(), "the big square stays whole and the pixel layers on top")
}

func TestAdditiveScenario(t *testing.T) {
	t.Parallel()
	e, err := engine.Open(context.Background(), memory.New(), strategy.AdditiveWins)
	require.NoError(t, err)
	ctx := context.Background()

	require.NoError(t, e.Apply(ctx, op.Op{ID: "a", Kind: op.KindAdd, Box: box(0, 0, 4, 4)}))
	require.NoError(t, e.Apply(ctx, op.Op{ID: "b", Kind: op.KindRemove, Box: box(1, 1, 3, 3)}))

	require.False(t, e.Contains([]float64{2, 2}), "the removed hole must be empty")
	require.True(t, e.Contains([]float64{0.5, 2}), "the shell around the hole must remain")
}

func TestIdempotentApply(t *testing.T) {
	t.Parallel()
	e, err := engine.Open(context.Background(), memory.New(), strategy.LWW)
	require.NoError(t, err)
	ctx := context.Background()
	o := op.Op{ID: "a", Kind: op.KindAdd, Box: box(0, 0, 2, 2)}

	require.NoError(t, e.Apply(ctx, o))
	require.NoError(t, e.Apply(ctx, o))
	require.Len(t, e.Ops(), 1)
	require.Equal(t, []space.Box{box(0, 0, 2, 2)}, e.Materialize())
}

func TestApplyInvalidOp(t *testing.T) {
	t.Parallel()
	e, err := engine.Open(context.Background(), memory.New(), strategy.LWW)
	require.NoError(t, err)
	err = e.Apply(context.Background(), op.Op{ID: "bad", Kind: op.KindAdd, Box: space.NewBox([]float64{5}, []float64{1})})
	require.ErrorIs(t, err, space.ErrInverted)
}

func TestApplyDimsMismatch(t *testing.T) {
	t.Parallel()
	e, err := engine.Open(context.Background(), memory.New(), strategy.LWW)
	require.NoError(t, err)
	require.NoError(t, e.Apply(context.Background(), op.Op{ID: "a", Kind: op.KindAdd, Box: box(0, 0, 2, 2)}))

	err = e.Apply(context.Background(), op.Op{ID: "b", Kind: op.KindAdd, Box: space.NewBox([]float64{0, 0, 0}, []float64{2, 2, 2})})
	require.ErrorContains(t, err, "dimensions")
}

func TestApplyAll(t *testing.T) {
	t.Parallel()
	e, err := engine.Open(context.Background(), memory.New(), strategy.AdditiveWins)
	require.NoError(t, err)
	ctx := context.Background()
	require.NoError(t, e.ApplyAll(ctx, []op.Op{
		{ID: "a", Kind: op.KindAdd, Box: box(0, 0, 2, 2)},
		{ID: "b", Kind: op.KindAdd, Box: box(4, 0, 6, 2)},
	}))
	require.Len(t, e.Materialize(), 2)
	require.NoError(t, e.ApplyAll(ctx, nil))
}

func TestOpsSorted(t *testing.T) {
	t.Parallel()
	e, err := engine.Open(context.Background(), memory.New(), strategy.GrowOnly)
	require.NoError(t, err)
	ctx := context.Background()
	require.NoError(t, e.ApplyAll(ctx, []op.Op{
		{ID: "c", Kind: op.KindAdd, Box: box(0, 0, 1, 1)},
		{ID: "a", Kind: op.KindAdd, Box: box(1, 0, 2, 1)},
		{ID: "b", Kind: op.KindAdd, Box: box(2, 0, 3, 1)},
	}))
	ids := make([]string, 0, 3)
	for _, o := range e.Ops() {
		ids = append(ids, o.ID)
	}
	require.Equal(t, []string{"a", "b", "c"}, ids)
}

func TestUnknownStrategy(t *testing.T) {
	t.Parallel()
	_, err := engine.Open(context.Background(), memory.New(), strategy.Strategy(99))
	require.ErrorIs(t, err, strategy.ErrUnknownStrategy)
}

func TestQueries(t *testing.T) {
	t.Parallel()
	e, err := engine.Open(context.Background(), memory.New(), strategy.LWW)
	require.NoError(t, err)
	require.NoError(t, e.Apply(context.Background(), op.Op{ID: "a", Kind: op.KindAdd, Box: box(0, 0, 2, 2)}))

	require.True(t, e.Contains([]float64{1, 1}))
	require.False(t, e.Contains([]float64{5, 5}))
	require.True(t, e.Overlaps(box(1, 1, 3, 3)))
	require.False(t, e.Overlaps(box(3, 3, 4, 4)))
}

func TestLamportClockStamps(t *testing.T) {
	t.Parallel()
	e, err := engine.Open(context.Background(), memory.New(), strategy.GrowOnly, engine.WithClock(clock.NewLamport()))
	require.NoError(t, err)
	ctx := context.Background()
	require.NoError(t, e.Apply(ctx, op.Op{ID: "a", Kind: op.KindAdd, Box: box(0, 0, 1, 1)}))
	require.NoError(t, e.Apply(ctx, op.Op{ID: "b", Kind: op.KindAdd, Box: box(1, 0, 2, 1)}))

	byID := map[string]int64{}
	for _, o := range e.Ops() {
		byID[o.ID] = o.TS
	}
	require.Equal(t, int64(1), byID["a"])
	require.Equal(t, int64(2), byID["b"])
}

func TestSnapshotRestore(t *testing.T) {
	t.Parallel()
	for _, s := range []strategy.Strategy{strategy.LWW, strategy.FWW, strategy.AdditiveWins, strategy.GrowOnly} {
		t.Run(s.String(), func(t *testing.T) {
			t.Parallel()
			st := memory.New()
			ctx := context.Background()
			e, err := engine.Open(ctx, st, s)
			require.NoError(t, err)
			ops := []op.Op{
				{ID: "a", Kind: op.KindAdd, TS: 1, Box: box(0, 0, 3, 3)},
				{ID: "b", Kind: op.KindAdd, TS: 2, Box: box(2, 0, 5, 3)},
				{ID: "c", Kind: op.KindRemove, TS: 3, Box: box(1, 1, 2, 2)},
			}
			require.NoError(t, e.ApplyAll(ctx, ops[:2]))
			require.NoError(t, e.Snapshot(ctx))
			require.NoError(t, e.Apply(ctx, ops[2]))

			reopened, err := engine.Open(ctx, st, s)
			require.NoError(t, err)
			require.Equal(t, strategy.Materialize(s, ops), reopened.Materialize())
			require.Equal(t, e.Materialize(), reopened.Materialize())
		})
	}
}

func TestSnapshotEvery(t *testing.T) {
	t.Parallel()
	st := memory.New()
	ctx := context.Background()
	e, err := engine.Open(ctx, st, strategy.LWW, engine.WithSnapshotEvery(2))
	require.NoError(t, err)
	require.NoError(t, e.ApplyAll(ctx, []op.Op{
		{ID: "a", Kind: op.KindAdd, TS: 1, Box: box(0, 0, 1, 1)},
		{ID: "b", Kind: op.KindAdd, TS: 2, Box: box(1, 0, 2, 1)},
	}))

	_, version, err := st.LoadSnapshot(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(2), version)
}

func TestSnapshotIgnoredForDifferentStrategy(t *testing.T) {
	t.Parallel()
	st := memory.New()
	ctx := context.Background()
	e, err := engine.Open(ctx, st, strategy.LWW)
	require.NoError(t, err)
	require.NoError(t, e.Apply(ctx, op.Op{ID: "a", Kind: op.KindAdd, TS: 1, Box: box(0, 0, 2, 2)}))
	require.NoError(t, e.Snapshot(ctx))

	fww, err := engine.Open(ctx, st, strategy.FWW)
	require.NoError(t, err)
	require.Equal(t, []space.Box{box(0, 0, 2, 2)}, fww.Materialize())
}

func TestConvergence(t *testing.T) {
	t.Parallel()
	for _, s := range []strategy.Strategy{strategy.LWW, strategy.FWW, strategy.AdditiveWins, strategy.GrowOnly} {
		t.Run(s.String(), func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			a, err := engine.Open(ctx, memory.New(), s)
			require.NoError(t, err)
			b, err := engine.Open(ctx, memory.New(), s)
			require.NoError(t, err)

			require.NoError(t, a.Apply(ctx, op.Op{ID: "x", Kind: op.KindAdd, TS: 1, Box: box(0, 0, 4, 4)}))
			require.NoError(t, a.Apply(ctx, op.Op{ID: "y", Kind: op.KindRemove, TS: 2, Box: box(1, 1, 3, 3)}))
			require.NoError(t, b.Apply(ctx, op.Op{ID: "y", Kind: op.KindRemove, TS: 2, Box: box(1, 1, 3, 3)}))
			require.NoError(t, b.Apply(ctx, op.Op{ID: "x", Kind: op.KindAdd, TS: 1, Box: box(0, 0, 4, 4)}))

			require.NoError(t, a.ApplyAll(ctx, b.Ops()))
			require.NoError(t, b.ApplyAll(ctx, a.Ops()))
			require.Equal(t, a.Materialize(), b.Materialize())
		})
	}
}

func TestVersionConflictRecovery(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := memory.New()
	a, err := engine.Open(ctx, st, strategy.LWW)
	require.NoError(t, err)
	b, err := engine.Open(ctx, st, strategy.LWW)
	require.NoError(t, err)

	require.NoError(t, a.Apply(ctx, op.Op{ID: "x", Kind: op.KindAdd, TS: 1, Box: box(0, 0, 2, 2)}))
	require.NoError(t, b.Apply(ctx, op.Op{ID: "y", Kind: op.KindAdd, TS: 2, Box: box(1, 0, 3, 2)}))

	// b's append conflicted, so it caught up x and then appended y.
	require.Len(t, b.Materialize(), 2)

	// a catches up by exchanging with b, and the replicas converge.
	require.NoError(t, a.ApplyAll(ctx, b.Ops()))
	require.Equal(t, a.Materialize(), b.Materialize())
}

func TestCorruptSnapshot(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	st, err := jsonl.Open(dir)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "boxes.stream.jsonl"), []byte(`{"snapshot":`+"\n"), 0o644))

	_, err = engine.Open(context.Background(), st, strategy.LWW)
	require.Error(t, err)
}

func TestCompactUnsupported(t *testing.T) {
	t.Parallel()
	e, err := engine.Open(context.Background(), memory.New(), strategy.LWW)
	require.NoError(t, err)
	require.Error(t, e.Compact(context.Background()))
}

func TestCompactRoundTrips(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := t.TempDir()
	st, err := jsonl.Open(dir)
	require.NoError(t, err)
	e, err := engine.Open(ctx, st, strategy.AdditiveWins)
	require.NoError(t, err)

	require.NoError(t, e.Apply(ctx, op.Op{ID: "a", Kind: op.KindAdd, Box: box(0, 0, 4, 4)}))
	require.NoError(t, e.Apply(ctx, op.Op{ID: "b", Kind: op.KindRemove, Box: box(1, 1, 3, 3)}))
	want := e.Materialize()

	require.NoError(t, e.Compact(ctx))

	st2, err := jsonl.Open(dir)
	require.NoError(t, err)
	reopened, err := engine.Open(ctx, st2, strategy.AdditiveWins)
	require.NoError(t, err)
	require.True(t, space.Equal(want, reopened.Materialize()), "compaction preserves the picture")
}

func TestCanonicalizerApplied(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	e, err := engine.Open(ctx, memory.New(), strategy.AdditiveWins,
		engine.WithCanonicalizer(splitCanonicalizer))
	require.NoError(t, err)

	require.NoError(t, e.Apply(ctx, op.Op{ID: "a", Kind: op.KindAdd, Box: box(0, 0, 2, 2)}))

	require.Len(t, e.Materialize(), 2, "the canonicalizer splits the box along x")
	require.True(t, e.Contains([]float64{0.5, 1}), "coverage must be preserved")
	require.True(t, e.Contains([]float64{1.5, 1}))
	require.False(t, e.Contains([]float64{3, 3}))
}
