package main

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/d-led/eventfulranges/demo/internal/collab"
	"github.com/d-led/eventfulranges/demo/internal/morton"
)

func TestBoardPaintAndEraseProduceEntries(t *testing.T) {
	t.Parallel()
	b := newBoard()

	entries, err := b.Apply("alice", paintCmd(0, 0, 4, 4))
	require.NoError(t, err)
	require.Len(t, entries, 1, "an aligned 4x4 block is one event")
	require.Equal(t, "add", entries[0].Kind)
	require.Equal(t, "alice", entries[0].Client)

	_, err = b.Apply("bob", eraseCmd(1, 1, 3, 3))
	require.NoError(t, err)

	log := b.Log()
	require.Len(t, log, 5, "one paint plus four erased cells")
	require.Equal(t, "bob", log[len(log)-1].Client)
	require.Equal(t, "remove", log[len(log)-1].Kind)
}

func TestBoardRejectsUnknownCommand(t *testing.T) {
	t.Parallel()
	b := newBoard()
	_, err := b.Apply("x", collab.Cmd{Kind: "smudge", Data: json.RawMessage(`{"x0":0,"y0":0,"x1":1,"y1":1}`)})
	require.Error(t, err)
}

func TestBoardRejectsOutOfRange(t *testing.T) {
	t.Parallel()
	b := newBoard()
	_, err := b.Apply("x", paintCmd(morton.Limit, 0, morton.Limit+1, 1))
	require.ErrorIs(t, err, morton.ErrOutOfRange)
}

func paintCmd(x0, y0, x1, y1 int64) collab.Cmd {
	return cellCmd("paint", x0, y0, x1, y1)
}

func eraseCmd(x0, y0, x1, y1 int64) collab.Cmd {
	return cellCmd("erase", x0, y0, x1, y1)
}

func cellCmd(kind string, x0, y0, x1, y1 int64) collab.Cmd {
	data, _ := json.Marshal(map[string]int64{"x0": x0, "y0": y0, "x1": x1, "y1": y1})
	return collab.Cmd{Kind: kind, Data: data}
}
