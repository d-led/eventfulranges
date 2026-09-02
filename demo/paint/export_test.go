package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

// renderPNG streams the layers through the real encoder and decodes the result
// so a test samples the exact bytes a client would receive.
func renderPNG(t *testing.T, layers []Layer, w, h int) image.Image {
	t.Helper()
	rects, err := projectLayers(layers, w, h)
	require.NoError(t, err)
	var buf bytes.Buffer
	require.NoError(t, encodeRaster(&buf, rects, "png", w, h))
	img, err := png.Decode(&buf)
	require.NoError(t, err)
	return img
}

func renderJPEG(t *testing.T, layers []Layer, w, h int) image.Image {
	t.Helper()
	rects, err := projectLayers(layers, w, h)
	require.NoError(t, err)
	var buf bytes.Buffer
	require.NoError(t, encodeRaster(&buf, rects, "jpeg", w, h))
	img, err := jpeg.Decode(&buf)
	require.NoError(t, err)
	return img
}

func TestRenderLayers(t *testing.T) {
	t.Parallel()
	layers := []Layer{
		{X0: 0, Y0: 0, X1: 10, Y1: 10, Color: "#ff0000"},
		{X0: 4, Y0: 4, X1: 6, Y1: 6, Color: ""}, // erase
	}
	img := renderPNG(t, layers, 100, 100)
	require.Equal(t, 100, img.Bounds().Dx())
	require.Equal(t, 100, img.Bounds().Dy())

	// 10 board units scale to 100 px, so one unit is 10 px. The center
	// (board 5,5) lies inside the erase hole.
	r, _, _, a := img.At(10, 10).RGBA() // board (1,1): the painted square
	require.Equal(t, uint32(0xffff), r)
	require.Equal(t, uint32(0xffff), a)
	_, _, _, a = img.At(50, 50).RGBA() // board (5,5): erased
	require.Equal(t, uint32(0), a)
}

func TestRenderLayersLetterboxesOnBackground(t *testing.T) {
	t.Parallel()
	// A 2:1 drawing in a square image letterboxes above and below; JPEG fills
	// the margins with the opaque board background.
	layers := []Layer{{X0: 0, Y0: 0, X1: 10, Y1: 5, Color: "#ff0000"}}
	img := renderJPEG(t, layers, 100, 100)

	r, _, _, a := img.At(50, 10).RGBA() // above the drawing
	require.Equal(t, uint32(0xffff), a)
	require.Less(t, r, uint32(0x8000))    // dark background, not red
	r, _, _, _ = img.At(50, 50).RGBA()    // the drawing itself
	require.Greater(t, r, uint32(0xf000)) // red-dominant despite JPEG loss
}

func TestRenderLayersEraseLargerThanFrame(t *testing.T) {
	t.Parallel()
	// An erase wider than the painted frame must be clamped, not clipped.
	layers := []Layer{
		{X0: 0, Y0: 0, X1: 2, Y1: 2, Color: "#ff0000"},
		{X0: -10, Y0: -10, X1: 20, Y1: 20, Color: ""},
	}
	img := renderPNG(t, layers, 100, 100)
	_, _, _, a := img.At(50, 50).RGBA()
	require.Equal(t, uint32(0), a) // everything erased
}

func TestRenderLayersPaintOrder(t *testing.T) {
	t.Parallel()
	// A later stroke drawn above an earlier one must overdraw it, even though
	// it starts at a smaller y: the row is painted in layer order, not in
	// top-edge order.
	layers := []Layer{
		{X0: 0, Y0: 2, X1: 10, Y1: 10, Color: "#ff0000"}, // bottom, red
		{X0: 0, Y0: 0, X1: 8, Y1: 8, Color: "#00ff00"},   // top, green
	}
	img := renderPNG(t, layers, 100, 100)
	// Board (4,4) lies inside both; the green top layer must win.
	r, g, _, _ := img.At(40, 40).RGBA()
	require.Equal(t, uint32(0), r)
	require.Equal(t, uint32(0xffff), g)
}

