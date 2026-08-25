// Command pubsub demonstrates convergence over an in-process pub/sub bus.
// Each replica keeps its own in-memory set, mutates it independently, and then
// broadcasts its operations on the shared bus. Every replica folds in every
// broadcast it receives, so they all converge without talking to each other
// directly.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/cskr/pubsub/v2"

	"gitub.com/d-led/eventfulranges"
	"gitub.com/d-led/eventfulranges/interval"
	"gitub.com/d-led/eventfulranges/op"
	"gitub.com/d-led/eventfulranges/store/memory"
	"gitub.com/d-led/eventfulranges/strategy"
)

const (
	topic    = "ops"
	replicas = 3
)

// replica pairs a range set with its subscription to the bus.
type replica struct {
	set *eventfulranges.RangeSet
	ch  chan []op.Op
}

func run() error {
	ctx := context.Background()
	// The buffer holds one batch per replica, so every broadcast fits.
	bus := pubsub.New[string, []op.Op](replicas)

	reps, err := openReplicas(ctx, bus)
	if err != nil {
		return err
	}
	if err := broadcast(ctx, bus, reps); err != nil {
		return err
	}
	if err := fold(ctx, reps); err != nil {
		return err
	}
	view, err := converged(reps)
	if err != nil {
		return err
	}
	fmt.Println("converged to:", view)
	return nil
}

// openReplicas opens one replica per slot, each subscribed to the bus.
func openReplicas(ctx context.Context, bus *pubsub.PubSub[string, []op.Op]) ([]replica, error) {
	reps := make([]replica, replicas)
	for i := range reps {
		set, err := eventfulranges.OpenStore(ctx, memory.New(), strategy.LWW)
		if err != nil {
			return nil, err
		}
		reps[i] = replica{set: set, ch: bus.Sub(topic)}
	}
	return reps, nil
}

// broadcast lets each replica edit only its own copy, then publish its ops.
func broadcast(ctx context.Context, bus *pubsub.PubSub[string, []op.Op], reps []replica) error {
	for i := range reps {
		if err := edit(ctx, reps[i].set, i); err != nil {
			return err
		}
		bus.Pub(reps[i].set.Ops(), topic)
	}
	return nil
}

// edit applies one replica's own edits.
func edit(ctx context.Context, set *eventfulranges.RangeSet, i int) error {
	if _, err := set.Add(ctx, float64(i), float64(i+4)); err != nil {
		return err
	}
	if i%2 != 0 {
		return nil
	}
	_, err := set.Remove(ctx, float64(i)+1, float64(i)+2)
	return err
}

// fold applies every broadcast to every replica. ApplyAll ignores duplicates,
// so the order of arrival does not matter.
func fold(ctx context.Context, reps []replica) error {
	for i := range reps {
		for range reps {
			if err := reps[i].set.ApplyAll(ctx, <-reps[i].ch); err != nil {
				return err
			}
		}
	}
	return nil
}

// converged asserts every replica holds the same set and returns it.
func converged(reps []replica) ([]interval.Interval, error) {
	base := reps[0].set.Ranges()
	for i := 1; i < len(reps); i++ {
		if !interval.Equal(base, reps[i].set.Ranges()) {
			return nil, fmt.Errorf("replica %d diverged: %v vs %v", i, reps[i].set.Ranges(), base)
		}
	}
	return base, nil
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
