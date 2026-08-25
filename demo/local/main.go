// Command local demonstrates convergence without any network: several
// goroutines, each owning its own replica of the set, mutate their copies
// concurrently and then gossip their operations around until every replica
// has seen every operation and they all agree.
package main

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/d-led/eventfulranges"
	"github.com/d-led/eventfulranges/interval"
	"github.com/d-led/eventfulranges/store/memory"
	"github.com/d-led/eventfulranges/strategy"
)

const replicas = 3

func run() error {
	ctx := context.Background()
	sets, err := openReplicas(ctx, replicas)
	if err != nil {
		return err
	}
	if err := mutate(ctx, sets); err != nil {
		return err
	}
	if err := gossip(ctx, sets); err != nil {
		return err
	}
	ranges, err := converged(sets)
	if err != nil {
		return err
	}
	fmt.Println("converged to:", ranges)
	return nil
}

// openReplicas opens n independent in-memory replicas.
func openReplicas(ctx context.Context, n int) ([]*eventfulranges.RangeSet, error) {
	sets := make([]*eventfulranges.RangeSet, n)
	for i := range sets {
		s, err := eventfulranges.OpenStore(ctx, memory.New(), strategy.LWW)
		if err != nil {
			return nil, err
		}
		sets[i] = s
	}
	return sets, nil
}

// mutate lets each replica change its own copy concurrently.
func mutate(ctx context.Context, sets []*eventfulranges.RangeSet) error {
	errs := make(chan error, len(sets))
	var wg sync.WaitGroup
	for i, s := range sets {
		wg.Add(1)
		go func(i int, s *eventfulranges.RangeSet) {
			defer wg.Done()
			writeLocal(ctx, i, s, errs)
		}(i, s)
	}
	wg.Wait()
	close(errs)
	return firstError(errs)
}

// writeLocal applies one replica's own edits.
func writeLocal(ctx context.Context, i int, s *eventfulranges.RangeSet, errs chan<- error) {
	if _, err := s.Add(ctx, float64(i), float64(i+4)); err != nil {
		errs <- err
		return
	}
	if i%2 == 0 {
		if _, err := s.Remove(ctx, float64(i)+1, float64(i)+2); err != nil {
			errs <- err
		}
	}
}

// firstError drains errs and returns the first non-nil error.
func firstError(errs <-chan error) error {
	for err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// gossip floods every replica's operations to every other replica.
func gossip(ctx context.Context, sets []*eventfulranges.RangeSet) error {
	for i := range sets {
		for j := range sets {
			if i == j {
				continue
			}
			if err := sets[j].ApplyAll(ctx, sets[i].Ops()); err != nil {
				return err
			}
		}
	}
	return nil
}

// converged asserts every replica holds the same set and returns it.
func converged(sets []*eventfulranges.RangeSet) ([]interval.Interval, error) {
	base := sets[0].Ranges()
	for i := 1; i < len(sets); i++ {
		if !interval.Equal(base, sets[i].Ranges()) {
			return nil, fmt.Errorf("replica %d diverged: %v vs %v", i, sets[i].Ranges(), base)
		}
	}
	return base, nil
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
