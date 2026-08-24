package memory_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"gitub.com/d-led/eventfulranges/interval"
	"gitub.com/d-led/eventfulranges/op"
	"gitub.com/d-led/eventfulranges/store"
	"gitub.com/d-led/eventfulranges/store/memory"
)

func TestMemoryAppendRead(t *testing.T) {
	t.Parallel()
	s := memory.New()
	ctx := context.Background()
	events := []op.Op{{
		ID:       "a",
		Kind:     op.KindAdd,
		Interval: interval.Interval{Start: 1, End: 2},
		TS:       1,
	}}
	require.NoError(t, s.Append(ctx, 0, events))

	got, version, err := s.Read(ctx, 0)
	require.NoError(t, err)
	require.Equal(t, events, got)
	require.Equal(t, int64(1), version)

	got, version, err = s.Read(ctx, 1)
	require.NoError(t, err)
	require.Empty(t, got)
	require.Equal(t, int64(1), version)

	got, version, err = s.Read(ctx, 5)
	require.NoError(t, err)
	require.Empty(t, got)
	require.Equal(t, int64(1), version)

	got, _, err = s.Read(ctx, -1)
	require.NoError(t, err)
	require.Equal(t, events, got)
}

func TestMemoryVersionConflict(t *testing.T) {
	t.Parallel()
	s := memory.New()
	ctx := context.Background()
	first := op.Op{ID: "a", Kind: op.KindAdd, Interval: interval.Interval{Start: 1, End: 2}, TS: 1}
	require.NoError(t, s.Append(ctx, 0, []op.Op{first}))
	require.ErrorIs(t, s.Append(ctx, 0, []op.Op{first}), store.ErrVersionConflict)

	second := op.Op{ID: "b", Kind: op.KindRemove, Interval: interval.Interval{Start: 1, End: 2}, TS: 2}
	require.NoError(t, s.Append(ctx, 1, []op.Op{second}))
	got, version, err := s.Read(ctx, 0)
	require.NoError(t, err)
	require.Equal(t, int64(2), version)
	require.Len(t, got, 2)
}

func TestMemorySnapshot(t *testing.T) {
	t.Parallel()
	s := memory.New()
	ctx := context.Background()

	_, _, err := s.LoadSnapshot(ctx)
	require.ErrorIs(t, err, store.ErrSnapshotNotFound)

	require.NoError(t, s.SaveSnapshot(ctx, []byte("hello"), 3))
	data, version, err := s.LoadSnapshot(ctx)
	require.NoError(t, err)
	require.Equal(t, []byte("hello"), data)
	require.Equal(t, int64(3), version)
}
