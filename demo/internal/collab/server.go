package collab

import (
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(*http.Request) bool { return true },
}

// Heartbeat timing: the server pings and expects a pong within pongWait, so a
// client that vanished without a clean close is evicted from the roster.
const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
)

// emailOf resolves the connected user's identity. Behind oauth2-proxy it
// arrives as X-Auth-Request-Email; without a proxy the browser sends its local,
// self-chosen identity as the "u" query parameter, and as a last resort a
// random guest address keeps the roster working.
func emailOf(c *gin.Context) string {
	for _, name := range []string{"X-Auth-Request-Email", "X-Forwarded-Email"} {
		if e := c.GetHeader(name); e != "" {
			return e
		}
	}
	if u := c.Query("u"); u != "" {
		return u
	}
	return "guest-" + NewClientID() + "@local"
}

// NewRouter builds the gin engine for a demo: the UI filesystem at /ui/, a
// bare /ui/ redirect that mints a shareable session URL (preserving the named
// query parameters), and the /ws upgrade that keeps one browser in sync.
func NewRouter(sessions *Sessions, fs http.FileSystem, preserve []string) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/ui/")
	})

	r.Use(func(c *gin.Context) {
		if c.Request.URL.Path == "/ui/" && c.Query("s") == "" {
			target := "/ui/?s=" + NewSessionID()
			for _, key := range preserve {
				if v := c.Query(key); v != "" {
					target += "&" + key + "=" + url.QueryEscape(v)
				}
			}
			c.Redirect(http.StatusFound, target)
			c.Abort()
			return
		}
		c.Next()
	})

	r.StaticFS("/ui/", fs)
	r.GET("/ws", func(c *gin.Context) {
		handleWS(c, sessions)
	})
	return r
}

// handleWS keeps one browser in sync with the session's shared model: a reader
// goroutine folds this client's operations while the handler writes every
// broadcast event back to the socket.
func handleWS(c *gin.Context, s *Sessions) {
	id := c.Query("s")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing session id"})
		return
	}
	sess := s.Model(id)
	clientID := NewClientID()
	email := emailOf(c)

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("websocket upgrade: %v", err)
		return
	}
	defer func() { _ = conn.Close() }()

	// A read deadline plus ping/pong lets the reader detect a peer that
	// disappeared without a close frame, so its roster entry is released.
	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	updates := sess.Subscribe()
	defer sess.Unsubscribe(updates)
	presence := s.SubscribePresence()
	defer s.UnsubscribePresence(presence)

	ops, clients := sess.Join(email)
	defer sess.Leave(email)

	if err := conn.WriteJSON(Message{
		Type:     TypeState,
		State:    sess.Snapshot(),
		ClientID: clientID,
		Clients:  clients,
		Total:    s.Total(),
		Ops:      ops,
	}); err != nil {
		return
	}

	done := make(chan struct{})
	failures := make(chan error, 8)
	go readClientOps(conn, sess, clientID, done, failures)
	go pingLoop(conn, done)
	writeUpdates(conn, updates, presence, failures, done)
}

// pingLoop sends periodic pings until the connection closes; a ping that
// cannot be written forces the socket shut so the reader unblocks.
func pingLoop(conn *websocket.Conn, done <-chan struct{}) {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeWait)); err != nil {
				_ = conn.Close()
				return
			}
		}
	}
}

// readClientOps folds the commands this client contributes until the socket
// closes, attributing each one to the client's identity.
func readClientOps(conn *websocket.Conn, sess *Session, clientID string, done chan<- struct{}, failures chan<- error) {
	defer close(done)
	for {
		var cmd Cmd
		if err := conn.ReadJSON(&cmd); err != nil {
			return
		}
		if err := sess.Apply(clientID, cmd); err != nil {
			failures <- err
		}
	}
}

// writeUpdates streams every broadcast event and any fold errors back to the
// client until the socket closes.
func writeUpdates(conn *websocket.Conn, updates, presence <-chan Message, failures <-chan error, done <-chan struct{}) {
	for forward(conn, updates, presence, failures, done) {
	}
}

// forward sends the next event, presence notice, fold error, or shutdown
// signal to the socket, reporting whether writeUpdates should keep going.
func forward(conn *websocket.Conn, updates, presence <-chan Message, failures <-chan error, done <-chan struct{}) bool {
	select {
	case <-done:
		return false
	case msg, ok := <-updates:
		return ok && conn.WriteJSON(msg) == nil
	case msg, ok := <-presence:
		return ok && conn.WriteJSON(msg) == nil
	case err := <-failures:
		return conn.WriteJSON(Message{Type: TypeError, Error: err.Error()}) == nil
	}
}
