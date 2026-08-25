package op_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/d-led/eventfulranges/interval"
	"github.com/d-led/eventfulranges/op"
)

func TestKindString(t *testing.T) {
	t.Parallel()
	require.Equal(t, "add", op.KindAdd.String())
	require.Equal(t, "remove", op.KindRemove.String())
}

func TestKindText(t *testing.T) {
	t.Parallel()
	t.Run("marshal", func(t *testing.T) {
		add, err := op.KindAdd.MarshalText()
		require.NoError(t, err)
		require.Equal(t, "add", string(add))
		remove, err := op.KindRemove.MarshalText()
		require.NoError(t, err)
		require.Equal(t, "remove", string(remove))
	})
	t.Run("unmarshal", func(t *testing.T) {
		var k op.Kind
		require.NoError(t, k.UnmarshalText([]byte("add")))
		require.Equal(t, op.KindAdd, k)
		require.NoError(t, k.UnmarshalText([]byte("remove")))
		require.Equal(t, op.KindRemove, k)
		require.Error(t, k.UnmarshalText([]byte("paint")))
	})
}

func TestValidate(t *testing.T) {
	t.Parallel()
	closed := interval.Interval{Start: 1, End: 5}
	tests := []struct {
		name string
		o    op.Op
		err  error
	}{
		{name: "valid add", o: op.Op{ID: "a", Kind: op.KindAdd, Interval: closed}},
		{name: "valid remove", o: op.Op{ID: "a", Kind: op.KindRemove, Interval: closed}},
		{name: "missing id", o: op.Op{Kind: op.KindAdd, Interval: closed}, err: op.ErrMissingID},
		{name: "invalid interval", o: op.Op{ID: "a", Kind: op.KindAdd, Interval: interval.Interval{Start: 5, End: 1}}, err: interval.ErrInverted},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.err != nil {
				require.ErrorIs(t, tt.o.Validate(), tt.err)
			} else {
				require.NoError(t, tt.o.Validate())
			}
		})
	}
	t.Run("invalid kind error message", func(t *testing.T) {
		require.Error(t, op.Op{ID: "a", Kind: op.Kind(7), Interval: closed}.Validate())
	})
}

func TestConstructors(t *testing.T) {
	t.Parallel()
	t.Run("add", func(t *testing.T) {
		o := op.Add(1, 5)
		require.Equal(t, op.KindAdd, o.Kind)
		require.NotEmpty(t, o.ID)
		require.Equal(t, interval.Interval{Start: 1, End: 5}, o.Interval)
		require.NoError(t, o.Validate())
	})
	t.Run("add with bounds", func(t *testing.T) {
		o := op.AddWithBounds(1, 5, interval.Open, interval.Closed)
		require.Equal(t, interval.Open, o.Interval.StartBound)
		require.Equal(t, interval.Closed, o.Interval.EndBound)
	})
	t.Run("remove", func(t *testing.T) {
		o := op.Remove(2, 3)
		require.Equal(t, op.KindRemove, o.Kind)
		require.Equal(t, interval.Interval{Start: 2, End: 3}, o.Interval)
	})
	t.Run("remove with bounds", func(t *testing.T) {
		o := op.RemoveWithBounds(2, 3, interval.Closed, interval.Open)
		require.Equal(t, interval.Open, o.Interval.EndBound)
	})
	t.Run("ids are unique", func(t *testing.T) {
		require.NotEqual(t, op.Add(1, 2).ID, op.Add(1, 2).ID)
	})
}

func TestOpJSON(t *testing.T) {
	t.Parallel()
	o := op.Op{
		ID:       "x",
		Kind:     op.KindAdd,
		Interval: interval.Interval{Start: 1, End: 5, EndBound: interval.Open},
		TS:       7,
	}
	data, err := json.Marshal(o)
	require.NoError(t, err)
	require.JSONEq(t, `{"id":"x","kind":"add","interval":{"start":1,"end":5,"startBound":"closed","endBound":"open"},"ts":7}`, string(data))

	var back op.Op
	require.NoError(t, json.Unmarshal(data, &back))
	require.Equal(t, o, back)

	var bad op.Op
	require.NoError(t, json.Unmarshal([]byte(`{"id":"x","kind":"add","interval":{"start":5,"end":1,"startBound":"closed","endBound":"closed"},"ts":0}`), &bad), "unmarshal must not validate")
	require.Equal(t, "x", bad.ID)
}
