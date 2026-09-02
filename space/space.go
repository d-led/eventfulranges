// Package space generalizes the one-dimensional interval set to n dimensions.
//
// A Box is an axis-aligned rectangle whose every face is either closed
// (included) or open (excluded). The default is the half-open form
// [Min[i], Max[i]): Min faces closed, Max faces open, which makes set
// difference exact and boundary assignment deterministic. Boxes may carry
// explicit per-face bounds for closed ranges (see NewBoxWithBounds). Every
// coordinate is a float64 compared without arithmetic, so no error can
// accumulate.
package space

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/d-led/eventfulranges/interval"
)

var (
	// ErrDimensions is returned when Min and Max have different lengths.
	ErrDimensions = errors.New("space: min and max must have the same dimension")
	// ErrNoDimensions is returned when a box has no coordinates.
	ErrNoDimensions = errors.New("space: a box needs at least one dimension")
	// ErrBounds is returned when a bound slice has the wrong length.
	ErrBounds = errors.New("space: bound slices must match the dimension")
	// ErrNaN is returned when a coordinate is not a number.
	ErrNaN = errors.New("space: coordinates must not be NaN")
	// ErrInfinite is returned when a coordinate is infinite.
	ErrInfinite = errors.New("space: coordinates must be finite")
	// ErrInverted is returned when Min exceeds Max in some dimension.
	ErrInverted = errors.New("space: min must not exceed max")
	// ErrEmpty is returned when a box contains no point at all.
	ErrEmpty = errors.New("space: a box must contain at least one point")
)

// Bound marks whether a box face belongs to the box. It is the per-face
// counterpart of interval.Bound in the one-dimensional package: Closed
// includes the face, Open excludes it.
type Bound = interval.Bound

// The two bound values, re-exported for convenience.
const (
	Closed = interval.Closed
	Open   = interval.Open
)

// Box is an axis-aligned n-dimensional rectangle. The zero value has no
// dimensions and fails validation; build boxes with NewBox instead. A box with
// nil bound slices is the half-open form [Min, Max): every Min face closed,
// every Max face open. Meta carries arbitrary JSON-object metadata (for
// example a stroke color); it is opaque to the geometry and merged by the
// engine when boxes combine.
type Box struct {
	Min      []float64       `json:"min"`
	Max      []float64       `json:"max"`
	MinBound []Bound         `json:"minBound,omitempty"`
	MaxBound []Bound         `json:"maxBound,omitempty"`
	Meta     json.RawMessage `json:"meta,omitempty"`
}

// NewBox builds a half-open box [lo, hi): Min faces closed, Max faces open.
// Both slices are copied, so the caller keeps ownership of the originals.
func NewBox(lo, hi []float64) Box {
	return Box{Min: append([]float64(nil), lo...), Max: append([]float64(nil), hi...)}
}

// NewBoxWithBounds builds a box with explicit per-face inclusivity. A nil
// loB/hiB means the default: Min faces closed, Max faces open.
func NewBoxWithBounds(lo, hi []float64, loB, hiB []Bound) Box {
	return Box{
		Min:      append([]float64(nil), lo...),
		Max:      append([]float64(nil), hi...),
		MinBound: append([]Bound(nil), loB...),
		MaxBound: append([]Bound(nil), hiB...),
	}
}

// loBound reports whether the Min face on axis i is included, defaulting to
// Closed for boxes without explicit bounds (the historical half-open form).
func (b Box) loBound(i int) Bound {
	if i < len(b.MinBound) {
		return b.MinBound[i]
	}
	return Closed
}

// hiBound reports whether the Max face on axis i is included, defaulting to
// Open for boxes without explicit bounds.
func (b Box) hiBound(i int) Bound {
	if i < len(b.MaxBound) {
		return b.MaxBound[i]
	}
	return Open
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
	if len(b.MinBound) > 0 && len(b.MinBound) != len(b.Min) {
		return ErrBounds
	}
	if len(b.MaxBound) > 0 && len(b.MaxBound) != len(b.Min) {
		return ErrBounds
	}
	for i := range b.Min {
		if err := validateCoordinate(b.Min[i], b.Max[i]); err != nil {
			return err
		}
	}
	return nil
}

