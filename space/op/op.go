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
	// KindRetract cancels the operation its Ref names, undoing that one
	// edit without touching anyone else's.
	KindRetract
)

// String returns "add", "remove", or "retract".
func (k Kind) String() string {
	switch k {
	case KindAdd:
		return "add"
	case KindRemove:
		return "remove"
	case KindRetract:
		return "retract"
	}
	return fmt.Sprintf("kind(%d)", k)
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
	case "retract":
		*k = KindRetract
	default:
		return fmt.Errorf("invalid op kind %q", text)
	}
	return nil
}

// ErrMissingID is returned when an op has no identifier.
var ErrMissingID = errors.New("op id must not be empty")

// ErrMissingRef is returned when a retract op names no target.
var ErrMissingRef = errors.New("retract op must name a target")

// Op is an immutable box operation, the atomic unit of the event log.
type Op struct {
	ID     string    `json:"id"`
	Kind   Kind      `json:"kind"`
	Box    space.Box `json:"box"`
	TS     int64     `json:"ts"`
	Ref    string    `json:"ref,omitempty"`
	Origin string    `json:"origin,omitempty"`
}

// Validate reports whether the op is well-formed.
func (o Op) Validate() error {
	if o.ID == "" {
		return ErrMissingID
	}
	switch o.Kind {
	case KindAdd, KindRemove:
	case KindRetract:
		if o.Ref == "" {
			return ErrMissingRef
		}
	default:
		return fmt.Errorf("invalid op kind %d", o.Kind)
	}
	if err := o.Box.Validate(); err != nil {
		return err
	}
	if o.Box.Empty() {
		return space.ErrEmpty
	}
	return nil
}

// New builds an op of the given kind with a freshly generated ID.
func New(kind Kind, box space.Box) Op {
	return Op{ID: newID(), Kind: kind, Box: box}
}

// Add builds an Add op over the half-open box [lo, hi).
func Add(lo, hi []float64) Op {
	return New(KindAdd, space.NewBox(lo, hi))
}

// Remove builds a Remove op over the half-open box [lo, hi).
func Remove(lo, hi []float64) Op {
	return New(KindRemove, space.NewBox(lo, hi))
}

// Retract builds a Retract op that cancels the operation named ref, carrying
// that operation's box so the retraction validates against the set's geometry.
func Retract(ref string, box space.Box) Op {
	return Op{ID: newID(), Kind: KindRetract, Box: box, Ref: ref}
}

// newID returns a process-unique, roughly time-ordered identifier.
func newID() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return strconv.FormatInt(time.Now().UnixNano(), 36) + "-" + hex.EncodeToString(b[:])
}
