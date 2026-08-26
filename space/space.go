// Package space generalizes the one-dimensional interval set to n dimensions.
//
// A Box is an axis-aligned, half-open rectangle [Min[i], Max[i]) in each
// dimension. Half-open bounds make set difference exact: subtracting one box
// from another always yields a small set of boxes, with no boundary cases
// around shared edges. Every coordinate is a float64 compared without
// arithmetic, so no error can accumulate.
package space

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
)

var (
	// ErrDimensions is returned when Min and Max have different lengths.
	ErrDimensions = errors.New("space: min and max must have the same dimension")
	// ErrNoDimensions is returned when a box has no coordinates.
	ErrNoDimensions = errors.New("space: a box needs at least one dimension")
	// ErrNaN is returned when a coordinate is not a number.
	ErrNaN = errors.New("space: coordinates must not be NaN")
	// ErrInfinite is returned when a coordinate is infinite.
	ErrInfinite = errors.New("space: coordinates must be finite")
	// ErrInverted is returned when Min exceeds Max in some dimension.
	ErrInverted = errors.New("space: min must not exceed max")
	// ErrEmpty is returned when a box contains no point at all.
	ErrEmpty = errors.New("space: a box must contain at least one point")
)

// Box is an axis-aligned, half-open n-dimensional rectangle. The zero value
// has no dimensions and fails validation; build boxes with NewBox instead.
// Meta carries arbitrary JSON-object metadata (for example a stroke color);
// it is opaque to the geometry and merged by the engine when boxes combine.
type Box struct {
	Min  []float64       `json:"min"`
	Max  []float64       `json:"max"`
	Meta json.RawMessage `json:"meta,omitempty"`
}

// NewBox builds a box from the given lower and upper corners. Both slices are
// copied, so the caller keeps ownership of the originals.
func NewBox(min, max []float64) Box {
	return Box{Min: append([]float64(nil), min...), Max: append([]float64(nil), max...)}
}

// WithMeta returns a copy of b carrying the given metadata.
func (b Box) WithMeta(meta json.RawMessage) Box {
	b.Meta = append(json.RawMessage(nil), meta...)
	return b
}

// Dims returns the number of dimensions.
func (b Box) Dims() int {
	return len(b.Min)
}

// Validate reports whether the box is well-formed.
func (b Box) Validate() error {
	if len(b.Min) != len(b.Max) {
		return ErrDimensions
	}
	if len(b.Min) == 0 {
		return ErrNoDimensions
	}
	for i := range b.Min {
		switch {
		case math.IsNaN(b.Min[i]) || math.IsNaN(b.Max[i]):
			return ErrNaN
		case math.IsInf(b.Min[i], 0) || math.IsInf(b.Max[i], 0):
			return ErrInfinite
		case b.Min[i] > b.Max[i]:
			return ErrInverted
		}
	}
	return nil
}

// Empty reports whether the box contains no point at all.
func (b Box) Empty() bool {
	for i := range b.Min {
		if b.Min[i] >= b.Max[i] {
			return true
		}
	}
	return false
}

// Contains reports whether the point p lies inside the box.
func (b Box) Contains(p []float64) bool {
	for i := range b.Min {
		if p[i] < b.Min[i] || p[i] >= b.Max[i] {
			return false
		}
	}
	return true
}

// Overlaps reports whether the two boxes share at least one point.
func Overlaps(a, b Box) bool {
	for i := range a.Min {
		if a.Min[i] >= b.Max[i] || b.Min[i] >= a.Max[i] {
			return false
		}
	}
	return true
}

// subsumes reports whether b covers a entirely.
func subsumes(b, a Box) bool {
	for i := range a.Min {
		if b.Min[i] > a.Min[i] || b.Max[i] < a.Max[i] {
			return false
		}
	}
	return true
}

// String renders the box as "[min max]" per dimension, e.g.
// "[0 10] x [2 5]".
func (b Box) String() string {
	parts := make([]string, 0, len(b.Min))
	for i := range b.Min {
		parts = append(parts, fmt.Sprintf("[%g %g)", b.Min[i], b.Max[i]))
	}
	return strings.Join(parts, " x ")
}
