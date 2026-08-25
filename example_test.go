package eventfulranges_test

import (
	"context"
	"fmt"
	"os"

	"github.com/d-led/eventfulranges"
	"github.com/d-led/eventfulranges/strategy"
)

func Example() {
	ctx := context.Background()
	dir, err := os.MkdirTemp("", "eventfulranges-example")
	if err != nil {
		panic(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	rs, err := eventfulranges.Open(ctx, dir, strategy.LWW)
	if err != nil {
		panic(err)
	}

	_, _ = rs.Add(ctx, 1, 5)    // [1,5]
	_, _ = rs.Add(ctx, 3, 7)    // merges to [1,7]
	_, _ = rs.Remove(ctx, 2, 3) // cuts a hole

	for _, iv := range rs.Ranges() {
		fmt.Println(iv)
	}
	// Output:
	// [1,2)
	// (3,7]
}
