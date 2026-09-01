// Package interval defines open and closed real-valued intervals together
// with canonical set operations over them.
//
// Intervals use float64 endpoints with per-side inclusivity: [a,b], [a,b),
// (a,b] and (a,b). All operations compare endpoints only and never perform
// arithmetic on them, so floating-point values cannot accumulate error:
// every endpoint is treated exactly as provided.
package interval

import (
	"errors"
	"fmt"
	"math"
)

// Bound marks whether an interval endpoint belongs to the interval.
type Bound uint8

const (
	// Closed includes the endpoint in the interval.
	Closed Bound = iota
	// Open excludes the endpoint from the interval.
	Open
)

// String returns "closed" or "open".
func (b Bound) String() string {
	if b == Closed {
		return "closed"
	}
	return "open"
}

// MarshalText implements encoding.TextMarshaler.
func (b Bound) MarshalText() ([]byte, error) {
	return []byte(b.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (b *Bound) UnmarshalText(text []byte) error {
	switch string(text) {
	case "closed":
		*b = Closed
	case "open":
		*b = Open
	default:
		return fmt.Errorf("invalid bound %q", text)
	}
	return nil
}

// invert returns the opposite bound.
func invert(b Bound) Bound {
	if b == Closed {
		return Open
	}
	return Closed
}

var (
	// ErrNaN is returned when an endpoint is not a number.
	ErrNaN = errors.New("interval endpoints must not be NaN")
	// ErrInfinite is returned when an endpoint is infinite.
	ErrInfinite = errors.New("interval endpoints must be finite")
	// ErrInverted is returned when the start exceeds the end.
	ErrInverted = errors.New("interval start must not exceed its end")
	// ErrEmpty is returned when the interval contains no point.
	ErrEmpty = errors.New("interval must contain at least one point")
)

// Interval is a contiguous, non-empty range of real values.
// The zero value is the point interval [0,0].
type Interval struct {
	Start      float64 `json:"start"`
	End        float64 `json:"end"`
	StartBound Bound   `json:"startBound"`
	EndBound   Bound   `json:"endBound"`
}

// Validate reports whether the interval is well-formed: finite endpoints,
// start not after end, and at least one contained point.
func (iv Interval) Validate() error {
	switch {
	case math.IsNaN(iv.Start) || math.IsNaN(iv.End):
		return ErrNaN
	case math.IsInf(iv.Start, 0) || math.IsInf(iv.End, 0):
		return ErrInfinite
	case iv.Start > iv.End:
		return ErrInverted
	case iv.Start == iv.End && (iv.StartBound == Open || iv.EndBound == Open):
		return ErrEmpty
	}
	return nil
}

// Contains reports whether x belongs to the interval.
func (iv Interval) Contains(x float64) bool {
	if x < iv.Start || x > iv.End {
		return false
	}
	if x == iv.Start && iv.StartBound == Open {
		return false
	}
	if x == iv.End && iv.EndBound == Open {
		return false
	}
	return true
}

// empty reports whether the interval contains no point at all.
func (iv Interval) empty() bool {
	return iv.Start > iv.End || (iv.Start == iv.End && (iv.StartBound == Open || iv.EndBound == Open))
}

// String renders the interval in standard notation, e.g. "[1,5)".
func (iv Interval) String() string {
	left, right := "[", "]"
	if iv.StartBound == Open {
		left = "("
	}
	if iv.EndBound == Open {
		right = ")"
	}
	return fmt.Sprintf("%s%v,%v%s", left, iv.Start, iv.End, right)
}

// Overlaps reports whether the intervals share at least one point.
func (iv Interval) Overlaps(b Interval) bool {
	lo, loB := maxStart(iv, b)
	hi, hiB := minEnd(iv, b)
	if lo > hi {
		return false
	}
	if lo < hi {
		return true
	}
	return loB == Closed && hiB == Closed
}

// Touches reports whether the intervals meet at exactly one endpoint point
// that belongs to at least one of them, without sharing it.
func (iv Interval) Touches(b Interval) bool {
	switch {
	case iv.End == b.Start:
		if iv.EndBound == Open && b.StartBound == Open {
			return false
		}
		return !iv.Overlaps(b)
	case b.End == iv.Start:
		if b.EndBound == Open && iv.StartBound == Open {
			return false
		}
		return !iv.Overlaps(b)
	}
	return false
}

// Merge combines two intervals whose union is contiguous into the smallest
// interval containing both.
func Merge(a, b Interval) Interval {
	s, sb := a.Start, a.StartBound
	if b.Start < a.Start || (b.Start == a.Start && sb == Open && b.StartBound == Closed) {
		s, sb = b.Start, b.StartBound
	}
	e, eb := a.End, a.EndBound
	if b.End > a.End || (b.End == a.End && eb == Open && b.EndBound == Closed) {
		e, eb = b.End, b.EndBound
	}
	return Interval{Start: s, End: e, StartBound: sb, EndBound: eb}
}

// Subtract returns the parts of iv left after removing every point covered
// by cut: zero, one or two intervals in order.
func (iv Interval) Subtract(cut Interval) []Interval {
	if !iv.Overlaps(cut) && !iv.Touches(cut) {
		return []Interval{iv}
	}
	var out []Interval
	if left, ok := leftPart(iv, cut); ok {
		out = append(out, left)
	}
	if right, ok := rightPart(iv, cut); ok {
		out = append(out, right)
	}
	return out
}

// leftPart returns the part of iv before cut, when it exists.
func leftPart(iv, cut Interval) (Interval, bool) {
	if iv.Start < cut.Start {
		return Interval{
			Start:      iv.Start,
			End:        cut.Start,
			StartBound: iv.StartBound,
			EndBound:   invert(cut.StartBound),
		}, true
	}
	if iv.Start == cut.Start && iv.StartBound == Closed && cut.StartBound == Open {
		return Interval{Start: iv.Start, End: cut.Start, StartBound: Closed, EndBound: Closed}, true
	}
	return Interval{}, false
}

// rightPart returns the part of iv after cut, when it exists.
func rightPart(iv, cut Interval) (Interval, bool) {
	if cut.End < iv.End {
		return Interval{
			Start:      cut.End,
			End:        iv.End,
			StartBound: invert(cut.EndBound),
			EndBound:   iv.EndBound,
		}, true
	}
	if cut.End == iv.End && cut.EndBound == Open && iv.EndBound == Closed {
		return Interval{Start: cut.End, End: iv.End, StartBound: Closed, EndBound: Closed}, true
	}
	return Interval{}, false
}

// Intersection returns the shared interval. It requires a.Overlaps(b).
func Intersection(a, b Interval) Interval {
	s, sb := maxStart(a, b)
	e, eb := minEnd(a, b)
	return Interval{Start: s, End: e, StartBound: sb, EndBound: eb}
}

// maxStart returns the larger start and the bound of the shared start.
func maxStart(a, b Interval) (float64, Bound) {
	switch {
	case a.Start > b.Start:
		return a.Start, a.StartBound
	case a.Start < b.Start:
		return b.Start, b.StartBound
	default:
		if a.StartBound == Closed || b.StartBound == Closed {
			return a.Start, Closed
		}
		return a.Start, Open
	}
}

// minEnd returns the smaller end and the bound of the shared end.
func minEnd(a, b Interval) (float64, Bound) {
	switch {
	case a.End < b.End:
		return a.End, a.EndBound
	case a.End > b.End:
		return b.End, b.EndBound
	default:
		if a.EndBound == Closed || b.EndBound == Closed {
			return a.End, Closed
		}
		return a.End, Open
	}
}
