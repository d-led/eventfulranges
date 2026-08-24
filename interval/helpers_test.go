package interval_test

import (
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"gitub.com/d-led/eventfulranges/interval"
)

// gridInterval builds a valid interval from grid endpoints.
func gridInterval(start, end int, sb, eb interval.Bound) interval.Interval {
	iv := interval.Interval{
		Start:      float64(start),
		End:        float64(end),
		StartBound: sb,
		EndBound:   eb,
	}
	if start == end {
		iv.StartBound, iv.EndBound = interval.Closed, interval.Closed
	}
	return iv
}

// genValidInterval draws a valid interval with endpoints on the -3..3 grid.
func genValidInterval(t *rapid.T) *rapid.Generator[interval.Interval] {
	return rapid.Custom(func(t *rapid.T) interval.Interval {
		start := rapid.IntRange(-3, 3).Draw(t, "start")
		end := rapid.IntRange(start, 3).Draw(t, "end")
		sb := rapid.SampledFrom([]interval.Bound{interval.Closed, interval.Open}).Draw(t, "sb")
		eb := rapid.SampledFrom([]interval.Bound{interval.Closed, interval.Open}).Draw(t, "eb")
		if start == end {
			sb, eb = interval.Closed, interval.Closed
		}
		return interval.Interval{
			Start:      float64(start),
			End:        float64(end),
			StartBound: sb,
			EndBound:   eb,
		}
	})
}

// samplePoints returns grid points and midpoints covering every open segment.
func samplePoints() []float64 {
	pts := make([]float64, 0, 15)
	for k := -7; k <= 7; k++ {
		pts = append(pts, float64(k)/2)
	}
	return pts
}

// anyContains reports whether any interval of the set contains x.
func anyContains(set []interval.Interval, x float64) bool {
	for _, iv := range set {
		if iv.Contains(x) {
			return true
		}
	}
	return false
}

// intervalsFromBytes derives valid grid intervals deterministically.
func intervalsFromBytes(data []byte) []interval.Interval {
	if len(data) < 2 {
		return nil
	}
	n := int(data[0]%6) + 1
	out := make([]interval.Interval, 0, n)
	for i := 0; i < n && i*2+2 <= len(data); i++ {
		start := int(data[i*2]%7) - 3
		end := int(data[i*2+1]%7) - 3
		if start > end {
			start, end = end, start
		}
		sb, eb := interval.Closed, interval.Closed
		if data[i*2]&1 == 0 {
			sb = interval.Open
		}
		if data[i*2+1]&1 == 0 {
			eb = interval.Open
		}
		if start == end {
			sb, eb = interval.Closed, interval.Closed
		}
		out = append(out, interval.Interval{
			Start:      float64(start),
			End:        float64(end),
			StartBound: sb,
			EndBound:   eb,
		})
	}
	return out
}

// testLogger is the subset of *testing.T used by requireCanonical, so it
// works for both regular and property-based tests.
type testLogger interface {
	Helper()
	Errorf(format string, args ...any)
	FailNow()
	Fatalf(format string, args ...any)
}

// requireCanonical asserts that the set is sorted, non-overlapping,
// non-touching and free of empty intervals.
func requireCanonical(t testLogger, set []interval.Interval) {
	t.Helper()
	for _, iv := range set {
		require.NoError(t, iv.Validate(), "set must contain only valid intervals: %v", set)
	}
	for i := 1; i < len(set); i++ {
		prev, cur := set[i-1], set[i]
		if prev.Overlaps(cur) || prev.Touches(cur) || !interval.Less(prev, cur) {
			t.Fatalf("not canonical: %v then %v in %v", prev, cur, set)
		}
	}
}
