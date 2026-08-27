package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseAdminList(t *testing.T) {
	t.Parallel()
	list := parseAdminList("a@example.com, B@example.com ,, ")

	require.True(t, list.IsAdmin("a@example.com"))
	require.True(t, list.IsAdmin("b@example.com"))
	require.True(t, list.IsAdmin("  A@Example.Com  "), "matching is case-insensitive")
	require.False(t, list.IsAdmin("c@example.com"))
	require.False(t, list.IsAdmin(""))
}

func TestParseAdminListEmpty(t *testing.T) {
	t.Parallel()
	list := parseAdminList("")

	require.False(t, list.IsAdmin("a@example.com"))
}
