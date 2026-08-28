// Command hello is the smallest possible eventfulranges program: it opens an
// in-memory set, adds a range, removes a chunk of it, and prints what is left.
// No goroutines, no network — just the plain API.
package main

import (
	"context"
	"fmt"

	"github.com/d-led/eventfulranges"
	"github.com/d-led/eventfulranges/store/memory"
	"github.com/d-led/eventfulranges/strategy"
)

// run is the whole demo: add [1,10], remove [3,5], print the leftovers.
func run() {
	ctx := context.Background()

	set, err := eventfulranges.OpenStore(ctx, memory.New(), strategy.LWW)
	must("open the set", err)

	_, err = set.Add(ctx, 1, 10)
	must("add [1,10]", err)

	_, err = set.Remove(ctx, 3, 5)
	must("remove [3,5]", err)

	for _, iv := range set.Ranges() {
		fmt.Println(iv)
	}
	fmt.Printf("contains 2: %v\n", set.Contains(2))
	fmt.Printf("contains 4: %v\n", set.Contains(4))
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
