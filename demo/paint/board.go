// Command paint serves an infinite shared whiteboard: a pixel grid whose cells
// are folded into the library's n-dimensional range CRDT. Every share link is
// one session; browsers that open the same link receive the operation log and
// materialize the view themselves, so concurrent strokes converge no matter
// their arrival order. A filled rectangle of cells is one half-open box
// operation, demonstrating the region compression the library provides.
package main

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/d-led/eventfulranges/demo/internal/collab"
)

// board is one session's shared whiteboard, as the collab transport sees it:
// it turns client commands into event-log entries by delegating to a Canvas.
// Browsers receive the log and materialize the view themselves.
type board struct {
	mu     sync.Mutex
	canvas Canvas
	log    []collab.Entry
}

// newBoard opens a board backed by the current Canvas implementation.
func newBoard() *board {
	return &board{canvas: newBoxCanvas()}
}

// Apply folds one paint or erase command into the canvas and returns the
// events it produced, attributed to the client.
func (b *board) Apply(clientID string, cmd collab.Cmd) ([]collab.Entry, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	var events []Event
	var err error
	if cmd.Kind == cmdRetract {
		events, err = b.canvas.Retract(cmd.ID, cmd.Ref)
	} else {
		var rect Rect
		if err := json.Unmarshal(cmd.Data, &rect); err != nil {
			return nil, fmt.Errorf("paint: bad command payload: %w", err)
		}
		events, err = b.fold(cmd.Kind, cmd.ID, rect, cmd.Meta)
	}
	if err != nil {
		return nil, err
	}
	entries := make([]collab.Entry, 0, len(events))
	for _, ev := range events {
		entry := collab.Entry{
			ID:     ev.ID,
			Client: clientID,
			Kind:   ev.Kind,
			Ref:    ev.Ref,
			Data:   ev.Data,
			Meta:   cmd.Meta,
			Detail: ev.Detail,
			At:     time.Now(),
		}
		entries = append(entries, entry)
		b.log = append(b.log, entry)
	}
	return entries, nil
}

// Replay folds a persisted log back into a fresh board, re-materializing the
// canvas while preserving each entry's attribution and timestamp.
func (b *board) Replay(entries []collab.Entry) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.log = b.log[:0]
	for _, e := range entries {
		events, err := b.replayEntry(e)
		if err != nil {
			return err
		}
		for _, ev := range events {
			b.log = append(b.log, collab.Entry{
				ID:     ev.ID,
				Client: e.Client,
				Kind:   ev.Kind,
				Ref:    ev.Ref,
				Data:   ev.Data,
				Meta:   e.Meta,
				Detail: ev.Detail,
				At:     e.At,
			})
		}
	}
	return nil
}

// replayEntry folds one persisted entry back into the canvas, preserving the
// entry's operation ID so later retractions still resolve.
func (b *board) replayEntry(e collab.Entry) ([]Event, error) {
	if e.Kind == "retract" {
		return b.canvas.Retract(e.ID, e.Ref)
	}
	rect, err := replayRect(e.Data)
	if err != nil {
		return nil, err
	}
	kind, err := commandKind(e.Kind)
	if err != nil {
		return nil, err
	}
	return b.fold(kind, e.ID, rect, e.Meta)
}

// replayRect decodes the box shape the event log stores — {"min":[…],"max":[…]}
// — into a board rectangle. The wire command uses {"x0","y0","x1","y1"}, but
// the log keeps the materialized box, so replay must read that shape.
func replayRect(data json.RawMessage) (Rect, error) {
	var box struct {
		Min []float64 `json:"min"`
		Max []float64 `json:"max"`
	}
	if err := json.Unmarshal(data, &box); err != nil {
		return Rect{}, fmt.Errorf("paint: replay: %w", err)
	}
	if len(box.Min) != 2 || len(box.Max) != 2 {
		return Rect{}, fmt.Errorf("paint: replay: box must have two dimensions")
	}
	return Rect{X0: box.Min[0], Y0: box.Min[1], X1: box.Max[0], Y1: box.Max[1]}, nil
}

// commandKind maps an event-log kind back to the command it came from.
func commandKind(kind string) (string, error) {
	switch kind {
	case "add":
		return cmdPaint, nil
	case "remove":
		return cmdErase, nil
	default:
		return "", fmt.Errorf("paint: replay: unknown entry kind %q", kind)
	}
}

// fold dispatches a command to the canvas.
func (b *board) fold(kind, id string, rect Rect, meta json.RawMessage) ([]Event, error) {
	switch kind {
	case cmdPaint:
		return b.canvas.Paint(id, rect, meta)
	case cmdErase:
		return b.canvas.Erase(id, rect, meta)
	default:
		return nil, fmt.Errorf("paint: unknown command %q", kind)
	}
}

// Log returns the session's operation log in append order.
func (b *board) Log() []collab.Entry {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]collab.Entry(nil), b.log...)
}

// Snapshot returns nil: the whiteboard exposes only its log, and browsers
// materialize the view themselves.
func (b *board) Snapshot() any {
	return nil
}
