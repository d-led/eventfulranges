package morton

import (
	"math/rand"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 2000; i++ {
		x := randCoord(rng)
		y := randCoord(rng)
		code, err := Encode(x, y)
		require.NoError(t, err)
		dx, dy := Decode(code)
		require.Equal(t, x, dx, "x round trip for code %d", code)
		require.Equal(t, y, dy, "y round trip for code %d", code)
	}
}

func TestEncodeRejectsOutOfRange(t *testing.T) {
	t.Parallel()
	for _, pt := range [][2]int64{{bias, 0}, {-bias - 1, 0}, {0, bias}, {0, -bias - 1}} {
		_, err := Encode(pt[0], pt[1])
		require.ErrorIs(t, err, ErrOutOfRange, "point (%d,%d)", pt[0], pt[1])
	}
}

func TestSingleCellIsOneRange(t *testing.T) {
	t.Parallel()
	code, err := Encode(3, -7)
	require.NoError(t, err)
	ranges, err := Ranges(3, -7, 4, -6)
	require.NoError(t, err)
	require.Equal(t, []Range{{Lo: code, Hi: code + 1}}, ranges)
}

func TestRangesMatchesBruteForceExhaustive(t *testing.T) {
	t.Parallel()
	// Every rectangle within an 8x8 cell grid.
	for x0 := int64(0); x0 < 8; x0++ {
		for y0 := int64(0); y0 < 8; y0++ {
			for x1 := x0 + 1; x1 <= 8; x1++ {
				for y1 := y0 + 1; y1 <= 8; y1++ {
					assertRanges(t, x0, y0, x1, y1)
				}
			}
		}
	}
}

func TestRangesMatchesBruteForceRandom(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewSource(2))
	for i := 0; i < 500; i++ {
		x0 := rng.Int63n(32) - 16
		y0 := rng.Int63n(32) - 16
		x1 := x0 + 1 + rng.Int63n(16)
		y1 := y0 + 1 + rng.Int63n(16)
		if x1 > 16 || y1 > 16 {
			continue
		}
		assertRanges(t, x0, y0, x1, y1)
	}
}

func TestRangesRejectsBadInput(t *testing.T) {
	t.Parallel()
	_, err := Ranges(4, 0, 2, 2)
	require.Error(t, err, "inverted x")

	_, err = Ranges(0, 0, 2, 2)
	require.NoError(t, err)

	_, err = Ranges(-bias-1, 0, 1, 1)
	require.ErrorIs(t, err, ErrOutOfRange)

	_, err = Ranges(0, 0, bias+1, 1)
	require.ErrorIs(t, err, ErrOutOfRange)
}

// assertRanges checks that the efficient decomposition equals the brute-force
// enumeration of every cell's Morton code, merged into consecutive runs.
func assertRanges(t *testing.T, x0, y0, x1, y1 int64) {
	t.Helper()
	got, err := Ranges(x0, y0, x1, y1)
	require.NoError(t, err)
	require.Equal(t, bruteRanges(x0, y0, x1, y1), got, "rectangle (%d,%d)-(%d,%d)", x0, y0, x1, y1)
}

func bruteRanges(x0, y0, x1, y1 int64) []Range {
	var codes []uint64
	for x := x0; x < x1; x++ {
		for y := y0; y < y1; y++ {
			code, _ := Encode(x, y)
			codes = append(codes, code)
		}
	}
	sort.Slice(codes, func(i, j int) bool { return codes[i] < codes[j] })
	var out []Range
	for _, code := range codes {
		if n := len(out); n > 0 && code == out[n-1].Hi {
			out[n-1].Hi++
		} else {
			out = append(out, Range{Lo: code, Hi: code + 1})
		}
	}
	return out
}

func randCoord(rng *rand.Rand) int64 {
	return rng.Int63n(2*bias) - bias
}
