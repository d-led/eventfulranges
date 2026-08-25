package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/cskr/pubsub/v2"

	"github.com/d-led/eventfulranges/space"
)

// topic is the pub/sub channel inside one session's hub. Every session owns
// its own hub, so this constant just names the channel in each isolated model.
const topic = "events"

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

// opRecord is one entry in the session's shared activity log.
type opRecord struct {
	Client string    `json:"client"`
	Kind   string    `json:"kind"`
	Min    []float64 `json:"min,omitempty"`
	Max    []float64 `json:"max,omitempty"`
	Dims   int       `json:"dims,omitempty"`
	At     time.Time `json:"at"`
}

// hub is one session's shared model: it owns the view and the activity log and
// broadcasts every change through its own pub/sub topic, isolated from every
// other session. It also counts the clients currently watching the session.
type hub struct {
	mu      sync.Mutex
	events  *pubsub.PubSub[string, serverMsg]
	adds    []space.Box
	removes []space.Box
	dims    int // session dimension: -1 until fixed by a dims op or the first box
	clients int
	ops     []opRecord
}

func newHub() *hub {
	return &hub{
		events: pubsub.New[string, serverMsg](1024),
		dims:   -1,
	}
}

type opKind string

const (
	opAdd    opKind = "add"
	opRemove opKind = "remove"
	opDims   opKind = "dims"
)

// maxDims is the highest dimension the visualizer can draw: a 3D canvas plus
// one sliceable w axis. The library supports arbitrary n; the demo stops at 4
// so every box can be rendered.
const maxDims = 4

// apply folds one anonymous operation into the shared view and broadcasts the
// new state. Tests use it to exercise the model without a client identity.
func (h *hub) apply(kind opKind, min, max []float64) (view, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	v, err := h.foldLocked(kind, min, max)
	if err != nil {
		return view{}, err
	}
	h.events.Pub(serverMsg{Type: msgState, State: &v}, topic)
	return v, nil
}

// record applies one operation attributed to a client, appends it to the
// activity log, and broadcasts the log entry together with the new state.
func (h *hub) record(clientID string, kind opKind, min, max []float64) (view, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	v, err := h.foldLocked(kind, min, max)
	if err != nil {
		return view{}, err
	}
	rec := opRecord{Client: clientID, Kind: string(kind), Min: min, Max: max, At: time.Now()}
	h.ops = append(h.ops, rec)
	h.events.Pub(serverMsg{Type: msgOp, Op: &rec, State: &v}, topic)
	return v, nil
}

// join registers a watcher and reports the new count together with the
// session's activity log so a late joiner can catch up.
func (h *hub) join() (log []opRecord, clients int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients++
	log = append([]opRecord(nil), h.ops...)
	h.events.Pub(serverMsg{Type: msgPresence, Clients: h.clients}, topic)
	return log, h.clients
}

// leave unregisters a watcher.
func (h *hub) leave() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients > 0 {
		h.clients--
	}
	h.events.Pub(serverMsg{Type: msgPresence, Clients: h.clients}, topic)
}

// snapshot returns the current view without broadcasting it.
func (h *hub) snapshot() view {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.materializeLocked()
}

// subscribe returns a channel receiving every broadcast event.
func (h *hub) subscribe() chan serverMsg {
	return h.events.Sub(topic)
}

func (h *hub) unsubscribe(ch chan serverMsg) {
	h.events.Unsub(ch, topic)
}

// foldLocked validates and folds one operation, returning the new view. The
// caller must hold h.mu.
func (h *hub) foldLocked(kind opKind, min, max []float64) (view, error) {
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
	return h.materializeLocked(), nil
}

// foldDimsLocked fixes the session's dimension. The dimension is session
// metadata, not box content: it persists across reload, and once the view
// holds boxes it can no longer change (start a new session to switch). The
// caller must hold h.mu.
func (h *hub) foldDimsLocked(dims int) (view, error) {
	if dims < 1 || dims > maxDims {
		return view{}, fmt.Errorf("dimension %d out of range [1,%d]", dims, maxDims)
	}
	if (len(h.adds) > 0 || len(h.removes) > 0) && h.dims != dims {
		return view{}, fmt.Errorf("view already has %d dimensions", h.dims)
	}
	h.dims = dims
	return h.materializeLocked(), nil
}

// applyDims fixes the session dimension without a client identity; tests use it.
func (h *hub) applyDims(dims int) (view, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	v, err := h.foldDimsLocked(dims)
	if err != nil {
		return view{}, err
	}
	h.events.Pub(serverMsg{Type: msgState, State: &v}, topic)
	return v, nil
}

// setDims fixes the session dimension for a client, appends the change to the
// activity log, and broadcasts the log entry together with the new state.
func (h *hub) setDims(clientID string, dims int) (view, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	v, err := h.foldDimsLocked(dims)
	if err != nil {
		return view{}, err
	}
	rec := opRecord{Client: clientID, Kind: string(opDims), Dims: dims, At: time.Now()}
	h.ops = append(h.ops, rec)
	h.events.Pub(serverMsg{Type: msgOp, Op: &rec, State: &v}, topic)
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
