package engine_test

import (
	"runtime"
	"testing"
	"time"

	"github.com/d-led/eventfulranges/space/op"
	"github.com/d-led/eventfulranges/space/strategy"
)

// fixedHistory is a small deterministic workload that exercises additions and
// removals across the grid.
func fixedHistory() []op.Op {
	return []op.Op{
		{ID: "a", Kind: op.KindAdd, TS: 1, Box: box(-2, -2, 2, 2)},
		{ID: "b", Kind: op.KindRemove, TS: 2, Box: box(-1, -1, 0, 0)},
		{ID: "c", Kind: op.KindAdd, TS: 3, Box: box(1, 1, 3, 3)},
		{ID: "d", Kind: op.KindRemove, TS: 4, Box: box(0, 0, 2, 2)},
	}
}

// TestEngineNoGoroutineLeak runs the concurrent replica scenario repeatedly
// and fails if any goroutine survives it.
func TestEngineNoGoroutineLeak(t *testing.T) {
	if _, err := runReplicaScenario(strategy.LWW, fixedHistory(), 3); err != nil {
		t.Fatal(err)
	}

	baseline := runtime.NumGoroutine()
	for i := 0; i < 50; i++ {
		if _, err := runReplicaScenario(strategy.LWW, fixedHistory(), 3); err != nil {
			t.Fatal(err)
		}
	}
	assertGoroutinesSettle(t, baseline)
}

// TestEngineNoMemoryLeak opens and drops engines in a loop and fails if the
// live heap grows with the iteration count rather than the workload.
func TestEngineNoMemoryLeak(t *testing.T) {
	for i := 0; i < 10; i++ {
		if _, err := runReplicaScenario(strategy.LWW, fixedHistory(), 3); err != nil {
			t.Fatal(err)
		}
	}
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	for i := 0; i < 100; i++ {
		if _, err := runReplicaScenario(strategy.LWW, fixedHistory(), 3); err != nil {
			t.Fatal(err)
		}
	}
	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	// A leak would retain live objects across iterations; a fixed workload
	// must leave only a small, constant footprint behind.
	const maxGrowth = 4 << 20 // 4 MiB
	if growth := int64(after.HeapAlloc) - int64(before.HeapAlloc); growth > maxGrowth {
		t.Fatalf("heap grew by %d bytes across iterations", growth)
	}
}

// assertGoroutinesSettle fails unless the goroutine count returns to baseline
// within a short window (shutdown is asynchronous).
func assertGoroutinesSettle(t *testing.T, baseline int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if got := runtime.NumGoroutine(); got <= baseline {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("goroutine leak: %d goroutines before, %d after", baseline, runtime.NumGoroutine())
		}
		time.Sleep(10 * time.Millisecond)
	}
}
