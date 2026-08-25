package strategy_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/d-led/eventfulranges/interval"
	"github.com/d-led/eventfulranges/op"
	"github.com/d-led/eventfulranges/strategy"
)

func addOp(id string, ts int64, start, end int) op.Op {
	return op.Op{ID: id, Kind: op.KindAdd, TS: ts, Interval: iv(start, end, interval.Closed, interval.Closed)}
}

func removeOp(id string, ts int64, start, end int) op.Op {
	return op.Op{ID: id, Kind: op.KindRemove, TS: ts, Interval: iv(start, end, interval.Closed, interval.Closed)}
}

func TestStrategyString(t *testing.T) {
	t.Parallel()
	tests := map[strategy.Strategy]string{
		strategy.LWW:          "lww",
		strategy.FWW:          "fww",
		strategy.AdditiveWins: "additive-wins",
		strategy.GrowOnly:     "grow-only",
	}
	for s, want := range tests {
		require.Equal(t, want, s.String())
	}
	require.Equal(t, "unknown(99)", strategy.Strategy(99).String())
}

func TestParse(t *testing.T) {
	t.Parallel()
	tests := map[string]strategy.Strategy{
		"lww":           strategy.LWW,
		"fww":           strategy.FWW,
		"additive-wins": strategy.AdditiveWins,
		"grow-only":     strategy.GrowOnly,
	}
	for name, want := range tests {
		got, err := strategy.Parse(name)
		require.NoError(t, err)
		require.Equal(t, want, got)
	}
	_, err := strategy.Parse("bogus")
	require.ErrorIs(t, err, strategy.ErrUnknownStrategy)
}

func TestStrategyText(t *testing.T) {
	t.Parallel()
	t.Run("marshal", func(t *testing.T) {
		data, err := strategy.LWW.MarshalText()
		require.NoError(t, err)
		require.Equal(t, "lww", string(data))
		_, err = strategy.Strategy(99).MarshalText()
		require.ErrorIs(t, err, strategy.ErrUnknownStrategy)
	})
	t.Run("unmarshal", func(t *testing.T) {
		var s strategy.Strategy
		require.NoError(t, s.UnmarshalText([]byte("fww")))
		require.Equal(t, strategy.FWW, s)
		require.ErrorIs(t, s.UnmarshalText([]byte("bogus")), strategy.ErrUnknownStrategy)
	})
}

func TestMaterializeLWW(t *testing.T) {
	t.Parallel()
	t.Run("overlapping additions merge", func(t *testing.T) {
		ops := []op.Op{addOp("a", 1, 1, 5), addOp("b", 2, 3, 7)}
		require.Equal(t, []interval.Interval{iv(1, 7, interval.Closed, interval.Closed)}, strategy.Materialize(strategy.LWW, ops))
	})
	t.Run("subtract cuts a hole", func(t *testing.T) {
		ops := []op.Op{addOp("a", 1, 1, 5), addOp("b", 2, 3, 7), removeOp("c", 3, 2, 3)}
		require.Equal(t, []interval.Interval{
			iv(1, 2, interval.Closed, interval.Open),
			iv(3, 7, interval.Open, interval.Closed),
		}, strategy.Materialize(strategy.LWW, ops))
	})
	t.Run("later remove wins", func(t *testing.T) {
		ops := []op.Op{addOp("a", 1, 1, 5), removeOp("b", 2, 1, 5)}
		require.Empty(t, strategy.Materialize(strategy.LWW, ops))
	})
	t.Run("later add wins", func(t *testing.T) {
		ops := []op.Op{removeOp("a", 1, 1, 5), addOp("b", 2, 1, 5)}
		require.Equal(t, []interval.Interval{iv(1, 5, interval.Closed, interval.Closed)}, strategy.Materialize(strategy.LWW, ops))
	})
	t.Run("id breaks timestamp ties", func(t *testing.T) {
		ops := []op.Op{addOp("a", 1, 1, 5), addOp("b", 1, 3, 7)}
		require.Equal(t, []interval.Interval{iv(1, 7, interval.Closed, interval.Closed)}, strategy.Materialize(strategy.LWW, ops))
	})
	t.Run("order independent", func(t *testing.T) {
		ops := []op.Op{addOp("a", 1, 1, 5), addOp("b", 2, 3, 7), removeOp("c", 3, 2, 3)}
		reordered := []op.Op{ops[2], ops[0], ops[1]}
		require.Equal(t, strategy.Materialize(strategy.LWW, ops), strategy.Materialize(strategy.LWW, reordered))
	})
}

