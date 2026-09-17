package space

import (
	"container/heap"
	"math"

	"github.com/d-led/eventfulranges/meta"
)

// merger runs the greedy fixpoint of MergeAdjacent without ever walking the
// cover.
//
// The rule it has to reproduce is: find the first mergeable pair in canonical
// cover order, merge it, repeat. Finding that pair by testing every pair, and
// re-normalizing the whole cover after each merge, costs a pass over the cover
// per merge — and a partition hands this function thousands of cells, so a pass
// per merge is the difference between milliseconds and minutes. Three indexes
// replace the passes:
//
//   - cross-section buckets answer "which boxes could this one merge with" in
//     constant time, because two boxes can only merge along one axis and must
//     therefore agree on every other axis;
//   - a heap of mergeable pairs answers "which pair is first" without a scan,
//     since pairs are ordered by the canonical order of their boxes;
//   - a coarse uniform grid answers "which boxes overlap this one", which is
//     all a union needs in order to find the boxes it swallows.
//
// A disjoint cover — what a partition produces, and the visualizer's case —
// takes the shortest path of all: merging two boxes that share only a face
// leaves the result disjoint from every other box, so no union can ever swallow
// one, and the grid pass after each merge is skipped entirely.
type merger struct {
	boxes    []Box
	live     []bool
	buckets  map[uint64][]int
	grid     *coverGrid
	pairs    pairHeap
	dims     int
	disjoint bool // no two live boxes share a point, so a union cannot swallow one
}

// pair is two boxes that can merge, in canonical cover order.
type pair struct{ first, second int }

// pairHeap is a min-heap of mergeable pairs ordered by canonical cover order:
// the pair whose first box — and then whose second box — comes first is the pair
// the plain pairwise scan would have picked.
type pairHeap struct {
	items []pair
	less  func(a, b pair) bool
}

func (h *pairHeap) Len() int           { return len(h.items) }
func (h *pairHeap) Less(i, j int) bool { return h.less(h.items[i], h.items[j]) }
func (h *pairHeap) Swap(i, j int)      { h.items[i], h.items[j] = h.items[j], h.items[i] }
func (h *pairHeap) Push(x any)         { h.items = append(h.items, x.(pair)) }

func (h *pairHeap) Pop() any {
	last := len(h.items) - 1
	x := h.items[last]
	h.items = h.items[:last]
	return x
}

// newMerger indexes a cover. It works on its own copy of the slice, because
// retiring a box rewrites it and a canonicalizer must not scribble on what it
// was handed. Empty boxes are left out of the cover rather than removed from
// the slice, so box ids stay stable.
func newMerger(boxes []Box) *merger {
	m := &merger{
		boxes:   append([]Box(nil), boxes...),
		live:    make([]bool, len(boxes)),
		buckets: make(map[uint64][]int, 2*len(boxes)*len(boxes[0].Min)),
		dims:    len(boxes[0].Min),
	}
	for id := range m.boxes {
		m.live[id] = !m.boxes[id].Empty()
		if m.live[id] {
			m.index(id)
		}
	}
	m.grid = newCoverGrid(m.boxes, m.dims)
	m.dedupe()
	m.disjoint = m.detectDisjoint()
	m.pairs.less = m.pairLess
	for id := range m.boxes {
		if m.live[id] {
			m.offer(id)
		}
	}
	return m
}

// dedupe drops every live box that another live box covers, folding the
// metadata of the dropped box into its subsumer. It is what Normalize does to a
// cover before a merge, done through the grid instead of by testing every pair:
// a box can only be covered by a box it overlaps, and the grid lists those.
func (m *merger) dedupe() {
	for id := range m.boxes {
		if !m.live[id] {
			continue
		}
		subsumer := m.subsumer(id)
		if subsumer < 0 {
			continue
		}
		m.boxes[subsumer].Meta = meta.Union(m.boxes[subsumer].Meta, m.boxes[id].Meta)
		m.kill(id)
	}
}

// subsumer returns a live box that covers the given one, or -1 when none does.
// The lowest id wins, so boxes with identical geometry collapse to one — the
// one with the lowest id — however they arrived.
func (m *merger) subsumer(id int) int {
	best := -1
	for _, c := range m.candidates(id) {
		if c == id || !m.live[c] || !subsumes(m.boxes[c], m.boxes[id]) {
			continue
		}
		if best < 0 || c < best {
			best = c
		}
	}
	return best
}

