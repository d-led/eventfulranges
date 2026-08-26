package main

import (
	"crypto/rand"
	"encoding/base32"
	"log"
	"sync/atomic"
	"time"

	"github.com/cskr/pubsub/v2"
	"github.com/patrickmn/go-cache"
)

// sessionTTL is how long a shared view stays alive after its last use. The
// cache below expires each entry on that timer, so abandoned models free
// themselves instead of accumulating forever.
const sessionTTL = 24 * time.Hour

// presenceTopic names the global channel every client also subscribes to, so a
// join in one session refreshes the "total connected" count on every screen.
const presenceTopic = "presence"

// sessions owns the in-memory collection of shared views, one per session ID.
// Every share link carries a session ID; the pub/sub model for that ID is the
// topic its collaborators converge on, isolated from every other link. When
// dir is non-empty, each session's activity log is also persisted to disk, so
// a restart reloads the view instead of starting it empty.
type sessions struct {
	models   *cache.Cache
	total    atomic.Int64                      // connected clients across all sessions
	presence *pubsub.PubSub[string, serverMsg] // global presence topic
	dir      string                            // "" disables persistence
}

func newSessions(ttl time.Duration) *sessions {
	return newSessionsDir(ttl, "")
}

func newPersistentSessions(ttl time.Duration, dir string) *sessions {
	return newSessionsDir(ttl, dir)
}

func newSessionsDir(ttl time.Duration, dir string) *sessions {
	cleanup := ttl / 6
	if cleanup < time.Minute {
		cleanup = time.Minute
	}
	return &sessions{
		models:   cache.New(ttl, cleanup),
		presence: pubsub.New[string, serverMsg](1024),
		dir:      dir,
	}
}

// model returns the live shared view for id, creating it on first use or once
// the previous one has expired. Each access resets the idle timer, so an
// actively shared model never expires under its collaborators. A persistent
// registry replays the session's stored log into a fresh view before sharing.
func (s *sessions) model(id string, compact bool) *hub {
	if h, ok := s.models.Get(id); ok {
		s.models.SetDefault(id, h)
		return h.(*hub)
	}
	h := newHub(compact)
	h.total = &s.total
	h.presence = s.presence
	if s.dir != "" {
		store, err := openHubStore(s.dir, id)
		if err != nil {
			log.Printf("persist: %v", err)
		} else {
			if records, lerr := store.load(); lerr != nil {
				log.Printf("persist load: %v", lerr)
			} else {
				h.mu.Lock()
				for _, rec := range records {
					switch opKind(rec.Kind) {
					case opDims:
						_, _ = h.foldDimsLocked(rec.Dims)
					default:
						_, _ = h.foldLocked(opKind(rec.Kind), rec.Min, rec.Max)
					}
					h.ops = append(h.ops, rec)
				}
				h.mu.Unlock()
			}
			h.persist = store.append
		}
	}
	s.models.SetDefault(id, h)
	return h
}

// subscribePresence returns a channel receiving every global presence update.
func (s *sessions) subscribePresence() chan serverMsg {
	return s.presence.Sub(presenceTopic)
}

func (s *sessions) unsubscribePresence(ch chan serverMsg) {
	s.presence.Unsub(ch, presenceTopic)
}

// newSessionID mints a short, URL-safe, unguessable identifier for a fresh
// share link.
func newSessionID() string {
	var b [10]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err) // crypto/rand never fails on supported platforms
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b[:])
}

// newClientID mints a short, human-readable identity for one connected
// browser, so the activity log can attribute operations to collaborators.
func newClientID() string {
	var b [3]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err) // crypto/rand never fails on supported platforms
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b[:])
}
