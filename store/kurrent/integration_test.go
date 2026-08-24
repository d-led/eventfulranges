//go:build kurrent

package kurrent

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"gitub.com/d-led/eventfulranges/interval"
	"gitub.com/d-led/eventfulranges/op"
	"gitub.com/d-led/eventfulranges/store"
)

// TestIntegrationRoundTrip runs against a real KurrentDB when
// KURRENTDB_CONNECTION is set; otherwise it is skipped. Streams are given
// unique names so repeated runs never clash.
func TestIntegrationRoundTrip(t *testing.T) {
	conn := os.Getenv("KURRENTDB_CONNECTION")
	if conn == "" {
		t.Skip("KURRENTDB_CONNECTION not set; run kurrent-up.sh first")
	}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	opsStream = "itest-ops-" + suffix
	snapshotStream = "itest-snap-" + suffix

	s, err := Open(conn)
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	ctx := context.Background()
	ops := []op.Op{
		{ID: "a", Kind: op.KindAdd, TS: 1, Interval: interval.Interval{Start: 1, End: 5}},
		{ID: "b", Kind: op.KindRemove, TS: 2, Interval: interval.Interval{Start: 3, End: 4}},
	}

	require.NoError(t, s.Append(ctx, 0, ops))
	require.ErrorIs(t, s.Append(ctx, 0, ops), store.ErrVersionConflict, "stale version must conflict")

	got, version, err := s.Read(ctx, 0)
	require.NoError(t, err)
	require.Equal(t, ops, got)
	require.Equal(t, int64(2), version)

	require.NoError(t, s.SaveSnapshot(ctx, []byte("snap"), version))
	data, snapVersion, err := s.LoadSnapshot(ctx)
	require.NoError(t, err)
	require.Equal(t, []byte("snap"), data)
	require.Equal(t, version, snapVersion)
}