// candidates returns the live boxes that may share a point with the given one.
func (m *merger) candidates(id int) []int {
	ids, exhaustive := m.grid.overlapping(m.boxes[id])
	if exhaustive {
		return allIDs(len(m.boxes))
	}
	return ids
}

// detectDisjoint reports whether no two live boxes share a point. Merging
// preserves that, so it only has to be established once: the union of two boxes
// covers only points its halves already covered, so it can never cover a box
// that was disjoint from both of them.
func (m *merger) detectDisjoint() bool {
	for id := range m.boxes {
		if !m.live[id] {
			continue
		}
		for _, c := range m.candidates(id) {
			if c != id && m.live[c] && Overlaps(m.boxes[id], m.boxes[c]) {
				return false
			}
		}
	}
	return true
}

// run merges until no mergeable pair is left and returns the surviving cover.
func (m *merger) run() []Box {
	for {
		p, ok := m.nextPair()
		if !ok {
			return m.result()
		}
		m.merge(p)
	}
}

// nextPair pops the first pair that can still be merged, discarding the pairs
// whose boxes an earlier merge has already consumed.
func (m *merger) nextPair() (pair, bool) {
	for m.pairs.Len() > 0 {
		p := heap.Pop(&m.pairs).(pair)
		if !m.live[p.first] || !m.live[p.second] {
			continue
		}
		if _, ok := mergePair(m.boxes[p.first], m.boxes[p.second]); !ok {
			continue
		}
		return p, true
	}
	return pair{}, false
}

// merge replaces two boxes with their union, then offers the pairs the union
// itself forms.
func (m *merger) merge(p pair) {
	merged, _ := mergePair(m.boxes[p.first], m.boxes[p.second])
	m.kill(p.first)
	m.kill(p.second)
	union := m.add(merged)
	m.absorb(union)
	m.offer(union)
}

// offer pushes every pair the given box forms with a partner.
func (m *merger) offer(id int) {
	for _, other := range m.partners(id) {
		heap.Push(&m.pairs, m.orderedPair(id, other))
	}
}

// partners returns the ids of the live boxes that can merge with id. Two boxes
// can only merge along one axis, so a partner has to share a cross-section with
// id, and the buckets keyed by those cross-sections contain every candidate.
func (m *merger) partners(id int) []int {
	box := m.boxes[id]
	var out []int
	for d := 0; d < m.dims; d++ {
		for _, other := range m.buckets[crossSectionKey(box, d)] {
			if other == id || !m.live[other] {
				continue
			}
			if _, ok := mergePair(box, m.boxes[other]); ok {
				out = append(out, other)
			}
		}
	}
	return out
}

// orderedPair returns the pair with the canonically first box first.
func (m *merger) orderedPair(a, b int) pair {
	if Less(m.boxes[b], m.boxes[a]) {
		return pair{first: b, second: a}
	}
	return pair{first: a, second: b}
}

// pairLess orders two pairs the way the pairwise scan would meet them: by the
// first box, then by the second. Pairs whose boxes compare equal are
// interchangeable — that can only happen once a merge has retired one of them,
// and retired pairs are discarded without being merged.
func (m *merger) pairLess(a, b pair) bool {
	if c := m.compareBoxes(a.first, b.first); c != 0 {
		return c < 0
	}
	return m.compareBoxes(a.second, b.second) < 0
}

// compareBoxes orders two boxes by canonical cover order.
func (m *merger) compareBoxes(i, j int) int {
	switch {
	case Less(m.boxes[i], m.boxes[j]):
		return -1
	case Less(m.boxes[j], m.boxes[i]):
		return 1
	default:
		return 0
	}
}

// absorb folds away every live box the union covers. A disjoint cover has
// nothing to fold: the union consists only of points its two halves already
// covered, so it cannot cover a box that was disjoint from both of them.
//
// The union always survives, too. A box covering it would have covered both
// halves, and Normalize would have dropped those before the merge.
func (m *merger) absorb(union int) {
	if m.disjoint {
		return
	}
	for _, c := range m.candidates(union) {
		m.absorbOne(union, c)
	}
}

