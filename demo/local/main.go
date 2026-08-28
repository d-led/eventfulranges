// Command local demonstrates convergence without any network: several
// goroutines, each owning its own replica of the set, mutate their copies
// concurrently and then gossip their operations around until every replica
// has seen every operation and they all agree.
package main

import (
	"context"
	"fmt"
	"sync"

	"github.com/d-led/eventfulranges"
	"github.com/d-led/eventfulranges/interval"
	"github.com/d-led/eventfulranges/store/memory"
	"github.com/d-led/eventfulranges/strategy"
)

const replicas = 3

// run opens three replicas, mutates them concurrently, and gossips their
// operations until every replica holds the same set.
func run() {
	ctx := context.Background()
	sets := openReplicas(ctx, replicas)
	mutate(ctx, sets)
	gossip(ctx, sets)
	fmt.Println("converged to:", converged(sets))
}

// openReplicas opens n independent in-memory replicas.
func openReplicas(ctx context.Context, n int) []*eventfulranges.RangeSet {
	sets := make([]*eventfulranges.RangeSet, n)
	for i := range sets {
		s, err := eventfulranges.OpenStore(ctx, memory.New(), strategy.LWW)
		must("open a replica", err)
		sets[i] = s
	}
	return sets
}

// mutate lets each replica change its own copy concurrently.
func mutate(ctx context.Context, sets []*eventfulranges.RangeSet) {
	var wg sync.WaitGroup
	for i, s := range sets {
		wg.Add(1)
		go func(i int, s *eventfulranges.RangeSet) {
			defer wg.Done()
			writeLocal(ctx, i, s)
		}(i, s)
	}
	wg.Wait()
}

// writeLocal applies one replica's own edits.
func writeLocal(ctx context.Context, i int, s *eventfulranges.RangeSet) {
	_, err := s.Add(ctx, float64(i), float64(i+4))
	must("add own range", err)
	if i%2 == 0 {
		_, err = s.Remove(ctx, float64(i)+1, float64(i)+2)
		must("remove own range", err)
	}
}

// gossip floods every replica's operations to every other replica.
func gossip(ctx context.Context, sets []*eventfulranges.RangeSet) {
	for i := range sets {
		for j := range sets {
			if i == j {
				continue
			}
			must("apply gossip", sets[j].ApplyAll(ctx, sets[i].Ops()))
		}
	}
}

// converged returns the shared set, panicking if any replica diverged.
func converged(sets []*eventfulranges.RangeSet) []interval.Interval {
	base := sets[0].Ranges()
	for i := 1; i < len(sets); i++ {
		if !interval.Equal(base, sets[i].Ranges()) {
			panic(fmt.Errorf("replica %d diverged: %v vs %v", i, sets[i].Ranges(), base))
		}
	}
	return base
}

// must aborts the demo with what on error. Demos crash loudly on purpose;
// production code should propagate errors instead.
func must(what string, err error) {
	if err != nil {
		panic(fmt.Errorf("%s: %w", what, err))
	}
}

func main() {
	run()
}
