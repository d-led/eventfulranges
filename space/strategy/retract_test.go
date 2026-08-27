package strategy_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/d-led/eventfulranges/space"
	"github.com/d-led/eventfulranges/space/op"
	"github.com/d-led/eventfulranges/space/strategy"
)

func TestEffectiveDropsRetractedOps(t *testing.T) {
	t.Parallel()
	ops := []op.Op{
		{ID: "a", Kind: op.KindAdd, Box: box(0, 0, 4, 4)},
		{ID: "b", Kind: op.KindAdd, Box: box(1, 1, 3, 3)},
		{ID: "ra", Kind: op.KindRetract, Ref: "a", Box: box(0, 0, 4, 4)},
	}
	got := strategy.Effective(ops)
	require.Equal(t, []op.Op{ops[1]}, got, "the retracted add and the retract itself drop out")
}

func TestRetractEraseRestoresTheBox(t *testing.T) {
	t.Parallel()
	ops := []op.Op{
		{ID: "a", Kind: op.KindAdd, Box: box(0, 0, 4, 4)},
		{ID: "e", Kind: op.KindRemove, Box: box(1, 1, 3, 3)},
		{ID: "r", Kind: op.KindRetract, Ref: "e", Box: box(1, 1, 3, 3)},
	}
	got := strategy.Materialize(strategy.AdditiveWins, ops)
	require.Equal(t, []space.Box{box(0, 0, 4, 4)}, got, "undoing the erase restores the whole rectangle")
}

func TestRetractAddRemovesOnlyThatAdd(t *testing.T) {
	t.Parallel()
	ops := []op.Op{
		{ID: "alice", Kind: op.KindAdd, Box: box(0, 0, 4, 4)},
		{ID: "bob", Kind: op.KindAdd, Box: box(2, 2, 6, 6)},
		{ID: "r", Kind: op.KindRetract, Ref: "bob", Box: box(2, 2, 6, 6)},
	}
	got := strategy.Materialize(strategy.AdditiveWins, ops)
	require.Equal(t, []space.Box{box(0, 0, 4, 4)}, got, "undoing bob's stroke leaves alice's rectangle intact")
}
