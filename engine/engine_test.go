package engine_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"gitub.com/d-led/eventfulranges/clock"
	"gitub.com/d-led/eventfulranges/engine"
	"gitub.com/d-led/eventfulranges/interval"
	"gitub.com/d-led/eventfulranges/op"
	"gitub.com/d-led/eventfulranges/store/jsonl"
	"gitub.com/d-led/eventfulranges/store/memory"
	"gitub.com/d-led/eventfulranges/strategy"
)

func TestOpenEmpty(t *testing.T) {
	t.Parallel()
	e, err := engine.Open(context.Background(), memory.New(), strategy.LWW)
	require.NoError(t, err)
	require.Empty(t, e.Materialize())
	require.Empty(t, e.Ops())
}

func TestChatScenario(t *testing.T) {
	t.Parallel()
	e, err := engine.Open(context.Background(), memory.New(), strategy.LWW)
	require.NoError(t, err)
	ctx := context.Background()

	require.NoError(t, e.Apply(ctx, op.Op{ID: "a", Kind: op.KindAdd, Interval: iv(1, 5, interval.Closed, interval.Closed)}))
	require.NoError(t, e.Apply(ctx, op.Op{ID: "b", Kind: op.KindAdd, Interval: iv(3, 7, interval.Closed, interval.Closed)}))
	require.NoError(t, e.Apply(ctx, op.Op{ID: "c", Kind: op.KindRemove, Interval: iv(2, 3, interval.Closed, interval.Closed)}))

	require.Equal(t, []interval.Interval{
		iv(1, 2, interval.Closed, interval.Open),
		iv(3, 7, interval.Open, interval.Closed),
	}, e.Materialize())
}

func TestIdempotentApply(t *testing.T) {
	t.Parallel()
	e, err := engine.Open(context.Background(), memory.New(), strategy.LWW)
	require.NoError(t, err)
	ctx := context.Background()
	o := op.Op{ID: "a", Kind: op.KindAdd, Interval: iv(1, 5, interval.Closed, interval.Closed)}

	require.NoError(t, e.Apply(ctx, o))
	require.NoError(t, e.Apply(ctx, o))
	require.Len(t, e.Ops(), 1)
	require.Equal(t, []interval.Interval{iv(1, 5, interval.Closed, interval.Closed)}, e.Materialize())
}

func TestApplyInvalidOp(t *testing.T) {
	t.Parallel()
	e, err := engine.Open(context.Background(), memory.New(), strategy.LWW)
	require.NoError(t, err)
	err = e.Apply(context.Background(), op.Op{ID: "bad", Kind: op.KindAdd, Interval: interval.Interval{Start: 5, End: 1}})
	require.ErrorIs(t, err, interval.ErrInverted)
}

func TestApplyAll(t *testing.T) {
	t.Parallel()
	e, err := engine.Open(context.Background(), memory.New(), strategy.AdditiveWins)
	require.NoError(t, err)
	ctx := context.Background()
	require.NoError(t, e.ApplyAll(ctx, []op.Op{
		{ID: "a", Kind: op.KindAdd, Interval: iv(1, 5, interval.Closed, interval.Closed)},
		{ID: "b", Kind: op.KindAdd, Interval: iv(3, 7, interval.Closed, interval.Closed)},
	}))
	require.Equal(t, []interval.Interval{iv(1, 7, interval.Closed, interval.Closed)}, e.Materialize())
	require.NoError(t, e.ApplyAll(ctx, nil))
}

