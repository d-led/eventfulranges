package interval

import (
	"container/heap"
	"sort"
)

// Cover is one maximal piece of a swept union: the interval and the index of
// the best interval covering it.
type Cover struct {
	Interval Interval
	Index    int
}

// Sweep partitions the union of ivs into maximal pieces. Each piece is
// labeled with the index of the best interval covering it, where better(i, j)
// reports whether ivs[i] outranks ivs[j] on the points both cover. Points no
// interval covers are gaps and are omitted. The result is canonical: pieces
// are disjoint, ordered, and each is the whole run of one winner.
func Sweep(ivs []Interval, better func(i, j int) bool) []Cover {
	events := sweepEvents(ivs)
	h := &coverHeap{better: better, gone: make([]bool, len(ivs))}
	var runs []fineRun
	var prev finePoint
	started := false
	for i := 0; i < len(events); {
		p := events[i].p
		if started {
			if idx, ok := h.best(); ok {
				appendRun(&runs, prev, p, idx)
			}
		}
		i = applyEvents(h, events, i, p)
		prev = p
		started = true
	}
	return coverList(runs)
}

// sweepEvents builds the start and end events of every interval and sorts them
// by fine point, breaking ties by index for determinism.
func sweepEvents(ivs []Interval) []sweepEvent {
	events := make([]sweepEvent, 0, 2*len(ivs))
	for i, iv := range ivs {
		lo, hi := fine(iv)
		events = append(events,
			sweepEvent{p: lo, idx: i, start: true},
			sweepEvent{p: hi, idx: i},
		)
	}
	sort.Slice(events, func(a, b int) bool { return lessEvent(events[a], events[b]) })
	return events
}

func lessEvent(a, b sweepEvent) bool {
	if fineLess(a.p, b.p) {
		return true
	}
	if fineLess(b.p, a.p) {
		return false
	}
	return a.idx < b.idx
}

// applyEvents processes every event at point p, updating the active set, and
// returns the index of the next event to sweep.
func applyEvents(h *coverHeap, events []sweepEvent, i int, p finePoint) int {
	for i < len(events) && events[i].p == p {
		if events[i].start {
			heap.Push(h, events[i].idx)
		} else {
			h.gone[events[i].idx] = true
		}
		i++
	}
	return i
}

// finePoint is a point on the doubled line Sweep sweeps: every real coordinate
// x appears twice, as the point x itself (past false) and as the point just
// after x (past true). Closed endpoints include the point, open endpoints do
// not, so a sweep over these points needs no open/closed special cases.
type finePoint struct {
	x    float64
	past bool
}

// fineLess orders fine points: by coordinate, then the point itself before the
// point just after it.
func fineLess(a, b finePoint) bool {
	if a.x != b.x {
		return a.x < b.x
	}
	return !a.past && b.past
}

// fine returns the half-open span of fine points iv covers: a fine point p
// satisfies lo <= p < hi exactly when the real point p.x lies in iv.
func fine(iv Interval) (lo, hi finePoint) {
	lo = finePoint{x: iv.Start, past: iv.StartBound == Open}
	hi = finePoint{x: iv.End, past: iv.EndBound == Closed}
	return lo, hi
}

type fineRun struct {
	lo, hi finePoint
	idx    int
}

func appendRun(runs *[]fineRun, lo, hi finePoint, idx int) {
	if n := len(*runs); n > 0 && (*runs)[n-1].idx == idx && (*runs)[n-1].hi == lo {
		(*runs)[n-1].hi = hi
		return
	}
	*runs = append(*runs, fineRun{lo: lo, hi: hi, idx: idx})
}

func coverList(runs []fineRun) []Cover {
	out := make([]Cover, len(runs))
	for i, r := range runs {
		out[i] = Cover{Interval: fromFine(r.lo, r.hi), Index: r.idx}
	}
	return out
}

func fromFine(lo, hi finePoint) Interval {
	sb, eb := Open, Open
	if !lo.past {
		sb = Closed
	}
	if hi.past {
		eb = Closed
	}
	return Interval{Start: lo.x, End: hi.x, StartBound: sb, EndBound: eb}
}

type sweepEvent struct {
	p     finePoint
	idx   int
	start bool
}

type coverHeap struct {
	better func(i, j int) bool
	gone   []bool
	items  []int
}

func (h *coverHeap) Len() int { return len(h.items) }

func (h *coverHeap) Less(i, j int) bool {
	a, b := h.items[i], h.items[j]
	if h.better(a, b) {
		return true
	}
	if h.better(b, a) {
		return false
	}
	return a < b
}

func (h *coverHeap) Swap(i, j int) { h.items[i], h.items[j] = h.items[j], h.items[i] }

func (h *coverHeap) Push(x any) { h.items = append(h.items, x.(int)) }

func (h *coverHeap) Pop() any {
	old := h.items
	n := len(old)
	x := old[n-1]
	h.items = old[:n-1]
	return x
}

func (h *coverHeap) best() (int, bool) {
	for h.Len() > 0 {
		idx := h.items[0]
		if !h.gone[idx] {
			return idx, true
		}
		heap.Pop(h)
	}
	return 0, false
}
