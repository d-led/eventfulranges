// Command kurrent demonstrates the same convergence, but with the event log
// stored in KurrentDB instead of a local file. Build and run it with the
// kurrent tag after starting the database (scripts/kurrent-up.sh).
package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/d-led/eventfulranges"
	"github.com/d-led/eventfulranges/store/kurrent"
	"github.com/d-led/eventfulranges/strategy"
)

// run stores the same add/remove pair in KurrentDB and prints the result.
func run(connectionString string) {
	ctx := context.Background()

	store, err := kurrent.Open(connectionString)
	must("open KurrentDB", err)
	defer func() { _ = store.Close() }()

	set, err := eventfulranges.OpenStore(ctx, store, strategy.LWW)
	must("open the set", err)

	_, err = set.Add(ctx, 1, 10)
	must("add [1,10]", err)

	_, err = set.Remove(ctx, 3, 5)
	must("remove [3,5]", err)

	fmt.Println("materialized:", set.Ranges())
}

// must aborts the demo with what on error. Demos crash loudly on purpose;
// production code should propagate errors instead.
func must(what string, err error) {
	if err != nil {
		panic(fmt.Errorf("%s: %w", what, err))
	}
}

func main() {
	conn := flag.String("conn", "esdb://localhost:2113?tls=false", "KurrentDB connection string")
	flag.Parse()
	run(*conn)
}
