package interval_test

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"gitub.com/d-led/eventfulranges/interval"
)

func TestBoundString(t *testing.T) {
	t.Parallel()
	require.Equal(t, "closed", interval.Closed.String())
	require.Equal(t, "open", interval.Open.String())
}

func TestBoundText(t *testing.T) {
	t.Parallel()
	t.Run("marshal", func(t *testing.T) {
		for b, want := range map[interval.Bound]string{interval.Closed: "closed", interval.Open: "open"} {
			data, err := b.MarshalText()
			require.NoError(t, err)
			require.Equal(t, want, string(data))
		}
	})
	t.Run("unmarshal", func(t *testing.T) {
		var b interval.Bound
		require.NoError(t, b.UnmarshalText([]byte("closed")))
		require.Equal(t, interval.Closed, b)
		require.NoError(t, b.UnmarshalText([]byte("open")))
		require.Equal(t, interval.Open, b)
		require.Error(t, b.UnmarshalText([]byte("half")))
	})
}

func TestValidate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		iv   interval.Interval
		err  error
	}{
		{name: "closed", iv: gridInterval(0, 1, interval.Closed, interval.Closed)},
		{name: "half open right", iv: gridInterval(0, 1, interval.Closed, interval.Open)},
		{name: "half open left", iv: gridInterval(0, 1, interval.Open, interval.Closed)},
		{name: "open", iv: gridInterval(0, 1, interval.Open, interval.Open)},
		{name: "point", iv: gridInterval(1, 1, interval.Closed, interval.Closed)},
		{name: "zero value", iv: interval.Interval{}},
		{name: "nan start", iv: interval.Interval{Start: math.NaN(), End: 1}, err: interval.ErrNaN},
		{name: "nan end", iv: interval.Interval{Start: 0, End: math.NaN()}, err: interval.ErrNaN},
		{name: "infinite", iv: interval.Interval{Start: math.Inf(1), End: 1}, err: interval.ErrInfinite},
		{name: "inverted", iv: interval.Interval{Start: 2, End: 1}, err: interval.ErrInverted},
		{name: "empty open", iv: interval.Interval{Start: 1, End: 1, StartBound: interval.Open, EndBound: interval.Open}, err: interval.ErrEmpty},
		{name: "empty left open", iv: interval.Interval{Start: 1, End: 1, StartBound: interval.Open}, err: interval.ErrEmpty},
		{name: "empty right open", iv: interval.Interval{Start: 1, End: 1, EndBound: interval.Open}, err: interval.ErrEmpty},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.ErrorIs(t, tt.iv.Validate(), tt.err)
		})
	}
}

func TestContains(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		iv   interval.Interval
		in   []float64
		out  []float64
	}{
		{name: "closed", iv: gridInterval(1, 5, interval.Closed, interval.Closed), in: []float64{1, 3, 5}, out: []float64{0, 6}},
		{name: "half open right", iv: gridInterval(1, 5, interval.Closed, interval.Open), in: []float64{1, 3}, out: []float64{0, 5, 6}},
		{name: "half open left", iv: gridInterval(1, 5, interval.Open, interval.Closed), in: []float64{3, 5}, out: []float64{0, 1, 6}},
		{name: "open", iv: gridInterval(1, 5, interval.Open, interval.Open), in: []float64{3}, out: []float64{0, 1, 5, 6}},
		{name: "point", iv: gridInterval(2, 2, interval.Closed, interval.Closed), in: []float64{2}, out: []float64{1, 3}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			for _, x := range tt.in {
				require.True(t, tt.iv.Contains(x), "%v should contain %v", tt.iv, x)
			}
			for _, x := range tt.out {
				require.False(t, tt.iv.Contains(x), "%v should not contain %v", tt.iv, x)
			}
		})
	}
}

func TestString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		iv   interval.Interval
		want string
	}{
		{gridInterval(1, 5, interval.Closed, interval.Closed), "[1,5]"},
		{gridInterval(1, 5, interval.Closed, interval.Open), "[1,5)"},
		{gridInterval(1, 5, interval.Open, interval.Closed), "(1,5]"},
		{gridInterval(1, 5, interval.Open, interval.Open), "(1,5)"},
		{interval.Interval{}, "[0,0]"},
	}
	for _, tt := range tests {
		require.Equal(t, tt.want, tt.iv.String())
	}
}

