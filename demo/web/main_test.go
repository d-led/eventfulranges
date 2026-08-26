package main

import (
	"fmt"
	"io"
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
	h := newHub(false)

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
	first := newHub(false)
	second := newHub(false)

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
	h := newHub(false)
	_, err := h.apply(opAdd, []float64{0, 0}, []float64{4, 4})
	require.NoError(t, err)

	_, err = h.apply(opAdd, []float64{0, 0, 0}, []float64{4, 4, 4})
	require.Error(t, err, "dimension mismatch")

	_, err = h.apply(opAdd, []float64{math.NaN(), 0}, []float64{1, 1})
	require.Error(t, err, "NaN coordinate")

	_, err = h.apply(opKind("paint"), []float64{0, 0}, []float64{1, 1})
	require.Error(t, err, "unknown operation")
}

func TestViewDimensionIsFixedByTheFirstBox(t *testing.T) {
	t.Parallel()
	h := newHub(false)
	_, err := h.apply(opAdd, []float64{0}, []float64{1})
	require.NoError(t, err)

	// A box of a different dimension cannot join a fixed session.
	_, err = h.apply(opAdd, []float64{0, 0, 0}, []float64{1, 1, 1})
	require.Error(t, err)
	require.Equal(t, 1, h.snapshot().Dims)
}

func TestDimsFixesTheSessionDimension(t *testing.T) {
	t.Parallel()
	h := newHub(false)

	v, err := h.applyDims(4)
	require.NoError(t, err)
	require.Equal(t, 4, v.Dims)
	require.Empty(t, v.Boxes)
	require.Equal(t, 4, h.snapshot().Dims)

	// An empty session may still choose another dimension.
	_, err = h.applyDims(2)
	require.NoError(t, err)
	require.Equal(t, 2, h.snapshot().Dims)
}

func TestDimsRejectsOutOfRange(t *testing.T) {
	t.Parallel()
	h := newHub(false)
	for _, d := range []int{0, -1, 5} {
		_, err := h.applyDims(d)
		require.Error(t, err, "dimension %d must be rejected", d)
	}
	require.Equal(t, -1, h.snapshot().Dims)
}

