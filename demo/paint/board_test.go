package main

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/d-led/eventfulranges/demo/internal/collab"
)

func TestBoardPaintAndEraseProduceEntries(t *testing.T) {
	t.Parallel()
	b := newBoard()

	entries, err := b.Apply("alice", paintCmd(0, 0, 4, 4))
	require.NoError(t, err)
	require.Len(t, entries, 1, "one rectangle is one box event")
	require.Equal(t, "add", entries[0].Kind)
	require.Equal(t, "alice", entries[0].Client)

	_, err = b.Apply("bob", eraseCmd(1, 1, 3, 3))
	require.NoError(t, err)

	log := b.Log()
	require.Len(t, log, 2, "one paint plus one erase")
	require.Equal(t, "bob", log[1].Client)
	require.Equal(t, "remove", log[1].Kind)
}

func TestBoardRejectsUnknownCommand(t *testing.T) {
	t.Parallel()
	b := newBoard()
	_, err := b.Apply("x", collab.Cmd{Kind: "smudge", Data: json.RawMessage(`{"x0":0,"y0":0,"x1":1,"y1":1}`)})
	require.Error(t, err)
}

func TestBoardRejectsInvertedRect(t *testing.T) {
	t.Parallel()
	b := newBoard()
	_, err := b.Apply("x", paintCmd(4, 0, 2, 2))
	require.Error(t, err)
}

func TestBoardAcceptsSubdividedCell(t *testing.T) {
	t.Parallel()
	b := newBoard()

	entries, err := b.Apply("alice", paintCmd(0, 0, 0.5, 0.5))
	require.NoError(t, err)
	require.Len(t, entries, 1, "one subdivided cell is one box event")
	require.Equal(t, "add", entries[0].Kind)
}

func TestBoardAttachesMetadata(t *testing.T) {
	t.Parallel()
	b := newBoard()

	entries, err := b.Apply("alice", collab.Cmd{
		Kind: "paint",
		Data: json.RawMessage(`{"x0":0,"y0":0,"x1":1,"y1":1}`),
		Meta: json.RawMessage(`{"color":"#ff5500"}`),
	})
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.JSONEq(t, `{"color":"#ff5500"}`, string(entries[0].Meta))
	require.JSONEq(t, `{"color":"#ff5500"}`, string(b.Log()[0].Meta))
}

func paintCmd(x0, y0, x1, y1 float64) collab.Cmd {
	return cellCmd("paint", x0, y0, x1, y1)
}

func eraseCmd(x0, y0, x1, y1 float64) collab.Cmd {
	return cellCmd("erase", x0, y0, x1, y1)
}

func cellCmd(kind string, x0, y0, x1, y1 float64) collab.Cmd {
	data, _ := json.Marshal(map[string]float64{"x0": x0, "y0": y0, "x1": x1, "y1": y1})
	return collab.Cmd{Kind: kind, Data: data}
}