func TestOpsSorted(t *testing.T) {
	t.Parallel()
	e, err := engine.Open(context.Background(), memory.New(), strategy.GrowOnly)
	require.NoError(t, err)
	ctx := context.Background()
	require.NoError(t, e.ApplyAll(ctx, []op.Op{
		{ID: "c", Kind: op.KindAdd, Interval: iv(1, 2, interval.Closed, interval.Closed)},
		{ID: "a", Kind: op.KindAdd, Interval: iv(3, 4, interval.Closed, interval.Closed)},
		{ID: "b", Kind: op.KindAdd, Interval: iv(5, 6, interval.Closed, interval.Closed)},
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
	require.NoError(t, e.Apply(context.Background(), op.Op{ID: "a", Kind: op.KindAdd, Interval: iv(1, 5, interval.Closed, interval.Closed)}))

	require.True(t, e.Contains(3))
	require.False(t, e.Contains(9))
	require.True(t, e.Overlaps(iv(4, 6, interval.Closed, interval.Closed)))
	require.False(t, e.Overlaps(iv(6, 7, interval.Closed, interval.Closed)))
}

func TestLamportClockStamps(t *testing.T) {
	t.Parallel()
	e, err := engine.Open(context.Background(), memory.New(), strategy.GrowOnly, engine.WithClock(clock.NewLamport()))
	require.NoError(t, err)
	ctx := context.Background()
	require.NoError(t, e.Apply(ctx, op.Op{ID: "a", Kind: op.KindAdd, Interval: iv(1, 2, interval.Closed, interval.Closed)}))
	require.NoError(t, e.Apply(ctx, op.Op{ID: "b", Kind: op.KindAdd, Interval: iv(3, 4, interval.Closed, interval.Closed)}))

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
				{ID: "a", Kind: op.KindAdd, TS: 1, Interval: iv(1, 5, interval.Closed, interval.Closed)},
				{ID: "b", Kind: op.KindAdd, TS: 2, Interval: iv(3, 7, interval.Closed, interval.Closed)},
				{ID: "c", Kind: op.KindRemove, TS: 3, Interval: iv(2, 3, interval.Closed, interval.Closed)},
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
		{ID: "a", Kind: op.KindAdd, TS: 1, Interval: iv(1, 2, interval.Closed, interval.Closed)},
		{ID: "b", Kind: op.KindAdd, TS: 2, Interval: iv(3, 4, interval.Closed, interval.Closed)},
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
	require.NoError(t, e.Apply(ctx, op.Op{ID: "a", Kind: op.KindAdd, TS: 1, Interval: iv(1, 5, interval.Closed, interval.Closed)}))
	require.NoError(t, e.Snapshot(ctx))

	fww, err := engine.Open(ctx, st, strategy.FWW)
	require.NoError(t, err)
	require.Equal(t, []interval.Interval{iv(1, 5, interval.Closed, interval.Closed)}, fww.Materialize())
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

			require.NoError(t, a.Apply(ctx, op.Op{ID: "x", Kind: op.KindAdd, TS: 1, Interval: iv(1, 5, interval.Closed, interval.Closed)}))
			require.NoError(t, a.Apply(ctx, op.Op{ID: "y", Kind: op.KindRemove, TS: 2, Interval: iv(2, 3, interval.Closed, interval.Closed)}))
			require.NoError(t, b.Apply(ctx, op.Op{ID: "y", Kind: op.KindRemove, TS: 2, Interval: iv(2, 3, interval.Closed, interval.Closed)}))
			require.NoError(t, b.Apply(ctx, op.Op{ID: "x", Kind: op.KindAdd, TS: 1, Interval: iv(1, 5, interval.Closed, interval.Closed)}))

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

	require.NoError(t, a.Apply(ctx, op.Op{ID: "x", Kind: op.KindAdd, TS: 1, Interval: iv(1, 5, interval.Closed, interval.Closed)}))
	require.NoError(t, b.Apply(ctx, op.Op{ID: "y", Kind: op.KindAdd, TS: 2, Interval: iv(3, 7, interval.Closed, interval.Closed)}))

	// b's append conflicted, so it caught up x and then appended y.
	require.Equal(t, []interval.Interval{iv(1, 7, interval.Closed, interval.Closed)}, b.Materialize())

	// a catches up by exchanging with b, and the replicas converge.
	require.NoError(t, a.ApplyAll(ctx, b.Ops()))
	require.Equal(t, []interval.Interval{iv(1, 7, interval.Closed, interval.Closed)}, a.Materialize())
	require.Equal(t, a.Materialize(), b.Materialize())
}

func TestCorruptSnapshot(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	st, err := jsonl.Open(dir)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ranges.snapshot.json"), []byte("garbage"), 0o644))

	_, err = engine.Open(context.Background(), st, strategy.LWW)
	require.Error(t, err)
}
