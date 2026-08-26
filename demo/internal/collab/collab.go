// Package collab provides the shared session, presence, and WebSocket
// transport used by the interactive demos. Each demo contributes a Model — a
// pure event log that folds client commands into events — while this package
// owns connection bookkeeping, presence counts, and the wire format the
// bundled UIs speak. Clients receive the log and materialize the view.
package collab

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cskr/pubsub/v2"
	"github.com/patrickmn/go-cache"
)

// SessionTTL is how long a shared model stays alive after its last use. The
// cache below expires each entry on that timer, so abandoned models free
// themselves instead of accumulating forever.
const SessionTTL = 24 * time.Hour

const (
	sessionTopic  = "events"
	presenceTopic = "presence"
)

// Cmd is one anonymous client command. Kind names the command; Data is its
// JSON payload, interpreted by the Model.
type Cmd struct {
	Kind string          `json:"kind"`
	Data json.RawMessage `json:"data,omitempty"`
}

// Entry is one event in a session's log. ID identifies the event for
// idempotent delivery; Kind names the event; Data carries the raw payload so
// clients can materialize the view, export JSONL, or build a share link.
type Entry struct {
	ID     string          `json:"id,omitempty"`
	Client string          `json:"client,omitempty"`
	Kind   string          `json:"kind"`
	Data   json.RawMessage `json:"data,omitempty"`
	Detail string          `json:"detail,omitempty"`
	At     time.Time       `json:"at"`
}

// Message is the wire envelope for every server→client event.
type Message struct {
	Type     string          `json:"type"`
	State    json.RawMessage `json:"state,omitempty"`
	Op       *Entry          `json:"op,omitempty"`
	Ops      []Entry         `json:"ops,omitempty"`
	Clients  int             `json:"clients,omitempty"`
	Total    int             `json:"total,omitempty"`
	ClientID string          `json:"clientID,omitempty"`
	Error    string          `json:"error,omitempty"`
}

// Message types, named so writers and readers agree on the envelope kinds
// instead of repeating string literals.
const (
	TypeState    = "state"
	TypeOp       = "op"
	TypePresence = "presence"
	TypeError    = "error"
)

// Model is one session's event log. Apply folds one attributed client command
// into the log and returns the events it produced; Log observes the full
// history; Snapshot optionally returns a materialized state for fast-forward.
// Implementations must be safe for concurrent use.
type Model interface {
	Apply(clientID string, cmd Cmd) ([]Entry, error)
	Log() []Entry
	Snapshot() any
}

// Sessions owns the in-memory collection of shared models, one per session ID,
// plus the global presence topic every session shares.
type Sessions struct {
	models   *cache.Cache
	total    atomic.Int64
	presence *pubsub.PubSub[string, Message]
	factory  func() Model
}

// NewSessions builds a session registry whose models come from factory.
func NewSessions(ttl time.Duration, factory func() Model) *Sessions {
	cleanup := ttl / 6
	if cleanup < time.Minute {
		cleanup = time.Minute
	}
	return &Sessions{
		models:   cache.New(ttl, cleanup),
		presence: pubsub.New[string, Message](1024),
		factory:  factory,
	}
}

// Model returns the live Session for id, creating it on first use or once the
// previous one has expired. Each access resets the idle timer, so an actively
// shared model never expires under its collaborators.
func (s *Sessions) Model(id string) *Session {
	if h, ok := s.models.Get(id); ok {
		s.models.SetDefault(id, h)
		return h.(*Session)
	}
	sess := newSession(s.factory(), &s.total, s.presence)
	s.models.SetDefault(id, sess)
	return sess
}

// Total returns the number of clients connected across all sessions.
func (s *Sessions) Total() int {
	return int(s.total.Load())
}

// SubscribePresence returns a channel receiving every global presence update.
func (s *Sessions) SubscribePresence() chan Message {
	return s.presence.Sub(presenceTopic)
}

// UnsubscribePresence stops delivering global presence updates to ch.
func (s *Sessions) UnsubscribePresence(ch chan Message) {
	s.presence.Unsub(ch, presenceTopic)
}

// Session binds one Model to connection bookkeeping and a broadcast topic.
type Session struct {
	model    Model
	mu       sync.Mutex
	clients  int
	events   *pubsub.PubSub[string, Message]
	total    *atomic.Int64
	presence *pubsub.PubSub[string, Message]
}

func newSession(m Model, total *atomic.Int64, presence *pubsub.PubSub[string, Message]) *Session {
	return &Session{
		model:    m,
		events:   pubsub.New[string, Message](1024),
		total:    total,
		presence: presence,
	}
}

// Apply folds one attributed command and broadcasts the events it produced to
// every watcher, including the sender.
func (s *Session) Apply(clientID string, cmd Cmd) error {
	entries, err := s.model.Apply(clientID, cmd)
	if err != nil {
		return err
	}
	msg := Message{Type: TypeOp, State: s.snapshot()}
	if len(entries) == 1 {
		entry := entries[0]
		msg.Op = &entry
	} else {
		msg.Ops = entries
	}
	s.events.Pub(msg, sessionTopic)
	return nil
}

// Snapshot returns the current materialized state, ready for the wire, or nil
// when the model exposes only its log.
func (s *Session) Snapshot() json.RawMessage {
	return s.snapshot()
}

func (s *Session) snapshot() json.RawMessage {
	state := s.model.Snapshot()
	if state == nil {
		return nil
	}
	raw, err := json.Marshal(state)
	if err != nil {
		return json.RawMessage(`null`)
	}
	return raw
}

// Subscribe returns a channel receiving every event broadcast to the session.
func (s *Session) Subscribe() chan Message {
	return s.events.Sub(sessionTopic)
}

// Unsubscribe stops delivering session events to ch.
func (s *Session) Unsubscribe(ch chan Message) {
	s.events.Unsub(ch, sessionTopic)
}

// Join registers a watcher, returns the session's activity log and the new
// watcher count, and publishes the updated presence.
func (s *Session) Join() (log []Entry, clients int) {
	s.mu.Lock()
	s.clients++
	clients = s.clients
	s.mu.Unlock()
	s.total.Add(1)
	s.publishPresence(clients)
	return s.model.Log(), clients
}

// Leave unregisters a watcher and publishes the updated presence.
func (s *Session) Leave() {
	s.mu.Lock()
	if s.clients > 0 {
		s.clients--
		s.total.Add(-1)
	}
	clients := s.clients
	s.mu.Unlock()
	s.publishPresence(clients)
}

func (s *Session) publishPresence(clients int) {
	s.events.Pub(Message{Type: TypePresence, Clients: clients}, sessionTopic)
	s.presence.Pub(Message{Type: TypePresence, Total: int(s.total.Load())}, presenceTopic)
}

// NewSessionID mints a short, URL-safe, unguessable identifier for a fresh
// share link.
func NewSessionID() string {
	var b [10]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err) // crypto/rand never fails on supported platforms
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b[:])
}

// NewClientID mints a short, human-readable identity for one connected
// browser, so the activity log can attribute operations to collaborators.
func NewClientID() string {
	var b [3]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b[:])
}
