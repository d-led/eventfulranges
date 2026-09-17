package space

import (
	"math"
	"sort"

	"github.com/d-led/eventfulranges/meta"
)

// merger runs the greedy fixpoint of MergeAdjacent without ever scanning every
// pair of boxes.
//
// The rule it has to reproduce is: find the first mergeable pair in canonical
// cover order, merge it, repeat. Two boxes are candidates only if they agree on
// every axis but one — on that axis they touch, on all the others their spans
// and face inclusivity are identical. So the agreement is a key, and the boxes
// sharing a key are exactly the possible partners: partner lookup becomes a hash
// lookup over a handful of boxes instead of a walk over the whole cover.
//
// Boxes are addressed by a stable id so the candidate index survives edits; the
// live ones are kept in canonical cover order, which is the order the greedy's
// "first pair" is defined in.
type merger struct {
	boxes   []Box // by id; a merged-away box is zeroed and no longer referenced
	live    []int // ids in canonical cover order
	rank    []int // id -> position in live, or -1 once merged away
	buckets map[uint64][]int
	dims    int
}

// newMerger indexes a cover that is already normalized.
func newMerger(boxes []Box) *merger {
	m := &merger{
		boxes:   boxes,
		live:    make([]int, len(boxes)),
		rank:    make([]int, len(boxes)),
		buckets: make(map[uint64][]int, 2*len(boxes[0].Min)*len(boxes)),
		dims:    len(boxes[0].Min),
	}
	for id := range boxes {
		m.live[id] = id
		m.rank[id] = id
		m.index(id)
	}
	return m
}

// firstMergeablePair returns the ids of the pair the greedy would merge next:
// the earliest box that can merge with anything, together with its earliest
// possible partner. "Earliest" is canonical cover order, the order the plain
// pair scan works in.
func (m *merger) firstMergeablePair() (int, int, bool) {
	for i, id := range m.live {
		if j, ok := m.earliestPartner(i, id); ok {
			return id, m.live[j], true
		}
	}
	return 0, 0, false
}

// earliestPartner finds the earliest box after position i that merges with id.
func (m *merger) earliestPartner(i, id int) (int, bool) {
	best := -1
	for d := 0; d < m.dims; d++ {
		for _, other := range m.buckets[crossSectionKey(m.boxes[id], d)] {
			j := m.rank[other]
			if j <= i || (best >= 0 && j >= best) {
				continue // not after i, or not earlier than the best so far
			}
			if _, ok := mergePair(m.boxes[id], m.boxes[other]); ok {
				best = j
			}
		}
	}
	return best, best >= 0
}

// merge replaces two boxes with their union, folded into the cover the way
// Normalize would fold it.
func (m *merger) merge(a, b int) {
	merged, _ := mergePair(m.boxes[a], m.boxes[b])
	m.remove(a)
	m.remove(b)
	m.insert(merged)
}

// insert places the union in canonical order, absorbing every box it covers.
//
// The union always survives: a box covering it would cover both of its parts,
// and Normalize would have dropped those two before the merge ever happened.
// So only the boxes the union absorbs have to be folded into it, in the order
// Normalize folds them — higher positions first.
func (m *merger) insert(merged Box) {
	for k := len(m.live) - 1; k >= 0; k-- {
		id := m.live[k]
		if !subsumes(merged, m.boxes[id]) {
			continue
		}
		merged.Meta = meta.Union(merged.Meta, m.boxes[id].Meta)
		m.removeAt(k)
	}
	m.insertInOrder(merged)
}

// insertInOrder inserts a box at its canonical cover position and indexes it.
func (m *merger) insertInOrder(b Box) {
	pos := sort.Search(len(m.live), func(k int) bool {
		return !Less(m.boxes[m.live[k]], b)
	})
	id := len(m.boxes)
	m.boxes = append(m.boxes, b)
	m.rank = append(m.rank, pos)
	m.live = append(m.live, 0)
	copy(m.live[pos+1:], m.live[pos:])
	m.live[pos] = id
	for k := pos; k < len(m.live); k++ {
		m.rank[m.live[k]] = k
	}
	m.index(id)
}

// remove drops one box by id.
func (m *merger) remove(id int) {
	m.removeAt(m.rank[id])
}

// removeAt drops the box at one position, unindexing it and closing the gap in
// the live order.
func (m *merger) removeAt(pos int) {
	id := m.live[pos]
	copy(m.live[pos:], m.live[pos+1:])
	m.live = m.live[:len(m.live)-1]
	for k := pos; k < len(m.live); k++ {
		m.rank[m.live[k]] = k
	}
	m.rank[id] = -1
	m.unindex(id)
	m.boxes[id] = Box{}
}

// result returns the surviving boxes in canonical cover order.
func (m *merger) result() []Box {
	out := make([]Box, 0, len(m.live))
	for _, id := range m.live {
		out = append(out, m.boxes[id])
	}
	return out
}

// index records a box as a candidate under every axis it could merge along.
func (m *merger) index(id int) {
	for d := 0; d < m.dims; d++ {
		k := crossSectionKey(m.boxes[id], d)
		m.buckets[k] = append(m.buckets[k], id)
	}
}

// unindex drops a box from the candidate buckets of every axis.
func (m *merger) unindex(id int) {
	for d := 0; d < m.dims; d++ {
		k := crossSectionKey(m.boxes[id], d)
		ids := m.buckets[k]
		for i, other := range ids {
			if other == id {
				m.buckets[k] = append(ids[:i], ids[i+1:]...)
				break
			}
		}
	}
}

// crossSectionKey hashes everything about a box except its span on one axis:
// the corners and face inclusivity of every other axis. Equal keys are the
// necessary condition for a candidate pair, so a pair that can merge always
// shares a bucket; mergePair remains the judge of the pairs that do not.
//
// The axis itself is not part of the key. Two boxes that differ on one axis
// hash the remaining axes, so a key taken on one axis and a key taken on
// another can only collide by coincidence — and a collision costs a rejected
// candidate check, never a missed merge.
//
// Zeros are folded to positive zero before their bits are mixed in, because -0
// and 0 compare and merge equal but do not share a bit pattern.
func crossSectionKey(b Box, axis int) uint64 {
	const (
		offsetBasis = 14695981039346656037 // FNV-1a
		prime       = 1099511628211
	)
	h := uint64(offsetBasis)
	mix := func(v uint64) { h = (h ^ v) * prime }
	for d := range b.Min {
		if d == axis {
			continue
		}
		mix(canonicalBits(b.Min[d]))
		mix(canonicalBits(b.Max[d]))
		mix(uint64(b.loBound(d)) | uint64(b.hiBound(d))<<8)
	}
	return h
}

// canonicalBits returns the bit pattern of x with every zero normalized to
// positive zero.
func canonicalBits(x float64) uint64 {
	if x == 0 {
		return 0
	}
	return math.Float64bits(x)
}
