package main

import (
	"log"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// clientOp is one add/remove/dims command the browser submits over the socket.
type clientOp struct {
	Kind string    `json:"kind"`
	Min  []float64 `json:"min"`
	Max  []float64 `json:"max"`
	Dims int       `json:"dims"`
}

// serverMsg is the envelope the server sends back over the socket.
type serverMsg struct {
	Type     string     `json:"type"` // one of msgState, msgOp, msgPresence, msgError
	State    *view      `json:"state,omitempty"`
	Op       *opRecord  `json:"op,omitempty"`
	Ops      []opRecord `json:"ops,omitempty"` // the full log, sent on join
	Clients  int        `json:"clients,omitempty"`
	ClientID string     `json:"clientID,omitempty"`
	Error    string     `json:"error,omitempty"`
}

// serverMsg.Type values, named so writers and readers agree on the envelope
// kinds instead of repeating string literals.
const (
	msgState    = "state"
	msgOp       = "op"
	msgPresence = "presence"
	msgError    = "error"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(*http.Request) bool { return true },
}

func newRouter(s *sessions, fs http.FileSystem) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/ui/")
	})

	// A bare /ui/ visit has no session yet: mint one and redirect to the
	// shareable URL so every collaborator who opens it joins the same model.
	r.Use(func(c *gin.Context) {
		if c.Request.URL.Path == "/ui/" && c.Query("s") == "" {
			target := "/ui/?s=" + newSessionID()
			if d := c.Query("dims"); d != "" {
				target += "&dims=" + url.QueryEscape(d)
			}
			c.Redirect(http.StatusFound, target)
			c.Abort()
			return
		}
		c.Next()
	})

	r.StaticFS("/ui/", fs)
	r.GET("/ws", func(c *gin.Context) {
		handleWS(c, s)
	})
	return r
}

// handleWS keeps one browser in sync with the session's shared view: a reader
// goroutine folds this client's operations while the handler writes every
// broadcast event back to the socket.
func handleWS(c *gin.Context, s *sessions) {
	id := c.Query("s")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing session id"})
		return
	}
	h := s.model(id)
	clientID := newClientID()

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("websocket upgrade: %v", err)
		return
	}
	defer func() { _ = conn.Close() }()

	updates := h.subscribe()
	defer h.unsubscribe(updates)

	log, clients := h.join()
	defer h.leave()

	// A late joiner catches up with the view, the watcher count, and the log.
	snap := h.snapshot()
	if err := conn.WriteJSON(serverMsg{Type: msgState, State: &snap, ClientID: clientID, Clients: clients, Ops: log}); err != nil {
		return
	}

	done := make(chan struct{})
	failures := make(chan error, 8)
	go readClientOps(conn, h, clientID, done, failures)
	writeUpdates(conn, updates, failures, done)
}

// readClientOps folds the operations this client contributes until the socket
// closes, attributing each one to the client's identity.
func readClientOps(conn *websocket.Conn, h *hub, clientID string, done chan<- struct{}, failures chan<- error) {
	defer close(done)
	for {
		var op clientOp
		if err := conn.ReadJSON(&op); err != nil {
			return
		}
		if op.Kind == string(opDims) {
			if _, err := h.setDims(clientID, op.Dims); err != nil {
				failures <- err
			}
			continue
		}
		if _, err := h.record(clientID, opKind(op.Kind), op.Min, op.Max); err != nil {
			failures <- err
		}
	}
}

// writeUpdates streams every broadcast event and any fold errors back to the
// client until the socket closes.
func writeUpdates(conn *websocket.Conn, updates <-chan serverMsg, failures <-chan error, done <-chan struct{}) {
	for {
		select {
		case <-done:
			return
		case msg, ok := <-updates:
			if !ok {
				return
			}
			if conn.WriteJSON(msg) != nil {
				return
			}
		case err := <-failures:
			if conn.WriteJSON(serverMsg{Type: msgError, Error: err.Error()}) != nil {
				return
			}
		}
	}
}
