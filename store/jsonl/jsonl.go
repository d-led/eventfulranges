// Package jsonl persists the event log as a single JSON Lines stream:
// operation records are appended in order and materialized snapshots are
// embedded as records in the same file, so the stream can be compacted. A
// store instance is not safe for concurrent use by multiple processes; the
// engine's optimistic concurrency detects lost updates within one process.
package jsonl

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/d-led/eventfulranges/op"
	"github.com/d-led/eventfulranges/store"
)

// Store persists operations and snapshots in one JSON Lines stream.
type Store struct {
	mu      sync.RWMutex
	opsPath string

	// appendLines writes the serialized events to the log. It is replaceable
	// in tests to simulate write failures.
	appendLines func(file *os.File, events []op.Op) error
}

// Open returns a store rooted at dir, creating it if needed. The append-only
// stream lives in ranges.stream.jsonl and carries both operations and
// snapshots.
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	return &Store{opsPath: filepath.Join(dir, "ranges.stream.jsonl")}, nil
}

// Append appends the events as JSON Lines if the version matches the current
// operation count.
func (s *Store) Append(_ context.Context, expectedVersion int64, events []op.Op) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	version, err := s.count()
	if err != nil {
		return err
	}
	if version != expectedVersion {
		return store.ErrVersionConflict
	}
	file, err := os.OpenFile(s.opsPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	appendLines := s.appendLines
	if appendLines == nil {
		appendLines = s.appendEvents
	}
	return appendLines(file, events)
}

// appendEvents writes the serialized events as JSON Lines.
func (s *Store) appendEvents(file *os.File, events []op.Op) error {
	for _, event := range events {
		// op.Op is a plain struct of JSON types, so Marshal cannot fail.
		data, _ := json.Marshal(event)
		if _, err := file.Write(append(data, '\n')); err != nil {
			return err
		}
	}
	return nil
}

// Read returns the operations after fromVersion and the total operation
// count.
func (s *Store) Read(_ context.Context, fromVersion int64) ([]op.Op, int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.read(fromVersion)
}

// read is Read without the lock, so Compact can reuse it while holding it.
func (s *Store) read(fromVersion int64) ([]op.Op, int64, error) {
	var out []op.Op
	version, err := s.scan(func(ordinal int64, o *op.Op, _ *snapshotMarker) {
		if o != nil && ordinal > fromVersion {
			out = append(out, *o)
		}
	})
	return out, version, err
}

// SaveSnapshot appends a snapshot record to the end of the stream.
func (s *Store) SaveSnapshot(_ context.Context, data []byte, version int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	file, err := os.OpenFile(s.opsPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	return appendSnapshot(file, version, data)
}

// LoadSnapshot returns the newest snapshot record in the stream.
func (s *Store) LoadSnapshot(_ context.Context) ([]byte, int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var latest *snapshotMarker
	_, err := s.scan(func(_ int64, _ *op.Op, snap *snapshotMarker) {
		if snap != nil && (latest == nil || snap.Version >= latest.Version) {
			latest = snap
		}
	})
	if err != nil {
		return nil, 0, err
	}
	if latest == nil {
		return nil, 0, store.ErrSnapshotNotFound
	}
	return latest.Data, latest.Version, nil
}

// count returns the number of operation records in the log.
func (s *Store) count() (int64, error) {
	return s.scan(func(_ int64, _ *op.Op, _ *snapshotMarker) {})
}

// Compact rewrites the stream as the given snapshot followed by the
// operations it does not cover, collapsing the file to its smallest form.
func (s *Store) Compact(_ context.Context, snapshotData []byte, version int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tail, _, err := s.read(version)
	if err != nil {
		return err
	}
	payload, err := snapshotPayload(version, snapshotData)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	buf.Write(payload)
	buf.WriteByte('\n')
	for _, o := range tail {
		data, _ := json.Marshal(o)
		buf.Write(data)
		buf.WriteByte('\n')
	}
	return writeAtomic(s.opsPath, buf.Bytes())
}

// appendSnapshot writes one snapshot record, carrying the materialized view.
func appendSnapshot(file *os.File, version int64, data []byte) error {
	payload, err := snapshotPayload(version, data)
	if err != nil {
		return err
	}
	_, err = file.Write(append(payload, '\n'))
	return err
}

// snapshotPayload marshals the on-stream snapshot record for the given view.
func snapshotPayload(version int64, data []byte) ([]byte, error) {
	if !json.Valid(data) {
		return nil, errors.New("jsonl: snapshot data is not valid JSON")
	}
	return json.Marshal(struct {
		Snapshot snapshotMarker `json:"snapshot"`
	}{Snapshot: snapshotMarker{Version: version, Data: data}})
}

// writeAtomic replaces path with data through a temporary file and rename.
func writeAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// snapshotMarker is the on-stream snapshot record.
type snapshotMarker struct {
	Version int64           `json:"version"`
	Data    json.RawMessage `json:"data"`
}

// record discriminates between an operation record and a snapshot record.
type record struct {
	Snapshot *snapshotMarker `json:"snapshot,omitempty"`
}

// parseLine splits a stream line into its operation or snapshot form.
func parseLine(data []byte) (*op.Op, *snapshotMarker, error) {
	var r record
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, nil, err
	}
	if r.Snapshot != nil {
		return nil, r.Snapshot, nil
	}
	var o op.Op
	if err := json.Unmarshal(data, &o); err != nil {
		return nil, nil, err
	}
	return &o, nil, nil
}

// scan walks every stream record, invoking fn with the running operation
// ordinal, the operation (for op records), or the snapshot (for snapshot
// records). A snapshot advances the ordinal baseline to its version, so
// operations after a compacted snapshot keep their logical positions. It
// returns the logical operation count.
func (s *Store) scan(fn func(ordinal int64, o *op.Op, snap *snapshotMarker)) (int64, error) {
	file, err := os.Open(s.opsPath)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	defer func() { _ = file.Close() }()

	var baseline, ordinal int64
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		o, snap, err := parseLine(scanner.Bytes())
		if err != nil {
			return ordinal, fmt.Errorf("jsonl: %w", err)
		}
		baseline, ordinal = advance(baseline, ordinal, o, snap, fn)
	}
	if ordinal < baseline {
		ordinal = baseline
	}
	return ordinal, scanner.Err()
}

// advance folds one record into the running ordinals and invokes fn. A
// snapshot raises the baseline to its version, so operations after a
// compacted snapshot keep their logical positions.
func advance(baseline, ordinal int64, o *op.Op, snap *snapshotMarker, fn func(ordinal int64, o *op.Op, snap *snapshotMarker)) (int64, int64) {
	if snap != nil {
		if snap.Version > baseline {
			baseline = snap.Version
		}
		fn(ordinal, nil, snap)
		return baseline, ordinal
	}
	if ordinal < baseline {
		ordinal = baseline
	}
	ordinal++
	fn(ordinal, o, nil)
	return baseline, ordinal
}
