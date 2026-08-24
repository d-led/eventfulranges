// Command kurrent demonstrates the same convergence, but with the event log
// stored in KurrentDB instead of a local file. Build and run it with the
// kurrent tag after starting the database (scripts/kurrent-up.sh).
package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"gitub.com/d-led/eventfulranges"
	"gitub.com/d-led/eventfulranges/store/kurrent"
	"gitub.com/d-led/eventfulranges/strategy"
)

func run(connectionString string) error {
	ctx := context.Background()

	store, err := kurrent.Open(connectionString)
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()

	set, err := eventfulranges.OpenStore(ctx, store, strategy.LWW)
	if err != nil {
		return err
	}

	if _, err := set.Add(ctx, 1, 10); err != nil {
		return err
	}
	if _, err := set.Remove(ctx, 3, 5); err != nil {
		return err
	}

	fmt.Println("materialized:", set.Ranges())
	return nil
}

func main() {
	conn := flag.String("conn", "kurrentdb://localhost:2113?tls=false", "KurrentDB connection string")
	flag.Parse()
	if err := run(*conn); err != nil {
		log.Fatal(err)
	}
}
