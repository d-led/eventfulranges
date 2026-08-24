package eventfulranges_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"gitub.com/d-led/eventfulranges"
	"gitub.com/d-led/eventfulranges/clock"
	"gitub.com/d-led/eventfulranges/interval"
	"gitub.com/d-led/eventfulranges/op"
	"gitub.com/d-led/eventfulranges/store/memory"
	"gitub.com/d-led/eventfulranges/strategy"
)

func TestOpenAndConvenienceMethods(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	rs, err := eventfulranges.Open(ctx, t.TempDir(), strategy.LWW,
		eventfulranges.WithClock(clock.NewLamport()),
		eventfulranges.WithSnapshotEvery(2))
	require.NoError(t, err)

	_, err = rs.Add(ctx, 1, 5)
	require.NoError(t, err)
	_, err = rs.AddWithBounds(ctx, 3, 7, interval.Closed, interval.Open)
	require.NoError(t, err)

	require.True(t, rs.Contains(6))
	require.False(t, rs.Contains(8))
	require.True(t, rs.Overlaps(interval.Interval{Start: 5, End: 9}))

	require.NoError(t, rs.Apply(ctx, op.RemoveWithBounds(2, 3, interval.Closed, interval.Closed)))
	_, err = rs.Remove(ctx, 1, 1)
	require.NoError(t, err)
	_, err = rs.RemoveWithBounds(ctx, 4, 5, interval.Open, interval.Closed)
	require.NoError(t, err)

	require.Len(t, rs.Ops(), 5)
	require.NoError(t, rs.Snapshot(ctx))
	require.NotEmpty(t, rs.Ranges())
}

func TestOpenStore(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	rs, err := eventfulranges.OpenStore(ctx, memory.New(), strategy.GrowOnly)
	require.NoError(t, err)
	_, err = rs.Add(ctx, 1, 2)
	require.NoError(t, err)
	require.Len(t, rs.Ranges(), 1)
}

func TestApplyAllForAntiEntropy(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	a, err := eventfulranges.OpenStore(ctx, memory.New(), strategy.LWW)
	require.NoError(t, err)
	b, err := eventfulranges.OpenStore(ctx, memory.New(), strategy.LWW)
	require.NoError(t, err)

	_, err = a.Add(ctx, 1, 4)
	require.NoError(t, err)
	_, err = b.Remove(ctx, 2, 3)
	require.NoError(t, err)

	// Exchange ops like two replicas doing anti-entropy.
	require.NoError(t, a.ApplyAll(ctx, b.Ops()))
	require.NoError(t, b.ApplyAll(ctx, a.Ops()))

	require.Equal(t, a.Ranges(), b.Ranges(), "replicas must converge")
}

func TestOpenInvalidStrategy(t *testing.T) {
	t.Parallel()
	_, err := eventfulranges.Open(context.Background(), t.TempDir(), strategy.Strategy(99))
	require.Error(t, err)
}

func TestOpenUnwritableDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	blocker := filepath.Join(dir, "file")
	require.NoError(t, os.WriteFile(blocker, nil, 0o644))
	_, err := eventfulranges.Open(context.Background(), filepath.Join(blocker, "sub"), strategy.LWW)
	require.Error(t, err)
}
