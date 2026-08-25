package engine_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/d-led/eventfulranges/space/engine"
	"github.com/d-led/eventfulranges/space/op"
	"github.com/d-led/eventfulranges/space/store"
	"github.com/d-led/eventfulranges/space/store/memory"
	"github.com/d-led/eventfulranges/space/strategy"
)

// stubStore is an EventStore that injects errors on demand.
type stubStore struct {
	appendErr error
	readErr   error
	readErrOn int // 1-based Read call to fail on; 0 means every Read fails
	loadErr   error
	snapErr   error
	loadData  []byte
	loadVer   int64
	hasLoad   bool
	reads     int
	ops       []op.Op
}

func (s *stubStore) Append(_ context.Context, _ int64, events []op.Op) error {
	if s.appendErr != nil {
		return s.appendErr
	}
	s.ops = append(s.ops, events...)
	return nil
}

func (s *stubStore) Read(_ context.Context, from int64) ([]op.Op, int64, error) {
	s.reads++
	if s.readErr != nil && (s.readErrOn == 0 || s.reads == s.readErrOn) {
		return nil, 0, s.readErr
	}
	if from < 0 {
		from = 0
	}
	if from > int64(len(s.ops)) {
		from = int64(len(s.ops))
	}
	return append([]op.Op(nil), s.ops[from:]...), int64(len(s.ops)), nil
}

func (s *stubStore) SaveSnapshot(_ context.Context, data []byte, version int64) error {
	if s.snapErr != nil {
		return s.snapErr
	}
	s.loadData = data
	s.loadVer = version
	s.hasLoad = true
	return nil
}

func (s *stubStore) LoadSnapshot(_ context.Context) ([]byte, int64, error) {
	if s.loadErr != nil {
		return nil, 0, s.loadErr
	}
	if !s.hasLoad {
		return nil, 0, store.ErrSnapshotNotFound
	}
	return s.loadData, s.loadVer, nil
}

// logOnlyStore hides the snapshot surface of an inner store, so the engine
// treats it as a stream-only backend (a Log without a Snapshotter).
type logOnlyStore struct {
	inner store.EventStore
}

func (s logOnlyStore) Append(ctx context.Context, version int64, events []op.Op) error {
	return s.inner.Append(ctx, version, events)
}

func (s logOnlyStore) Read(ctx context.Context, from int64) ([]op.Op, int64, error) {
	return s.inner.Read(ctx, from)
}

func validAdd(id string) op.Op {
	return op.Op{ID: id, Kind: op.KindAdd, Box: box(0, 0, 1, 1)}
}

func TestApplyNonConflictError(t *testing.T) {
	t.Parallel()
	st := &stubStore{appendErr: errors.New("boom")}
	e, err := engine.Open(context.Background(), st, strategy.LWW)
	require.NoError(t, err)
	require.ErrorContains(t, e.Apply(context.Background(), validAdd("a")), "boom")
}

func TestApplyConflictExhausted(t *testing.T) {
	t.Parallel()
	st := &stubStore{appendErr: store.ErrVersionConflict}
	e, err := engine.Open(context.Background(), st, strategy.LWW)
	require.NoError(t, err)
	require.ErrorIs(t, e.Apply(context.Background(), validAdd("a")), store.ErrVersionConflict)
}

func TestCatchUpReadError(t *testing.T) {
	t.Parallel()
	st := &stubStore{appendErr: store.ErrVersionConflict, readErr: errors.New("read boom"), readErrOn: 2}
	e, err := engine.Open(context.Background(), st, strategy.LWW)
	require.NoError(t, err)
	require.ErrorContains(t, e.Apply(context.Background(), validAdd("a")), "read boom")
}

func TestSnapshotError(t *testing.T) {
	t.Parallel()
	st := &stubStore{snapErr: errors.New("snap boom")}
	e, err := engine.Open(context.Background(), st, strategy.LWW, engine.WithSnapshotEvery(1))
	require.NoError(t, err)
	require.ErrorContains(t, e.Apply(context.Background(), validAdd("a")), "snap boom")
}

func TestOpenLoadError(t *testing.T) {
	t.Parallel()
	st := &stubStore{loadErr: errors.New("load boom")}
	_, err := engine.Open(context.Background(), st, strategy.LWW)
	require.ErrorContains(t, err, "load boom")
}

func TestOpenReadError(t *testing.T) {
	t.Parallel()
	st := &stubStore{readErr: errors.New("read boom")}
	_, err := engine.Open(context.Background(), st, strategy.LWW)
	require.ErrorContains(t, err, "read boom")
}

func TestOpenCorruptSnapshotData(t *testing.T) {
	t.Parallel()
	st := &stubStore{hasLoad: true, loadData: []byte("not json")}
	_, err := engine.Open(context.Background(), st, strategy.LWW)
	require.Error(t, err)
}

func TestReopenWithoutSnapshotReplaysLog(t *testing.T) {
	t.Parallel()
	st := memory.New()
	ctx := context.Background()
	e, err := engine.Open(ctx, st, strategy.AdditiveWins)
	require.NoError(t, err)
	require.NoError(t, e.ApplyAll(ctx, []op.Op{
		{ID: "a", Kind: op.KindAdd, Box: box(0, 0, 1, 1)},
		{ID: "b", Kind: op.KindAdd, Box: box(1, 0, 2, 1)},
		{ID: "c", Kind: op.KindRemove, Box: box(2, 0, 3, 1)},
	}))

	reopened, err := engine.Open(ctx, st, strategy.AdditiveWins)
	require.NoError(t, err)
	require.Equal(t, e.Materialize(), reopened.Materialize())
}

// TestLogOnlyStoreSkipsSnapshotting exercises the stream-only backend path:
// opening must not fail on a missing snapshot, and saving must be skipped
// without error, while the view still converges by replaying the log.
func TestLogOnlyStoreSkipsSnapshotting(t *testing.T) {
	t.Parallel()
	inner := memory.New()
	ctx := context.Background()

	e, err := engine.Open(ctx, logOnlyStore{inner: inner}, strategy.LWW, engine.WithSnapshotEvery(1))
	require.NoError(t, err)
	require.NoError(t, e.Apply(ctx, validAdd("a")))
	require.NoError(t, e.Snapshot(ctx))
	require.True(t, e.Contains([]float64{0.5, 0.5}))

	reopened, err := engine.Open(ctx, logOnlyStore{inner: inner}, strategy.LWW)
	require.NoError(t, err)
	require.True(t, reopened.Contains([]float64{0.5, 0.5}))
}
