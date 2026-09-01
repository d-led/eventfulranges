package main

import (
	"bytes"
	"encoding/json"
	"image"
	"image/jpeg"
	"image/png"
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

	// 10 board units scale to 100 px, so one unit is 10 px. The centre
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
