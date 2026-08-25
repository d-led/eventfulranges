# calendar

A standalone consumer of [`eventfulranges`](https://github.com/d-led/eventfulranges)
that verifies the published module resolves **by version** — no `replace`
directive, just `require github.com/d-led/eventfulranges v0.0.1`.

Dates are represented as whole days since the Unix epoch, so a date range is a
`float64` interval. The example books two overlapping vacations, cancels part
of one, and prints the resulting busy ranges and per-date availability.

```bash
cd examples/calendar
GOWORK=off go run .
```

`GOWORK=off` matters only when running from inside this repository: the root
`go.work` would otherwise resolve the import to the local module instead of
fetching `v0.0.1` from the module proxy. Outside the repository (e.g. a fresh
checkout of just this folder) no flag is needed.
