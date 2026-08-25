package jsonl_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/d-led/eventfulranges/space"
	"github.com/d-led/eventfulranges/space/op"
	"github.com/d-led/eventfulranges/space/store"
	"github.com/d-led/eventfulranges/space/store/jsonl"
)

func opAdd(id string, ts int64, x0, y0, x1, y1 int) op.Op {
	return op.Op{
		ID:   id,
		Kind: op.KindAdd,
		TS:   ts,
		Box:  space.NewBox([]float64{float64(x0), float64(y0)}, []float64{float64(x1), float64(y1)}),
	}
}

func TestJSONLAppendRead(t *testing.T) {
	t.Parallel()
	s, err := jsonl.Open(t.TempDir())
	require.NoError(t, err)
	ctx := context.Background()
	events := []op.Op{opAdd("a", 1, 1, 2, 3, 4), opAdd("b", 2, 5, 6, 7, 8)}

	require.NoError(t, s.Append(ctx, 0, events))
	got, version, err := s.Read(ctx, 0)
	require.NoError(t, err)
	require.Equal(t, events, got)
	require.Equal(t, int64(2), version)

	got, _, err = s.Read(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, events[1:], got)
}

func TestJSONLReadEmpty(t *testing.T) {
	t.Parallel()
	s, err := jsonl.Open(t.TempDir())
	require.NoError(t, err)
	got, version, err := s.Read(context.Background(), 0)
	require.NoError(t, err)
	require.Nil(t, got)
	require.Equal(t, int64(0), version)
}

func TestJSONLVersionConflict(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s1, err := jsonl.Open(dir)
	require.NoError(t, err)
	s2, err := jsonl.Open(dir)
	require.NoError(t, err)
	ctx := context.Background()

	require.NoError(t, s1.Append(ctx, 0, []op.Op{opAdd("a", 1, 1, 2, 3, 4)}))
	require.ErrorIs(t, s2.Append(ctx, 0, []op.Op{opAdd("b", 2, 5, 6, 7, 8)}), store.ErrVersionConflict)
	require.NoError(t, s2.Append(ctx, 1, []op.Op{opAdd("b", 2, 5, 6, 7, 8)}))

	got, version, err := s2.Read(ctx, 0)
	require.NoError(t, err)
	require.Equal(t, int64(2), version)
	require.Len(t, got, 2)
}

func TestJSONLSnapshot(t *testing.T) {
	t.Parallel()
	s, err := jsonl.Open(t.TempDir())
	require.NoError(t, err)
	ctx := context.Background()

	_, _, err = s.LoadSnapshot(ctx)
	require.ErrorIs(t, err, store.ErrSnapshotNotFound)

	require.NoError(t, s.SaveSnapshot(ctx, []byte("payload"), 4))
	data, version, err := s.LoadSnapshot(ctx)
	require.NoError(t, err)
	require.Equal(t, []byte("payload"), data)
	require.Equal(t, int64(4), version)
}

func TestJSONLOpenCreatesDirectory(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "nested", "path")
	_, err := jsonl.Open(dir)
	require.NoError(t, err)
}

func TestJSONLCorruptLine(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, err := jsonl.Open(dir)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "boxes.stream.jsonl"), []byte("not json\n"), 0o644))

	s, err := jsonl.Open(dir)
	require.NoError(t, err)
	_, _, err = s.Read(context.Background(), 0)
	require.Error(t, err)
}

func TestJSONLCorruptSnapshot(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, err := jsonl.Open(dir)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "boxes.snapshot.json"), []byte("garbage"), 0o644))

	s, err := jsonl.Open(dir)
	require.NoError(t, err)
	_, _, err = s.LoadSnapshot(context.Background())
	require.Error(t, err)
}
