// Package jsonl persists the event log as JSON Lines with a sidecar
// snapshot file. A store instance is not safe for concurrent use by multiple
// processes; the engine's optimistic concurrency detects lost updates within
// one process.
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

	"gitub.com/d-led/eventfulranges/op"
	"gitub.com/d-led/eventfulranges/store"
)

// Store persists operations in a JSON Lines file with a sidecar snapshot.
type Store struct {
	mu       sync.Mutex
	opsPath  string
	snapPath string

	// appendLines writes the serialized events to the log. It is replaceable
	// in tests to simulate write failures.
	appendLines func(file *os.File, events []op.Op) error
}

// Open returns a store rooted at dir, creating it if needed.
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Store{
		opsPath:  filepath.Join(dir, "ranges.jsonl"),
		snapPath: filepath.Join(dir, "ranges.snapshot.json"),
	}, nil
}

// Append appends the events as JSON Lines if the version matches the current
// line count.
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
	file, err := os.OpenFile(s.opsPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	appendLines := s.appendLines
	if appendLines == nil {
		appendLines = s.appendEvents
	}
	if err := appendLines(file, events); err != nil {
		return err
	}
	return nil
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

// Read returns the operations after fromVersion and the line count.
func (s *Store) Read(_ context.Context, fromVersion int64) ([]op.Op, int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	file, err := os.Open(s.opsPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, 0, nil
	}
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = file.Close() }()
	var (
		out     []op.Op
		version int64
	)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		version++
		if version <= fromVersion {
			continue
		}
		var event op.Op
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return nil, 0, fmt.Errorf("jsonl: line %d: %w", version, err)
		}
		out = append(out, event)
	}
	return out, version, scanner.Err()
}

// SaveSnapshot atomically replaces the snapshot file.
func (s *Store) SaveSnapshot(_ context.Context, data []byte, version int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// snapshotFile is a plain struct of JSON types, so Marshal cannot fail.
	payload, _ := json.Marshal(snapshotFile{Version: version, Data: data})
	tmp := s.snapPath + ".tmp"
	if err := os.WriteFile(tmp, payload, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.snapPath)
}

// LoadSnapshot returns the stored snapshot data and version.
func (s *Store) LoadSnapshot(_ context.Context) ([]byte, int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.snapPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, 0, store.ErrSnapshotNotFound
	}
	if err != nil {
		return nil, 0, err
	}
	var snap snapshotFile
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, 0, err
	}
	return snap.Data, snap.Version, nil
}

// count returns the number of newline-terminated lines in the log.
func (s *Store) count() (int64, error) {
	data, err := os.ReadFile(s.opsPath)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return int64(bytes.Count(data, []byte{'\n'})), nil
}

// snapshotFile is the on-disk snapshot wrapper.
type snapshotFile struct {
	Version int64  `json:"version"`
	Data    []byte `json:"data"`
}