func TestMaterializeFWW(t *testing.T) {
	t.Parallel()
	t.Run("earlier add beats later remove", func(t *testing.T) {
		ops := []op.Op{addOp("a", 1, 1, 5), removeOp("b", 2, 2, 3)}
		require.Equal(t, []interval.Interval{iv(1, 5, interval.Closed, interval.Closed)}, strategy.Materialize(strategy.FWW, ops))
	})
	t.Run("earlier remove beats later add", func(t *testing.T) {
		ops := []op.Op{removeOp("a", 1, 2, 3), addOp("b", 2, 1, 5)}
		require.Equal(t, []interval.Interval{
			iv(1, 2, interval.Closed, interval.Open),
			iv(3, 5, interval.Open, interval.Closed),
		}, strategy.Materialize(strategy.FWW, ops))
	})
	t.Run("overlapping additions merge", func(t *testing.T) {
		ops := []op.Op{addOp("a", 1, 1, 5), addOp("b", 2, 3, 7)}
		require.Equal(t, []interval.Interval{iv(1, 7, interval.Closed, interval.Closed)}, strategy.Materialize(strategy.FWW, ops))
	})
}

func TestMaterializeAdditiveWins(t *testing.T) {
	t.Parallel()
	t.Run("union minus removals", func(t *testing.T) {
		ops := []op.Op{addOp("a", 1, 1, 5), addOp("b", 2, 3, 7), removeOp("c", 3, 2, 3)}
		require.Equal(t, []interval.Interval{
			iv(1, 2, interval.Closed, interval.Open),
			iv(3, 7, interval.Open, interval.Closed),
		}, strategy.Materialize(strategy.AdditiveWins, ops))
	})
	t.Run("remove always wins over add", func(t *testing.T) {
		ops := []op.Op{removeOp("a", 1, 1, 5), addOp("b", 2, 1, 5)}
		require.Empty(t, strategy.Materialize(strategy.AdditiveWins, ops))
	})
	t.Run("point remove", func(t *testing.T) {
		ops := []op.Op{addOp("a", 1, 1, 5), removeOp("b", 2, 1, 1)}
		require.Equal(t, []interval.Interval{iv(1, 5, interval.Open, interval.Closed)}, strategy.Materialize(strategy.AdditiveWins, ops))
	})
}

func TestMaterializeGrowOnly(t *testing.T) {
	t.Parallel()
	ops := []op.Op{addOp("a", 1, 1, 5), removeOp("b", 2, 2, 3), addOp("c", 3, 4, 6)}
	require.Equal(t, []interval.Interval{iv(1, 6, interval.Closed, interval.Closed)}, strategy.Materialize(strategy.GrowOnly, ops))
}

func TestMaterializeUnknown(t *testing.T) {
	t.Parallel()
	require.Nil(t, strategy.Materialize(strategy.Strategy(99), nil))
}

func TestSegments(t *testing.T) {
	t.Parallel()
	t.Run("undefined for set-based strategies", func(t *testing.T) {
		ops := []op.Op{addOp("a", 1, 1, 5)}
		require.Nil(t, strategy.Segments(strategy.AdditiveWins, ops))
		require.Nil(t, strategy.Segments(strategy.GrowOnly, ops))
	})
	for _, s := range []strategy.Strategy{strategy.LWW, strategy.FWW} {
		t.Run(s.String()+" matches materialize", func(t *testing.T) {
			ops := []op.Op{addOp("a", 1, 1, 5), addOp("b", 2, 3, 7), removeOp("c", 3, 2, 3)}
			segs := strategy.Segments(s, ops)
			require.NotEmpty(t, segs)
			for i := 1; i < len(segs); i++ {
				require.False(t, segs[i-1].Interval.Overlaps(segs[i].Interval), "segments must not overlap")
				require.True(t, interval.Less(segs[i-1].Interval, segs[i].Interval), "segments must be sorted")
			}
			require.Equal(t, strategy.Materialize(s, ops), strategy.ToIntervals(segs))
		})
	}
}

func TestCombineSegments(t *testing.T) {
	t.Parallel()
	first := []op.Op{addOp("a", 1, 1, 5)}
	second := []op.Op{removeOp("b", 2, 2, 3)}
	combined := strategy.CombineSegments(strategy.LWW, strategy.Segments(strategy.LWW, first), strategy.Segments(strategy.LWW, second))
	require.Equal(t, strategy.Segments(strategy.LWW, append(first, second...)), combined)
}
