//go:build !embed

package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/d-led/eventfulranges/demo/internal/collab"
)

// The dev gate is compiled only into non-embedded (development) builds, so
// these tests pin the behavior a local developer sees when running the server
// without oauth2-proxy and without an ADMIN_EMAILS secret.

func TestDevOpenGateAdmitsDirectRequests(t *testing.T) {
	t.Parallel()
	// A request with no reverse-proxy email is the local developer's browser.
	require.True(t, devOpenGate{}.IsAdmin(""))
	// A request that names an identity is not automatically admitted.
	require.False(t, devOpenGate{}.IsAdmin("someone@example.com"))
}

func TestDevAdminGateForOpensWithoutAList(t *testing.T) {
	t.Parallel()
	gate := adminGateFor("")
	require.True(t, gate.IsAdmin(""), "no ADMIN_EMAILS: a direct request may use the admin area")
	require.False(t, gate.IsAdmin("someone@example.com"), "a named identity is still gated")
}

func TestDevAdminGateForHonoursAConfiguredList(t *testing.T) {
	t.Parallel()
	gate := adminGateFor("admin@example.com, other@example.com")
	require.True(t, gate.IsAdmin("admin@example.com"))
	require.True(t, gate.IsAdmin("other@example.com"))
	require.False(t, gate.IsAdmin(""))
	require.False(t, gate.IsAdmin("stranger@example.com"))
}

// TestDevAdminAreaReachableWithoutProxy proves that with no ADMIN_EMAILS
// configured, the real admin API is served to a direct request — the request
// shape of a local developer's browser — while an identity-claiming request is
// still refused.
func TestDevAdminAreaReachableWithoutProxy(t *testing.T) {
	t.Parallel()
	reg := collab.NewSessions(time.Hour, func() collab.Model { return &board{} })
	router := gin.New()
	collab.RegisterAdminRoutes(router, reg, adminGateFor(""), http.Dir(t.TempDir()))

	// A direct request (no reverse-proxy email) is the local developer: served.
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/admin/api/info", nil))
	require.Equal(t, http.StatusOK, resp.Code)

	// A request that claims an identity is not silently trusted.
	resp = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/api/info", nil)
	req.Header.Set("X-Auth-Request-Email", "someone@example.com")
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusForbidden, resp.Code)
}
