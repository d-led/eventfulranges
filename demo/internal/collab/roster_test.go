package collab

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAnonymizeEmail(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want string
	}{
		{"", "anonymous"},
		{"dmitry@gmail.com", "dmi…@gmai…"},
		{"alice@example.com", "ali…@examp…"},
		{"bob@corp.io", "bob@cor…"},
		{"jane@x.co", "jan…@x.…"},
		{"a@bc.de", "a@bc…"},
		{"no-at-sign", "no-…@…"},
	}
	for _, c := range cases {
		require.Equal(t, c.want, anonymizeEmail(c.in), "anonymizeEmail(%q)", c.in)
	}
}

func TestRosterTracksUsersAndSessions(t *testing.T) {
	t.Parallel()
	reg := NewSessions(time.Hour, func() Model { return &counter{} })
	presence := reg.SubscribePresence()
	defer reg.UnsubscribePresence(presence)

	sessA := reg.Model("s1")
	sessB := reg.Model("s2")

	_, _ = sessA.Join("alice@example.com")
	require.Equal(t, []RosterEntry{{User: "ali…@examp…", Sessions: []string{"s1"}}}, rosterOf(t, presence))

	_, _ = sessB.Join("alice@example.com")
	require.Equal(t, []RosterEntry{{User: "ali…@examp…", Sessions: []string{"s1", "s2"}}}, rosterOf(t, presence))

	_, _ = reg.Model("s3").Join("bob@corp.io")
	require.Equal(t, []RosterEntry{
		{User: "ali…@examp…", Sessions: []string{"s1", "s2"}},
		{User: "bob@cor…", Sessions: []string{"s3"}},
	}, rosterOf(t, presence))

	sessA.Leave("alice@example.com")
	require.Equal(t, []RosterEntry{
		{User: "ali…@examp…", Sessions: []string{"s2"}},
		{User: "bob@cor…", Sessions: []string{"s3"}},
	}, rosterOf(t, presence))

	sessB.Leave("alice@example.com")
	require.Equal(t, []RosterEntry{{User: "bob@cor…", Sessions: []string{"s3"}}}, rosterOf(t, presence))
}

// rosterOf drains the presence channel until it sees the next roster snapshot.
func rosterOf(t *testing.T, ch chan Message) []RosterEntry {
	t.Helper()
	for {
		select {
		case msg := <-ch:
			if msg.Type == TypeRoster {
				return msg.Roster
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for roster")
		}
	}
}
