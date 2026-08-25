//go:build !kurrent

package kurrent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStubFailsEveryOperation(t *testing.T) {
	t.Parallel()

	s, err := Open("esdb://localhost:2113?tls=false")
	require.Nil(t, s)
	require.Error(t, err)

	stub := &Store{}
	require.Error(t, stub.Append(context.Background(), 0, nil))

	_, _, err = stub.Read(context.Background(), 0)
	require.Error(t, err)

	require.Error(t, stub.SaveSnapshot(context.Background(), nil, 0))

	_, _, err = stub.LoadSnapshot(context.Background())
	require.Error(t, err)

	require.NoError(t, stub.Close())
}
