//go:build !kurrent

// Package kurrent implements an EventStore backed by KurrentDB. When the
// library is built without the kurrent tag, this stub stands in for the real
// store so the package still compiles; every operation fails with a clear
// message.
package kurrent

import (
	"context"
	"errors"

	"github.com/d-led/eventfulranges/op"
)

var errNotBuilt = errors.New("kurrent: support not compiled; build with -tags kurrent")

// Store is a placeholder for the KurrentDB event store.
type Store struct{}

// Open always fails unless the library was built with -tags kurrent.
func Open(string) (*Store, error) {
	return nil, errNotBuilt
}

// Close is a no-op on the stub.
func (s *Store) Close() error {
	return nil
}

// Append always fails.
func (s *Store) Append(context.Context, int64, []op.Op) error {
	return errNotBuilt
}

// Read always fails.
func (s *Store) Read(context.Context, int64) ([]op.Op, int64, error) {
	return nil, 0, errNotBuilt
}

// SaveSnapshot always fails.
func (s *Store) SaveSnapshot(context.Context, []byte, int64) error {
	return errNotBuilt
}

// LoadSnapshot always fails.
func (s *Store) LoadSnapshot(context.Context) ([]byte, int64, error) {
	return nil, 0, errNotBuilt
}
