package main

import "encoding/json"

// Rect is a half-open cell rectangle [X0,X1) x [Y0,Y1) in board coordinates.
type Rect struct {
	X0 int64 `json:"x0"`
	Y0 int64 `json:"y0"`
	X1 int64 `json:"x1"`
	Y1 int64 `json:"y1"`
}

// Event is one entry a Canvas produces when it folds a stroke. Kind is the
// library operation kind ("add" or "remove"); Data is the payload a client
// materializes — an interval today, a box once the library grows an n-D
// engine.
type Event struct {
	ID     string
	Kind   string
	Data   json.RawMessage
	Detail string
}

// Canvas is the swappable CRDT backend of the whiteboard: it folds paint and
// erase strokes into an event-sourced range set and returns the events they
// produced. How cells are encoded is an implementation detail, which is what
// keeps the board and the transport agnostic of the underlying CRDT shape.
type Canvas interface {
	Paint(r Rect) ([]Event, error)
	Erase(r Rect) ([]Event, error)
}