func TestOverlaps(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		a, b interval.Interval
		want bool
	}{
		{"shared point both closed", gridInterval(0, 2, interval.Closed, interval.Closed), gridInterval(2, 3, interval.Closed, interval.Closed), true},
		{"shared point one open", gridInterval(0, 2, interval.Closed, interval.Open), gridInterval(2, 3, interval.Closed, interval.Closed), false},
		{"shared point both open", gridInterval(0, 2, interval.Closed, interval.Open), gridInterval(2, 3, interval.Open, interval.Closed), false},
		{"left closed right open", gridInterval(0, 2, interval.Closed, interval.Closed), gridInterval(2, 3, interval.Open, interval.Closed), false},
		{"right closed left open", gridInterval(0, 2, interval.Open, interval.Closed), gridInterval(2, 3, interval.Closed, interval.Closed), true},
		{"contained", gridInterval(0, 3, interval.Closed, interval.Closed), gridInterval(1, 2, interval.Closed, interval.Closed), true},
		{"disjoint", gridInterval(0, 1, interval.Closed, interval.Closed), gridInterval(5, 6, interval.Closed, interval.Closed), false},
		{"point at start", interval.Interval{}, gridInterval(0, 1, interval.Closed, interval.Closed), true},
		{"open containing closed", gridInterval(0, 2, interval.Open, interval.Open), gridInterval(0, 3, interval.Closed, interval.Closed), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, tt.a.Overlaps(tt.b))
			require.Equal(t, tt.want, tt.b.Overlaps(tt.a), "overlap must be symmetric")
		})
	}
}

func TestTouches(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		a, b interval.Interval
		want bool
	}{
		{"overlap is not touch", gridInterval(0, 2, interval.Closed, interval.Closed), gridInterval(2, 3, interval.Closed, interval.Closed), false},
		{"left open right closed", gridInterval(0, 2, interval.Closed, interval.Open), gridInterval(2, 3, interval.Closed, interval.Closed), true},
		{"both open", gridInterval(0, 2, interval.Closed, interval.Open), gridInterval(2, 3, interval.Open, interval.Closed), false},
		{"reversed order", gridInterval(2, 3, interval.Closed, interval.Closed), gridInterval(0, 2, interval.Closed, interval.Open), true},
		{"far apart", gridInterval(0, 1, interval.Closed, interval.Closed), gridInterval(2, 3, interval.Closed, interval.Closed), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, tt.a.Touches(tt.b))
			require.Equal(t, tt.want, tt.b.Touches(tt.a), "touch must be symmetric")
		})
	}
}

func TestMerge(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		a, b interval.Interval
		want interval.Interval
	}{
		{"touch with junction covered", gridInterval(0, 2, interval.Closed, interval.Open), gridInterval(2, 3, interval.Closed, interval.Closed), gridInterval(0, 3, interval.Closed, interval.Closed)},
		{"overlap", gridInterval(0, 2, interval.Closed, interval.Closed), gridInterval(2, 3, interval.Closed, interval.Closed), gridInterval(0, 3, interval.Closed, interval.Closed)},
		{"both open right", gridInterval(0, 2, interval.Closed, interval.Open), gridInterval(2, 3, interval.Closed, interval.Open), gridInterval(0, 3, interval.Closed, interval.Open)},
		{"contained", gridInterval(1, 2, interval.Closed, interval.Closed), gridInterval(0, 4, interval.Closed, interval.Closed), gridInterval(0, 4, interval.Closed, interval.Closed)},
		{"equal starts", gridInterval(0, 2, interval.Open, interval.Closed), gridInterval(0, 4, interval.Closed, interval.Closed), gridInterval(0, 4, interval.Closed, interval.Closed)},
		{"equal ends", gridInterval(0, 4, interval.Closed, interval.Open), gridInterval(2, 4, interval.Closed, interval.Closed), gridInterval(0, 4, interval.Closed, interval.Closed)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, interval.Merge(tt.a, tt.b))
			require.Equal(t, tt.want, interval.Merge(tt.b, tt.a), "merge must be symmetric")
		})
	}
}

