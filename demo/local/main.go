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

	"gitub.com/d-led/eventfulranges"
	"gitub.com/d-led/eventfulranges/interval"
	"gitub.com/d-led/eventfulranges/store/memory"
	"gitub.com/d-led/eventfulranges/strategy"
)

const replicas = 3

func run() error {
	ctx := context.Background()

	sets := make([]*eventfulranges.RangeSet, replicas)
	for i := range sets {
		s, err := eventfulranges.OpenStore(ctx, memory.New(), strategy.LWW)
		if err != nil {
			return err
		}
		sets[i] = s
	}

	// Each replica mutates its own copy concurrently.
	errs := make(chan error, replicas)
	var wg sync.WaitGroup
	for i := 0; i < replicas; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s := sets[i]
			if _, err := s.Add(ctx, float64(i), float64(i+4)); err != nil {
				errs <- err
				return
			}
			if i%2 == 0 {
				if _, err := s.Remove(ctx, float64(i)+1, float64(i)+2); err != nil {
					errs <- err
				}
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			return err
		}
	}

	// Anti-entropy: flood every replica's operations to every other replica.
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

	// Every replica must now hold the exact same set.
	base := sets[0].Ranges()
	for i := 1; i < replicas; i++ {
		if !interval.Equal(base, sets[i].Ranges()) {
			return fmt.Errorf("replica %d diverged: %v vs %v", i, sets[i].Ranges(), base)
		}
	}

	fmt.Println("converged to:", base)
	return nil
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