func TestDimsCannotChangeOnceBoxesExist(t *testing.T) {
	t.Parallel()
	h := newHub(false)
	_, err := h.apply(opAdd, []float64{0, 0, 0}, []float64{1, 1, 1})
	require.NoError(t, err)

	_, err = h.applyDims(4)
	require.Error(t, err)

	// Re-fixing the same dimension is a no-op.
	_, err = h.applyDims(3)
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
	h := newHub(false)

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

func TestPresenceSeparatesSessionAndTotal(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(newRouter(newSessions(time.Hour), GetFS()))
	defer srv.Close()

	alice := dial(t, "ws"+strings.TrimPrefix(srv.URL, "http")+"/ws?s=a")
	defer func() { _ = alice.Close() }()
	bob := dial(t, "ws"+strings.TrimPrefix(srv.URL, "http")+"/ws?s=b")
	defer func() { _ = bob.Close() }()

	// Each session reports one viewer, but two clients are connected overall.
	assertPresence(t, alice, 1, 2)
	assertPresence(t, bob, 1, 2)
}

// assertPresence reads until the session's viewer count and the global total
// have both been observed at the expected values. Session and global presence
// arrive as separate messages, so each field may come from a different one.
func assertPresence(t *testing.T, conn *websocket.Conn, wantClients, wantTotal int) {
	t.Helper()
	var clients, total int
	for clients != wantClients || total != wantTotal {
		var msg serverMsg
		require.NoError(t, conn.ReadJSON(&msg))
		if msg.Type != msgPresence {
			continue
		}
		if msg.Clients != 0 {
			clients = msg.Clients
		}
		if msg.Total != 0 {
			total = msg.Total
		}
	}
}

func TestHubLogsDimensionChanges(t *testing.T) {
	t.Parallel()
	h := newHub(false)

	_, err := h.setDims("alice", 4)
	require.NoError(t, err)
	_, err = h.record("bob", opAdd, []float64{0, 0, 0, 0}, []float64{1, 1, 1, 1})
	require.NoError(t, err)

	log, _ := h.join()
	require.Equal(t, []string{"dims", "add"}, []string{log[0].Kind, log[1].Kind})
	require.Equal(t, 4, log[0].Dims)
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

	alpha := s.model("alpha", false)
	beta := s.model("beta", false)

	_, err := alpha.apply(opAdd, []float64{0}, []float64{1})
	require.NoError(t, err)

	require.Len(t, alpha.snapshot().Boxes, 1)
	require.Empty(t, beta.snapshot().Boxes, "another session must not observe this edit")
}

func TestSessionExpires(t *testing.T) {
	t.Parallel()
	s := newSessions(10 * time.Millisecond)

	first := s.model("id", false)
	require.Same(t, first, s.model("id", false), "an active session is reused")

	time.Sleep(30 * time.Millisecond)
	require.NotSame(t, first, s.model("id", false), "an expired session is recreated fresh")
}

func TestPersistentSessionsReplayAfterRestart(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	first := newPersistentSessions(time.Hour, dir)
	_, err := first.model("id", false).record("alice", opAdd, []float64{0, 0}, []float64{4, 4})
	require.NoError(t, err)

	// A brand-new registry over the same directory is a "restart": the view is
	// recreated from the persisted log, not started empty.
	second := newPersistentSessions(time.Hour, dir)
	restored := second.model("id", false)

	require.Len(t, restored.ops, 1)
	require.Equal(t, "add", restored.ops[0].Kind)
	require.Len(t, restored.snapshot().Boxes, 1)
}

func TestNewSessionIDIsUnique(t *testing.T) {
	t.Parallel()
	first := newSessionID()
	second := newSessionID()
	require.NotEmpty(t, first)
	require.NotEqual(t, first, second)
}

func TestCompactionMergesAdjacentBoxes(t *testing.T) {
	t.Parallel()
	h := newHub(true)
	_, err := h.apply(opAdd, []float64{0, 0}, []float64{2, 4})
	require.NoError(t, err)
	_, err = h.apply(opAdd, []float64{2, 0}, []float64{4, 4})
	require.NoError(t, err)
	require.Len(t, h.snapshot().Boxes, 1, "adjacent boxes merge into one under compaction")
}

func TestNoCompactionKeepsCanonicalCover(t *testing.T) {
	t.Parallel()
	h := newHub(false)
	_, err := h.apply(opAdd, []float64{0, 0}, []float64{2, 4})
	require.NoError(t, err)
	_, err = h.apply(opAdd, []float64{2, 0}, []float64{4, 4})
	require.NoError(t, err)
	require.Len(t, h.snapshot().Boxes, 2, "canonical cover keeps adjacent boxes separate")
}

// exampleOps mirrors exampleFor in demo/web/ui-src/app.js: the same hollow
// shell in every dimension — a hypercube tiled into unit cubes (six in 1D,
// three per axis otherwise) with an aligned central block carved out. Keeping
// the two in sync is what makes the visualizer's "Load example" show the
// compaction difference this test pins down.
func exampleOps(dims int) []clientOp {
	side := 3
	carveLo, carveHi := 1.0, 2.0
	if dims == 1 {
		side = 6
		carveLo, carveHi = 2.0, 4.0
	}
	ops := make([]clientOp, 0, ipow(side, dims)+1)
	for code := 0; code < ipow(side, dims); code++ {
		min := make([]float64, dims)
		max := make([]float64, dims)
		c := code
		for d := 0; d < dims; d++ {
			v := c % side
			c /= side
			min[d] = float64(v)
			max[d] = float64(v + 1)
		}
		ops = append(ops, clientOp{Kind: "add", Min: min, Max: max})
	}
	lo := make([]float64, dims)
	hi := make([]float64, dims)
	for d := range lo {
		lo[d] = carveLo
		hi[d] = carveHi
	}
	return append(ops, clientOp{Kind: "remove", Min: lo, Max: hi})
}

// ipow raises base to a non-negative integer power.
func ipow(base, exp int) int {
	n := 1
	for i := 0; i < exp; i++ {
		n *= base
	}
	return n
}

// TestExamplesDifferByCompaction pins the visualizer's built-in example in
// every dimension: the same operations must cover the same points under both
// compaction modes, while canonical keeps each tile and merge adjacent
// collapses every side into a single box.
func TestExamplesDifferByCompaction(t *testing.T) {
	t.Parallel()
	for dims := 1; dims <= 4; dims++ {
		t.Run(fmt.Sprintf("%dD", dims), func(t *testing.T) {
			t.Parallel()
			canonical := newHub(false)
			merged := newHub(true)
			for _, op := range exampleOps(dims) {
				_, err := canonical.apply(opKind(op.Kind), op.Min, op.Max)
				require.NoError(t, err)
				_, err = merged.apply(opKind(op.Kind), op.Min, op.Max)
				require.NoError(t, err)
			}

			c := canonical.snapshot()
			m := merged.snapshot()
			require.Equal(t, dims, c.Dims)

			// Canonical keeps every surviving tile; merge adjacent collapses
			// touching tiles, so the same cover is represented by fewer boxes.
			require.Len(t, c.Boxes, canonicalTileCount(dims))
			require.Less(t, len(m.Boxes), len(c.Boxes),
				"merge adjacent must collapse some of the %d tiles", len(c.Boxes))

			// Both representations cover exactly the same points.
			requireCoverageEqual(t, c.Boxes, m.Boxes, dims)
		})
	}
}

// canonicalTileCount is the number of surviving unit tiles: the whole tiling
// minus the carved block. 1D tiles six unit segments and carves two of them.
func canonicalTileCount(dims int) int {
	if dims == 1 {
		return 4
	}
	return ipow(3, dims) - 1
}

// requireCoverageEqual samples the covered region on a half-unit grid and
// asserts both covers agree on every point. The canonicalizer is
// cover-preserving, so this is the property the examples must not break.
func requireCoverageEqual(t *testing.T, a, b []space.Box, dims int) {
	t.Helper()
	side := 3.0
	if dims == 1 {
		side = 6.0
	}
	var visit func([]float64, int)
	visit = func(prefix []float64, d int) {
		if d == dims {
			p := append([]float64(nil), prefix...)
			require.Equal(t, space.Contains(a, p), space.Contains(b, p),
				"point %v must be covered identically", p)
			return
		}
		for v := 0.0; v <= side; v += 0.5 {
			visit(append(prefix, v), d+1)
		}
	}
	visit(nil, 0)
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

func TestRouterPreservesChosenDimension(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(newRouter(newSessions(time.Hour), GetFS()))
	defer srv.Close()

	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Get(srv.URL + "/ui/?dims=4")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusFound, resp.StatusCode)
	require.Regexp(t, `\?s=[A-Z2-7]+&dims=4`, resp.Header.Get("Location"))
}

func TestUIURL(t *testing.T) {
	t.Parallel()
	require.Equal(t, "http://localhost:8080/ui/", uiURL(":8080"))
	require.Equal(t, "http://localhost:8080/ui/", uiURL("0.0.0.0:8080"))
	require.Equal(t, "http://127.0.0.1:18080/ui/", uiURL("127.0.0.1:18080"))
}

func TestPlaceholderServesBuildInstructions(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(newRouter(newSessions(time.Hour), placeholderFS()))
	defer srv.Close()

	res, err := http.Get(srv.URL + "/ui/?s=shared")
	require.NoError(t, err)
	defer func() { _ = res.Body.Close() }()
	require.Equal(t, http.StatusOK, res.StatusCode)

	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "hasn't been built")
}

