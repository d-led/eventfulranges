package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/d-led/eventfulranges"
	"github.com/d-led/eventfulranges/store/memory"
	"github.com/d-led/eventfulranges/strategy"
)

func TestHelloResult(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	set, err := eventfulranges.OpenStore(ctx, memory.New(), strategy.LWW)
	require.NoError(t, err)

	_, err = set.Add(ctx, 1, 10)
	require.NoError(t, err)
	_, err = set.Remove(ctx, 3, 5)
	require.NoError(t, err)

	require.Len(t, set.Ranges(), 2, "[1,10] minus [3,5] leaves two pieces")
	require.True(t, set.Contains(2), "2 is inside [1,3)")
	require.False(t, set.Contains(4), "4 is inside the removed gap")
}