func TestSubtract(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		iv, cut interval.Interval
		want    []interval.Interval
	}{
		{
			name: "hole in the middle",
			iv:   gridInterval(1, 7, interval.Closed, interval.Closed),
			cut:  gridInterval(2, 3, interval.Closed, interval.Closed),
			want: []interval.Interval{
				gridInterval(1, 2, interval.Closed, interval.Open),
				gridInterval(3, 7, interval.Open, interval.Closed),
			},
		},
		{
			name: "trim right edge",
			iv:   gridInterval(1, 5, interval.Closed, interval.Closed),
			cut:  gridInterval(3, 7, interval.Closed, interval.Closed),
			want: []interval.Interval{gridInterval(1, 3, interval.Closed, interval.Open)},
		},
		{
			name: "open cut",
			iv:   gridInterval(1, 7, interval.Closed, interval.Closed),
			cut:  gridInterval(2, 3, interval.Open, interval.Open),
			want: []interval.Interval{
				gridInterval(1, 2, interval.Closed, interval.Closed),
				gridInterval(3, 7, interval.Closed, interval.Closed),
			},
		},
		{
			name: "cut starts at start",
			iv:   gridInterval(1, 7, interval.Closed, interval.Closed),
			cut:  gridInterval(1, 3, interval.Closed, interval.Closed),
			want: []interval.Interval{gridInterval(3, 7, interval.Open, interval.Closed)},
		},
		{
			name: "cut covers all",
			iv:   gridInterval(1, 7, interval.Closed, interval.Closed),
			cut:  gridInterval(0, 8, interval.Closed, interval.Closed),
			want: nil,
		},
		{
			name: "cut covers interval exactly",
			iv:   gridInterval(1, 7, interval.Closed, interval.Closed),
			cut:  gridInterval(1, 7, interval.Closed, interval.Closed),
			want: nil,
		},
		{
			name: "cut at the end",
			iv:   gridInterval(1, 7, interval.Closed, interval.Closed),
			cut:  gridInterval(5, 9, interval.Closed, interval.Closed),
			want: []interval.Interval{gridInterval(1, 5, interval.Closed, interval.Open)},
		},
		{
			name: "cut at the start",
			iv:   gridInterval(1, 7, interval.Closed, interval.Closed),
			cut:  gridInterval(0, 2, interval.Closed, interval.Closed),
			want: []interval.Interval{gridInterval(2, 7, interval.Open, interval.Closed)},
		},
		{
			name: "point cut",
			iv:   gridInterval(1, 7, interval.Closed, interval.Closed),
			cut:  gridInterval(3, 3, interval.Closed, interval.Closed),
			want: []interval.Interval{
				gridInterval(1, 3, interval.Closed, interval.Open),
				gridInterval(3, 7, interval.Open, interval.Closed),
			},
		},
		{
			name: "point cut inside open interval",
			iv:   gridInterval(1, 7, interval.Open, interval.Open),
			cut:  gridInterval(3, 3, interval.Closed, interval.Closed),
			want: []interval.Interval{
				gridInterval(1, 3, interval.Open, interval.Open),
				gridInterval(3, 7, interval.Open, interval.Open),
			},
		},
		{
			name: "disjoint right",
			iv:   gridInterval(1, 7, interval.Closed, interval.Closed),
			cut:  gridInterval(9, 11, interval.Closed, interval.Closed),
			want: []interval.Interval{gridInterval(1, 7, interval.Closed, interval.Closed)},
		},
		{
			name: "disjoint left",
			iv:   gridInterval(1, 7, interval.Closed, interval.Closed),
			cut:  gridInterval(-2, 0, interval.Closed, interval.Closed),
			want: []interval.Interval{gridInterval(1, 7, interval.Closed, interval.Closed)},
		},
		{
			name: "cut shares open end",
			iv:   gridInterval(1, 7, interval.Closed, interval.Closed),
			cut:  gridInterval(7, 9, interval.Open, interval.Closed),
			want: []interval.Interval{gridInterval(1, 7, interval.Closed, interval.Closed)},
		},
		{
			name: "cut shares closed end",
			iv:   gridInterval(1, 7, interval.Closed, interval.Closed),
			cut:  gridInterval(7, 9, interval.Closed, interval.Closed),
			want: []interval.Interval{gridInterval(1, 7, interval.Closed, interval.Open)},
		},
		{
			name: "point fully covered",
			iv:   gridInterval(2, 2, interval.Closed, interval.Closed),
			cut:  gridInterval(1, 3, interval.Closed, interval.Closed),
			want: nil,
		},
		{
			name: "point cut at point",
			iv:   gridInterval(2, 2, interval.Closed, interval.Closed),
			cut:  gridInterval(2, 2, interval.Closed, interval.Closed),
			want: nil,
		},
		{
			name: "point cut at closed start of open interval",
			iv:   gridInterval(2, 5, interval.Closed, interval.Closed),
			cut:  gridInterval(1, 1, interval.Closed, interval.Closed),
			want: []interval.Interval{gridInterval(2, 5, interval.Closed, interval.Closed)},
		},
		{
			name: "left open point survives",
			iv:   gridInterval(2, 5, interval.Closed, interval.Closed),
			cut:  gridInterval(0, 2, interval.Open, interval.Open),
			want: []interval.Interval{gridInterval(2, 5, interval.Closed, interval.Closed)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, tt.iv.Subtract(tt.cut))
		})
	}
}

