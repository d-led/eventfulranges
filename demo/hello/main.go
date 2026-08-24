// Command hello is the smallest possible eventfulranges program: it opens an
// in-memory set, adds a range, removes a chunk of it, and prints what is left.
// No goroutines, no network — just the plain API.
package main

import (
	"context"
	"fmt"
	"log"

	"gitub.com/d-led/eventfulranges"
	"gitub.com/d-led/eventfulranges/store/memory"
	"gitub.com/d-led/eventfulranges/strategy"
)

func run() error {
	ctx := context.Background()

	set, err := eventfulranges.OpenStore(ctx, memory.New(), strategy.LWW)
	if err != nil {
		return err
	}

	if _, err := set.Add(ctx, 1, 10); err != nil {
		return err
	}
	if _, err := set.Remove(ctx, 3, 5); err != nil {
		return err
	}

	for _, iv := range set.Ranges() {
		fmt.Println(iv)
	}
	fmt.Printf("contains 2: %v\n", set.Contains(2))
	fmt.Printf("contains 4: %v\n", set.Contains(4))
	return nil
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
