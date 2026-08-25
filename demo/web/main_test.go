package main

import (
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"

	"github.com/d-led/eventfulranges/space"
)

func TestViewAddThenRemoveYieldsHollowShell(t *testing.T) {
	t.Parallel()
	h := newHub()

	_, err := h.apply(opAdd, []float64{0, 0, 0}, []float64{4, 4, 4})
	require.NoError(t, err)
	_, err = h.apply(opRemove, []float64{1, 1, 1}, []float64{3, 3, 3})
	require.NoError(t, err)

	v := h.snapshot()
	require.Len(t, v.Boxes, 6, "a cube minus a cube is a six-face shell")
	require.Equal(t, 1, v.Adds)
	require.Equal(t, 1, v.Removes)
	require.Equal(t, 3, v.Dims)

	// The hole is empty; the shell is not.
	require.False(t, space.Contains(v.Boxes, []float64{2, 2, 2}))
	require.True(t, space.Contains(v.Boxes, []float64{0.5, 2, 2}))
}

func TestViewConvergesRegardlessOfOrder(t *testing.T) {
	t.Parallel()
	first := newHub()
	second := newHub()

	// Replica A: add then remove. Replica B: remove then add.
	_, err := first.apply(opAdd, []float64{0, 0}, []float64{4, 4})
	require.NoError(t, err)
	_, err = first.apply(opRemove, []float64{1, 1}, []float64{3, 3})
	require.NoError(t, err)

	_, err = second.apply(opRemove, []float64{1, 1}, []float64{3, 3})
	require.NoError(t, err)
	_, err = second.apply(opAdd, []float64{0, 0}, []float64{4, 4})
	require.NoError(t, err)

	require.True(t, space.Equal(first.snapshot().Boxes, second.snapshot().Boxes))
}

func TestViewRejectsBadInput(t *testing.T) {
	t.Parallel()
	h := newHub()
	_, err := h.apply(opAdd, []float64{0, 0}, []float64{4, 4})
	require.NoError(t, err)

	_, err = h.apply(opAdd, []float64{0, 0, 0}, []float64{4, 4, 4})
	require.Error(t, err, "dimension mismatch")

	_, err = h.apply(opAdd, []float64{math.NaN(), 0}, []float64{1, 1})
	require.Error(t, err, "NaN coordinate")

	_, err = h.apply(opKind("paint"), []float64{0, 0}, []float64{1, 1})
	require.Error(t, err, "unknown operation")
}

func TestViewClearResetsDimensions(t *testing.T) {
	t.Parallel()
	h := newHub()
	_, err := h.apply(opAdd, []float64{0}, []float64{1})
	require.NoError(t, err)

	_, err = h.apply(opClear, nil, nil)
	require.NoError(t, err)

	v := h.snapshot()
	require.Empty(t, v.Boxes)
	require.Equal(t, -1, v.Dims)

	// A different dimension is accepted after clearing.
	_, err = h.apply(opAdd, []float64{0, 0, 0}, []float64{1, 1, 1})
	require.NoError(t, err)
	require.Equal(t, 3, h.snapshot().Dims)
}

func TestWebSocketBroadcastsToEveryClient(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(newRouter(newSessions(time.Hour), GetFS()))
	defer srv.Close()

	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws?s=shared"

	alice := dial(t, url)
	defer func() { _ = alice.Close() }()
	bob := dial(t, url)
	defer func() { _ = bob.Close() }()

	// Both clients receive the initial empty state.
	assertState(t, alice, func(s *view) { require.Empty(t, s.Boxes) })
	assertState(t, bob, func(s *view) { require.Empty(t, s.Boxes) })

	// Alice adds a box; both Alice and Bob observe it.
	err := alice.WriteJSON(clientOp{Kind: "add", Min: []float64{0, 0}, Max: []float64{4, 4}})
	require.NoError(t, err)

	assertState(t, alice, func(s *view) { require.Len(t, s.Boxes, 1) })
	assertState(t, bob, func(s *view) { require.Len(t, s.Boxes, 1) })
}

func TestHubTracksPresenceAndLog(t *testing.T) {
	t.Parallel()
	h := newHub()

	_, err := h.record("alice", opAdd, []float64{0}, []float64{1})
	require.NoError(t, err)
	_, err = h.record("bob", opRemove, []float64{2}, []float64{3})
	require.NoError(t, err)

	log, clients := h.join()
	require.Equal(t, 2, len(log))
	require.Equal(t, "alice", log[0].Client)
	require.Equal(t, "add", log[0].Kind)
	require.Equal(t, []float64{0}, log[0].Min)
	require.Equal(t, 1, clients)

	h.leave()
}

