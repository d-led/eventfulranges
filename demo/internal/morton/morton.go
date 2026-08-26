// Package morton maps two-dimensional integer cells onto a one-dimensional
// space-filling curve, so a 2-D grid can be stored in the library's 1-D range
// CRDT. A rectangle of cells decomposes into a small set of contiguous Morton
// code ranges — the same subdivision a quadtree/octree performs.
package morton

import (
	"errors"
	"fmt"
	"math/bits"
	"sort"
)

// CoordBits is how many bits each axis contributes to a Morton code. A code is
// 2*CoordBits wide: 26 keeps every code inside a float64's exact 53-bit
// mantissa, so the library's 1-D interval engine stores cell coordinates
// losslessly.
const CoordBits = 26

// bias shifts signed coordinates into the non-negative range. It is a power
// of two, so adding it preserves power-of-two alignment: an aligned rectangle
// stays contiguous along the curve.
const bias = 1 << (CoordBits - 1)

// Limit bounds each coordinate: a valid cell satisfies -Limit <= x < Limit.
const Limit = bias

// ErrOutOfRange is returned when a coordinate does not fit the canvas.
var ErrOutOfRange = errors.New("morton: coordinate out of range")

// Range is a half-open [Lo, Hi) span of Morton codes, i.e. the cells Lo..Hi-1
// along the curve.
type Range struct {
	Lo uint64
	Hi uint64
}

// Encode folds the signed cell (x, y) into one Morton code.
func Encode(x, y int64) (uint64, error) {
	if !fits(x) || !fits(y) {
		return 0, fmt.Errorf("%w: (%d, %d)", ErrOutOfRange, x, y)
	}
	return spread(uint32(x+bias)) | (spread(uint32(y+bias)) << 1), nil
}

// Decode unfolds a Morton code back into the cell it encodes.
func Decode(m uint64) (x, y int64) {
	return int64(compact(m)) - bias, int64(compact(m>>1)) - bias
}

// Ranges decomposes the half-open cell rectangle [x0,x1) x [y0,y1) into
// maximal contiguous Morton ranges, sorted by code.
func Ranges(x0, y0, x1, y1 int64) ([]Range, error) {
	if x0 >= x1 || y0 >= y1 {
		return nil, fmt.Errorf("morton: empty or inverted rectangle (%d,%d)-(%d,%d)", x0, y0, x1, y1)
	}
	if x0 < -bias || x1 > bias || y0 < -bias || y1 > bias {
		return nil, fmt.Errorf("%w: rectangle (%d,%d)-(%d,%d)", ErrOutOfRange, x0, y0, x1, y1)
	}
	var out []Range
	subdivide(x0, y0, x1, y1, &out)
	sort.Slice(out, func(i, j int) bool { return out[i].Lo < out[j].Lo })
	return merge(out), nil
}

// merge joins ranges that are adjacent along the curve into maximal runs, so
// a rectangle yields the fewest possible operations.
func merge(ranges []Range) []Range {
	if len(ranges) < 2 {
		return ranges
	}
	merged := ranges[:1]
	for _, r := range ranges[1:] {
		last := &merged[len(merged)-1]
		if r.Lo == last.Hi {
			last.Hi = r.Hi
		} else {
			merged = append(merged, r)
		}
	}
	return merged
}

// subdivide splits the rectangle into aligned blocks, each one contiguous
// along the curve, in the classic quadtree manner: emit the largest aligned
// block at the lower-left corner, then recurse on the two remaining strips.
func subdivide(loX, loY, hiX, hiY int64, out *[]Range) {
	if loX >= hiX || loY >= hiY {
		return
	}
	size := blockSize(loX, loY, hiX-loX, hiY-loY)
	code, _ := Encode(loX, loY)
	*out = append(*out, Range{Lo: code, Hi: code + uint64(size)*uint64(size)})
	subdivide(loX+size, loY, hiX, loY+size, out) // right strip
	subdivide(loX, loY+size, hiX, hiY, out)      // bottom strip
}

// blockSize returns the largest power-of-two block aligned at (loX, loY) that
// still fits inside the given extents.
func blockSize(loX, loY, extX, extY int64) int64 {
	tz := minInt(bits.TrailingZeros64(uint64(loX)), bits.TrailingZeros64(uint64(loY)))
	if tz > CoordBits {
		tz = CoordBits
	}
	size := int64(1) << tz
	maxSize := minInt64(extX, extY)
	for size > maxSize {
		size >>= 1
	}
	return size
}

func fits(v int64) bool {
	return v >= -bias && v < bias
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

// spread widens the low 32 bits of v into the even positions of a 64-bit word.
func spread(v uint32) uint64 {
	x := uint64(v)
	x = (x | x<<16) & 0x0000FFFF0000FFFF
	x = (x | x<<8) & 0x00FF00FF00FF00FF
	x = (x | x<<4) & 0x0F0F0F0F0F0F0F0F
	x = (x | x<<2) & 0x3333333333333333
	x = (x | x<<1) & 0x5555555555555555
	return x
}

// compact gathers the even positions of a 64-bit word into the low 32 bits.
func compact(v uint64) uint32 {
	x := v & 0x5555555555555555
	x = (x | x>>1) & 0x3333333333333333
	x = (x | x>>2) & 0x0F0F0F0F0F0F0F0F
	x = (x | x>>4) & 0x00FF00FF00FF00FF
	x = (x | x>>8) & 0x0000FFFF0000FFFF
	x = (x | x>>16) & 0x00000000FFFFFFFF
	return uint32(x)
}
