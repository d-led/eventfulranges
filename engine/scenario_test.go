package engine_test

import (
	"context"
	"sync"
	"testing"

	"pgregory.net/rapid"

	"gitub.com/d-led/eventfulranges/engine"
	"gitub.com/d-led/eventfulranges/interval"
	"gitub.com/d-led/eventfulranges/op"
	"gitub.com/d-led/eventfulranges/store/memory"
	"gitub.com/d-led/eventfulranges/strategy"
)

// runConcurrent executes fn once per slot in its own goroutine and waits for
// all of them. The first error wins; the rest are dropped.
func runConcurrent(slots int, fn func(int) error) error {
	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		first error
	)
	wg.Add(slots)
	for i := 0; i < slots; i++ {
		go func(slot int) {
			defer wg.Done()
			if err := fn(slot); err != nil {
				mu.Lock()
				if first == nil {
					first = err
				}
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	return first
}

// TestPropertyConcurrentReplicasConverge is the Jepsen-style scenario check.
// A history of operations is partitioned across several replicas, applied
// concurrently, then the replicas gossip (anti-entropy). Once every op has
// been delivered everywhere, all replicas must converge to the same value,
// and that value must equal a sequential replay of the whole history (the
// oracle).
func TestPropertyConcurrentReplicasConverge(t *testing.T) {
	t.Parallel()
	strategies := []strategy.Strategy{strategy.LWW, strategy.FWW, strategy.AdditiveWins, strategy.GrowOnly}
	for _, s := range strategies {
		t.Run(s.String(), func(t *testing.T) {
			t.Parallel()
			rapid.Check(t, func(t *rapid.T) {
				history := genOps(t)
				replicas := rapid.IntRange(2, 4).Draw(t, "replicas")

				views, err := runReplicaScenario(s, history, replicas)
				if err != nil {
					t.Fatal(err)
				}

				oracle := strategy.Materialize(s, history)
				for _, got := range views {
					if !interval.Equal(oracle, got) {
						t.Fatalf("%v scenario did not converge to the oracle\n"+
							"  replicas: %d\n"+
							"  history:  %v\n"+
							"  oracle:   %v\n"+
							"  replica:  %v",
							s, replicas, history, oracle, got)
					}
				}
			})
		})
	}
}

// runReplicaScenario applies the history concurrently across replicas, floods
// every op to every replica via full-mesh gossip, and returns one view per
// replica.
func runReplicaScenario(s strategy.Strategy, history []op.Op, replicas int) ([][]interval.Interval, error) {
	ctx := context.Background()
	engines := make([]*engine.Engine, replicas)
	for i := range engines {
		e, err := engine.Open(ctx, memory.New(), s)
		if err != nil {
			return nil, err
		}
		engines[i] = e
	}

	if err := runConcurrent(replicas, func(i int) error {
		var mine []op.Op
		for j := i; j < len(history); j += replicas {
			mine = append(mine, history[j])
		}
		return engines[i].ApplyAll(ctx, mine)
	}); err != nil {
		return nil, err
	}

	var pairs []func() error
	for i := range engines {
		for j := range engines {
			if i == j {
				continue
			}
			pairs = append(pairs, func() error {
				return engines[j].ApplyAll(ctx, engines[i].Ops())
			})
		}
	}
	if err := runConcurrent(len(pairs), func(k int) error {
		return pairs[k]()
	}); err != nil {
		return nil, err
	}

	views := make([][]interval.Interval, replicas)
	for i := range engines {
		views[i] = engines[i].Materialize()
	}
	return views, nil
}

// TestPropertyConcurrentSharedEngine applies the whole history to one shared
// engine from several goroutines at once. No operation may be lost: the final
// view must equal the sequential oracle.
func TestPropertyConcurrentSharedEngine(t *testing.T) {
	t.Parallel()
	strategies := []strategy.Strategy{strategy.LWW, strategy.FWW, strategy.AdditiveWins, strategy.GrowOnly}
	for _, s := range strategies {
		t.Run(s.String(), func(t *testing.T) {
			t.Parallel()
			rapid.Check(t, func(t *rapid.T) {
				history := genOps(t)
				workers := rapid.IntRange(2, 4).Draw(t, "workers")

				ctx := context.Background()
				e, err := engine.Open(ctx, memory.New(), s)
				if err != nil {
					t.Fatal(err)
				}

				if err := runConcurrent(workers, func(i int) error {
					var mine []op.Op
					for j := i; j < len(history); j += workers {
						mine = append(mine, history[j])
					}
					return e.ApplyAll(ctx, mine)
				}); err != nil {
					t.Fatal(err)
				}

				oracle := strategy.Materialize(s, history)
				got := e.Materialize()
				if !interval.Equal(oracle, got) {
					t.Fatalf("%v shared engine lost updates under concurrency\n"+
						"  history: %v\n"+
						"  oracle:  %v\n"+
						"  got:     %v",
						s, history, oracle, got)
				}
			})
		})
	}
}