// validateCoordinate checks one axis's lower and upper corners.
func validateCoordinate(lo, hi float64) error {
	switch {
	case math.IsNaN(lo) || math.IsNaN(hi):
		return ErrNaN
	case math.IsInf(lo, 0) || math.IsInf(hi, 0):
		return ErrInfinite
	case lo > hi:
		return ErrInverted
	}
	return nil
}

// Empty reports whether the box contains no point at all.
func (b Box) Empty() bool {
	for i := range b.Min {
		if b.Min[i] > b.Max[i] {
			return true
		}
		if b.Min[i] == b.Max[i] && (b.loBound(i) == Open || b.hiBound(i) == Open) {
			return true
		}
	}
	return false
}

// Contains reports whether the point p lies inside the box.
func (b Box) Contains(p []float64) bool {
	for i := range b.Min {
		if p[i] < b.Min[i] || p[i] > b.Max[i] {
			return false
		}
		if p[i] == b.Min[i] && b.loBound(i) == Open {
			return false
		}
		if p[i] == b.Max[i] && b.hiBound(i) == Open {
			return false
		}
	}
	return true
}

// Overlaps reports whether the two boxes share at least one point.
func Overlaps(a, b Box) bool {
	for i := range a.Min {
		if !intervalOverlap(
			a.Min[i], b.Min[i], a.Max[i], b.Max[i],
			a.loBound(i), b.loBound(i), a.hiBound(i), b.hiBound(i),
		) {
			return false
		}
	}
	return true
}

// intervalOverlap reports whether [alo, ahi] and [blo, bhi] share a point,
// respecting endpoint inclusivity.
func intervalOverlap(alo, blo, ahi, bhi float64, aloB, bloB, ahiB, bhiB Bound) bool {
	lo := math.Max(alo, blo)
	hi := math.Min(ahi, bhi)
	if lo < hi {
		return true
	}
	if lo != hi {
		return false
	}
	return inInterval(lo, alo, ahi, aloB, ahiB) && inInterval(lo, blo, bhi, bloB, bhiB)
}

// inInterval reports whether x belongs to [lo, hi] with the given bounds.
func inInterval(x, lo, hi float64, loB, hiB Bound) bool {
	if x < lo || x > hi {
		return false
	}
	if x == lo && loB == Open {
		return false
	}
	if x == hi && hiB == Open {
		return false
	}
	return true
}

// subsumes reports whether b covers a entirely.
func subsumes(b, a Box) bool {
	for i := range a.Min {
		if !intervalContains(b.Min[i], b.Max[i], b.loBound(i), b.hiBound(i), a.Min[i], a.Max[i], a.loBound(i), a.hiBound(i)) {
			return false
		}
	}
	return true
}

// intervalContains reports whether [alo, ahi] is a subset of [blo, bhi].
func intervalContains(blo, bhi float64, bloB, bhiB Bound, alo, ahi float64, aloB, ahiB Bound) bool {
	if alo < blo || ahi > bhi {
		return false
	}
	if alo == blo && aloB == Closed && bloB == Open {
		return false
	}
	if ahi == bhi && ahiB == Closed && bhiB == Open {
		return false
	}
	return true
}

// String renders the box with interval notation per dimension, e.g.
// "[0 10] x [2 5]".
func (b Box) String() string {
	parts := make([]string, 0, len(b.Min))
	for i := range b.Min {
		left, right := '[', ']'
		if b.loBound(i) == Open {
			left = '('
		}
		if b.hiBound(i) == Open {
			right = ')'
		}
		parts = append(parts, fmt.Sprintf("%c%g %g%c", left, b.Min[i], b.Max[i], right))
	}
	return strings.Join(parts, " x ")
}
