package collab

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

// counter is a minimal Model for exercising the shared plumbing.
type counter struct {
	mu  sync.Mutex
	n   int
	log []Entry
}

func (m *counter) Apply(clientID string, cmd Cmd) ([]Entry, error) {
	var p struct {
		Delta int `json:"delta"`
	}
	if err := json.Unmarshal(cmd.Data, &p); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.n += p.Delta
	entry := Entry{ID: "e", Client: clientID, Kind: cmd.Kind, Data: cmd.Data, At: time.Now()}
	m.log = append(m.log, entry)
	return []Entry{entry}, nil
}

func (m *counter) Replay(entries []Entry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.n = 0
	m.log = m.log[:0]
	for _, e := range entries {
		var p struct {
			Delta int `json:"delta"`
		}
		if err := json.Unmarshal(e.Data, &p); err != nil {
			return err
		}
		m.n += p.Delta
		m.log = append(m.log, e)
	}
	return nil
}

func (m *counter) Log() []Entry {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]Entry(nil), m.log...)
}

func (m *counter) Snapshot() any {
	m.mu.Lock()
	defer m.mu.Unlock()
	return map[string]int{"n": m.n}
}

// logOnly is a Model that exposes only its log, with no snapshot.
type logOnly struct {
	mu  sync.Mutex
	log []Entry
}

func (m *logOnly) Apply(clientID string, cmd Cmd) ([]Entry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry := Entry{ID: "e", Client: clientID, Kind: cmd.Kind, Data: cmd.Data, At: time.Now()}
	m.log = append(m.log, entry)
	return []Entry{entry}, nil
}

func (m *logOnly) Replay(entries []Entry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.log = append([]Entry(nil), entries...)
	return nil
}

func (m *logOnly) Log() []Entry {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]Entry(nil), m.log...)
}

func (m *logOnly) Snapshot() any {
	return nil
}

func TestSessionAppliesAndBroadcasts(t *testing.T) {
	t.Parallel()
	reg := NewSessions(time.Hour, func() Model { return &counter{} })
	sess := reg.Model("x")
	updates := sess.Subscribe()
	defer sess.Unsubscribe(updates)

	require.NoError(t, sess.Apply("alice", Cmd{Kind: "inc", Data: json.RawMessage(`{"delta":2}`)}))

	msg := <-updates
	require.Equal(t, TypeOp, msg.Type)
	require.NotNil(t, msg.Op)
	require.Equal(t, "alice", msg.Op.Client)
	require.Equal(t, "inc", msg.Op.Kind)
	require.JSONEq(t, `{"delta":2}`, string(msg.Op.Data))
	require.JSONEq(t, `{"n":2}`, string(msg.State))
}

func TestSessionWithoutSnapshotSendsNoState(t *testing.T) {
	t.Parallel()
	reg := NewSessions(time.Hour, func() Model { return &logOnly{} })
	sess := reg.Model("x")
	updates := sess.Subscribe()
	defer sess.Unsubscribe(updates)

	require.NoError(t, sess.Apply("alice", Cmd{Kind: "note", Data: json.RawMessage(`{}`)}))

	msg := <-updates
	require.Equal(t, TypeOp, msg.Type)
	require.Nil(t, msg.State)
}

func TestSessionTracksPresenceAndLog(t *testing.T) {
	t.Parallel()
	reg := NewSessions(time.Hour, func() Model { return &counter{} })
	sess := reg.Model("x")

	log, clients := sess.Join("")
	require.Empty(t, log)
	require.Equal(t, 1, clients)
	require.Equal(t, 1, reg.Total())

	require.NoError(t, sess.Apply("alice", Cmd{Kind: "inc", Data: json.RawMessage(`{"delta":1}`)}))

	log, clients = sess.Join("")
	require.Len(t, log, 1)
	require.Equal(t, 2, clients)
	require.Equal(t, 2, reg.Total())

	sess.Leave("")
	require.Equal(t, 1, reg.Total())
}

func TestPresenceSeparatesSessionAndTotal(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(NewRouter(NewSessions(time.Hour, func() Model { return &counter{} }), http.Dir(t.TempDir()), nil))
	defer srv.Close()

	alice := dialWS(t, "ws"+strings.TrimPrefix(srv.URL, "http")+"/ws?s=a")
	defer func() { _ = alice.Close() }()
	bob := dialWS(t, "ws"+strings.TrimPrefix(srv.URL, "http")+"/ws?s=b")
	defer func() { _ = bob.Close() }()

	assertPresence(t, alice, 1, 2)
	assertPresence(t, bob, 1, 2)
}

func TestRouterMintsSessionOnBareUIPath(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(NewRouter(NewSessions(time.Hour, func() Model { return &counter{} }), http.Dir(t.TempDir()), nil))
	defer srv.Close()

	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Get(srv.URL + "/ui/")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusFound, resp.StatusCode)
	loc, err := resp.Location()
	require.NoError(t, err)
	require.Equal(t, "/ui/", loc.Path)
	require.NotEmpty(t, loc.Query().Get("s"))
}

func TestRouterPreservesQueryParams(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(NewRouter(NewSessions(time.Hour, func() Model { return &counter{} }), http.Dir(t.TempDir()), []string{"dims"}))
	defer srv.Close()

	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Get(srv.URL + "/ui/?dims=4")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	loc, err := resp.Location()
	require.NoError(t, err)
	require.Equal(t, "4", loc.Query().Get("dims"))
	require.NotEmpty(t, loc.Query().Get("s"))
}

func dialWS(t *testing.T, url string) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	require.NoError(t, err)
	return conn
}

// assertPresence reads until the session's viewer count and the global total
// have both been observed at the expected values. Session and global presence
// arrive as separate messages, so each field may come from a different one.
func assertPresence(t *testing.T, conn *websocket.Conn, wantClients, wantTotal int) {
	t.Helper()
	var clients, total int
	for clients != wantClients || total != wantTotal {
		var msg Message
		require.NoError(t, conn.ReadJSON(&msg))
		if msg.Type != TypePresence {
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

func TestPersistentSessionsReplayLogAfterRestart(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	first := NewPersistentSessions(time.Hour, dir, func() Model { return &counter{} })
	require.NoError(t, first.Model("s1").Apply("alice", Cmd{Kind: "inc", Data: json.RawMessage(`{"delta":2}`)}))

	// A brand-new registry over the same directory is a "restart": the model
	// is recreated from the persisted log, not started empty.
	second := NewPersistentSessions(time.Hour, dir, func() Model { return &counter{} })
	restored := second.Model("s1")

	log, clients := restored.Join("")
	require.Equal(t, 1, clients)
	require.Len(t, log, 1)
	require.Equal(t, "alice", log[0].Client)
	require.Equal(t, "inc", log[0].Kind)
	require.JSONEq(t, `{"n":2}`, string(restored.Snapshot()))
}
