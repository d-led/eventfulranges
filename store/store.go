// Package store defines the append-only event log that backs an engine.
package store

import (
	"context"
	"errors"

	"github.com/d-led/eventfulranges/op"
)

var (
	// ErrVersionConflict is returned when appending with a stale expected
	// version, i.e. another writer appended first.
	ErrVersionConflict = errors.New("store: version conflict")
	// ErrSnapshotNotFound is returned when no snapshot exists.
	ErrSnapshotNotFound = errors.New("store: snapshot not found")
)

// Log is an append-only, ordered stream of operations. It is the smallest
// persistence contract: a stream-only backend (for example Kafka) needs only
// this.
type Log interface {
	// Append atomically appends the events at the given expected version.
	// It returns ErrVersionConflict if expectedVersion is stale.
	Append(ctx context.Context, expectedVersion int64, events []op.Op) error
	// Read returns the operations after fromVersion together with the new
	// version, which is the total number of operations in the log.
	Read(ctx context.Context, fromVersion int64) ([]op.Op, int64, error)
}

// Snapshotter stores and loads materialized snapshots. A Log may optionally
// implement it; backends that do not simply skip snapshotting.
type Snapshotter interface {
	// SaveSnapshot stores a materialized snapshot at the given version.
	SaveSnapshot(ctx context.Context, data []byte, version int64) error
	// LoadSnapshot returns the latest snapshot and its version.
	LoadSnapshot(ctx context.Context) ([]byte, int64, error)
}

// EventStore is the combined contract: an append-only log with snapshotting.
type EventStore interface {
	Log
	Snapshotter
}
