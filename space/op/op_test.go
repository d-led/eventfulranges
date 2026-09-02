package op_test

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/d-led/eventfulranges/space"
	"github.com/d-led/eventfulranges/space/op"
)

func box2(x0, y0, x1, y1 float64) space.Box {
	return space.NewBox([]float64{x0, y0}, []float64{x1, y1})
}

func TestKindString(t *testing.T) {
	t.Parallel()
	require.Equal(t, "add", op.KindAdd.String())
	require.Equal(t, "remove", op.KindRemove.String())
	require.Equal(t, "retract", op.KindRetract.String())
	require.Equal(t, "kind(7)", op.Kind(7).String())
}

func TestKindText(t *testing.T) {
	t.Parallel()
	t.Run("marshal", func(t *testing.T) {
		t.Parallel()
		add, err := op.KindAdd.MarshalText()
		require.NoError(t, err)
		require.Equal(t, "add", string(add))
		remove, err := op.KindRemove.MarshalText()
		require.NoError(t, err)
		require.Equal(t, "remove", string(remove))
		retract, err := op.KindRetract.MarshalText()
		require.NoError(t, err)
		require.Equal(t, "retract", string(retract))
	})
	t.Run("unmarshal", func(t *testing.T) {
		t.Parallel()
		var k op.Kind
		require.NoError(t, k.UnmarshalText([]byte("add")))
		require.Equal(t, op.KindAdd, k)
		require.NoError(t, k.UnmarshalText([]byte("remove")))
		require.Equal(t, op.KindRemove, k)
		require.NoError(t, k.UnmarshalText([]byte("retract")))
		require.Equal(t, op.KindRetract, k)
		require.Error(t, k.UnmarshalText([]byte("paint")))
	})
}

func TestValidate(t *testing.T) {
	t.Parallel()
	valid := space.NewBox([]float64{1, 2}, []float64{5, 6})
	tests := []struct {
		name string
		o    op.Op
		err  error
	}{
		{name: "valid add", o: op.Op{ID: "a", Kind: op.KindAdd, Box: valid}},
		{name: "valid remove", o: op.Op{ID: "a", Kind: op.KindRemove, Box: valid}},
		{name: "valid retract", o: op.Op{ID: "a", Kind: op.KindRetract, Ref: "t", Box: valid}},
		{name: "retract missing ref", o: op.Op{ID: "a", Kind: op.KindRetract, Box: valid}, err: op.ErrMissingRef},
		{name: "missing id", o: op.Op{Kind: op.KindAdd, Box: valid}, err: op.ErrMissingID},
		{
			name: "invalid box",
			o:    op.Op{ID: "a", Kind: op.KindAdd, Box: space.NewBox([]float64{5}, []float64{1})},
			err:  space.ErrInverted,
		},
		{
			name: "empty box",
			o:    op.Op{ID: "a", Kind: op.KindAdd, Box: space.NewBox([]float64{1}, []float64{1})},
			err:  space.ErrEmpty,
		},
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
		t.Parallel()
		require.Error(t, op.Op{ID: "a", Kind: op.Kind(7), Box: valid}.Validate())
	})
	t.Run("NaN box rejected", func(t *testing.T) {
		t.Parallel()
		o := op.Op{ID: "a", Kind: op.KindAdd, Box: space.NewBox([]float64{math.NaN()}, []float64{1})}
		require.ErrorIs(t, o.Validate(), space.ErrNaN)
	})
}

func TestConstructors(t *testing.T) {
	t.Parallel()
	t.Run("add", func(t *testing.T) {
		t.Parallel()
		o := op.Add([]float64{1, 2}, []float64{5, 6})
		require.Equal(t, op.KindAdd, o.Kind)
		require.NotEmpty(t, o.ID)
		require.Equal(t, box2(1, 2, 5, 6), o.Box)
		require.NoError(t, o.Validate())
	})
	t.Run("remove", func(t *testing.T) {
		t.Parallel()
		o := op.Remove([]float64{2}, []float64{3})
		require.Equal(t, op.KindRemove, o.Kind)
		require.Equal(t, space.NewBox([]float64{2}, []float64{3}), o.Box)
	})
	t.Run("retract", func(t *testing.T) {
		t.Parallel()
		box := space.NewBox([]float64{2}, []float64{3})
		o := op.Retract("target", box)
		require.Equal(t, op.KindRetract, o.Kind)
		require.Equal(t, "target", o.Ref)
		require.Equal(t, box, o.Box)
		require.NotEmpty(t, o.ID)
		require.NoError(t, o.Validate())
	})
	t.Run("add with bounds", func(t *testing.T) {
		t.Parallel()
		o := op.AddBounds([]float64{1}, []float64{2}, []space.Bound{space.Open}, []space.Bound{space.Closed})
		require.Equal(t, op.KindAdd, o.Kind)
		require.False(t, o.Box.Contains([]float64{1}), "open min excludes the lower endpoint")
		require.True(t, o.Box.Contains([]float64{2}), "closed max includes the upper endpoint")
		require.NoError(t, o.Validate())
	})
	t.Run("remove with bounds", func(t *testing.T) {
		t.Parallel()
		o := op.RemoveBounds([]float64{1}, []float64{2}, []space.Bound{space.Open}, []space.Bound{space.Closed})
		require.Equal(t, op.KindRemove, o.Kind)
		require.False(t, o.Box.Contains([]float64{1}))
		require.True(t, o.Box.Contains([]float64{2}))
		require.NoError(t, o.Validate())
	})
	t.Run("constructors copy input slices", func(t *testing.T) {
		t.Parallel()
		lo := []float64{0, 0}
		hi := []float64{1, 1}
		o := op.Add(lo, hi)
		lo[0] = 100
		require.Equal(t, float64(0), o.Box.Min[0])
	})
	t.Run("ids are unique", func(t *testing.T) {
		t.Parallel()
		require.NotEqual(t, op.Add([]float64{1}, []float64{2}).ID, op.Add([]float64{1}, []float64{2}).ID)
	})
}

func TestOpJSON(t *testing.T) {
	t.Parallel()
	o := op.Op{
		ID:   "x",
		Kind: op.KindAdd,
		Box:  space.NewBox([]float64{1, 2}, []float64{5, 6}),
		TS:   7,
	}
	data, err := json.Marshal(o)
	require.NoError(t, err)
	require.JSONEq(t, `{"id":"x","kind":"add","box":{"min":[1,2],"max":[5,6]},"ts":7}`, string(data))

	var back op.Op
	require.NoError(t, json.Unmarshal(data, &back))
	require.Equal(t, o, back)

	var bad op.Op
	require.NoError(t, json.Unmarshal([]byte(`{"id":"x","kind":"add","box":{"min":[5],"max":[1]},"ts":0}`), &bad), "unmarshal must not validate")
	require.Equal(t, "x", bad.ID)
}

func TestRetractJSON(t *testing.T) {
	t.Parallel()
	o := op.Retract("target", space.NewBox([]float64{1, 2}, []float64{5, 6}))
	data, err := json.Marshal(o)
	require.NoError(t, err)
	require.JSONEq(t, `{"id":"`+o.ID+`","kind":"retract","box":{"min":[1,2],"max":[5,6]},"ts":0,"ref":"target"}`, string(data))

	var back op.Op
	require.NoError(t, json.Unmarshal(data, &back))
	require.Equal(t, o, back)
	require.Equal(t, "target", back.Ref)
}
