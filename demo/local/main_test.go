package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunConverges(t *testing.T) {
	t.Parallel()
	require.NoError(t, run())
}
