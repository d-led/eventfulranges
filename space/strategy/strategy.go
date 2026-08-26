// Package strategy materializes a set of box operations into a canonical
// cover of boxes under a chosen conflict-resolution policy. It is the
// n-dimensional counterpart of the root strategy package: the same policies,
// but over space.Box instead of a 1-D interval.
package strategy

import (
	"errors"
	"fmt"
	"sort"

	"github.com/d-led/eventfulranges/meta"
	"github.com/d-led/eventfulranges/space"
	"github.com/d-led/eventfulranges/space/op"
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
	// AdditiveWinsLWW unions all additions and subtracts all removals, but
	// resolves each point's metadata by last-write-wins among the additions
	// covering it, so the latest stroke paints over earlier ones.
	AdditiveWinsLWW
)

// ErrUnknownStrategy is returned when parsing an unknown strategy name.
var ErrUnknownStrategy = errors.New("unknown strategy")

var names = map[Strategy]string{
	LWW:             "lww",
	FWW:             "fww",
	AdditiveWins:    "additive-wins",
	GrowOnly:        "grow-only",
	AdditiveWinsLWW: "additive-wins-lww",
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

// Winner is the operation that decided a region of space.
type Winner struct {
	TS   int64   `json:"ts"`
	ID   string  `json:"id"`
	Kind op.Kind `json:"kind"`
}

// Segment is a region of space decided by a single winner.
type Segment struct {
	Box    space.Box `json:"box"`
	Winner Winner    `json:"winner"`
}

// Materialize projects the operations into a canonical cover of boxes.
func Materialize(s Strategy, ops []op.Op) []space.Box {
	return MaterializeMerged(s, ops, meta.Union)
}

// MaterializeMerged is Materialize with a custom metadata join for the
// union-based strategies (AdditiveWins and GrowOnly). LWW, FWW, and
// AdditiveWinsLWW carry the winning operation's metadata unchanged, so the
// join does not apply there.
func MaterializeMerged(s Strategy, ops []op.Op, merge meta.Merge) []space.Box {
	switch s {
	case LWW, FWW:
		return ToBoxes(Segments(s, ops))
	case AdditiveWins:
		return space.DifferenceMerged(boxesOf(ops, op.KindAdd, merge), boxesOf(ops, op.KindRemove, merge), merge)
	case AdditiveWinsLWW:
		// The shape is the additive union of additions, but each point's
		// metadata is decided by the latest addition covering it. Partition
		// the additions by LWW, then carve out the removals.
		adds := ToBoxes(Segments(LWW, opsOf(ops, op.KindAdd)))
		return space.DifferenceMerged(adds, boxesOf(ops, op.KindRemove, merge), merge)
	case GrowOnly:
		return boxesOf(ops, op.KindAdd, merge)
	}
	return nil
}

// Segments projects the operations into canonical winner-annotated regions.
// It is defined for LWW and FWW only and returns nil otherwise.
func Segments(s Strategy, ops []op.Op) []Segment {
	if s != LWW && s != FWW {
		return nil
	}
	segs := make([]Segment, 0, len(ops))
	for _, o := range ops {
		segs = append(segs, Segment{
			Box:    o.Box,
			Winner: Winner{TS: o.TS, ID: o.ID, Kind: o.Kind},
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

// ToBoxes extracts the regions decided by Add operations as a canonical cover.
func ToBoxes(segs []Segment) []space.Box {
	boxes := make([]space.Box, 0, len(segs))
	for _, seg := range segs {
		if seg.Winner.Kind == op.KindAdd {
			boxes = append(boxes, seg.Box)
		}
	}
	return space.Normalize(boxes)
}

func boxesOf(ops []op.Op, kind op.Kind, merge meta.Merge) []space.Box {
	boxes := make([]space.Box, 0, len(ops))
	for _, o := range ops {
		if o.Kind == kind {
			boxes = append(boxes, o.Box)
		}
	}
	return space.NormalizeMerged(boxes, merge)
}

// opsOf returns the operations of one kind, in input order.
func opsOf(ops []op.Op, kind op.Kind) []op.Op {
	out := make([]op.Op, 0, len(ops))
	for _, o := range ops {
		if o.Kind == kind {
			out = append(out, o)
		}
	}
	return out
}

// normalize reduces overlapping regions to disjoint winner-annotated pieces
// by applying operations in priority order: higher-priority operations decide
// first and lower-priority ones fill the remaining gaps. Every decided region
// is kept with its winning operation (Add or Remove).
func normalize(s Strategy, segs []Segment) []Segment {
	sort.Slice(segs, func(i, j int) bool { return better(s, segs[i].Winner, segs[j].Winner) })
	var claimed []space.Box
	out := make([]Segment, 0, len(segs))
	for _, seg := range segs {
		free := space.DifferenceSorted([]space.Box{seg.Box}, claimed)
		claimed = space.InsertNormalized(claimed, seg.Box)
		for _, piece := range free {
			out = append(out, Segment{Box: piece, Winner: seg.Winner})
		}
	}
	sort.Slice(out, func(i, j int) bool { return space.Less(out[i].Box, out[j].Box) })
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
