package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// clientOp is one add/remove/clear the browser submits over the socket.
type clientOp struct {
	Kind string    `json:"kind"`
	Min  []float64 `json:"min"`
	Max  []float64 `json:"max"`
}

// serverMsg is the envelope the server sends back over the socket.
type serverMsg struct {
	Type  string `json:"type"` // "state" or "error"
	State *view  `json:"state,omitempty"`
	Error string `json:"error,omitempty"`
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(*http.Request) bool { return true },
}

func newRouter(h *hub, fs http.FileSystem) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	r.StaticFS("/ui/", fs)
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/ui/")
	})
	r.GET("/ws", func(c *gin.Context) {
		handleWS(c, h)
	})
	return r
}

// handleWS keeps one browser in sync with the shared view: a reader goroutine
// folds this client's operations while the handler writes every broadcast
// state back to the socket.
func handleWS(c *gin.Context, h *hub) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("websocket upgrade: %v", err)
		return
	}
	defer func() { _ = conn.Close() }()

	updates := h.subscribe()
	defer h.unsubscribe(updates)

	// A late joiner catches up immediately.
	snap := h.snapshot()
	if err := conn.WriteJSON(serverMsg{Type: "state", State: &snap}); err != nil {
		return
	}

	done := make(chan struct{})
	failures := make(chan error, 8)
	go readClientOps(conn, h, done, failures)
	writeUpdates(conn, updates, failures, done)
}

// readClientOps folds the operations this client contributes until the socket
// closes.
func readClientOps(conn *websocket.Conn, h *hub, done chan<- struct{}, failures chan<- error) {
	defer close(done)
	for {
		var op clientOp
		if err := conn.ReadJSON(&op); err != nil {
			return
		}
		if _, err := h.apply(opKind(op.Kind), op.Min, op.Max); err != nil {
			failures <- err
		}
	}
}

// writeUpdates streams the converged view and any fold errors back to the
// client until the socket closes.
func writeUpdates(conn *websocket.Conn, updates <-chan view, failures <-chan error, done <-chan struct{}) {
	for {
		select {
		case <-done:
			return
		case v, ok := <-updates:
			if !ok {
				return
			}
			if conn.WriteJSON(serverMsg{Type: "state", State: &v}) != nil {
				return
			}
		case err := <-failures:
			if conn.WriteJSON(serverMsg{Type: "error", Error: err.Error()}) != nil {
				return
			}
		}
	}
}
