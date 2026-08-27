package clock_test

import (
	"testing"

	"github.com/d-led/eventfulranges/clock"
)

// BenchmarkHybridTick measures the lock-free hybrid timestamp hot path.
func BenchmarkHybridTick(b *testing.B) {
	var h clock.Hybrid
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = h.Tick()
	}
}

// BenchmarkLamportTick measures the atomic logical-counter hot path.
func BenchmarkLamportTick(b *testing.B) {
	c := clock.NewLamport()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = c.Tick()
	}
}

// BenchmarkHybridTickParallel measures contention on the shared hybrid clock.
func BenchmarkHybridTickParallel(b *testing.B) {
	var h clock.Hybrid
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = h.Tick()
		}
	})
}

// BenchmarkLamportTickParallel measures contention on the shared lamport clock.
func BenchmarkLamportTickParallel(b *testing.B) {
	c := clock.NewLamport()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = c.Tick()
		}
	})
}