func TestRenderFrozenImageLayer(t *testing.T) {
	t.Parallel()
	// A 1x1 red PNG, embedded as the frozen layer's source bytes.
	src := image.NewRGBA(image.Rect(0, 0, 1, 1))
	src.Set(0, 0, color.RGBA{R: 0xff, A: 0xff})
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, src))
	dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())

	layers := []Layer{{X0: 0, Y0: 0, X1: 10, Y1: 10, Color: "#e6e8ee", Image: dataURL, Frozen: true}}
	img := renderPNG(t, layers, 100, 100)
	r, _, _, a := img.At(50, 50).RGBA()
	require.Equal(t, uint32(0xffff), r) // the red image fills the whole layer
	require.Equal(t, uint32(0xffff), a)
}

func TestRenderNestedFrozenImages(t *testing.T) {
	t.Parallel()
	solid := func(c color.RGBA) string {
		src := image.NewRGBA(image.Rect(0, 0, 1, 1))
		src.Set(0, 0, c)
		var buf bytes.Buffer
		require.NoError(t, png.Encode(&buf, src))
		return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
	}
	layers := []Layer{
		{X0: 0, Y0: 0, X1: 8, Y1: 8, Color: "#e6e8ee", Image: solid(color.RGBA{R: 0xff, A: 0xff}), Frozen: true},
		{X0: 1, Y0: 1, X1: 3, Y1: 3, Color: "#e6e8ee", Image: solid(color.RGBA{B: 0xff, A: 0xff}), Frozen: true},
	}
	img := renderPNG(t, layers, 100, 100)
	// scale 12.5 px/unit: board (2,2) is the small image, board (6,6) the big.
	_, _, b, _ := img.At(25, 25).RGBA()
	require.Greater(t, b, uint32(0xf000)) // small blue layered on top
	r, _, _, _ := img.At(75, 75).RGBA()
	require.Greater(t, r, uint32(0xf000)) // big red shows through around it
}

func TestProjectLayersEmpty(t *testing.T) {
	t.Parallel()
	_, err := projectLayers(nil, 100, 100)
	require.Error(t, err)

	// Erases widen nothing, so a board of only erases is empty too.
	_, err = projectLayers([]Layer{{X0: 0, Y0: 0, X1: 4, Y1: 4, Color: ""}}, 100, 100)
	require.Error(t, err)
}

func TestBoxCanvasLayers(t *testing.T) {
	t.Parallel()
	c := newBoxCanvas()

	_, err := c.Paint("a", Rect{0, 0, 4, 4}, json.RawMessage(`{"color":"#ff0000"}`))
	require.NoError(t, err)
	_, err = c.Erase("b", Rect{1, 1, 2, 2}, nil)
	require.NoError(t, err)

	layers := c.Layers()
	require.Len(t, layers, 2)
	require.Equal(t, "#ff0000", layers[0].Color)
	require.Equal(t, "", layers[1].Color)
	// The erase sits inside the painted square, kept as one whole box.
	require.Equal(t, 1.0, layers[1].X0)
	require.Equal(t, 1.0, layers[1].Y0)
	require.Equal(t, 2.0, layers[1].X1)
	require.Equal(t, 2.0, layers[1].Y1)
}

func TestEstimateRasterMemory(t *testing.T) {
	t.Parallel()
	perLayer := uint64(reflect.TypeOf(layerRect{}).Size())

	t.Run("is one row plus one rectangle per layer", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, uint64(40000*4)+2*perLayer, estimateRasterMemory(40000, 2))
	})

	t.Run("stays under a megabyte for a 40000px square", func(t *testing.T) {
		t.Parallel()
		require.Less(t, estimateRasterMemory(40000, 2), uint64(1<<20))
	})

	t.Run("grows with width and layer count, never height", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, 2*estimateRasterMemory(20000, 0), estimateRasterMemory(40000, 0))
		require.Equal(t, 10*perLayer, estimateRasterMemory(100, 10)-estimateRasterMemory(100, 0))
	})

	t.Run("a thin stroke costs the same per layer as a full fill", func(t *testing.T) {
		t.Parallel()
		// The estimator sees only the export width and the box count, never
		// the drawn area, so a 10-unit stroke and a full-board fill of the
		// same box count cost the same.
		require.Equal(t, uint64(40000*4)+perLayer, estimateRasterMemory(40000, 1))
	})
}
