package eventfulranges_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/d-led/eventfulranges"
	"github.com/d-led/eventfulranges/clock"
	"github.com/d-led/eventfulranges/meta"
	"github.com/d-led/eventfulranges/space"
	sop "github.com/d-led/eventfulranges/space/op"
	"github.com/d-led/eventfulranges/space/store/memory"
	"github.com/d-led/eventfulranges/space/strategy"
)

func TestOpenBoxesAndConvenienceMethods(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	bs, err := eventfulranges.OpenBoxes(ctx, t.TempDir(), strategy.LWW,
		eventfulranges.WithBoxClock(clock.NewLamport()),
		eventfulranges.WithBoxSnapshotEvery(2))
	require.NoError(t, err)

	_, err = bs.Add(ctx, []float64{1, 1}, []float64{5, 5})
	require.NoError(t, err)
	_, err = bs.Add(ctx, []float64{3, 3}, []float64{7, 7})
	require.NoError(t, err)

	require.True(t, bs.Contains([]float64{6, 6}))
	require.False(t, bs.Contains([]float64{8, 8}))
	require.True(t, bs.Overlaps(space.NewBox([]float64{5, 5}, []float64{9, 9})))

	_, err = bs.Remove(ctx, []float64{2, 2}, []float64{3, 3})
	require.NoError(t, err)

	require.Len(t, bs.Ops(), 3)
	require.NoError(t, bs.Snapshot(ctx))
	require.NotEmpty(t, bs.Boxes())
}

func TestOpenBoxStore(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	bs, err := eventfulranges.OpenBoxStore(ctx, memory.New(), strategy.GrowOnly)
	require.NoError(t, err)
	_, err = bs.Add(ctx, []float64{1, 1}, []float64{2, 2})
	require.NoError(t, err)
	require.NoError(t, bs.Apply(ctx, sop.Op{ID: "z", Kind: sop.KindAdd, Box: space.NewBox([]float64{3, 3}, []float64{4, 4})}))
	require.Len(t, bs.Boxes(), 2)
}

func TestBoxSetRegionQueries(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	bs, err := eventfulranges.OpenBoxStore(ctx, memory.New(), strategy.AdditiveWins)
	require.NoError(t, err)
	_, err = bs.Add(ctx, []float64{0, 0}, []float64{10, 10})
	require.NoError(t, err)
	_, err = bs.Add(ctx, []float64{20, 0}, []float64{30, 10})
	require.NoError(t, err)

	// A horizontal path crosses the first box, skips empty space, then
	// crosses the second.
	p := space.NewPath([]float64{0, 0}, []float64{40, 0})
	first := space.NewBox([]float64{0, 0}, []float64{10, 10})
	second := space.NewBox([]float64{20, 0}, []float64{30, 10})

	require.Equal(t, []space.Box{first, second}, bs.Crossed(p))
	require.Equal(t, []space.PathSegment{
		{Start: 0, End: 0.25, Covered: true, Boxes: []space.Box{first}},
		{Start: 0.25, End: 0.5, Covered: false},
		{Start: 0.5, End: 0.75, Covered: true, Boxes: []space.Box{second}},
		{Start: 0.75, End: 1, Covered: false},
	}, bs.Traverse(p))
}

func TestBoxApplyAllForAntiEntropy(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	a, err := eventfulranges.OpenBoxStore(ctx, memory.New(), strategy.LWW)
	require.NoError(t, err)
	b, err := eventfulranges.OpenBoxStore(ctx, memory.New(), strategy.LWW)
	require.NoError(t, err)

	_, err = a.Add(ctx, []float64{1, 1}, []float64{4, 4})
	require.NoError(t, err)
	_, err = b.Remove(ctx, []float64{2, 2}, []float64{3, 3})
	require.NoError(t, err)

	require.NoError(t, a.ApplyAll(ctx, b.Ops()))
	require.NoError(t, b.ApplyAll(ctx, a.Ops()))

	require.Equal(t, a.Boxes(), b.Boxes(), "replicas must converge")
}

func TestOpenBoxesInvalidStrategy(t *testing.T) {
	t.Parallel()
	_, err := eventfulranges.OpenBoxes(context.Background(), t.TempDir(), strategy.Strategy(99))
	require.Error(t, err)
}

func TestOpenBoxesUnwritableDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	blocker := filepath.Join(dir, "file")
	require.NoError(t, os.WriteFile(blocker, nil, 0o644))
	_, err := eventfulranges.OpenBoxes(context.Background(), filepath.Join(blocker, "sub"), strategy.LWW)
	require.Error(t, err)
}

func TestBoxSetCanonicalizer(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	split := func(boxes []space.Box) []space.Box {
		out := make([]space.Box, 0, 2*len(boxes))
		for _, b := range boxes {
			if len(b.Min) == 0 || b.Max[0]-b.Min[0] < 2 {
				out = append(out, b)
				continue
			}
			mid := (b.Min[0] + b.Max[0]) / 2
			left := space.NewBox(append([]float64(nil), b.Min...), append([]float64(nil), b.Max...))
			right := space.NewBox(append([]float64(nil), b.Min...), append([]float64(nil), b.Max...))
			left.Max[0] = mid
			right.Min[0] = mid
			out = append(out, left, right)
		}
		return out
	}

	bs, err := eventfulranges.OpenBoxStore(ctx, memory.New(), strategy.GrowOnly,
		eventfulranges.WithBoxCanonicalizer(split))
	require.NoError(t, err)
	_, err = bs.Add(ctx, []float64{0, 0}, []float64{2, 2})
	require.NoError(t, err)
	require.Len(t, bs.Boxes(), 2, "the canonicalizer splits the box along x")
}

