// The wire protocol shared by the native server and the in-browser engine.
// A clientOp is one add/remove/dims command the page submits; a serverMsg is
// one envelope the engine sends back. Both builds speak exactly the same
// JSON, so the UI is identical whether the hub lives in the Go server (over a
// WebSocket) or compiled to WebAssembly in the page.
package main

import "github.com/d-led/eventfulranges/space"

// clientOp is one add/remove/dims/search command the browser submits over the
// socket (native) or to the wasm bridge (browser-only).
type clientOp struct {
	Kind string    `json:"kind"`
	Min  []float64 `json:"min"`
	Max  []float64 `json:"max"`
	Dims int       `json:"dims"`
}

// searchKind names the read-only client command that asks the board's spatial
// index which boxes overlap a region, without changing the view.
const searchKind = "search"

// searchResult is the answer to a search: the query region echoed back and
// the boxes that overlap it.
type searchResult struct {
	Min   []float64   `json:"min"`
	Max   []float64   `json:"max"`
	Boxes []space.Box `json:"boxes"`
}

// serverMsg is the envelope an engine sends back: state materializations,
// activity log entries, presence counts, search results, and fold errors.
type serverMsg struct {
	Type     string        `json:"type"` // one of msgState, msgOp, msgSearch, msgPresence, msgError
	State    *view         `json:"state,omitempty"`
	Op       *opRecord     `json:"op,omitempty"`
	Ops      []opRecord    `json:"ops,omitempty"`     // the full log, sent on join
	Search   *searchResult `json:"search,omitempty"`  // a region query's answer
	Clients  int           `json:"clients,omitempty"` // clients watching this session
	Total    int           `json:"total,omitempty"`   // clients connected across all sessions
	ClientID string        `json:"clientID,omitempty"`
	Error    string        `json:"error,omitempty"`
}

// serverMsg.Type values, named so writers and readers agree on the envelope
// kinds instead of repeating string literals.
const (
	msgState    = "state"
	msgOp       = "op"
	msgSearch   = "search"
	msgPresence = "presence"
	msgError    = "error"
)
