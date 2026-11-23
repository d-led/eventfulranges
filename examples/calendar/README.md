# calendar

A standalone consumer of
[`eventfulranges`](https://github.com/d-led/eventfulranges) that proves the
published module resolves **by version**: no `replace` directive, just
`require github.com/d-led/eventfulranges v0.0.1`.

A date is a whole number of days since the Unix epoch, so a date range is a
plain `float64` interval. The program books two overlapping vacations, cancels
part of one, then prints the busy ranges and per-date availability.

```bash
cd examples/calendar
GOWORK=off go run .
```

`GOWORK=off` matters only when running from inside this repository: the root
`go.work` would otherwise resolve the import to the local module instead of
fetching `v0.0.1` from the module proxy. A fresh checkout of just this folder
needs no flag.

**Read next:** [CRDT map](../../docs/CRDT.md) ·
[root README](../../README.md) · [Design](../../docs/DESIGN.md)
