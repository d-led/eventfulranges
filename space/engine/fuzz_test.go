package engine_test

import (
	"context"
	"testing"

	"github.com/d-led/eventfulranges/space"
	"github.com/d-led/eventfulranges/space/engine"
	"github.com/d-led/eventfulranges/space/store/memory"
	"github.com/d-led/eventfulranges/space/strategy"
)

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
			if !space.Equal(a.Materialize(), b.Materialize()) {
				t.Fatalf("%v not order independent: %v vs %v", s, a.Materialize(), b.Materialize())
			}
		}
	})
}
