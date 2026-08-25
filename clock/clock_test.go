package clock_test

import (
	"math"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/d-led/eventfulranges/clock"
)

func TestLamportTick(t *testing.T) {
	t.Parallel()
	c := clock.NewLamport()
	require.Equal(t, int64(1), c.Tick())
	require.Equal(t, int64(2), c.Tick())
	require.Equal(t, int64(3), c.Tick())
}

func TestLamportObserve(t *testing.T) {
	t.Parallel()
	c := clock.NewLamport()
	c.Observe(10)
	require.Equal(t, int64(11), c.Tick())
	c.Observe(3) // must not move the clock backwards
	require.Equal(t, int64(12), c.Tick())
}

func TestHybridTickIsStrictlyIncreasing(t *testing.T) {
	t.Parallel()
	var h clock.Hybrid
	first := h.Tick()
	second := h.Tick()
	require.Less(t, first, second)
}

func TestHybridObserve(t *testing.T) {
	t.Parallel()
	var h clock.Hybrid
	observed := int64(math.MaxInt64 - 10)
	h.Observe(observed)
	require.Greater(t, h.Tick(), observed)
}

func TestHybridConcurrentTicksAreUnique(t *testing.T) {
	t.Parallel()
	var h clock.Hybrid
	const goroutines = 32
	seen := make(map[int64]struct{}, goroutines)
	var mu sync.Mutex
	var wg sync.WaitGroup
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ts := h.Tick()
			mu.Lock()
			seen[ts] = struct{}{}
			mu.Unlock()
		}()
	}
	wg.Wait()
	require.Len(t, seen, goroutines)
}
