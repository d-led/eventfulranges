package collab

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

// AdminGate reports whether an email may use the admin area. The gate is
// deployment-specific: behind oauth2-proxy it is the admin email list.
type AdminGate interface {
	IsAdmin(email string) bool
}

// SessionInfo describes one session's resource usage for the admin view.
type SessionInfo struct {
	ID      string `json:"id"`
	Bytes   int64  `json:"bytes"`
	Clients int    `json:"clients"`
}

// AdminInfo is the aggregate the admin page displays.
type AdminInfo struct {
	StorageBytes int64         `json:"storageBytes"`
	Users        []string      `json:"users"`
	Sessions     []SessionInfo `json:"sessions"`
}

// ErrSessionActive is returned when deleting a session that still has
// connected clients.
var ErrSessionActive = errors.New("collab: session has connected clients")

// ErrInvalidSessionID is returned when a session id is not a safe file name.
var ErrInvalidSessionID = errors.New("collab: invalid session id")

// errorField is the JSON key of every error response body.
const errorField = "error"

// seenUsers remembers every email that has ever joined any session, in memory
// only: a restart forgets the list, by design.
type seenUsers struct {
	mu    sync.Mutex
	users map[string]struct{}
}

func newSeenUsers() *seenUsers {
	return &seenUsers{users: make(map[string]struct{})}
}

func (s *seenUsers) add(email string) {
	if email == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.users[email] = struct{}{}
}

func (s *seenUsers) list() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.users))
	for email := range s.users {
		out = append(out, email)
	}
	sort.Strings(out)
	return out
}

// Users returns the emails of every user who has ever logged in on this
// instance, in memory only.
func (s *Sessions) Users() []string {
	return s.seen.list()
}

// AdminInfo reports the storage used by the session logs, the users seen, and
// each session's size with its live client count.
func (s *Sessions) AdminInfo() AdminInfo {
	onDisk := sessionFiles(s.dir)
	inMemory, sessions := s.sessionInfos(onDisk)
	// A file whose session expired from memory is still inactive storage.
	for id, size := range onDisk {
		if inMemory[id] {
			continue
		}
		sessions = append(sessions, SessionInfo{ID: id, Bytes: size})
	}

	info := AdminInfo{Users: s.Users(), Sessions: sessions}
	for _, sess := range sessions {
		info.StorageBytes += sess.Bytes
	}
	sort.Slice(sessions, func(i, j int) bool { return sessions[i].ID < sessions[j].ID })
	return info
}

// sessionFiles maps each persisted session id to its log file size. Session
// logs are one file each, named after the URL-safe session id.
func sessionFiles(dir string) map[string]int64 {
	files := map[string]int64{}
	if dir == "" {
		return files
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return files
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		if meta, err := entry.Info(); err == nil {
			files[strings.TrimSuffix(entry.Name(), ".jsonl")] = meta.Size()
		}
	}
	return files
}

// sessionInfos lists the in-memory sessions with their sizes, and returns the
// set of session ids currently held in memory.
func (s *Sessions) sessionInfos(onDisk map[string]int64) (map[string]bool, []SessionInfo) {
	inMemory := map[string]bool{}
	sessions := make([]SessionInfo, 0, len(onDisk))
	for id, item := range s.models.Items() {
		sess, ok := item.Object.(*Session)
		if !ok {
			continue
		}
		inMemory[id] = true
		sessions = append(sessions, SessionInfo{ID: id, Bytes: onDisk[id], Clients: sess.clientCount()})
	}
	return inMemory, sessions
}

// DeleteSession removes an inactive session — one with no connected clients —
// from memory and disk. Deleting an active session fails with ErrSessionActive.
func (s *Sessions) DeleteSession(id string) error {
	path, err := s.deletePath(id)
	if err != nil {
		return err
	}
	if h, ok := s.models.Get(id); ok {
		if sess := h.(*Session); sess.clientCount() > 0 {
			return ErrSessionActive
		}
		s.models.Delete(id)
	}
	if path == "" {
		return nil
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// deletePath resolves id to its log file under the data directory, rejecting
// any id that would escape it. An in-memory registry has no path to resolve.
func (s *Sessions) deletePath(id string) (string, error) {
	if s.dir == "" {
		return "", nil
	}
	if id == "" || id == "." || id == ".." {
		return "", ErrInvalidSessionID
	}
	path := filepath.Join(s.dir, id+".jsonl")
	if filepath.Dir(path) != filepath.Clean(s.dir) {
		return "", ErrInvalidSessionID
	}
	return path, nil
}

// proxyEmail resolves the authenticated email from the reverse-proxy headers
// only. Unlike the WebSocket identity (which falls back to a client-chosen
// name), the admin gate must trust the proxy alone.
func proxyEmail(c *gin.Context) string {
	for _, name := range []string{"X-Auth-Request-Email", "X-Forwarded-Email"} {
		if e := c.GetHeader(name); e != "" {
			return e
		}
	}
	return ""
}

// RegisterAdminRoutes mounts the admin page and its JSON API behind the gate.
// The page and script are served from the same UI filesystem as the main UI,
// so a demo only has to build admin.html and admin.js into its dist directory.
func RegisterAdminRoutes(r gin.IRouter, sessions *Sessions, gate AdminGate, fs http.FileSystem) {
	admin := r.Group("/admin", func(c *gin.Context) {
		if gate == nil || !gate.IsAdmin(proxyEmail(c)) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{errorField: "admin access required"})
			return
		}
		c.Next()
	})

	admin.GET("/", func(c *gin.Context) { c.FileFromFS("admin.html", fs) })
	admin.GET("/admin.js", func(c *gin.Context) { c.FileFromFS("admin.js", fs) })
	admin.GET("/api/info", func(c *gin.Context) {
		c.JSON(http.StatusOK, sessions.AdminInfo())
	})
	admin.DELETE("/api/sessions/:id", func(c *gin.Context) {
		err := sessions.DeleteSession(c.Param("id"))
		switch {
		case errors.Is(err, ErrSessionActive):
			c.JSON(http.StatusConflict, gin.H{errorField: err.Error()})
		case errors.Is(err, ErrInvalidSessionID):
			c.JSON(http.StatusBadRequest, gin.H{errorField: err.Error()})
		case err != nil:
			c.JSON(http.StatusInternalServerError, gin.H{errorField: "could not delete session"})
		default:
			c.Status(http.StatusNoContent)
		}
	})
}
