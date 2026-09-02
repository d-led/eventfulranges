//go:build !js

package main

import (
	"log"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
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
			if m := c.Query("compact"); m != "" {
				target += "&compact=" + url.QueryEscape(m)
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
	h := s.model(id, c.Query("compact") == compactMerge)
	clientID := newClientID()

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("websocket upgrade: %v", err)
		return
	}
	defer func() { _ = conn.Close() }()

	updates := h.subscribe()
	defer h.unsubscribe(updates)
	presence := s.subscribePresence()
	defer s.unsubscribePresence(presence)

	log, clients := h.join()
	defer h.leave()

	// A late joiner catches up with the view, the watcher counts, and the log.
	snap := h.snapshot()
	if err := conn.WriteJSON(serverMsg{Type: msgState, State: &snap, ClientID: clientID, Clients: clients, Total: int(s.total.Load()), Ops: log}); err != nil {
		return
	}

	done := make(chan struct{})
	failures := make(chan error, 8)
	go readClientOps(conn, h, clientID, done, failures)
	writeUpdates(conn, updates, presence, failures, done)
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
func writeUpdates(conn *websocket.Conn, updates, presence <-chan serverMsg, failures <-chan error, done <-chan struct{}) {
	for {
		if !forward(conn, updates, presence, failures, done) {
			return
		}
	}
}

// forward sends the next event, presence notice, fold error, or shutdown
// signal to the socket, reporting whether writeUpdates should keep going.
func forward(conn *websocket.Conn, updates, presence <-chan serverMsg, failures <-chan error, done <-chan struct{}) bool {
	select {
	case <-done:
		return false
	case msg, ok := <-updates:
		return writeMsg(conn, msg, ok)
	case msg, ok := <-presence:
		return writeMsg(conn, msg, ok)
	case err := <-failures:
		return writeErr(conn, err)
	}
}

// writeMsg writes one broadcast envelope, reporting false when the source
// channel closed or the socket rejected the write.
func writeMsg(conn *websocket.Conn, msg serverMsg, ok bool) bool {
	if !ok {
		return false
	}
	return conn.WriteJSON(msg) == nil
}

// writeErr writes one fold error as an error envelope, reporting false when
// the socket rejected the write.
func writeErr(conn *websocket.Conn, err error) bool {
	return conn.WriteJSON(serverMsg{Type: msgError, Error: err.Error()}) == nil
}