func TestHubClearResetsLog(t *testing.T) {
	t.Parallel()
	h := newHub()

	_, err := h.record("alice", opAdd, []float64{0}, []float64{1})
	require.NoError(t, err)
	_, err = h.record("alice", opRemove, []float64{2}, []float64{3})
	require.NoError(t, err)
	_, err = h.record("alice", opClear, nil, nil)
	require.NoError(t, err)
	_, err = h.record("bob", opAdd, []float64{5}, []float64{6})
	require.NoError(t, err)

	log, _ := h.join()
	require.Equal(t, []string{"clear", "add"}, []string{log[0].Kind, log[1].Kind},
		"a clear wipes the history, leaving only the reset and what follows")
}

func TestWebSocketReportsIdentityAndLog(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(newRouter(newSessions(time.Hour), GetFS()))
	defer srv.Close()

	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws?s=shared"

	alice := dial(t, url)
	defer func() { _ = alice.Close() }()

	var aliceHello serverMsg
	require.NoError(t, alice.ReadJSON(&aliceHello))
	require.Equal(t, "state", aliceHello.Type)
	require.NotEmpty(t, aliceHello.ClientID)
	require.Equal(t, 1, aliceHello.Clients)

	bob := dial(t, url)
	defer func() { _ = bob.Close() }()

	var bobHello serverMsg
	require.NoError(t, bob.ReadJSON(&bobHello))
	require.Equal(t, "state", bobHello.Type)
	require.NotEmpty(t, bobHello.ClientID)
	require.NotEqual(t, aliceHello.ClientID, bobHello.ClientID)
	require.Equal(t, 2, bobHello.Clients)

	// Alice edits; Bob sees the operation attributed to Alice's identity.
	require.NoError(t, alice.WriteJSON(clientOp{Kind: "add", Min: []float64{0}, Max: []float64{1}}))

	var opMsg serverMsg
	for {
		require.NoError(t, bob.ReadJSON(&opMsg))
		if opMsg.Type == "op" {
			break
		}
	}
	require.Equal(t, aliceHello.ClientID, opMsg.Op.Client)
	require.Equal(t, "add", opMsg.Op.Kind)
	require.Equal(t, []float64{0}, opMsg.Op.Min)
	require.NotNil(t, opMsg.State)
}

func TestSessionsAreIsolated(t *testing.T) {
	t.Parallel()
	s := newSessions(time.Hour)

	alpha := s.model("alpha")
	beta := s.model("beta")

	_, err := alpha.apply(opAdd, []float64{0}, []float64{1})
	require.NoError(t, err)

	require.Len(t, alpha.snapshot().Boxes, 1)
	require.Empty(t, beta.snapshot().Boxes, "another session must not observe this edit")
}

func TestSessionExpires(t *testing.T) {
	t.Parallel()
	s := newSessions(10 * time.Millisecond)

	first := s.model("id")
	require.Same(t, first, s.model("id"), "an active session is reused")

	time.Sleep(30 * time.Millisecond)
	require.NotSame(t, first, s.model("id"), "an expired session is recreated fresh")
}

func TestNewSessionIDIsUnique(t *testing.T) {
	t.Parallel()
	first := newSessionID()
	second := newSessionID()
	require.NotEmpty(t, first)
	require.NotEqual(t, first, second)
}

func TestRouterServesTheUI(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(newRouter(newSessions(time.Hour), GetFS()))
	defer srv.Close()

	// A bare /ui/ visit mints a session and redirects to its shareable URL.
	// The client must not follow the redirect, so it can observe the 302.
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Get(srv.URL + "/ui/")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusFound, resp.StatusCode)
	require.Regexp(t, `\?s=[A-Z2-7]+`, resp.Header.Get("Location"))

	// That shareable URL serves the UI itself.
	ui, err := http.Get(srv.URL + resp.Header.Get("Location"))
	require.NoError(t, err)
	defer func() { _ = ui.Body.Close() }()
	require.Equal(t, http.StatusOK, ui.StatusCode)
	require.Contains(t, ui.Header.Get("Content-Type"), "text/html")
}

func TestUIURL(t *testing.T) {
	t.Parallel()
	require.Equal(t, "http://localhost:8080/ui/", uiURL(":8080"))
	require.Equal(t, "http://localhost:8080/ui/", uiURL("0.0.0.0:8080"))
	require.Equal(t, "http://127.0.0.1:18080/ui/", uiURL("127.0.0.1:18080"))
}

func TestEnsureUIFindsTheBuiltUI(t *testing.T) {
	t.Parallel()
	require.NoError(t, ensureUI(), "the committed dist/ must be present")
}

func dial(t *testing.T, url string) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	require.NoError(t, err)
	return conn
}

// readState reads messages until one carries a view, skipping presence and
// log events that may interleave with state broadcasts.
func readState(t *testing.T, conn *websocket.Conn) *view {
	t.Helper()
	for {
		var msg serverMsg
		require.NoError(t, conn.ReadJSON(&msg))
		if msg.State != nil {
			return msg.State
		}
	}
}

func assertState(t *testing.T, conn *websocket.Conn, check func(*view)) {
	t.Helper()
	check(readState(t, conn))
}
