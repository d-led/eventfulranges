// Package memory provides an in-memory EventStore for tests and
// single-process use.
package memory

import (
	"context"
	"sync"

	"github.com/d-led/eventfulranges/op"
	"github.com/d-led/eventfulranges/store"
)

// Store is an in-memory EventStore.
type Store struct {
	mu          sync.Mutex
	ops         []op.Op
	snapshot    []byte
	snapshotVer int64
	hasSnapshot bool
}

// New returns an empty in-memory store.
func New() *Store {
	return &Store{}
}

// Append appends the events if the version matches the current log length.
func (s *Store) Append(_ context.Context, expectedVersion int64, events []op.Op) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if int64(len(s.ops)) != expectedVersion {
		return store.ErrVersionConflict
	}
	s.ops = append(s.ops, events...)
	return nil
}

// Read returns the operations after fromVersion and the log length.
func (s *Store) Read(_ context.Context, fromVersion int64) ([]op.Op, int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if fromVersion < 0 {
		fromVersion = 0
	}
	n := int64(len(s.ops))
	if fromVersion > n {
		fromVersion = n
	}
	return append([]op.Op(nil), s.ops[fromVersion:]...), n, nil
}

// SaveSnapshot stores a copy of the snapshot data.
func (s *Store) SaveSnapshot(_ context.Context, data []byte, version int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot = append([]byte(nil), data...)
	s.snapshotVer = version
	s.hasSnapshot = true
	return nil
}

// LoadSnapshot returns the stored snapshot.
func (s *Store) LoadSnapshot(_ context.Context) ([]byte, int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.hasSnapshot {
		return nil, 0, store.ErrSnapshotNotFound
	}
	return append([]byte(nil), s.snapshot...), s.snapshotVer, nil
}
