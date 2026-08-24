package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunConverges(t *testing.T) {
	t.Parallel()
	// Port 0 asks the kernel for free ephemeral ports.
	require.NoError(t, run([]int{0, 0}))
}
