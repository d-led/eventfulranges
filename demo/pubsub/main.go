// Command pubsub demonstrates convergence over an in-process pub/sub bus.
// Each replica keeps its own in-memory set, mutates it independently, and then
// broadcasts its operations on the shared bus. Every replica folds in every
// broadcast it receives, so they all converge without talking to each other
// directly.
package main

import (
	"context"
	"fmt"

	"github.com/cskr/pubsub/v2"

	"github.com/d-led/eventfulranges"
	"github.com/d-led/eventfulranges/interval"
	"github.com/d-led/eventfulranges/op"
	"github.com/d-led/eventfulranges/store/memory"
	"github.com/d-led/eventfulranges/strategy"
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

// run opens three replicas, edits each one, and folds every broadcast until
// they all converge.
func run() {
	ctx := context.Background()
	// The buffer holds one batch per replica, so every broadcast fits.
	bus := pubsub.New[string, []op.Op](replicas)

	reps := openReplicas(ctx, bus)
	broadcast(ctx, bus, reps)
	fold(ctx, reps)
	fmt.Println("converged to:", converged(reps))
}

// openReplicas opens one replica per slot, each subscribed to the bus.
func openReplicas(ctx context.Context, bus *pubsub.PubSub[string, []op.Op]) []replica {
	reps := make([]replica, replicas)
	for i := range reps {
		set, err := eventfulranges.OpenStore(ctx, memory.New(), strategy.LWW)
		must("open a replica", err)
		reps[i] = replica{set: set, ch: bus.Sub(topic)}
	}
	return reps
}

// broadcast lets each replica edit only its own copy, then publish its ops.
func broadcast(ctx context.Context, bus *pubsub.PubSub[string, []op.Op], reps []replica) {
	for i := range reps {
		edit(ctx, reps[i].set, i)
		bus.Pub(reps[i].set.Ops(), topic)
	}
}

// edit applies one replica's own edits.
func edit(ctx context.Context, set *eventfulranges.RangeSet, i int) {
	_, err := set.Add(ctx, float64(i), float64(i+4))
	must("add own range", err)
	if i%2 != 0 {
		return
	}
	_, err = set.Remove(ctx, float64(i)+1, float64(i)+2)
	must("remove own range", err)
}

// fold applies every broadcast to every replica. ApplyAll ignores duplicates,
// so the order of arrival does not matter.
func fold(ctx context.Context, reps []replica) {
	for i := range reps {
		for range reps {
			must("apply broadcast", reps[i].set.ApplyAll(ctx, <-reps[i].ch))
		}
	}
}

// converged returns the shared set, panicking if any replica diverged.
func converged(reps []replica) []interval.Interval {
	base := reps[0].set.Ranges()
	for i := 1; i < len(reps); i++ {
		if !interval.Equal(base, reps[i].set.Ranges()) {
			panic(fmt.Errorf("replica %d diverged: %v vs %v", i, reps[i].set.Ranges(), base))
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
