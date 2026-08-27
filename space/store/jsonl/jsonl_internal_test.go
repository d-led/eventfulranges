package jsonl

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/d-led/eventfulranges/space"
	"github.com/d-led/eventfulranges/space/op"
)

func testOp(id string) op.Op {
	return op.Op{
		ID:   id,
		Kind: op.KindAdd,
		TS:   1,
		Box:  space.NewBox([]float64{1, 2}, []float64{3, 4}),
	}
}

func TestOpenMkdirError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	blocker := filepath.Join(dir, "file")
	require.NoError(t, os.WriteFile(blocker, nil, 0o644))
	_, err := Open(filepath.Join(blocker, "sub"))
	require.Error(t, err)
}

func TestAppendCountError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	blocker := filepath.Join(dir, "file")
	require.NoError(t, os.WriteFile(blocker, nil, 0o644))
	s := &Store{opsPath: filepath.Join(blocker, "boxes.jsonl")}
	require.Error(t, s.Append(context.Background(), 0, []op.Op{testOp("a")}))
}

func TestAppendOpenError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := &Store{opsPath: filepath.Join(dir, "boxes.jsonl")}
	require.NoError(t, os.WriteFile(s.opsPath, nil, 0o444))
	require.Error(t, s.Append(context.Background(), 0, []op.Op{testOp("a")}))
}

func TestReadOpenError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	blocker := filepath.Join(dir, "file")
	require.NoError(t, os.WriteFile(blocker, nil, 0o644))
	s := &Store{opsPath: filepath.Join(blocker, "boxes.jsonl")}
	_, _, err := s.Read(context.Background(), 0)
	require.Error(t, err)
}

func TestReadTooLongLine(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := &Store{opsPath: filepath.Join(dir, "boxes.jsonl")}
	long := make([]byte, 70*1024)
	for i := range long {
		long[i] = 'a'
	}
	require.NoError(t, os.WriteFile(s.opsPath, append(long, '\n'), 0o644))
	_, _, err := s.Read(context.Background(), 0)
	require.Error(t, err)
}

func TestSaveSnapshotAppendError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := &Store{opsPath: filepath.Join(dir, "boxes.jsonl")}
	require.NoError(t, os.Mkdir(s.opsPath, 0o755))
	require.Error(t, s.SaveSnapshot(context.Background(), []byte(`{}`), 1))
}

func TestSaveSnapshotInvalidData(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := &Store{opsPath: filepath.Join(dir, "boxes.jsonl")}
	require.Error(t, s.SaveSnapshot(context.Background(), []byte("not json"), 1))
}

func TestLoadSnapshotReadError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := &Store{opsPath: filepath.Join(dir, "boxes.jsonl")}
	require.NoError(t, os.Mkdir(s.opsPath, 0o755))
	_, _, err := s.LoadSnapshot(context.Background())
	require.Error(t, err)
}

func TestCompactReadError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := &Store{opsPath: filepath.Join(dir, "boxes.jsonl")}
	require.NoError(t, os.WriteFile(s.opsPath, []byte("not json\n"), 0o644))
	require.Error(t, s.Compact(context.Background(), []byte(`{}`), 0))
}

func TestCompactInvalidData(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := &Store{opsPath: filepath.Join(dir, "boxes.jsonl")}
	require.Error(t, s.Compact(context.Background(), []byte("not json"), 0))
}

func TestWriteAtomicWriteError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "boxes.jsonl")
	require.NoError(t, os.Mkdir(path+".tmp", 0o755))
	require.Error(t, writeAtomic(path, []byte("x")))
}

func TestWriteAtomicRenameError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "boxes.jsonl")
	require.NoError(t, os.Mkdir(path, 0o755))
	require.Error(t, writeAtomic(path, []byte("x")))
}

func TestAppendEventsWriteError(t *testing.T) {
	t.Parallel()
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	require.NoError(t, reader.Close())
	defer func() { _ = writer.Close() }()

	s := &Store{}
	require.Error(t, s.appendEvents(writer, []op.Op{testOp("a")}))
}

func TestAppendWriteErrorInjected(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := &Store{
		opsPath: filepath.Join(dir, "boxes.jsonl"),
		appendLines: func(*os.File, []op.Op) error {
			return errors.New("write failed")
		},
	}
	require.Error(t, s.Append(context.Background(), 0, []op.Op{testOp("a")}))
}
