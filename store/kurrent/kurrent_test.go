//go:build kurrent

package kurrent

import (
	"context"
	"io"
	"testing"

	"github.com/EventStore/EventStore-Client-Go/v4/esdb"
	"github.com/stretchr/testify/require"

	"gitub.com/d-led/eventfulranges/interval"
	"gitub.com/d-led/eventfulranges/op"
	"gitub.com/d-led/eventfulranges/store"
)

// fakeClient is an in-memory client double: appends are recorded as events in
// the relevant stream, reads replay them.
type fakeClient struct {
	ops   []*esdb.ResolvedEvent
	snaps []*esdb.ResolvedEvent
	err   error
}

func (f *fakeClient) AppendToStream(_ context.Context, streamID string, _ esdb.AppendToStreamOptions, events ...esdb.EventData) (*esdb.WriteResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	target := &f.ops
	if streamID == snapshotStream {
		target = &f.snaps
	}
	base := uint64(len(*target))
	for i, e := range events {
		*target = append(*target, &esdb.ResolvedEvent{Event: &esdb.RecordedEvent{
			EventNumber:  base + uint64(i),
			Data:         e.Data,
			UserMetadata: e.Metadata,
		}})
	}
	return &esdb.WriteResult{}, nil
}

func (f *fakeClient) ReadStream(_ context.Context, streamID string, opts esdb.ReadStreamOptions, _ uint64) (reader, error) {
	if f.err != nil {
		return nil, f.err
	}
	events := f.ops
	if streamID == snapshotStream {
		events = f.snaps
	}
	if opts.Direction == esdb.Backwards {
		reversed := make([]*esdb.ResolvedEvent, len(events))
		for i, e := range events {
			reversed[len(events)-1-i] = e
		}
		events = reversed
	}
	return &fakeReader{events: events}, nil
}

func (f *fakeClient) Close() error { return nil }

// fakeReader iterates a preloaded list of events.
type fakeReader struct {
	events []*esdb.ResolvedEvent
	pos    int
	closed bool
}

func (r *fakeReader) Recv() (*esdb.ResolvedEvent, error) {
	if r.pos >= len(r.events) {
		return nil, io.EOF
	}
	e := r.events[r.pos]
	r.pos++
	return e, nil
}

func (r *fakeReader) Close() { r.closed = true }

func testOp(id string, ts int64, kind op.Kind, start, end int) op.Op {
	return op.Op{
		ID:       id,
		Kind:     kind,
		TS:       ts,
		Interval: interval.Interval{Start: float64(start), End: float64(end)},
	}
}

func TestAppendAndRead(t *testing.T) {
	t.Parallel()
	client := &fakeClient{}
	s := &Store{client: client}

	ops := []op.Op{
		testOp("a", 1, op.KindAdd, 1, 5),
		testOp("b", 2, op.KindRemove, 3, 4),
	}
	require.NoError(t, s.Append(context.Background(), 0, ops))

	got, version, err := s.Read(context.Background(), 0)
	require.NoError(t, err)
	require.Equal(t, int64(2), version)
	require.Equal(t, ops, got)
}

func TestAppendVersionConflict(t *testing.T) {
	t.Parallel()
	s := &Store{client: &fakeClient{err: errWrongVersion}}
	err := s.Append(context.Background(), 0, []op.Op{testOp("a", 1, op.KindAdd, 1, 2)})
	require.ErrorIs(t, err, store.ErrVersionConflict)
}

func TestReadMissingStream(t *testing.T) {
	t.Parallel()
	s := &Store{client: &fakeClient{err: errStreamNotFound}}
	ops, version, err := s.Read(context.Background(), 0)
	require.NoError(t, err)
	require.Nil(t, ops)
	require.Zero(t, version)
}

func TestSnapshotRoundTrip(t *testing.T) {
	t.Parallel()
	s := &Store{client: &fakeClient{}}
	require.NoError(t, s.SaveSnapshot(context.Background(), []byte("payload"), 7))

	data, version, err := s.LoadSnapshot(context.Background())
	require.NoError(t, err)
	require.Equal(t, []byte("payload"), data)
	require.Equal(t, int64(7), version)
}

func TestLoadSnapshotMissing(t *testing.T) {
	t.Parallel()
	s := &Store{client: &fakeClient{err: errStreamNotFound}}
	_, _, err := s.LoadSnapshot(context.Background())
	require.ErrorIs(t, err, store.ErrSnapshotNotFound)
}

func TestExpectedRevision(t *testing.T) {
	t.Parallel()
	require.IsType(t, esdb.NoStream{}, expectedRevision(0))
	require.Equal(t, esdb.Revision(2), expectedRevision(3))
}
