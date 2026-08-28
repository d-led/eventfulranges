package main

import (
	"testing"

	"github.com/automerge/automerge-go"
	"github.com/stretchr/testify/require"
)

func TestSyncConverges(t *testing.T) {
	t.Parallel()

	a := automerge.New()
	require.NoError(t, a.RootMap().Set("title", "shopping list"))
	_, err := a.Commit("init")
	require.NoError(t, err)

	b, err := a.Fork()
	require.NoError(t, err)

	require.NoError(t, a.RootMap().Set("alice", "apples"))
	_, err = a.Commit("alice adds apples")
	require.NoError(t, err)

	require.NoError(t, b.RootMap().Set("bob", "bananas"))
	_, err = b.Commit("bob adds bananas")
	require.NoError(t, err)

	syncPair(a, b)

	require.True(t, equalDocs(a, b), "replicas must converge after sync")
	require.Equal(t, map[string]string{
		"title": "shopping list",
		"alice": "apples",
		"bob":   "bananas",
	}, docEntries(a))
}
