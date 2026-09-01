package main

import (
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"math"
	"net/http"
	"sort"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/d-led/eventfulranges/demo/internal/collab"
)

// maxExportSide caps a server-side raster export so one board cannot exhaust
// memory. It is well above what browsers rasterize, so the server export is
// strictly more capable than the client canvas.
const maxExportSide = 16384

var boardBackground = color.RGBA{R: 0x0e, G: 0x10, B: 0x15, A: 0xff}

// exportHandler streams the session's materialized board as a PNG or JPEG.
func exportHandler(sessions *collab.Sessions) gin.HandlerFunc {
	return func(c *gin.Context) {
		sess := sessions.Model(c.Query("s"))
		b, ok := sess.Model().(*board)
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "unknown session"})
			return
		}
		format := c.DefaultQuery("format", "png")
		width, errW := strconv.Atoi(c.DefaultQuery("w", "0"))
		height, errH := strconv.Atoi(c.DefaultQuery("h", "0"))
		if errW != nil || errH != nil || width <= 0 || height <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "bad dimensions"})
			return
		}
		if width > maxExportSide || height > maxExportSide {
			c.JSON(http.StatusBadRequest, gin.H{"error": "image too large — max 16384 px per side"})
			return
		}
		rects, err := projectLayers(b.Layers(), width, height)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.Header("Content-Type", contentType(format))
		if err := encodeRaster(c.Writer, rects, format, width, height); err != nil {
			log.Printf("paint: export encode: %v", err)
		}
	}
}

func contentType(format string) string {
	if format == "jpeg" {
		return "image/jpeg"
	}
	return "image/png"
}

// layerRect is one layer projected into pixel space, ready for the y-sweep.
type layerRect struct {
	y0, y1, x0, x1 int
	color          color.RGBA
	erase          bool
}

// projectLayers fits the layered front into a width × height view with the
// same contain transform the browser uses, and returns the pixel rectangles in
// bottom-to-top paint order, sorted by their top edge for the sweep.
func projectLayers(layers []Layer, width, height int) ([]layerRect, error) {
	min0, min1, bw, bh := layerBounds(layers)
	if bw <= 0 || bh <= 0 {
		return nil, errors.New("nothing to export")
	}
	scale := math.Min(float64(width)/bw, float64(height)/bh)
	ox := (float64(width) - bw*scale) / 2
	oy := (float64(height) - bh*scale) / 2

	rects := make([]layerRect, 0, len(layers))
	for _, l := range layers {
		r := layerRect{
			x0: clamp(int(math.Round((l.X0-min0)*scale+ox)), width),
			x1: clamp(int(math.Round((l.X1-min0)*scale+ox)), width),
			y0: clamp(int(math.Round((l.Y0-min1)*scale+oy)), height),
			y1: clamp(int(math.Round((l.Y1-min1)*scale+oy)), height),
		}
		if r.x1 <= r.x0 || r.y1 <= r.y0 {
			continue
		}
		if l.Color == "" {
			r.erase = true
		} else {
			r.color = rgbaHex(l.Color)
		}
		rects = append(rects, r)
	}
	sort.Slice(rects, func(i, j int) bool { return rects[i].y0 < rects[j].y0 })
	return rects, nil
}

func clamp(v, hi int) int {
	if v < 0 {
		return 0
	}
	if v > hi {
		return hi
	}
	return v
}

// encodeRaster writes the projected rectangles as a PNG or JPEG, rendering one
// row at a time through a lazy image so the whole raster never lives in memory
// at once.
func encodeRaster(w io.Writer, rects []layerRect, format string, width, height int) error {
	img := &streamImage{
		rects:       rects,
		width:       width,
		height:      height,
		transparent: format == "png",
		row:         make([]uint8, width*4),
		rowY:        -1,
	}
	if format == "jpeg" {
		return jpeg.Encode(w, img, &jpeg.Options{Quality: 92})
	}
	return png.Encode(w, img)
}

// streamImage implements image.Image by rendering one row on demand. The
// standard PNG and JPEG encoders pull pixels top-to-bottom, so the active
// rectangles are swept by y and only the current row is buffered.
type streamImage struct {
	rects       []layerRect
	width       int
	height      int
	transparent bool

	row    []uint8
	rowY   int
	addIdx int
	active []int
}

func (m *streamImage) ColorModel() color.Model { return color.RGBAModel }

func (m *streamImage) Bounds() image.Rectangle { return image.Rect(0, 0, m.width, m.height) }

func (m *streamImage) At(x, y int) color.Color {
	if y != m.rowY {
		m.renderRow(y)
		m.rowY = y
	}
	i := x * 4
	return color.RGBA{R: m.row[i], G: m.row[i+1], B: m.row[i+2], A: m.row[i+3]}
}

func (m *streamImage) renderRow(y int) {
	if m.transparent {
		clear(m.row)
	} else {
		m.fillSpan(0, m.width, boardBackground)
	}
	for m.addIdx < len(m.rects) && m.rects[m.addIdx].y0 <= y {
		m.active = append(m.active, m.addIdx)
		m.addIdx++
	}
	kept := m.active[:0]
	for _, idx := range m.active {
		if m.rects[idx].y1 > y {
			kept = append(kept, idx)
		}
	}
	m.active = kept
	for _, idx := range m.active {
		r := m.rects[idx]
		switch {
		case r.erase && m.transparent:
			m.fillSpan(r.x0, r.x1, color.RGBA{}) // erase repaints transparent
		case r.erase:
			m.fillSpan(r.x0, r.x1, boardBackground)
		default:
			m.fillSpan(r.x0, r.x1, r.color)
		}
	}
}

func (m *streamImage) fillSpan(x0, x1 int, c color.RGBA) {
	for x := x0; x < x1; x++ {
		i := x * 4
		m.row[i], m.row[i+1], m.row[i+2], m.row[i+3] = c.R, c.G, c.B, c.A
	}
}

// layerBounds returns the painted drawing's bounding box; erases never widen
// the frame.
func layerBounds(layers []Layer) (min0, min1, bw, bh float64) {
	first := true
	var max0, max1 float64
	for _, l := range layers {
		if l.Color == "" {
			continue
		}
		if first {
			min0, min1, max0, max1 = l.X0, l.Y0, l.X1, l.Y1
			first = false
			continue
		}
		if l.X0 < min0 {
			min0 = l.X0
		}
		if l.Y0 < min1 {
			min1 = l.Y0
		}
		if l.X1 > max0 {
			max0 = l.X1
		}
		if l.Y1 > max1 {
			max1 = l.Y1
		}
	}
	if first {
		return 0, 0, 0, 0
	}
	return min0, min1, max0 - min0, max1 - min1
}

// rgbaHex parses "#rrggbb"; anything else falls back to the default stroke
// colour the browser uses.
func rgbaHex(s string) color.RGBA {
	fallback := color.RGBA{R: 0xe6, G: 0xe8, B: 0xee, A: 0xff}
	if len(s) != 7 || s[0] != '#' {
		return fallback
	}
	r, errR := strconv.ParseUint(s[1:3], 16, 8)
	g, errG := strconv.ParseUint(s[3:5], 16, 8)
	b, errB := strconv.ParseUint(s[5:7], 16, 8)
	if errR != nil || errG != nil || errB != nil {
		return fallback
	}
	return color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 0xff}
}
