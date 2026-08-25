package main

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"

	"gitub.com/d-led/eventfulranges/space"
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
	h := newHub()
	srv := httptest.NewServer(newRouter(h, GetFS()))
	defer srv.Close()

	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"

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

func TestRouterServesTheUI(t *testing.T) {
	t.Parallel()
	h := newHub()
	srv := httptest.NewServer(newRouter(h, GetFS()))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/ui/")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, resp.Header.Get("Content-Type"), "text/html")
}

func dial(t *testing.T, url string) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	require.NoError(t, err)
	return conn
}

func assertState(t *testing.T, conn *websocket.Conn, check func(*view)) {
	t.Helper()
	var msg serverMsg
	require.NoError(t, conn.ReadJSON(&msg))
	require.Equal(t, "state", msg.Type, string(mustJSON(t, msg)))
	require.NotNil(t, msg.State)
	check(msg.State)
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	return data
}
