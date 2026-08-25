// Command calendar demonstrates eventfulranges on calendar dates.
//
// A date is stored as a whole number of days since the Unix epoch, so a date
// range is just a float64 interval (day numbers stay exact in a float64 for
// millennia). This module is deliberately standalone: it has its own go.mod
// and depends on the published module github.com/d-led/eventfulranges@v0.0.1,
// with no replace directive, to prove that versioned imports resolve.
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/d-led/eventfulranges"
	"github.com/d-led/eventfulranges/store/memory"
	"github.com/d-led/eventfulranges/strategy"
)

const day = 24 * time.Hour

// days maps a date to its day number (days since the Unix epoch, UTC).
func days(t time.Time) float64 { return float64(t.Unix() / int64(day/time.Second)) }

// dateOf maps a day number back to a UTC date.
func dateOf(d float64) time.Time { return time.Unix(int64(d)*int64(day/time.Second), 0).UTC() }

// date parses a YYYY-MM-DD string.
func date(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func main() {
	ctx := context.Background()

	// AdditiveWins means the busy calendar is the union of everyone's
	// bookings minus cancellations, so concurrent edits converge.
	cal, err := eventfulranges.OpenStore(ctx, memory.New(), strategy.AdditiveWins)
	if err != nil {
		panic(err)
	}

	book := func(from, to string) {
		if _, err := cal.Add(ctx, days(date(from)), days(date(to))); err != nil {
			panic(err)
		}
	}
	cancel := func(from, to string) {
		if _, err := cal.Remove(ctx, days(date(from)), days(date(to))); err != nil {
			panic(err)
		}
	}

	book("2026-07-01", "2026-07-10")   // Alice's vacation
	book("2026-07-06", "2026-07-15")   // Bob's vacation (overlaps Alice's)
	cancel("2026-07-08", "2026-07-10") // Alice cuts the trip short

	fmt.Println("busy ranges:")
	for _, r := range cal.Ranges() {
		fmt.Printf("  %s  (%s .. %s)\n", r.String(),
			dateOf(r.Start).Format("2006-01-02"),
			dateOf(r.End).Format("2006-01-02"))
	}

	fmt.Println("availability:")
	for _, d := range []string{
		"2026-07-01", "2026-07-05", "2026-07-08", "2026-07-09",
		"2026-07-10", "2026-07-12", "2026-07-15", "2026-07-20",
	} {
		fmt.Printf("  %s busy? %v\n", d, cal.Contains(days(date(d))))
	}
}
