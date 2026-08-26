package space

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWithMetaCopiesAndDoesNotAlias(t *testing.T) {
	t.Parallel()
	b := NewBox([]float64{0, 0}, []float64{1, 1})
	m := json.RawMessage(`{"color":"#ff0000"}`)

	got := b.WithMeta(m)
	require.JSONEq(t, `{"color":"#ff0000"}`, string(got.Meta))
	require.Nil(t, b.Meta, "WithMeta must not mutate the receiver")

	// Mutating the input must not leak into the copy.
	m[0] = '{'
	require.JSONEq(t, `{"color":"#ff0000"}`, string(got.Meta))
}
