// Package op defines the box operations that make up the n-dimensional CRDT
// event log. It is the n-dimensional counterpart of the root op package: the
// same operation shapes, but over a space.Box instead of a 1-D interval.
package op

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/d-led/eventfulranges/space"
)

// Kind is the type of a box operation.
type Kind uint8

const (
	// KindAdd includes the box in the materialized set.
	KindAdd Kind = iota
	// KindRemove excludes the box from the materialized set.
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

// Op is an immutable box operation, the atomic unit of the event log.
type Op struct {
	ID     string    `json:"id"`
	Kind   Kind      `json:"kind"`
	Box    space.Box `json:"box"`
	TS     int64     `json:"ts"`
	Origin string    `json:"origin,omitempty"`
}

// Validate reports whether the op is well-formed.
func (o Op) Validate() error {
	if o.ID == "" {
		return ErrMissingID
	}
	if o.Kind != KindAdd && o.Kind != KindRemove {
		return fmt.Errorf("invalid op kind %d", o.Kind)
	}
	return o.Box.Validate()
}

// New builds an op of the given kind with a freshly generated ID.
func New(kind Kind, box space.Box) Op {
	return Op{ID: newID(), Kind: kind, Box: box}
}

// Add builds an Add op over the half-open box [min, max).
func Add(min, max []float64) Op {
	return New(KindAdd, space.NewBox(min, max))
}

// Remove builds a Remove op over the half-open box [min, max).
func Remove(min, max []float64) Op {
	return New(KindRemove, space.NewBox(min, max))
}

// newID returns a process-unique, roughly time-ordered identifier.
func newID() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return strconv.FormatInt(time.Now().UnixNano(), 36) + "-" + hex.EncodeToString(b[:])
}