// absorbOne folds one candidate into the union when the union really covers it.
// A candidate can appear more than once, so this stays idempotent.
func (m *merger) absorbOne(union, candidate int) {
	if candidate == union || !m.live[candidate] || !subsumes(m.boxes[union], m.boxes[candidate]) {
		return
	}
	m.boxes[union].Meta = meta.Union(m.boxes[union].Meta, m.boxes[candidate].Meta)
	m.kill(candidate)
}

// add appends a box, indexes it, and returns its id.
func (m *merger) add(b Box) int {
	id := len(m.boxes)
	m.boxes = append(m.boxes, b)
	m.live = append(m.live, true)
	m.index(id)
	m.grid.add(id, b)
	return id
}

// kill retires a box: it leaves the indexes and can never be merged again. The
// pairs and bucket entries that mention it are dropped lazily, when they are
// next looked at — and its geometry stays, because the heap orders pairs by it
// and that order must not change under a pair's feet. Only the metadata, which
// no comparison reads, is released.
func (m *merger) kill(id int) {
	m.live[id] = false
	m.unindex(id)
	m.grid.remove(id)
	m.boxes[id].Meta = nil
}

// result returns the surviving boxes in canonical cover order.
func (m *merger) result() []Box {
	out := make([]Box, 0, len(m.boxes))
	for id := range m.boxes {
		if m.live[id] {
			out = append(out, m.boxes[id])
		}
	}
	sortBoxes(out)
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

// maxBinsPerAxis bounds how far a registered box may reach: four bins per axis
// covers a box up to four times the typical extent. Beyond that a box is
// cheaper to return on every query than to list in every bin it touches.
const maxBinsPerAxis = 4

// coverGrid is a coarse uniform grid over the cover, answering "which boxes
// overlap this one". Bins are sized to hold about one box each, and a box too
// wide to list is kept aside and returned by every query, so an answer is
// always a superset of the real overlaps however the cover is shaped.
type coverGrid struct {
	cell     []float64
	origin   []float64
	bins     map[uint64][]int
	binOf    [][]uint64 // box id -> the bins it was registered in
	big      []int      // boxes too wide to register
	bigAt    map[int]int
	disjoint bool
}

// newCoverGrid sizes a grid over the cover's bounding box, registers every box
// in it, and only then records whether the cover is disjoint — the answer is a
// property of the registered grid, and it is what lets the merger skip looking
// for boxes a union swallows.
func newCoverGrid(boxes []Box, dims int) *coverGrid {
	origin, extent := coverBounds(boxes, dims)
	g := &coverGrid{
		cell:   make([]float64, dims),
		origin: origin,
		bins:   make(map[uint64][]int, 4*len(boxes)),
		binOf:  make([][]uint64, len(boxes)),
		bigAt:  make(map[int]int),
	}
	side := binsPerAxis(len(boxes), dims)
	for d := range g.cell {
		g.cell[d] = extent[d] / float64(side)
		if g.cell[d] <= 0 {
			g.cell[d] = 1 // a cover flat in this axis still needs a bin width
		}
	}
	for id := range boxes {
		g.add(id, boxes[id])
	}
	g.disjoint = g.isDisjoint(boxes)
	return g
}

// binsPerAxis is the smallest bin count whose dims-dimensional grid holds one
// box per bin, capped so the grid stays coarse.
func binsPerAxis(n, dims int) int {
	side, capacity := 1, 1
	for capacity < n && side < 64 {
		side++
		capacity = 1
		for range dims {
			capacity *= side
		}
	}
	return side
}

// coverBounds returns the lower corner of the cover and its extent per axis.
func coverBounds(boxes []Box, dims int) (origin, extent []float64) {
	origin = make([]float64, dims)
	extent = make([]float64, dims)
	if len(boxes) == 0 {
		return origin, extent
	}
	upper := make([]float64, dims)
	copy(origin, boxes[0].Min)
	copy(upper, boxes[0].Max)
	for _, b := range boxes {
		for d := range origin {
			origin[d] = min(origin[d], b.Min[d])
			upper[d] = max(upper[d], b.Max[d])
		}
	}
	for d := range extent {
		extent[d] = upper[d] - origin[d]
	}
	return origin, extent
}

// isDisjoint reports whether no two boxes in the cover share a point. Merging
// preserves that — a union of two boxes only covers points its halves already
// covered — so it has to be established just once.
func (g *coverGrid) isDisjoint(boxes []Box) bool {
	for id := range boxes {
		candidates, exhaustive := g.overlapping(boxes[id])
		if exhaustive {
			candidates = allIDs(len(boxes))
		}
		for _, c := range candidates {
			if c != id && c < len(boxes) && Overlaps(boxes[id], boxes[c]) {
				return false
			}
		}
	}
	return true
}

// allIDs returns the ids 0..n-1.
func allIDs(n int) []int {
	ids := make([]int, n)
	for i := range ids {
		ids[i] = i
	}
	return ids
}

// binRange returns the inclusive bin index a box touches on each axis,
// reporting false when it touches too many bins to list.
func (g *coverGrid) binRange(b Box) (lo, hi []int, ok bool) {
	lo = make([]int, len(g.cell))
	hi = make([]int, len(g.cell))
	reached := 1
	budget := binsBudget(len(g.cell))
	for d := range g.cell {
		lo[d] = int(math.Floor((b.Min[d] - g.origin[d]) / g.cell[d]))
		hi[d] = int(math.Floor((b.Max[d] - g.origin[d]) / g.cell[d]))
		reached *= hi[d] - lo[d] + 1
		if reached > budget {
			return nil, nil, false
		}
	}
	return lo, hi, true
}

// binsBudget is how many bins one box may be listed in.
func binsBudget(dims int) int {
	budget := 1
	for range dims {
		budget *= maxBinsPerAxis
	}
	return budget
}

// add registers a box under every bin it touches, or keeps it aside when that
// would be too many bins.
func (g *coverGrid) add(id int, b Box) {
	g.grow(id)
	lo, hi, ok := g.binRange(b)
	if !ok {
		g.bigAt[id] = len(g.big)
		g.big = append(g.big, id)
		return
	}
	keys := make([]uint64, 0, binsBudget(len(g.cell)))
	g.eachBin(lo, hi, func(k uint64) {
		g.bins[k] = append(g.bins[k], id)
		keys = append(keys, k)
	})
	g.binOf[id] = keys
}

// remove unregisters a box from wherever it was registered.
func (g *coverGrid) remove(id int) {
	if at, ok := g.bigAt[id]; ok {
		g.big[at] = g.big[len(g.big)-1]
		g.big = g.big[:len(g.big)-1]
		delete(g.bigAt, id)
		if at < len(g.big) {
			g.bigAt[g.big[at]] = at
		}
		return
	}
	for _, k := range g.binOf[id] {
		ids := g.bins[k]
		for i, other := range ids {
			if other == id {
				g.bins[k] = append(ids[:i], ids[i+1:]...)
				break
			}
		}
	}
	g.binOf[id] = nil
}

// grow makes room for one more box id.
func (g *coverGrid) grow(id int) {
	for len(g.binOf) <= id {
		g.binOf = append(g.binOf, nil)
	}
}

// overlapping returns the boxes that may share a point with q, reporting true
// when q was too wide to look up and the caller has to consider the whole cover.
func (g *coverGrid) overlapping(q Box) ([]int, bool) {
	lo, hi, ok := g.binRange(q)
	if !ok {
		return nil, true
	}
	var out []int
	g.eachBin(lo, hi, func(k uint64) {
		out = append(out, g.bins[k]...)
	})
	return append(out, g.big...), false
}

// eachBin calls visit with the key of every bin in the inclusive range.
func (g *coverGrid) eachBin(lo, hi []int, visit func(uint64)) {
	at := make([]int, len(lo))
	copy(at, lo)
	for {
		visit(binKey(at))
		d := 0
		for ; d < len(at); d++ {
			at[d]++
			if at[d] <= hi[d] {
				break
			}
			at[d] = lo[d]
		}
		if d == len(at) {
			return
		}
	}
}

// binKey hashes a bin coordinate. Different coordinates always hash apart — two
// distinct bins can only collide by coincidence, which adds a candidate the
// caller rejects — and the coordinate is mixed in as the whole number it is,
// because bins are counted, never measured.
func binKey(at []int) uint64 {
	const (
		offsetBasis = 14695981039346656037 // FNV-1a
		prime       = 1099511628211
	)
	h := uint64(offsetBasis)
	mix := func(v uint64) { h = (h ^ v) * prime }
	for _, v := range at {
		mix(canonicalBits(float64(v)))
		mix(prime) // a separator, so [1,2] and [12] cannot collide
	}
	return h
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
