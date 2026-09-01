package main

import "encoding/json"

// Command kinds the board and canvas speak, mirrored by the browser UI. They
// name the half-open box operations a client submits.
const (
	cmdPaint   = "paint"
	cmdErase   = "erase"
	cmdRetract = "retract"
)

// Rect is a half-open cell rectangle [X0,X1) x [Y0,Y1) in board coordinates.
// Cells lie on a fractal grid: at subdivision level n a cell has side 2^-n,
// so strokes at finer zoom levels produce fractional bounds.
type Rect struct {
	X0 float64 `json:"x0"`
	Y0 float64 `json:"y0"`
	X1 float64 `json:"x1"`
	Y1 float64 `json:"y1"`
}

// Event is one entry a Canvas produces when it folds a stroke. Kind is the
// library operation kind ("add" or "remove"); Data is the half-open box the
// client materializes.
type Event struct {
	ID     string
	Kind   string
	Ref    string
	Data   json.RawMessage
	Detail string
}

// Layer is one rectangle of the materialized board in paint order: a stroke
// with its colour, or an erase (Color empty) that repaints the background.
type Layer struct {
	X0, Y0, X1, Y1 float64
	Color          string
}

// Canvas is the swappable CRDT backend of the whiteboard: it folds paint and
// erase strokes into an event-sourced range set and returns the events they
// produced. Meta is arbitrary JSON-object metadata (for example a stroke
// color) carried by the folded ranges. How cells are encoded is an
// implementation detail, which is what keeps the board and the transport
// agnostic of the underlying CRDT shape. id is the client-chosen operation
// identifier: when non-empty it is used as the operation's ID, so a client can
// retract its own edits by ID.
type Canvas interface {
	Paint(id string, r Rect, meta json.RawMessage) ([]Event, error)
	Erase(id string, r Rect, meta json.RawMessage) ([]Event, error)
	Retract(id, ref string) ([]Event, error)
	Layers() []Layer
}
