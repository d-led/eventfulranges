package meta

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func raw(t *testing.T, s string) json.RawMessage {
	t.Helper()
	if s == "" {
		return nil
	}
	if !json.Valid([]byte(s)) {
		t.Fatalf("invalid test metadata %q", s)
	}
	return json.RawMessage(s)
}

func TestUnionKeepsKeysFromOneSideOnly(t *testing.T) {
	t.Parallel()
	got := Union(raw(t, `{"color":"#ff0000"}`), raw(t, `{"author":"alice"}`))
	require.JSONEq(t, `{"color":"#ff0000","author":"alice"}`, string(got))
}

func TestUnionCollapsesEqualValues(t *testing.T) {
	t.Parallel()
	got := Union(raw(t, `{"color":"#ff0000"}`), raw(t, `{"color":"#ff0000"}`))
	require.JSONEq(t, `{"color":"#ff0000"}`, string(got))
}

func TestUnionResolvesConflictingScalarsDeterministically(t *testing.T) {
	t.Parallel()
	a := raw(t, `{"color":"#ff0000"}`)
	b := raw(t, `{"color":"#0000ff"}`)
	ab := Union(a, b)
	ba := Union(b, a)
	require.Equal(t, string(ab), string(ba), "the join must not depend on argument order")
	require.JSONEq(t, `{"color":"#ff0000"}`, string(ab), "the lexicographically larger scalar wins")
}

func TestUnionRecursesIntoNestedObjects(t *testing.T) {
	t.Parallel()
	got := Union(
		raw(t, `{"style":{"weight":2,"dash":false}}`),
		raw(t, `{"style":{"color":"#ff0000"}}`),
	)
	require.JSONEq(t, `{"style":{"weight":2,"dash":false,"color":"#ff0000"}}`, string(got))
}

func TestUnionTreatsMissingOrNonObjectAsTheOtherSide(t *testing.T) {
	t.Parallel()
	obj := raw(t, `{"color":"#ff0000"}`)
	require.JSONEq(t, `{"color":"#ff0000"}`, string(Union(nil, obj)))
	require.JSONEq(t, `{"color":"#ff0000"}`, string(Union(obj, nil)))
	require.Nil(t, Union(nil, nil))
	// A scalar is not an object, so the object side survives untouched.
	require.JSONEq(t, `{"color":"#ff0000"}`, string(Union(raw(t, `"red"`), obj)))
	require.JSONEq(t, `{"color":"#ff0000"}`, string(Union(obj, raw(t, `"red"`))))
}

func TestUnionIsCommutativeAndIdempotent(t *testing.T) {
	t.Parallel()
	a := raw(t, `{"a":1,"nested":{"x":1}}`)
	b := raw(t, `{"b":2,"nested":{"y":2}}`)
	require.Equal(t, string(Union(a, b)), string(Union(b, a)))
	require.Equal(t, string(Union(a, b)), string(Union(Union(a, b), a)))
	require.Equal(t, string(Union(a, a)), string(a))
}