func TestIntersection(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		a, b interval.Interval
		want interval.Interval
	}{
		{"contained", gridInterval(1, 7, interval.Closed, interval.Closed), gridInterval(2, 3, interval.Closed, interval.Closed), gridInterval(2, 3, interval.Closed, interval.Closed)},
		{"shared point", gridInterval(0, 2, interval.Closed, interval.Closed), gridInterval(2, 3, interval.Closed, interval.Closed), gridInterval(2, 2, interval.Closed, interval.Closed)},
		{"mixed bounds", gridInterval(0, 2, interval.Open, interval.Closed), gridInterval(1, 3, interval.Open, interval.Closed), gridInterval(1, 2, interval.Open, interval.Closed)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.True(t, tt.a.Overlaps(tt.b))
			require.Equal(t, tt.want, interval.Intersection(tt.a, tt.b))
			require.Equal(t, tt.want, interval.Intersection(tt.b, tt.a))
		})
	}
}

func TestLess(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		a, b   interval.Interval
		before bool
	}{
		{"by start", gridInterval(0, 2, interval.Closed, interval.Closed), gridInterval(1, 1, interval.Closed, interval.Closed), true},
		{"closed start first", gridInterval(0, 2, interval.Closed, interval.Closed), gridInterval(0, 2, interval.Open, interval.Closed), true},
		{"by end", gridInterval(1, 1, interval.Closed, interval.Closed), gridInterval(1, 2, interval.Closed, interval.Closed), true},
		{"closed end first", gridInterval(1, 2, interval.Closed, interval.Closed), gridInterval(1, 2, interval.Closed, interval.Open), true},
		{"equal", gridInterval(1, 2, interval.Closed, interval.Closed), gridInterval(1, 2, interval.Closed, interval.Closed), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.before, interval.Less(tt.a, tt.b))
		})
	}
}

func TestNormalize(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   []interval.Interval
		want []interval.Interval
	}{
		{name: "empty", in: nil, want: nil},
		{
			name: "sorts and merges",
			in: []interval.Interval{
				gridInterval(3, 4, interval.Closed, interval.Closed),
				gridInterval(1, 2, interval.Closed, interval.Closed),
				gridInterval(2, 3, interval.Closed, interval.Closed),
			},
			want: []interval.Interval{gridInterval(1, 4, interval.Closed, interval.Closed)},
		},
		{
			name: "both open do not merge",
			in: []interval.Interval{
				gridInterval(1, 2, interval.Closed, interval.Open),
				gridInterval(2, 3, interval.Open, interval.Closed),
			},
			want: []interval.Interval{
				gridInterval(1, 2, interval.Closed, interval.Open),
				gridInterval(2, 3, interval.Open, interval.Closed),
			},
		},
		{
			name: "one open one closed merge",
			in: []interval.Interval{
				gridInterval(1, 2, interval.Closed, interval.Open),
				gridInterval(2, 3, interval.Closed, interval.Closed),
			},
			want: []interval.Interval{gridInterval(1, 3, interval.Closed, interval.Closed)},
		},
		{
			name: "point merges with neighbor",
			in: []interval.Interval{
				gridInterval(2, 2, interval.Closed, interval.Closed),
				gridInterval(2, 3, interval.Closed, interval.Closed),
			},
			want: []interval.Interval{gridInterval(2, 3, interval.Closed, interval.Closed)},
		},
		{
			name: "drops empty",
			in: []interval.Interval{
				{Start: 5, End: 5, StartBound: interval.Open, EndBound: interval.Open},
				gridInterval(1, 2, interval.Closed, interval.Closed),
			},
			want: []interval.Interval{gridInterval(1, 2, interval.Closed, interval.Closed)},
		},
		{
			name: "duplicates collapse",
			in: []interval.Interval{
				gridInterval(1, 2, interval.Closed, interval.Closed),
				gridInterval(1, 2, interval.Closed, interval.Closed),
			},
			want: []interval.Interval{gridInterval(1, 2, interval.Closed, interval.Closed)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, interval.Normalize(tt.in))
		})
	}
}

