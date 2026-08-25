//go:build kurrent

package kurrent

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/EventStore/EventStore-Client-Go/v4/esdb"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/d-led/eventfulranges/interval"
	"github.com/d-led/eventfulranges/op"
	"github.com/d-led/eventfulranges/store"
)

// openTestStore skips the test when no KurrentDB is reachable and points the
// package-level stream names at unique, per-test streams.
func openTestStore(t *testing.T) *Store {
	t.Helper()
	conn := os.Getenv("KURRENTDB_CONNECTION")
	if conn == "" {
		t.Skip("KURRENTDB_CONNECTION not set; run kurrent-up.sh first")
	}
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	opsStream = "itest-ops-" + suffix
	snapshotStream = "itest-snap-" + suffix

	s, err := Open(conn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func newEventID(t *testing.T) uuid.UUID {
	t.Helper()
	id, err := uuid.NewRandom()
	require.NoError(t, err)
	return id
}

// TestIntegrationRoundTrip covers the empty state and the happy path against
// a real KurrentDB.
func TestIntegrationRoundTrip(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	// A fresh log reads back empty, and no snapshot exists yet.
	got, version, err := s.Read(ctx, 0)
	require.NoError(t, err)
	require.Empty(t, got)
	require.Zero(t, version)

	_, _, err = s.LoadSnapshot(ctx)
	require.ErrorIs(t, err, store.ErrSnapshotNotFound)

	ops := []op.Op{
		{ID: "a", Kind: op.KindAdd, TS: 1, Interval: interval.Interval{Start: 1, End: 5}},
		{ID: "b", Kind: op.KindRemove, TS: 2, Interval: interval.Interval{Start: 3, End: 4}},
	}

	require.NoError(t, s.Append(ctx, 0, ops))
	require.ErrorIs(t, s.Append(ctx, 0, ops), store.ErrVersionConflict, "stale version must conflict")

	got, version, err = s.Read(ctx, 0)
	require.NoError(t, err)
	require.Equal(t, ops, got)
	require.Equal(t, int64(2), version)

	require.NoError(t, s.SaveSnapshot(ctx, []byte("snap"), version))
	data, snapVersion, err := s.LoadSnapshot(ctx)
	require.NoError(t, err)
	require.Equal(t, []byte("snap"), data)
	require.Equal(t, version, snapVersion)
}

// TestIntegrationCorruptData breaks the payloads so the decode error paths
// are met against a real server.
func TestIntegrationCorruptData(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	// An operation event with a malformed payload makes Read fail.
	_, err := s.client.AppendToStream(ctx, opsStream, esdb.AppendToStreamOptions{
		ExpectedRevision: esdb.Any{},
	}, esdb.EventData{
		EventID:     newEventID(t),
		EventType:   "range-op",
		ContentType: esdb.ContentTypeJson,
		Data:        []byte("{not-json"),
	})
	require.NoError(t, err)
	_, _, err = s.Read(ctx, 0)
	require.Error(t, err)

	// A snapshot event with malformed metadata makes LoadSnapshot fail.
	_, err = s.client.AppendToStream(ctx, snapshotStream, esdb.AppendToStreamOptions{
		ExpectedRevision: esdb.Any{},
	}, esdb.EventData{
		EventID:     newEventID(t),
		EventType:   "snapshot",
		ContentType: esdb.ContentTypeJson,
		Data:        []byte("{}"),
		Metadata:    []byte("{not-json"),
	})
	require.NoError(t, err)
	_, _, err = s.LoadSnapshot(ctx)
	require.Error(t, err)
}

// TestIntegrationCanceledContext breaks the input with a canceled context so
// the non-EOF read error paths are met.
func TestIntegrationCanceledContext(t *testing.T) {
	s := openTestStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := s.Read(ctx, 0)
	require.Error(t, err)

	_, _, err = s.LoadSnapshot(ctx)
	require.Error(t, err)
}
