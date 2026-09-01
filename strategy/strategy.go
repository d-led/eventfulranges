// Package strategy materializes a set of range operations into canonical
// non-overlapping intervals under a chosen conflict-resolution policy.
package strategy

import (
	"errors"
	"fmt"
	"sort"

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
// by applying operations in priority order: higher-priority operations decide
// first and lower-priority ones fill the remaining gaps. Every decided region
// is kept with its winning operation (Add or Remove).
func normalize(s Strategy, segs []Segment) []Segment {
	sort.Slice(segs, func(i, j int) bool { return better(s, segs[i].Winner, segs[j].Winner) })
	var claimed []interval.Interval
	out := make([]Segment, 0, len(segs))
	for _, seg := range segs {
		free := interval.Difference([]interval.Interval{seg.Interval}, claimed)
		claimed = interval.Union(claimed, []interval.Interval{seg.Interval})
		for _, piece := range free {
			out = append(out, Segment{Interval: piece, Winner: seg.Winner})
		}
	}
	sort.Slice(out, func(i, j int) bool { return interval.Less(out[i].Interval, out[j].Interval) })
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
