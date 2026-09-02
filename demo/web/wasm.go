//go:build js

// The wasm build of the visualizer turns the session hub into a same-page
// engine: open the local build (dist-local/) from any static host and the
// UI runs with no Go server at all. The bridge speaks the same JSON envelopes
// as the WebSocket server, so the page cannot tell the difference: join()
// returns the catch-up envelope a late joiner would receive, and op() folds
// one client command, pushing every resulting broadcast envelope to the page
// through the same dispatch the UI already handles.
//
// Persistence is the browser's job: reloads are healed from the localStorage
// reserve copy app.js keeps, exactly as a reconnecting socket would be healed.
package main

import (
	"encoding/json"
	"syscall/js"
)

// Engine state for one page. Browser-only mode has a single viewer per tab,
// so one hub is enough; a fresh session (new share link) or a reload replaces
// it with a new empty hub.
var (
	theSessionID string
	theCompact   bool
	theHub       *hub
	theClientID  string
	theDispatch  js.Value // page callback receiving each broadcast envelope
)

func main() {
	// One identity per page load, so the activity log can attribute this
	// tab's operations the way a fresh socket identity would.
	theClientID = newClientID()

	js.Global().Set("__eventfulranges_wasm", map[string]any{
		"join": js.FuncOf(wasmJoin),
		"op":   js.FuncOf(wasmOp),
	})
	select {}
}

// wasmJoin opens (or reuses) the hub for a session and returns the catch-up
// envelope as JSON: the full activity log, the materialized state, and this
// viewer's identity and presence. args: (sessionID, compact, dispatch).
func wasmJoin(this js.Value, args []js.Value) any {
	if len(args) < 3 {
		return errorEnvelope("join: expected (sessionID, compact, dispatch)")
	}
	sessionID := args[0].String()
	compact := args[1].String() == compactMerge
	theDispatch = args[2]
	if theHub == nil || sessionID != theSessionID || compact != theCompact {
		theSessionID = sessionID
		theCompact = compact
		theHub = newHub(compact)
		theHub.onEvent = func(m serverMsg) { emit(m) }
	}
	snap := theHub.snapshot()
	log := append([]opRecord(nil), theHub.ops...)
	return marshal(serverMsg{Type: msgState, State: &snap, ClientID: theClientID, Clients: 1, Total: 1, Ops: log})
}

// wasmOp folds one client command, exactly as readClientOps would over a
// socket. args[0] is the JSON clientOp. The resulting broadcast envelopes (the
// new log entry with the materialized state, or a fold error) are pushed to
// the page through the join-time dispatch.
func wasmOp(this js.Value, args []js.Value) any {
	if len(args) < 1 {
		emit(serverMsg{Type: msgError, Error: "op: expected a clientOp"})
		return nil
	}
	var op clientOp
	if err := json.Unmarshal([]byte(args[0].String()), &op); err != nil {
		emit(serverMsg{Type: msgError, Error: err.Error()})
		return nil
	}
	if op.Kind == string(opDims) {
		if _, err := theHub.setDims(theClientID, op.Dims); err != nil {
			emit(serverMsg{Type: msgError, Error: err.Error()})
		}
		return nil
	}
	if _, err := theHub.record(theClientID, opKind(op.Kind), op.Min, op.Max); err != nil {
		emit(serverMsg{Type: msgError, Error: err.Error()})
	}
	return nil
}

// emit pushes one envelope to the page. The dispatch runs synchronously inside
// the fold that produced it, so the page observes state in the same order the
// server's sockets would.
func emit(m serverMsg) {
	if theDispatch.IsUndefined() || theDispatch.IsNull() {
		return
	}
	theDispatch.Invoke(marshal(m))
}

// marshal encodes an envelope as a JSON string, degrading to an error envelope
// if encoding fails (it cannot for the plain types used here).
func marshal(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return `{"type":"error","error":"encode failed"}`
	}
	return string(b)
}

// errorEnvelope encodes a bare error envelope for calls that fail before a
// dispatch exists (an invalid join).
func errorEnvelope(msg string) string {
	return marshal(serverMsg{Type: msgError, Error: msg})
}
