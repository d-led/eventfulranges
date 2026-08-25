//go:build kurrent

package kurrent

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/EventStore/EventStore-Client-Go/v4/esdb"
	"github.com/stretchr/testify/require"

	"gitub.com/d-led/eventfulranges/interval"
	"gitub.com/d-led/eventfulranges/op"
)

// TestTranslateError pins the error-code mapping. The wrong-version and
// stream-not-found branches are exercised against a real server in
// integration_test.go; here we cover the nil and pass-through paths.
func TestTranslateError(t *testing.T) {
	t.Parallel()
	require.Nil(t, translateError(nil))

	boom := errors.New("boom")
	require.Equal(t, boom, translateError(boom))
}

func TestExpectedRevision(t *testing.T) {
	t.Parallel()
	require.IsType(t, esdb.NoStream{}, expectedRevision(0))
	require.Equal(t, esdb.Revision(2), expectedRevision(3))
}

// TestOpenInvalidConnectionString breaks the connection string so Open fails
// during parsing or validation, before any network activity.
func TestOpenInvalidConnectionString(t *testing.T) {
	t.Parallel()
	// An unsupported scheme fails during parsing.
	_, err := Open("http://localhost:2113")
	require.Error(t, err)

	// A lone user certificate without its key fails client validation.
	_, err = Open("esdb://localhost:2113?usercertfile=/tmp/cert.pem")
	require.Error(t, err)
}

// TestAppendMarshalError feeds an operation whose NaN endpoint JSON cannot
// encode, so the marshal error surfaces before any network call.
func TestAppendMarshalError(t *testing.T) {
	t.Parallel()
	s := &Store{}
	err := s.Append(context.Background(), 0, []op.Op{{
		ID:       "nan",
		Kind:     op.KindAdd,
		Interval: interval.Interval{Start: math.NaN(), End: 1},
	}})
	require.Error(t, err)
}
