// Package op defines the range operations that make up the CRDT event log.
package op

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"

	"gitub.com/d-led/eventfulranges/interval"
)

// Kind is the type of a range operation.
type Kind uint8

const (
	// KindAdd includes the interval in the materialized set.
	KindAdd Kind = iota
	// KindRemove excludes the interval from the materialized set.
	KindRemove
)

// String returns "add" or "remove".
func (k Kind) String() string {
	if k == KindAdd {
		return "add"
	}
	return "remove"
}

// MarshalText implements encoding.TextMarshaler.
func (k Kind) MarshalText() ([]byte, error) {
	return []byte(k.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (k *Kind) UnmarshalText(text []byte) error {
	switch string(text) {
	case "add":
		*k = KindAdd
	case "remove":
		*k = KindRemove
	default:
		return fmt.Errorf("invalid op kind %q", text)
	}
	return nil
}

// ErrMissingID is returned when an op has no identifier.
var ErrMissingID = errors.New("op id must not be empty")

// Op is an immutable range operation, the atomic unit of the event log.
type Op struct {
	ID       string            `json:"id"`
	Kind     Kind              `json:"kind"`
	Interval interval.Interval `json:"interval"`
	TS       int64             `json:"ts"`
	Origin   string            `json:"origin,omitempty"`
}

// Validate reports whether the op is well-formed.
func (o Op) Validate() error {
	if o.ID == "" {
		return ErrMissingID
	}
	if o.Kind != KindAdd && o.Kind != KindRemove {
		return fmt.Errorf("invalid op kind %d", o.Kind)
	}
	return o.Interval.Validate()
}

// New builds an op of the given kind with a freshly generated ID.
func New(kind Kind, iv interval.Interval) Op {
	return Op{ID: newID(), Kind: kind, Interval: iv}
}

// Add builds a closed Add op over [start, end].
func Add(start, end float64) Op {
	return New(KindAdd, interval.Interval{Start: start, End: end})
}

// AddWithBounds builds an Add op with explicit boundary inclusivity.
func AddWithBounds(start, end float64, sb, eb interval.Bound) Op {
	return New(KindAdd, interval.Interval{Start: start, End: end, StartBound: sb, EndBound: eb})
}

// Remove builds a closed Remove op over [start, end].
func Remove(start, end float64) Op {
	return New(KindRemove, interval.Interval{Start: start, End: end})
}

// RemoveWithBounds builds a Remove op with explicit boundary inclusivity.
func RemoveWithBounds(start, end float64, sb, eb interval.Bound) Op {
	return New(KindRemove, interval.Interval{Start: start, End: end, StartBound: sb, EndBound: eb})
}

// newID returns a process-unique, roughly time-ordered identifier.
func newID() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return strconv.FormatInt(time.Now().UnixNano(), 36) + "-" + hex.EncodeToString(b[:])
}
