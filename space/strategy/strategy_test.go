package strategy_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/d-led/eventfulranges/space"
	"github.com/d-led/eventfulranges/space/op"
	"github.com/d-led/eventfulranges/space/strategy"
)

func TestStrategyString(t *testing.T) {
	t.Parallel()
	require.Equal(t, "lww", strategy.LWW.String())
	require.Equal(t, "fww", strategy.FWW.String())
	require.Equal(t, "additive-wins", strategy.AdditiveWins.String())
	require.Equal(t, "grow-only", strategy.GrowOnly.String())
	require.Equal(t, "additive-wins-lww", strategy.AdditiveWinsLWW.String())
	require.Equal(t, "unknown(7)", strategy.Strategy(7).String())
}

func TestStrategyText(t *testing.T) {
	t.Parallel()
	t.Run("marshal", func(t *testing.T) {
		t.Parallel()
		for _, s := range []strategy.Strategy{strategy.LWW, strategy.FWW, strategy.AdditiveWins, strategy.GrowOnly, strategy.AdditiveWinsLWW} {
			data, err := s.MarshalText()
			require.NoError(t, err)
			require.Equal(t, s.String(), string(data))
		}
		_, err := strategy.Strategy(7).MarshalText()
		require.ErrorIs(t, err, strategy.ErrUnknownStrategy)
	})
	t.Run("unmarshal", func(t *testing.T) {
		t.Parallel()
		for _, s := range []strategy.Strategy{strategy.LWW, strategy.FWW, strategy.AdditiveWins, strategy.GrowOnly, strategy.AdditiveWinsLWW} {
			var got strategy.Strategy
			require.NoError(t, got.UnmarshalText([]byte(s.String())))
			require.Equal(t, s, got)
		}
		var got strategy.Strategy
		require.ErrorIs(t, got.UnmarshalText([]byte("paint")), strategy.ErrUnknownStrategy)
	})
}

func TestParse(t *testing.T) {
	t.Parallel()
	for _, s := range []strategy.Strategy{strategy.LWW, strategy.FWW, strategy.AdditiveWins, strategy.GrowOnly, strategy.AdditiveWinsLWW} {
		got, err := strategy.Parse(s.String())
		require.NoError(t, err)
		require.Equal(t, s, got)
	}
	_, err := strategy.Parse("paint")
	require.ErrorIs(t, err, strategy.ErrUnknownStrategy)
}

func TestSegmentsDefinedForPriorityStrategiesOnly(t *testing.T) {
	t.Parallel()
	o := op.Add([]float64{0, 0}, []float64{2, 2})
	require.NotNil(t, strategy.Segments(strategy.LWW, []op.Op{o}))
	require.NotNil(t, strategy.Segments(strategy.FWW, []op.Op{o}))
	require.Nil(t, strategy.Segments(strategy.AdditiveWins, []op.Op{o}))
	require.Nil(t, strategy.Segments(strategy.GrowOnly, []op.Op{o}))
}

func TestCombineSegmentsMerges(t *testing.T) {
	t.Parallel()
	a := strategy.Segments(strategy.LWW, []op.Op{{ID: "a", Kind: op.KindAdd, TS: 2, Box: box(0, 0, 2, 2)}})
	b := strategy.Segments(strategy.LWW, []op.Op{{ID: "b", Kind: op.KindAdd, TS: 1, Box: box(4, 0, 6, 2)}})
	combined := strategy.CombineSegments(strategy.LWW, a, b)
	require.Len(t, combined, 2)
}

func TestMaterializeDisjointAddsUnderLWW(t *testing.T) {
	t.Parallel()
	first := box(0, 0, 2, 2)
	second := box(4, 0, 6, 2)
	ops := []op.Op{
		{ID: "a", Kind: op.KindAdd, TS: 1, Box: first},
		{ID: "b", Kind: op.KindAdd, TS: 2, Box: second},
	}
	require.Equal(t, []space.Box{first, second}, strategy.Materialize(strategy.LWW, ops))
}

func TestMaterializeLWWRemoveCarvesHole(t *testing.T) {
	t.Parallel()
	ops := []op.Op{
		{ID: "a", Kind: op.KindAdd, TS: 1, Box: box(0, 0, 4, 4)},
		{ID: "b", Kind: op.KindRemove, TS: 2, Box: box(1, 1, 3, 3)},
	}
	got := strategy.Materialize(strategy.LWW, ops)
	require.False(t, space.Contains(got, []float64{2, 2}), "the hole must be empty")
	require.True(t, space.Contains(got, []float64{0.5, 2}), "the shell around the hole must remain")
}

func TestMaterializeFWWEarlierWins(t *testing.T) {
	t.Parallel()
	ops := []op.Op{
		{ID: "a", Kind: op.KindAdd, TS: 2, Box: box(0, 0, 2, 2)},
		{ID: "b", Kind: op.KindRemove, TS: 1, Box: box(0, 0, 2, 2)},
	}
	got := strategy.Materialize(strategy.FWW, ops)
	require.False(t, space.Contains(got, []float64{1, 1}), "the earlier remove wins over the later add")
}

func TestMaterializeAdditiveWins(t *testing.T) {
	t.Parallel()
	ops := []op.Op{
		{ID: "a", Kind: op.KindAdd, TS: 1, Box: box(0, 0, 4, 4)},
		{ID: "b", Kind: op.KindRemove, TS: 2, Box: box(1, 1, 3, 3)},
	}
	got := strategy.Materialize(strategy.AdditiveWins, ops)
	require.False(t, space.Contains(got, []float64{2, 2}))
	require.True(t, space.Contains(got, []float64{0.5, 2}))
}

func TestMaterializeAdditiveWinsLWWResolvesColorPerPoint(t *testing.T) {
	t.Parallel()
	red := box(0, 0, 4, 4).WithMeta(json.RawMessage(`{"color":"#ff0000"}`))
	blue := box(2, 2, 6, 6).WithMeta(json.RawMessage(`{"color":"#0000ff"}`))
	ops := []op.Op{
		{ID: "a", Kind: op.KindAdd, TS: 1, Box: red},
		{ID: "b", Kind: op.KindAdd, TS: 2, Box: blue},
		{ID: "c", Kind: op.KindRemove, TS: 3, Box: box(5, 0, 6, 6)},
	}
	got := strategy.Materialize(strategy.AdditiveWinsLWW, ops)

	metaAt := func(p []float64) string {
		for _, b := range got {
			if b.Contains(p) {
				return string(b.Meta)
			}
		}
		return ""
	}
	require.JSONEq(t, `{"color":"#0000ff"}`, metaAt([]float64{3, 3}), "the later stroke wins the overlap")
	require.JSONEq(t, `{"color":"#ff0000"}`, metaAt([]float64{1, 1}), "red survives where blue never reached")
	require.False(t, space.Contains(got, []float64{5.5, 3}), "removals still carve the union")
}

func TestMaterializeGrowOnlyIgnoresRemoves(t *testing.T) {
	t.Parallel()
	ops := []op.Op{
		{ID: "a", Kind: op.KindAdd, TS: 1, Box: box(0, 0, 2, 2)},
		{ID: "b", Kind: op.KindRemove, TS: 2, Box: box(0, 0, 2, 2)},
	}
	got := strategy.Materialize(strategy.GrowOnly, ops)
	require.True(t, space.Contains(got, []float64{1, 1}), "removals are ignored")
}

func TestMaterializeUnknownStrategyIsNil(t *testing.T) {
	t.Parallel()
	require.Nil(t, strategy.Materialize(strategy.Strategy(7), nil))
}
