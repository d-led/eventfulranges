package interval

import "sort"

// Less reports whether a sorts before b in canonical set order.
func Less(a, b Interval) bool {
	if a.Start != b.Start {
		return a.Start < b.Start
	}
	if a.StartBound != b.StartBound {
		return a.StartBound == Closed
	}
	if a.End != b.End {
		return a.End < b.End
	}
	if a.EndBound != b.EndBound {
		return a.EndBound == Closed
	}
	return false
}

// Normalize sorts the intervals, merges overlapping or touching ones, and
// drops intervals that contain no point. Input intervals must satisfy
// Validate.
func Normalize(ivs []Interval) []Interval {
	if len(ivs) == 0 {
		return nil
	}
	sorted := append([]Interval(nil), ivs...)
	sort.Slice(sorted, func(i, j int) bool { return Less(sorted[i], sorted[j]) })
	out := make([]Interval, 0, len(sorted))
	for _, iv := range sorted {
		if iv.empty() {
			continue
		}
		if n := len(out); n > 0 && (out[n-1].Overlaps(iv) || out[n-1].Touches(iv)) {
			out[n-1] = Merge(out[n-1], iv)
		} else {
			out = append(out, iv)
		}
	}
	return out
}

// Union merges two interval sets into one canonical set.
func Union(a, b []Interval) []Interval {
	merged := make([]Interval, 0, len(a)+len(b))
	merged = append(merged, a...)
	merged = append(merged, b...)
	return Normalize(merged)
}

// Difference returns the points of a that are not covered by b, as a
// canonical set.
func Difference(a, b []Interval) []Interval {
	if len(a) == 0 || len(b) == 0 {
		return Normalize(a)
	}
	out := append([]Interval(nil), a...)
	for _, cut := range b {
		next := make([]Interval, 0, len(out)+1)
		for _, iv := range out {
			next = append(next, iv.Subtract(cut)...)
		}
		out = next
	}
	return Normalize(out)
}

// Contains reports whether x belongs to any interval of the set.
func Contains(set []Interval, x float64) bool {
	for _, iv := range set {
		if iv.Contains(x) {
			return true
		}
	}
	return false
}

// Overlaps reports whether any interval of the set shares a point with iv.
func Overlaps(set []Interval, iv Interval) bool {
	for _, x := range set {
		if x.Overlaps(iv) {
			return true
		}
	}
	return false
}

// Equal reports whether the sets contain exactly the same intervals.
func Equal(a, b []Interval) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
