# calendar

**What it is.** A standalone consumer of
[`eventfulranges`](https://github.com/d-led/eventfulranges) that proves the
published module resolves **by version**: no `replace` directive, just
`require github.com/d-led/eventfulranges v0.0.1`.

**The idea.** A date is a whole number of days since the Unix epoch, so a date
range is a plain `float64` interval.

**What it does.**

1. Books two overlapping vacations.
2. Cancels part of one.
3. Prints the busy ranges and per-date availability.

Run it:

```bash
cd examples/calendar
GOWORK=off go run .
```

`GOWORK=off` matters only when running from inside this repository: the root
`go.work` would otherwise resolve the import to the local module instead of
fetching `v0.0.1` from the module proxy. Outside the repository (for example a
fresh checkout of just this folder) no flag is needed.

**Read next:** [CRDT map](../../docs/CRDT.md) ·
[root README](../../README.md) · [Design](../../docs/DESIGN.md)
