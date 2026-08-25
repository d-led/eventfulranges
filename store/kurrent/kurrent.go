//go:build kurrent

// Package kurrent implements an EventStore backed by KurrentDB.
package kurrent

import (
	"context"
	"encoding/json"
	"errors"
	"io"

	"github.com/EventStore/EventStore-Client-Go/v4/esdb"
	"github.com/google/uuid"

	"gitub.com/d-led/eventfulranges/op"
	"gitub.com/d-led/eventfulranges/store"
)

var (
	// opsStream holds the operation log.
	opsStream = "eventfulranges-ops"
	// snapshotStream holds the latest materialized snapshot.
	snapshotStream = "eventfulranges-snapshot"

	// errStreamNotFound marks a missing KurrentDB stream: an empty log or no
	// snapshot yet, depending on which stream was read.
	errStreamNotFound = errors.New("kurrent: stream not found")
)

// translateError maps KurrentDB error codes onto store-level errors. Other
// errors pass through unchanged.
func translateError(err error) error {
	if err == nil {
		return nil
	}
	esdbErr, _ := esdb.FromError(err)
	switch esdbErr.Code() {
	case esdb.ErrorCodeWrongExpectedVersion:
		return store.ErrVersionConflict
	case esdb.ErrorCodeResourceNotFound:
		return errStreamNotFound
	default:
		return err
	}
}

// streamNotFound reports whether err signals a missing KurrentDB stream.
func streamNotFound(err error) bool {
	return errors.Is(translateError(err), errStreamNotFound)
}

// Store is an event log backed by KurrentDB. It satisfies store.Log and
// store.Snapshotter.
type Store struct {
	client *esdb.Client
}

// Open connects to KurrentDB at the given connection string, for example
// "esdb://localhost:2113?tls=false".
func Open(connectionString string) (*Store, error) {
	config, err := esdb.ParseConnectionString(connectionString)
	if err != nil {
		return nil, err
	}
	c, err := esdb.NewClient(config)
	if err != nil {
		return nil, err
	}
	return &Store{client: c}, nil
}

// Close releases the underlying connection.
func (s *Store) Close() error {
	return s.client.Close()
}

// Append atomically appends the events at the given expected log length. It
// returns store.ErrVersionConflict if another writer appended first.
func (s *Store) Append(ctx context.Context, expectedVersion int64, events []op.Op) error {
	data := make([]esdb.EventData, 0, len(events))
	for _, e := range events {
		payload, err := json.Marshal(e)
		if err != nil {
			return err
		}
		id, err := uuid.NewRandom()
		if err != nil {
			return err
		}
		data = append(data, esdb.EventData{
			EventID:     id,
			EventType:   "range-op",
			ContentType: esdb.ContentTypeJson,
			Data:        payload,
		})
	}
	_, err := s.client.AppendToStream(ctx, opsStream, esdb.AppendToStreamOptions{
		ExpectedRevision: expectedRevision(expectedVersion),
	}, data...)
	return translateError(err)
}

// Read returns the operations at positions >= fromVersion and the new log
// length.
func (s *Store) Read(ctx context.Context, fromVersion int64) ([]op.Op, int64, error) {
	stream, err := s.client.ReadStream(ctx, opsStream, esdb.ReadStreamOptions{
		Direction: esdb.Forwards,
		From:      esdb.Revision(uint64(max(fromVersion, 0))),
	}, ^uint64(0))
	if err != nil {
		if streamNotFound(err) {
			return nil, 0, nil
		}
		return nil, 0, err
	}
	defer stream.Close()

	var (
		ops     []op.Op
		version = fromVersion
	)
	for {
		ev, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			if streamNotFound(err) {
				return nil, 0, nil
			}
			return nil, 0, err
		}
		rec := ev.OriginalEvent()
		var o op.Op
		if err := json.Unmarshal(rec.Data, &o); err != nil {
			return nil, 0, err
		}
		ops = append(ops, o)
		version = int64(rec.EventNumber) + 1
	}
	return ops, version, nil
}

// snapshotMeta is stored as the event metadata of a snapshot event.
type snapshotMeta struct {
	Version int64 `json:"version"`
}

// SaveSnapshot stores a materialized snapshot at the given version.
func (s *Store) SaveSnapshot(ctx context.Context, data []byte, version int64) error {
	// snapshotMeta is a plain struct of JSON types, so Marshal cannot fail.
	meta, _ := json.Marshal(snapshotMeta{Version: version})
	id, err := uuid.NewRandom()
	if err != nil {
		return err
	}
	_, err = s.client.AppendToStream(ctx, snapshotStream, esdb.AppendToStreamOptions{
		ExpectedRevision: esdb.Any{},
	}, esdb.EventData{
		EventID:     id,
		EventType:   "snapshot",
		ContentType: esdb.ContentTypeJson,
		Data:        data,
		Metadata:    meta,
	})
	return translateError(err)
}

// LoadSnapshot returns the latest snapshot and its version, or
// store.ErrSnapshotNotFound when none exists.
func (s *Store) LoadSnapshot(ctx context.Context) ([]byte, int64, error) {
	stream, err := s.client.ReadStream(ctx, snapshotStream, esdb.ReadStreamOptions{
		Direction: esdb.Backwards,
		From:      esdb.End{},
	}, 1)
	if err != nil {
		if streamNotFound(err) {
			return nil, 0, store.ErrSnapshotNotFound
		}
		return nil, 0, err
	}
	defer stream.Close()

	ev, err := stream.Recv()
	if errors.Is(err, io.EOF) {
		return nil, 0, store.ErrSnapshotNotFound
	}
	if err != nil {
		if streamNotFound(err) {
			return nil, 0, store.ErrSnapshotNotFound
		}
		return nil, 0, err
	}
	rec := ev.OriginalEvent()
	var meta snapshotMeta
	if err := json.Unmarshal(rec.UserMetadata, &meta); err != nil {
		return nil, 0, err
	}
	return rec.Data, meta.Version, nil
}

// expectedRevision maps our log-length version onto KurrentDB revisions,
// which are zero-based.
func expectedRevision(version int64) esdb.ExpectedRevision {
	if version == 0 {
		return esdb.NoStream{}
	}
	return esdb.Revision(uint64(version - 1))
}
