package main

import (
	"testing"

	am "github.com/develerltd/go-automerge/automerge"
	"github.com/stretchr/testify/require"
)

func TestSyncConverges(t *testing.T) {
	t.Parallel()

	a := am.New()
	require.NoError(t, a.Put(am.Root, "title", am.NewStr("shopping list")))
	a.Commit("init", 1000)

	b := a.Fork()

	require.NoError(t, a.Put(am.Root, "alice", am.NewStr("apples")))
	a.Commit("alice adds apples", 2000)

	require.NoError(t, b.Put(am.Root, "bob", am.NewStr("bananas")))
	b.Commit("bob adds bananas", 2000)

	syncPair(a, b)

	require.True(t, equalDocs(a, b), "replicas must converge after sync")
	require.Equal(t, map[string]string{
		"title": "shopping list",
		"alice": "apples",
		"bob":   "bananas",
	}, docEntries(a))
}
