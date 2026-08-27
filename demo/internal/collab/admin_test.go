package collab

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// staticGate is a fixed admin email set for tests.
type staticGate struct {
	emails map[string]bool
}

func (g staticGate) IsAdmin(email string) bool { return g.emails[email] }

func TestSeenUsersKeepsEveryLoginInMemory(t *testing.T) {
	t.Parallel()
	reg := NewSessions(time.Hour, func() Model { return &counter{} })

	sess := reg.Model("s1")
	_, clients := sess.Join("alice@example.com")
	require.Equal(t, 1, clients)
	sess.Leave("alice@example.com")

	reg.Model("s2").Join("bob@example.com")

	require.Equal(t, []string{"alice@example.com", "bob@example.com"}, reg.Users())
}

func TestAdminInfoReportsStorageUsersAndSessions(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	reg := NewPersistentSessions(time.Hour, dir, func() Model { return &counter{} })

	s1 := reg.Model("s1")
	require.NoError(t, s1.Apply("alice", Cmd{Kind: "inc", Data: json.RawMessage(`{"delta":2}`)}))
	s1.Join("alice@example.com")

	reg.Model("s2").Join("bob@example.com")

	info := reg.AdminInfo()
	require.Equal(t, []string{"alice@example.com", "bob@example.com"}, info.Users)
	require.Len(t, info.Sessions, 2)
	require.True(t, info.StorageBytes > 0, "the persisted session log occupies space")

	var bytes int64
	for _, s := range info.Sessions {
		if s.ID == "s1" {
			require.True(t, s.Bytes > 0, "s1 wrote entries to disk")
		}
		bytes += s.Bytes
	}
	require.Equal(t, info.StorageBytes, bytes, "storage is the sum of the session sizes")
}

func TestDeleteSessionRemovesInactiveSession(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	reg := NewPersistentSessions(time.Hour, dir, func() Model { return &counter{} })

	require.NoError(t, reg.Model("s1").Apply("alice", Cmd{Kind: "inc", Data: json.RawMessage(`{"delta":2}`)}))
	require.True(t, reg.AdminInfo().StorageBytes > 0)

	require.NoError(t, reg.DeleteSession("s1"))
	require.Empty(t, reg.AdminInfo().Sessions)
	require.Zero(t, reg.AdminInfo().StorageBytes)
}

func TestDeleteSessionRejectsActiveSession(t *testing.T) {
	t.Parallel()
	reg := NewSessions(time.Hour, func() Model { return &counter{} })

	sess := reg.Model("s1")
	_, clients := sess.Join("alice@example.com")
	require.Equal(t, 1, clients)

	require.ErrorIs(t, reg.DeleteSession("s1"), ErrSessionActive)
}

func TestDeleteSessionIgnoresMissingFile(t *testing.T) {
	t.Parallel()
	reg := NewSessions(time.Hour, func() Model { return &counter{} })
	require.NoError(t, reg.DeleteSession("ghost"))
}

func TestAdminRoutesGateByProxyEmail(t *testing.T) {
	t.Parallel()
	reg := NewSessions(time.Hour, func() Model { return &counter{} })
	router := gin.New()
	RegisterAdminRoutes(router, reg, staticGate{emails: map[string]bool{"admin@example.com": true}}, http.Dir(t.TempDir()))
	srv := httptest.NewServer(router)
	defer srv.Close()

	// No proxy email: forbidden.
	resp, err := http.Get(srv.URL + "/admin/api/info")
	require.NoError(t, err)
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
	require.NoError(t, resp.Body.Close())

	// A non-admin email: forbidden.
	req, err := http.NewRequest(http.MethodGet, srv.URL+"/admin/api/info", nil)
	require.NoError(t, err)
	req.Header.Set("X-Auth-Request-Email", "stranger@example.com")
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
	require.NoError(t, resp.Body.Close())

	// An admin email: allowed.
	req, err = http.NewRequest(http.MethodGet, srv.URL+"/admin/api/info", nil)
	require.NoError(t, err)
	req.Header.Set("X-Forwarded-Email", "admin@example.com")
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var info AdminInfo
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&info))
	require.NotNil(t, info.Users)
	require.NotNil(t, info.Sessions)
}

func TestAdminDeleteSessionRoute(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	reg := NewPersistentSessions(time.Hour, dir, func() Model { return &counter{} })
	require.NoError(t, reg.Model("idle").Apply("alice", Cmd{Kind: "inc", Data: json.RawMessage(`{"delta":1}`)}))
	reg.Model("busy").Join("alice@example.com")

	router := gin.New()
	RegisterAdminRoutes(router, reg, staticGate{emails: map[string]bool{"admin@example.com": true}}, http.Dir(t.TempDir()))
	srv := httptest.NewServer(router)
	defer srv.Close()

	do := func(method, path string) *http.Response {
		t.Helper()
		req, err := http.NewRequest(method, srv.URL+path, nil)
		require.NoError(t, err)
		req.Header.Set("X-Auth-Request-Email", "admin@example.com")
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		return resp
	}

	// A session with connected clients cannot be deleted.
	resp := do(http.MethodDelete, "/admin/api/sessions/busy")
	require.Equal(t, http.StatusConflict, resp.StatusCode)
	require.NoError(t, resp.Body.Close())

	// An inactive session can.
	resp = do(http.MethodDelete, "/admin/api/sessions/idle")
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	require.NoError(t, resp.Body.Close())

	// The idle session is gone; the active one remains.
	remaining := reg.AdminInfo().Sessions
	require.Len(t, remaining, 1)
	require.Equal(t, "busy", remaining[0].ID)
}

func TestAdminRoutesRejectNilGate(t *testing.T) {
	t.Parallel()
	router := gin.New()
	RegisterAdminRoutes(router, NewSessions(time.Hour, func() Model { return &counter{} }), nil, http.Dir(t.TempDir()))
	srv := httptest.NewServer(router)
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/admin/api/info", nil)
	require.NoError(t, err)
	req.Header.Set("X-Auth-Request-Email", "admin@example.com")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
}
