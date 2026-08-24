package strategy_test

import (
	"sort"
	"testing"

	biogo "github.com/biogo/store/interval"
	"github.com/stretchr/testify/require"

	"gitub.com/d-led/eventfulranges/interval"
	"gitub.com/d-led/eventfulranges/op"
	"gitub.com/d-led/eventfulranges/strategy"
)

// This file is the sequential reference model (oracle) for the property-based
// tests. It keeps its working set in a biogo interval tree, so ordering,
// overlap lookup, storage and traversal all come from an independent, widely
// used library. The boolean geometry (merge/subtract) is the shared semantic
// kernel from the interval package.

type floatKey float64

func (k floatKey) Compare(c biogo.Comparable) int {
	o := c.(floatKey)
	switch {
	case k < o:
		return -1
	case k > o:
		return 1
	default:
		return 0
	}
}

type oracleEntry struct {
	iv interval.Interval
	id uintptr
}

func (e oracleEntry) Overlap(r biogo.Range) bool {
	switch rv := r.(type) {
	case oracleEntry:
		return e.iv.Overlaps(rv.iv)
	case *oracleMutable:
		return e.iv.Overlaps(rv.iv)
	default:
		panic("oracle: unknown range type")
	}
}

func (e oracleEntry) Start() biogo.Comparable { return floatKey(e.iv.Start) }
func (e oracleEntry) End() biogo.Comparable   { return floatKey(e.iv.End) }
func (e oracleEntry) ID() uintptr             { return e.id }
func (e oracleEntry) NewMutable() biogo.Mutable {
	return &oracleMutable{iv: e.iv}
}

type oracleMutable struct {
	iv interval.Interval
}

func (m *oracleMutable) Start() biogo.Comparable { return floatKey(m.iv.Start) }
func (m *oracleMutable) End() biogo.Comparable   { return floatKey(m.iv.End) }
func (m *oracleMutable) SetStart(c biogo.Comparable) {
	m.iv.Start = float64(c.(floatKey))
}
func (m *oracleMutable) SetEnd(c biogo.Comparable) {
	m.iv.End = float64(c.(floatKey))
}

// oracleSet is a sequential, disjoint set of intervals backed by a biogo tree.
type oracleSet struct {
	tree *biogo.Tree
	next uintptr
}

func newOracleSet() *oracleSet {
	return &oracleSet{tree: &biogo.Tree{}}
}

func (s *oracleSet) insert(iv interval.Interval) {
	s.next++
	_ = s.tree.Insert(oracleEntry{iv: iv, id: s.next}, false)
}

func (s *oracleSet) delete(e oracleEntry) {
	_ = s.tree.Delete(e, false)
}

func (s *oracleSet) get(iv interval.Interval) []oracleEntry {
	matches := s.tree.Get(oracleEntry{iv: iv})
	out := make([]oracleEntry, 0, len(matches))
	for _, m := range matches {
		out = append(out, m.(oracleEntry))
	}
	return out
}

// addUnion inserts iv merged with every overlapping entry.
func (s *oracleSet) addUnion(iv interval.Interval) {
	for _, e := range s.get(iv) {
		s.delete(e)
		iv = interval.Merge(iv, e.iv)
	}
	s.insert(iv)
}

// remove deletes the parts of iv that are currently covered.
func (s *oracleSet) remove(iv interval.Interval) {
	for _, e := range s.get(iv) {
		s.delete(e)
		for _, piece := range e.iv.Subtract(iv) {
			s.insert(piece)
		}
	}
}

// ranges returns the entries in tree order.
func (s *oracleSet) ranges() []interval.Interval {
	out := make([]interval.Interval, 0, s.tree.Len())
	s.tree.Do(func(e biogo.Interface) (done bool) {
		out = append(out, e.(oracleEntry).iv)
		return
	})
	return out
}

// oracle replays operations sequentially onto tree-backed sets.
type oracle struct {
	strategy strategy.Strategy
	decided  *oracleSet
	result   *oracleSet
}

func newOracle(s strategy.Strategy) *oracle {
	return &oracle{strategy: s, decided: newOracleSet(), result: newOracleSet()}
}

func (o *oracle) applyAll(ops []op.Op) {
	switch o.strategy {
	case strategy.LWW:
		for _, p := range priorityOrder(ops, false) {
			o.applyPriority(p)
		}
	case strategy.FWW:
		for _, p := range priorityOrder(ops, true) {
			o.applyPriority(p)
		}
	case strategy.AdditiveWins:
		for _, p := range ops {
			if p.Kind == op.KindAdd {
				o.result.addUnion(p.Interval)
			}
		}
		for _, p := range ops {
			if p.Kind == op.KindRemove {
				o.result.remove(p.Interval)
			}
		}
	case strategy.GrowOnly:
		for _, p := range ops {
			o.applyGrow(p)
		}
	}
}

// applyPriority replays one operation in priority order: higher-priority
// operations are applied first and lower-priority ones fill the gaps.
func (o *oracle) applyPriority(p op.Op) {
	free := interval.Difference([]interval.Interval{p.Interval}, o.decided.ranges())
	o.decided.addUnion(p.Interval)
	if p.Kind == op.KindAdd {
		for _, piece := range free {
			o.result.addUnion(piece)
		}
	}
}

func (o *oracle) applyGrow(p op.Op) {
	if p.Kind == op.KindAdd {
		o.result.addUnion(p.Interval)
	}
}

func (o *oracle) ranges() []interval.Interval {
	return o.result.ranges()
}

// priorityOrder sorts the operations by (timestamp, id).
func priorityOrder(ops []op.Op, ascending bool) []op.Op {
	out := append([]op.Op(nil), ops...)
	sort.Slice(out, func(i, j int) bool {
		if ascending {
			return earlier(out[i], out[j])
		}
		return later(out[i], out[j])
	})
	return out
}

func TestOracleSanity(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		s    strategy.Strategy
		ops  []op.Op
		want []interval.Interval
	}{
		{
			name: "lww subtract cuts a hole",
			s:    strategy.LWW,
			ops: []op.Op{
				addOp("a", 1, 1, 5),
				addOp("b", 2, 3, 7),
				removeOp("c", 3, 2, 3),
			},
			want: []interval.Interval{
				iv(1, 2, interval.Closed, interval.Open),
				iv(3, 7, interval.Open, interval.Closed),
			},
		},
		{
			name: "fww earlier add beats later remove",
			s:    strategy.FWW,
			ops: []op.Op{
				addOp("a", 1, 1, 5),
				removeOp("b", 2, 2, 3),
			},
			want: []interval.Interval{iv(1, 5, interval.Closed, interval.Closed)},
		},
		{
			name: "additive union minus removals",
			s:    strategy.AdditiveWins,
			ops: []op.Op{
				addOp("a", 1, 1, 5),
				addOp("b", 2, 3, 7),
				removeOp("c", 3, 2, 3),
			},
			want: []interval.Interval{
				iv(1, 2, interval.Closed, interval.Open),
				iv(3, 7, interval.Open, interval.Closed),
			},
		},
		{
			name: "grow ignores removals",
			s:    strategy.GrowOnly,
			ops: []op.Op{
				addOp("a", 1, 1, 5),
				removeOp("b", 2, 2, 3),
			},
			want: []interval.Interval{iv(1, 5, interval.Closed, interval.Closed)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			o := newOracle(tt.s)
			o.applyAll(tt.ops)
			require.Equal(t, tt.want, interval.Normalize(o.ranges()))
		})
	}
}
