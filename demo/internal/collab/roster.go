package collab

import (
	"sort"
	"strings"
	"sync"
)

// RosterEntry is one connected user in the "who's here" list: their anonymized
// email and the session IDs they are connected to.
type RosterEntry struct {
	User     string   `json:"user"`
	Sessions []string `json:"sessions"`
}

// roster tracks, per connected user, which sessions they are connected to. It
// is the shared, global view published to every client whenever someone joins
// or leaves any session.
type roster struct {
	mu    sync.Mutex
	users map[string]map[string]int // email -> sessionID -> connection count
}

func newRoster() *roster {
	return &roster{users: make(map[string]map[string]int)}
}

// connect registers one connection of email to sessionID.
func (r *roster) connect(email, sessionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	sessions, ok := r.users[email]
	if !ok {
		sessions = make(map[string]int)
		r.users[email] = sessions
	}
	sessions[sessionID]++
}

// disconnect unregisters one connection; the user drops out of the roster once
// none of their sessions has a live connection left.
func (r *roster) disconnect(email, sessionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	sessions, ok := r.users[email]
	if !ok {
		return
	}
	if sessions[sessionID] > 1 {
		sessions[sessionID]--
		return
	}
	delete(sessions, sessionID)
	if len(sessions) == 0 {
		delete(r.users, email)
	}
}

// snapshot returns the roster as a deterministically ordered list, with emails
// anonymized so the wire never carries a raw address.
func (r *roster) snapshot() []RosterEntry {
	r.mu.Lock()
	defer r.mu.Unlock()
	entries := make([]RosterEntry, 0, len(r.users))
	for email, sessions := range r.users {
		ids := make([]string, 0, len(sessions))
		for id := range sessions {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		entries = append(entries, RosterEntry{User: anonymizeEmail(email), Sessions: ids})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].User != entries[j].User {
			return entries[i].User < entries[j].User
		}
		return strings.Join(entries[i].Sessions, "\x00") < strings.Join(entries[j].Sessions, "\x00")
	})
	return entries
}

// anonymizeEmail masks an address for display: the local part keeps its first
// quarter (at least three characters), the domain its first half, and the rest
// collapses to a single "…".
func anonymizeEmail(email string) string {
	if email == "" {
		return "anonymous"
	}
	local, domain := email, ""
	if at := strings.IndexByte(email, '@'); at >= 0 {
		local, domain = email[:at], email[at+1:]
	}
	return maskPrefix(local, 4, 3) + "@" + maskPrefix(domain, 2, 1)
}

// maskPrefix keeps the first len(s)/divisor characters (at least minKeep) and
// appends "…" when anything is hidden.
func maskPrefix(s string, divisor, minKeep int) string {
	if s == "" {
		return "…"
	}
	keep := len(s) / divisor
	if keep < minKeep {
		keep = minKeep
	}
	if keep >= len(s) {
		return s
	}
	return s[:keep] + "…"
}
