package engine_test

import (
	"context"
	"strconv"
	"testing"

	"gitub.com/d-led/eventfulranges/engine"
	"gitub.com/d-led/eventfulranges/interval"
	"gitub.com/d-led/eventfulranges/op"
	"gitub.com/d-led/eventfulranges/store/memory"
	"gitub.com/d-led/eventfulranges/strategy"
)

func opsFromBytes(data []byte) []op.Op {
	if len(data) < 3 {
		return nil
	}
	n := int(data[0]%8) + 1
	ops := make([]op.Op, 0, n)
	for i := 0; i < n && i*3+3 <= len(data); i++ {
		start := int(data[i*3]%7) - 3
		end := int(data[i*3+1]%7) - 3
		if start > end {
			start, end = end, start
		}
		sb, eb := interval.Closed, interval.Closed
		if data[i*3]&1 == 0 {
			sb = interval.Open
		}
		if data[i*3+1]&1 == 0 {
			eb = interval.Open
		}
		if start == end {
			sb, eb = interval.Closed, interval.Closed
		}
		kind := op.KindAdd
		if data[i*3+2]&1 == 0 {
			kind = op.KindRemove
		}
		ops = append(ops, op.Op{
			ID:       strconv.Itoa(i),
			Kind:     kind,
			TS:       int64(data[i*3+2]%4) + 1,
			Interval: interval.Interval{Start: float64(start), End: float64(end), StartBound: sb, EndBound: eb},
		})
	}
	return ops
}

func reverseOps(ops []op.Op) []op.Op {
	out := make([]op.Op, len(ops))
	for i, o := range ops {
		out[len(ops)-1-i] = o
	}
	return out
}

func FuzzEngineDeterministic(f *testing.F) {
	f.Add([]byte("eventfulranges"))
	f.Fuzz(func(t *testing.T, data []byte) {
		ops := opsFromBytes(data)
		ctx := context.Background()
		for _, s := range []strategy.Strategy{strategy.LWW, strategy.FWW, strategy.AdditiveWins, strategy.GrowOnly} {
			a, err := engine.Open(ctx, memory.New(), s)
			if err != nil {
				t.Fatal(err)
			}
			b, err := engine.Open(ctx, memory.New(), s)
			if err != nil {
				t.Fatal(err)
			}
			if err := a.ApplyAll(ctx, ops); err != nil {
				t.Fatal(err)
			}
			if err := b.ApplyAll(ctx, reverseOps(ops)); err != nil {
				t.Fatal(err)
			}
			if !interval.Equal(a.Materialize(), b.Materialize()) {
				t.Fatalf("%v not order independent: %v vs %v", s, a.Materialize(), b.Materialize())
			}
		}
	})
}
