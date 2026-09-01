// Package strategy materializes a set of range operations into canonical
// non-overlapping intervals under a chosen conflict-resolution policy.
package strategy

import (
	"errors"
	"fmt"

	"github.com/d-led/eventfulranges/interval"
	"github.com/d-led/eventfulranges/op"
)

// Strategy selects how concurrent operations resolve.
type Strategy uint8

const (
	// LWW lets the operation with the highest (timestamp, id) win at each point.
	LWW Strategy = iota
	// FWW lets the operation with the lowest (timestamp, id) win at each point.
	FWW
	// AdditiveWins unions all additions and subtracts all removals.
	AdditiveWins
	// GrowOnly unions all additions and ignores removals.
	GrowOnly
)

// ErrUnknownStrategy is returned when parsing an unknown strategy name.
var ErrUnknownStrategy = errors.New("unknown strategy")

var names = map[Strategy]string{
	LWW:          "lww",
	FWW:          "fww",
	AdditiveWins: "additive-wins",
	GrowOnly:     "grow-only",
}

// String returns the canonical name of the strategy.
func (s Strategy) String() string {
	if name, ok := names[s]; ok {
		return name
	}
	return fmt.Sprintf("unknown(%d)", s)
}

// MarshalText implements encoding.TextMarshaler.
func (s Strategy) MarshalText() ([]byte, error) {
	if _, ok := names[s]; !ok {
		return nil, ErrUnknownStrategy
	}
	return []byte(s.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (s *Strategy) UnmarshalText(text []byte) error {
	parsed, err := Parse(string(text))
	if err != nil {
		return err
	}
	*s = parsed
	return nil
}

// Parse maps a strategy name to its value.
func Parse(name string) (Strategy, error) {
	for s, n := range names {
		if n == name {
			return s, nil
		}
	}
	return 0, fmt.Errorf("%w: %q", ErrUnknownStrategy, name)
}

// Winner is the operation that decided a segment.
type Winner struct {
	TS   int64   `json:"ts"`
	ID   string  `json:"id"`
	Kind op.Kind `json:"kind"`
}

// Segment is a maximal region decided by a single winner.
type Segment struct {
	Interval interval.Interval `json:"interval"`
	Winner   Winner            `json:"winner"`
}

// Materialize projects the operations into canonical intervals.
func Materialize(s Strategy, ops []op.Op) []interval.Interval {
	switch s {
	case LWW, FWW:
		return ToIntervals(Segments(s, ops))
	case AdditiveWins:
		return interval.Difference(IntervalsOf(ops, op.KindAdd), IntervalsOf(ops, op.KindRemove))
	case GrowOnly:
		return IntervalsOf(ops, op.KindAdd)
	}
	return nil
}

// Segments projects the operations into canonical winner-annotated segments.
// It is defined for LWW and FWW only and returns nil otherwise.
func Segments(s Strategy, ops []op.Op) []Segment {
	if s != LWW && s != FWW {
		return nil
	}
	segs := make([]Segment, 0, len(ops))
	for _, o := range ops {
		segs = append(segs, Segment{
			Interval: o.Interval,
			Winner:   Winner{TS: o.TS, ID: o.ID, Kind: o.Kind},
		})
	}
	return normalize(s, segs)
}

// CombineSegments merges two canonical segment sets under the strategy.
func CombineSegments(s Strategy, a, b []Segment) []Segment {
	segs := make([]Segment, 0, len(a)+len(b))
	segs = append(segs, a...)
	segs = append(segs, b...)
	return normalize(s, segs)
}

// ToIntervals extracts the segments decided by Add operations as a canonical
// interval set.
func ToIntervals(segs []Segment) []interval.Interval {
	ivs := make([]interval.Interval, 0, len(segs))
	for _, seg := range segs {
		if seg.Winner.Kind == op.KindAdd {
			ivs = append(ivs, seg.Interval)
		}
	}
	return interval.Normalize(ivs)
}

// IntervalsOf returns the normalized intervals of the operations of one kind.
func IntervalsOf(ops []op.Op, kind op.Kind) []interval.Interval {
	ivs := make([]interval.Interval, 0, len(ops))
	for _, o := range ops {
		if o.Kind == kind {
			ivs = append(ivs, o.Interval)
		}
	}
	return interval.Normalize(ivs)
}

// normalize reduces overlapping segments to disjoint winner-annotated pieces
// by sweeping the doubled line: every point is decided by the best-priority
// segment covering it, so each maximal run becomes one piece.
func normalize(s Strategy, segs []Segment) []Segment {
	if len(segs) == 0 {
		return nil
	}
	ivs := make([]interval.Interval, len(segs))
	for i, seg := range segs {
		ivs[i] = seg.Interval
	}
	covers := interval.Sweep(ivs, func(i, j int) bool {
		return better(s, segs[i].Winner, segs[j].Winner)
	})
	out := make([]Segment, 0, len(covers))
	for _, c := range covers {
		out = append(out, Segment{Interval: c.Interval, Winner: segs[c.Index].Winner})
	}
	return out
}

func better(s Strategy, a, b Winner) bool {
	if s == FWW {
		if a.TS != b.TS {
			return a.TS < b.TS
		}
		return a.ID < b.ID
	}
	if a.TS != b.TS {
		return a.TS > b.TS
	}
	return a.ID > b.ID
}
