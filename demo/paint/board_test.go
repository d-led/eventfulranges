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

func TestBoardReplayRestoresLog(t *testing.T) {
	t.Parallel()
	original := newBoard()
	_, err := original.Apply("alice", paintCmd(0, 0, 4, 4))
	require.NoError(t, err)
	_, err = original.Apply("bob", eraseCmd(1, 1, 3, 3))
	require.NoError(t, err)

	restored := newBoard()
	require.NoError(t, restored.Replay(original.Log()))

	log := restored.Log()
	require.Len(t, log, 2)
	require.Equal(t, "add", log[0].Kind)
	require.Equal(t, "alice", log[0].Client)
	require.JSONEq(t, `{"min":[0,0],"max":[4,4]}`, string(log[0].Data))
	require.Equal(t, "remove", log[1].Kind)
	require.Equal(t, "bob", log[1].Client)
}

func TestBoardRetractProducesRetractEntry(t *testing.T) {
	t.Parallel()
	b := newBoard()

	entries, err := b.Apply("alice", collab.Cmd{ID: "paint-1", Kind: "paint", Data: json.RawMessage(`{"x0":0,"y0":0,"x1":4,"y1":4}`)})
	require.NoError(t, err)
	require.Equal(t, "paint-1", entries[0].ID)

	ret, err := b.Apply("alice", collab.Cmd{ID: "retract-1", Kind: "retract", Ref: "paint-1"})
	require.NoError(t, err)
	require.Len(t, ret, 1)
	require.Equal(t, "retract", ret[0].Kind)
	require.Equal(t, "paint-1", ret[0].Ref)
	require.Equal(t, "retract-1", ret[0].ID, "the retraction keeps the client's own ID")

	restored := newBoard()
	require.NoError(t, restored.Replay(b.Log()))
	require.Len(t, restored.Log(), 2)
	require.Equal(t, "retract", restored.Log()[1].Kind)
	require.Equal(t, "paint-1", restored.Log()[1].Ref)
	require.Equal(t, "retract-1", restored.Log()[1].ID)
}
