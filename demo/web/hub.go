package main

import (
	"fmt"
	"sync"

	"github.com/cskr/pubsub/v2"

	"gitub.com/d-led/eventfulranges/space"
)

// topic fans materialized-view updates out to every connected browser.
const topic = "view"

// view is the materialized result of the shared, append-only operation log.
// It is the n-dimensional generalization of the library's AdditiveWins
// strategy: the union of every addition minus the union of every removal, so
// the result is independent of the order in which concurrent edits arrive.
type view struct {
	Boxes   []space.Box `json:"boxes"`
	Adds    int         `json:"adds"`
	Removes int         `json:"removes"`
	Dims    int         `json:"dims"`
}

// hub owns the shared view and broadcasts every change through pubsub.
type hub struct {
	mu      sync.Mutex
	events  *pubsub.PubSub[string, view]
	adds    []space.Box
	removes []space.Box
	dims    int // -1 until the first operation fixes the dimension
}

func newHub() *hub {
	return &hub{
		events: pubsub.New[string, view](1024),
		dims:   -1,
	}
}

type opKind string

const (
	opAdd    opKind = "add"
	opRemove opKind = "remove"
	opClear  opKind = "clear"
)

// apply folds one operation into the shared view and broadcasts the result.
func (h *hub) apply(kind opKind, min, max []float64) (view, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if kind == opClear {
		h.adds = nil
		h.removes = nil
		h.dims = -1
		return h.publishLocked()
	}

	b := space.NewBox(min, max)
	if err := b.Validate(); err != nil {
		return view{}, err
	}
	if h.dims == -1 {
		h.dims = b.Dims()
	} else if b.Dims() != h.dims {
		return view{}, fmt.Errorf("box has %d dimensions but the view has %d", b.Dims(), h.dims)
	}

	switch kind {
	case opAdd:
		h.adds = append(h.adds, b)
	case opRemove:
		h.removes = append(h.removes, b)
	default:
		return view{}, fmt.Errorf("unknown operation %q", kind)
	}
	return h.publishLocked()
}

// snapshot returns the current view without broadcasting it.
func (h *hub) snapshot() view {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.materializeLocked()
}

// subscribe returns a channel receiving every broadcast view.
func (h *hub) subscribe() chan view {
	return h.events.Sub(topic)
}

func (h *hub) unsubscribe(ch chan view) {
	h.events.Unsub(ch, topic)
}

func (h *hub) publishLocked() (view, error) {
	v := h.materializeLocked()
	h.events.Pub(v, topic)
	return v, nil
}

func (h *hub) materializeLocked() view {
	adds := space.Union(h.adds, nil)
	removes := space.Union(h.removes, nil)
	return view{
		Boxes:   space.Difference(adds, removes),
		Adds:    len(h.adds),
		Removes: len(h.removes),
		Dims:    h.dims,
	}
}