func TestBoxSetStoresAndMergesMetadata(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	bs, err := eventfulranges.OpenBoxStore(ctx, memory.New(), strategy.AdditiveWins)
	require.NoError(t, err)

	red := json.RawMessage(`{"color":"#ff0000"}`)
	alice := json.RawMessage(`{"author":"alice"}`)

	applied, err := bs.AddWithMeta(ctx, []float64{0, 0}, []float64{4, 4}, red)
	require.NoError(t, err)
	require.JSONEq(t, `{"color":"#ff0000"}`, string(applied.Box.Meta))

	_, err = bs.AddWithMeta(ctx, []float64{0, 0}, []float64{8, 8}, alice)
	require.NoError(t, err)

	boxes := bs.Boxes()
	require.Len(t, boxes, 1, "the larger box subsumes the smaller and folds in its metadata")
	require.JSONEq(t, `{"color":"#ff0000","author":"alice"}`, string(boxes[0].Meta))
}

func TestBoxSetRemoveWithMeta(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	bs, err := eventfulranges.OpenBoxStore(ctx, memory.New(), strategy.AdditiveWins)
	require.NoError(t, err)

	_, err = bs.Add(ctx, []float64{0, 0}, []float64{4, 4})
	require.NoError(t, err)

	removed, err := bs.RemoveWithMeta(ctx, []float64{1, 1}, []float64{3, 3}, json.RawMessage(`{"tool":"erase"}`))
	require.NoError(t, err)
	require.JSONEq(t, `{"tool":"erase"}`, string(removed.Box.Meta))

	require.False(t, bs.Contains([]float64{2, 2}), "the erased hole is empty")
	require.True(t, bs.Contains([]float64{0.5, 2}), "the shell around the hole survives")
}

func TestBoxSetRetract(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	bs, err := eventfulranges.OpenBoxStore(ctx, memory.New(), strategy.AdditiveWins)
	require.NoError(t, err)

	added, err := bs.Add(ctx, []float64{0, 0}, []float64{4, 4})
	require.NoError(t, err)
	removed, err := bs.Remove(ctx, []float64{1, 1}, []float64{3, 3})
	require.NoError(t, err)
	require.False(t, bs.Contains([]float64{2, 2}), "the hole is erased")

	_, err = bs.Retract(ctx, removed.ID)
	require.NoError(t, err)
	require.True(t, bs.Contains([]float64{2, 2}), "undoing the erase restores the hole")

	_, err = bs.Retract(ctx, added.ID)
	require.NoError(t, err)
	require.False(t, bs.Contains([]float64{0.5, 0.5}), "undoing the paint clears the rectangle")

	_, err = bs.Retract(ctx, "missing")
	require.Error(t, err, "retracting an unknown op fails")
}

func TestBoxSetCannotRetractARetraction(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	bs, err := eventfulranges.OpenBoxStore(ctx, memory.New(), strategy.AdditiveWins)
	require.NoError(t, err)

	added, err := bs.Add(ctx, []float64{0, 0}, []float64{4, 4})
	require.NoError(t, err)
	retracted, err := bs.Retract(ctx, added.ID)
	require.NoError(t, err)

	_, err = bs.Retract(ctx, retracted.ID)
	require.Error(t, err, "a retraction cannot itself be retracted")
}

func TestBoxSetRetractWithID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	bs, err := eventfulranges.OpenBoxStore(ctx, memory.New(), strategy.AdditiveWins)
	require.NoError(t, err)

	added, err := bs.Add(ctx, []float64{0, 0}, []float64{4, 4})
	require.NoError(t, err)

	retracted, err := bs.RetractWithID(ctx, "undo-1", added.ID)
	require.NoError(t, err)
	require.Equal(t, "undo-1", retracted.ID, "the retraction keeps the caller's ID")
	require.False(t, bs.Contains([]float64{0.5, 0.5}), "the retracted paint is gone")
}

func TestBoxSetCustomMetaMerge(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	constant := meta.Merge(func(_, _ json.RawMessage) json.RawMessage {
		return json.RawMessage(`{"merged":true}`)
	})

	bs, err := eventfulranges.OpenBoxStore(ctx, memory.New(), strategy.AdditiveWins,
		eventfulranges.WithBoxMetaMerge(constant))
	require.NoError(t, err)

	_, err = bs.AddWithMeta(ctx, []float64{0, 0}, []float64{4, 4}, json.RawMessage(`{"color":"#ff0000"}`))
	require.NoError(t, err)
	_, err = bs.AddWithMeta(ctx, []float64{0, 0}, []float64{8, 8}, json.RawMessage(`{"author":"alice"}`))
	require.NoError(t, err)

	require.JSONEq(t, `{"merged":true}`, string(bs.Boxes()[0].Meta), "the custom join decides the merged metadata")
}

func TestBoxSetCompactRoundTrips(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := t.TempDir()

	bs, err := eventfulranges.OpenBoxes(ctx, dir, strategy.AdditiveWinsLWW,
		eventfulranges.WithBoxSnapshotEvery(2))
	require.NoError(t, err)

	_, err = bs.Add(ctx, []float64{0, 0}, []float64{4, 4})
	require.NoError(t, err)
	_, err = bs.Remove(ctx, []float64{1, 1}, []float64{3, 3})
	require.NoError(t, err)
	want := bs.Boxes()
	wantOps := bs.Ops()

	require.NoError(t, bs.Compact(ctx))

	reopened, err := eventfulranges.OpenBoxes(ctx, dir, strategy.AdditiveWinsLWW)
	require.NoError(t, err)
	require.True(t, space.Equal(want, reopened.Boxes()), "compaction preserves the picture")
	require.Equal(t, wantOps, reopened.Ops(), "compaction preserves the operation history")
}
