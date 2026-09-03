package main

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSearchReturnsBoxesOverlappingTheQuery(t *testing.T) {
	h := newHubMode(compactCanonical)
	_, err := h.apply(opAdd, []float64{0, 0}, []float64{4, 4})
	require.NoError(t, err)
	_, err = h.apply(opAdd, []float64{10, 10}, []float64{12, 12})
	require.NoError(t, err)

	res, err := h.search([]float64{1, 1}, []float64{5, 5})
	require.NoError(t, err)
	require.Len(t, res.Boxes, 1, "only the first box overlaps the query")
	require.Equal(t, []float64{0, 0}, res.Boxes[0].Min)
	require.Equal(t, []float64{1, 1}, res.Min, "the query is echoed back")

	// The index must reflect later edits: removing the first box empties the
	// answer without anyone rebuilding the index by hand.
	_, err = h.apply(opRemove, []float64{0, 0}, []float64{4, 4})
	require.NoError(t, err)
	res, err = h.search([]float64{1, 1}, []float64{5, 5})
	require.NoError(t, err)
	require.Empty(t, res.Boxes)
}

func TestSearchOnEmptyBoard(t *testing.T) {
	h := newHubMode(compactCanonical)
	res, err := h.search([]float64{0}, []float64{1})
	require.NoError(t, err)
	require.Empty(t, res.Boxes)
}

func TestSearchRejectsBadQueries(t *testing.T) {
	h := newHubMode(compactCanonical)
	_, err := h.apply(opAdd, []float64{0, 0}, []float64{1, 1})
	require.NoError(t, err)

	_, err = h.search([]float64{0, 0, 0}, []float64{1, 1, 1})
	require.Error(t, err, "dimension mismatch")

	_, err = h.search([]float64{math.NaN(), 0}, []float64{1, 1})
	require.Error(t, err, "NaN coordinate")
}

func TestSearchBroadcastsAResult(t *testing.T) {
	h := newHubMode(compactCanonical)
	_, err := h.apply(opAdd, []float64{0}, []float64{1})
	require.NoError(t, err)

	ch := h.subscribe()
	defer h.unsubscribe(ch)

	res, err := h.search([]float64{0.5}, []float64{1.5})
	require.NoError(t, err)
	require.Len(t, res.Boxes, 1)

	msg := <-ch
	require.Equal(t, msgSearch, msg.Type)
	require.NotNil(t, msg.Search)
	require.Len(t, msg.Search.Boxes, 1)
	require.Equal(t, []float64{0.5}, msg.Search.Min)
}
