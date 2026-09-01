package main

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"math"
	"net/http"
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
		img, err := renderLayers(b.Layers(), format, width, height)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		encoded, err := encodeImage(img, format)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "encode failed"})
			return
		}
		c.Data(http.StatusOK, contentType(format), encoded)
	}
}

func contentType(format string) string {
	if format == "jpeg" {
		return "image/jpeg"
	}
	return "image/png"
}

// encodeImage renders the image in the requested format; png is the fallback.
func encodeImage(img *image.RGBA, format string) ([]byte, error) {
	var buf bytes.Buffer
	if format == "jpeg" {
		err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 92})
		return buf.Bytes(), err
	}
	err := png.Encode(&buf, img)
	return buf.Bytes(), err
}

// renderLayers rasterizes the layered front into a width × height RGBA image
// with the same contain fit the browser uses, so the file matches the board.
func renderLayers(layers []Layer, format string, width, height int) (*image.RGBA, error) {
	min0, min1, bw, bh := layerBounds(layers)
	if bw <= 0 || bh <= 0 {
		return nil, errors.New("nothing to export")
	}
	scale := math.Min(float64(width)/bw, float64(height)/bh)
	ox := (float64(width) - bw*scale) / 2
	oy := (float64(height) - bh*scale) / 2

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	transparent := format == "png"
	if !transparent {
		draw.Draw(img, img.Bounds(), image.NewUniform(boardBackground), image.Point{}, draw.Src)
	}
	for _, l := range layers {
		r := projectLayer(l, min0, min1, scale, ox, oy)
		if r.Empty() {
			continue
		}
		if l.Color == "" {
			// An erase repaints the background: transparent for PNG, the flat
			// board background for JPEG.
			if transparent {
				draw.Draw(img, r, image.Transparent, image.Point{}, draw.Src)
			} else {
				draw.Draw(img, r, image.NewUniform(boardBackground), image.Point{}, draw.Src)
			}
			continue
		}
		draw.Draw(img, r, image.NewUniform(rgbaHex(l.Color)), image.Point{}, draw.Src)
	}
	return img, nil
}

// projectLayer maps one layer box to its pixel rectangle in the export view.
func projectLayer(l Layer, min0, min1, scale, ox, oy float64) image.Rectangle {
	x0 := int(math.Round((l.X0-min0)*scale + ox))
	y0 := int(math.Round((l.Y0-min1)*scale + oy))
	x1 := int(math.Round((l.X1-min0)*scale + ox))
	y1 := int(math.Round((l.Y1-min1)*scale + oy))
	return image.Rect(x0, y0, x1, y1)
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
