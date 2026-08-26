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
	var rect Rect
	if err := json.Unmarshal(cmd.Data, &rect); err != nil {
		return nil, fmt.Errorf("paint: bad command payload: %w", err)
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	events, err := b.fold(cmd.Kind, rect)
	if err != nil {
		return nil, err
	}
	entries := make([]collab.Entry, 0, len(events))
	for _, ev := range events {
		entry := collab.Entry{
			ID:     ev.ID,
			Client: clientID,
			Kind:   ev.Kind,
			Data:   ev.Data,
			Detail: ev.Detail,
			At:     time.Now(),
		}
		entries = append(entries, entry)
		b.log = append(b.log, entry)
	}
	return entries, nil
}

// fold dispatches a command to the canvas.
func (b *board) fold(kind string, rect Rect) ([]Event, error) {
	switch kind {
	case "paint":
		return b.canvas.Paint(rect)
	case "erase":
		return b.canvas.Erase(rect)
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