func TestUnion(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		a, b []interval.Interval
		want []interval.Interval
	}{
		{
			name: "overlapping additions merge",
			a:    []interval.Interval{gridInterval(1, 5, interval.Closed, interval.Closed)},
			b:    []interval.Interval{gridInterval(3, 7, interval.Closed, interval.Closed)},
			want: []interval.Interval{gridInterval(1, 7, interval.Closed, interval.Closed)},
		},
		{
			name: "disjoint stay separate",
			a:    []interval.Interval{gridInterval(0, 1, interval.Closed, interval.Closed)},
			b:    []interval.Interval{gridInterval(2, 3, interval.Closed, interval.Closed)},
			want: []interval.Interval{
				gridInterval(0, 1, interval.Closed, interval.Closed),
				gridInterval(2, 3, interval.Closed, interval.Closed),
			},
		},
		{name: "empty left", a: nil, b: []interval.Interval{gridInterval(0, 1, interval.Closed, interval.Closed)}, want: []interval.Interval{gridInterval(0, 1, interval.Closed, interval.Closed)}},
		{name: "empty right", a: []interval.Interval{gridInterval(0, 1, interval.Closed, interval.Closed)}, b: nil, want: []interval.Interval{gridInterval(0, 1, interval.Closed, interval.Closed)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, interval.Union(tt.a, tt.b))
		})
	}
}

func TestDifference(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		a, b []interval.Interval
		want []interval.Interval
	}{
		{
			name: "one cut gives two ranges",
			a:    []interval.Interval{gridInterval(1, 7, interval.Closed, interval.Closed)},
			b:    []interval.Interval{gridInterval(2, 3, interval.Closed, interval.Closed)},
			want: []interval.Interval{
				gridInterval(1, 2, interval.Closed, interval.Open),
				gridInterval(3, 7, interval.Open, interval.Closed),
			},
		},
		{
			name: "two cuts give three ranges",
			a:    []interval.Interval{gridInterval(0, 10, interval.Closed, interval.Closed)},
			b: []interval.Interval{
				gridInterval(1, 2, interval.Closed, interval.Closed),
				gridInterval(5, 6, interval.Closed, interval.Closed),
			},
			want: []interval.Interval{
				gridInterval(0, 1, interval.Closed, interval.Open),
				gridInterval(2, 5, interval.Open, interval.Open),
				gridInterval(6, 10, interval.Open, interval.Closed),
			},
		},
		{
			name: "disjoint cuts keep everything",
			a:    []interval.Interval{gridInterval(1, 2, interval.Closed, interval.Closed)},
			b:    []interval.Interval{gridInterval(5, 6, interval.Closed, interval.Closed)},
			want: []interval.Interval{gridInterval(1, 2, interval.Closed, interval.Closed)},
		},
		{name: "empty minuend", a: nil, b: []interval.Interval{gridInterval(5, 6, interval.Closed, interval.Closed)}, want: nil},
		{name: "empty subtrahend", a: []interval.Interval{gridInterval(1, 2, interval.Closed, interval.Closed)}, b: nil, want: []interval.Interval{gridInterval(1, 2, interval.Closed, interval.Closed)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, interval.Difference(tt.a, tt.b))
		})
	}
}

func TestSetQueries(t *testing.T) {
	t.Parallel()
	set := []interval.Interval{
		gridInterval(1, 2, interval.Closed, interval.Closed),
		gridInterval(4, 5, interval.Closed, interval.Closed),
	}
	t.Run("contains", func(t *testing.T) {
		require.True(t, interval.Contains(set, 1.5))
		require.False(t, interval.Contains(set, 3))
		require.False(t, interval.Contains(nil, 1))
	})
	t.Run("overlaps", func(t *testing.T) {
		require.True(t, interval.Overlaps(set, gridInterval(5, 6, interval.Closed, interval.Closed)))
		require.False(t, interval.Overlaps(set, gridInterval(3, 4, interval.Open, interval.Open)))
		require.False(t, interval.Overlaps(nil, gridInterval(0, 9, interval.Closed, interval.Closed)))
	})
	t.Run("equal", func(t *testing.T) {
		require.True(t, interval.Equal(set, append([]interval.Interval(nil), set...)))
		require.False(t, interval.Equal(set, []interval.Interval{gridInterval(1, 2, interval.Closed, interval.Closed)}))
		require.False(t, interval.Equal(set, []interval.Interval{
			gridInterval(1, 2, interval.Closed, interval.Closed),
			gridInterval(7, 8, interval.Closed, interval.Closed),
		}))
		require.True(t, interval.Equal(nil, nil))
	})
}

func TestIntervalJSON(t *testing.T) {
	t.Parallel()
	iv := gridInterval(1, 5, interval.Closed, interval.Open)
	data, err := json.Marshal(iv)
	require.NoError(t, err)
	require.JSONEq(t, `{"start":1,"end":5,"startBound":"closed","endBound":"open"}`, string(data))

	var back interval.Interval
	require.NoError(t, json.Unmarshal(data, &back))
	require.Equal(t, iv, back)

	var bad interval.Interval
	require.Error(t, json.Unmarshal([]byte(`{"start":0,"end":1,"startBound":"half","endBound":"closed"}`), &bad))
}
