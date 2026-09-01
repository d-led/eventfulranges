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
	"reflect"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/d-led/eventfulranges/demo/internal/collab"
)

const (
	// maxExportSide caps a server-side raster export and matches the client's
	// documented maximum, so client and server agree on one limit. Streaming
	// keeps memory bounded regardless of size; the cap bounds encode time.
	maxExportSide = 40000

	formatPNG  = "png"
	formatJPEG = "jpeg"
	errorKey   = "error"
)

var boardBackground = color.RGBA{R: 0x0e, G: 0x10, B: 0x15, A: 0xff}

// exportParams is the validated request for one raster export.
type exportParams struct {
	format        string
	width, height int
}

// exportHandler streams the session's materialized board as a PNG or JPEG.
func exportHandler(sessions *collab.Sessions) gin.HandlerFunc {
	return func(c *gin.Context) {
		params, errMsg := parseExportParams(c)
		if errMsg != "" {
			c.JSON(http.StatusBadRequest, gin.H{errorKey: errMsg})
			return
		}
		b, ok := sessionBoard(sessions, c.Query("s"))
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{errorKey: "unknown session"})
			return
		}
		rects, err := projectLayers(b.Layers(), params.width, params.height)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{errorKey: err.Error()})
			return
		}
		c.Header("Content-Type", contentType(params.format))
		if err := encodeRaster(c.Writer, rects, params.format, params.width, params.height); err != nil {
			log.Printf("paint: export encode: %v", err)
		}
	}
}

// parseExportParams reads and validates the request parameters, returning a
// user-facing error message when they are not acceptable.
func parseExportParams(c *gin.Context) (exportParams, string) {
	format := c.DefaultQuery("format", formatPNG)
	if format != formatPNG && format != formatJPEG {
		return exportParams{}, "unsupported format"
	}
	width, errW := strconv.Atoi(c.DefaultQuery("w", "0"))
	height, errH := strconv.Atoi(c.DefaultQuery("h", "0"))
	if errW != nil || errH != nil || width <= 0 || height <= 0 {
		return exportParams{}, "bad dimensions"
	}
	if width > maxExportSide || height > maxExportSide {
		return exportParams{}, "image too large — max 40000 px per side"
	}
	return exportParams{format: format, width: width, height: height}, ""
}

// sessionBoard resolves a session id to its materializable board.
func sessionBoard(sessions *collab.Sessions, id string) (*board, bool) {
	b, ok := sessions.Model(id).Model().(*board)
	return b, ok
}

func contentType(format string) string {
	if format == formatJPEG {
		return "image/jpeg"
	}
	return "image/png"
}

// layerRect is one layer projected into pixel space, in paint order.
type layerRect struct {
	y0, y1, x0, x1 int
	color          color.RGBA
	erase          bool
}

// projectLayers fits the layered front into a width × height view with the
// same contain transform the browser uses, returning the pixel rectangles in
// bottom-to-top paint order so later strokes overdraw earlier ones.
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

// estimateRasterMemory reports the peak memory the streaming raster export
// allocates for an image width pixels wide with layerCount materialized
// layers. Height never appears: rows are rendered one at a time and streamed
// to the client, so the full raster is not held in memory.
func estimateRasterMemory(width, layerCount uint64) uint64 {
	row := width * 4 // one RGBA row
	perLayer := uint64(reflect.TypeOf(layerRect{}).Size())
	return row + layerCount*perLayer
}

// encodeRaster writes the projected rectangles as a PNG or JPEG, rendering one
// row at a time through a lazy image so the whole raster never lives in memory
// at once.
func encodeRaster(w io.Writer, rects []layerRect, format string, width, height int) error {
	img := &streamImage{
		rects:       rects,
		width:       width,
		height:      height,
		transparent: format == formatPNG,
		row:         make([]uint8, width*4),
		rowY:        -1,
	}
	if format == formatJPEG {
		return jpeg.Encode(w, img, &jpeg.Options{Quality: 92})
	}
	return png.Encode(w, img)
}

// streamImage implements image.Image by rendering one row on demand. The
// standard PNG and JPEG encoders pull pixels top-to-bottom, so each row is
// painted in layer order and only that row is buffered.
type streamImage struct {
	rects       []layerRect
	width       int
	height      int
	transparent bool

	row  []uint8
	rowY int
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
	// Paint bottom-to-top, so a later stroke overdraws an earlier one.
	for _, r := range m.rects {
		if y < r.y0 || y >= r.y1 {
			continue
		}
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
	min0, min1 = math.Inf(1), math.Inf(1)
	max0, max1 := math.Inf(-1), math.Inf(-1)
	for _, l := range layers {
		if l.Color == "" {
			continue
		}
		min0 = math.Min(min0, l.X0)
		min1 = math.Min(min1, l.Y0)
		max0 = math.Max(max0, l.X1)
		max1 = math.Max(max1, l.Y1)
	}
	if math.IsInf(min0, 1) {
		return 0, 0, 0, 0
	}
	return min0, min1, max0 - min0, max1 - min1
}

// rgbaHex parses "#rrggbb"; anything else falls back to the default stroke
// color the browser uses.
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