func TestGetFSAlwaysServesAnIndexPage(t *testing.T) {
	t.Parallel()
	f, err := GetFS().Open("index.html")
	require.NoError(t, err, "a fresh checkout without a built UI must still serve a page")
	defer func() { _ = f.Close() }()
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

func assertError(t *testing.T, conn *websocket.Conn, check func(string)) {
	t.Helper()
	for {
		var msg serverMsg
		require.NoError(t, conn.ReadJSON(&msg))
		if msg.Type == "error" {
			check(msg.Error)
			return
		}
	}
}

func TestWebSocketBroadcastsDimensionToEveryClient(t *testing.T) {
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

	// Alice fixes the dimension; both clients observe it, boxes or no boxes.
	require.NoError(t, alice.WriteJSON(clientOp{Kind: "dims", Dims: 4}))

	assertState(t, alice, func(s *view) { require.Equal(t, 4, s.Dims) })
	assertState(t, bob, func(s *view) { require.Equal(t, 4, s.Dims) })
}

func TestWebSocketRejectsIllegalCommands(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(newRouter(newSessions(time.Hour), GetFS()))
	defer srv.Close()

	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws?s=shared"
	alice := dial(t, url)
	defer func() { _ = alice.Close() }()

	assertState(t, alice, func(s *view) { require.Empty(t, s.Boxes) })

	require.NoError(t, alice.WriteJSON(clientOp{Kind: "dims", Dims: 9}))
	assertError(t, alice, func(msg string) { require.Contains(t, msg, "out of range") })
}
