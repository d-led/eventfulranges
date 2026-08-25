# eventfulranges

[![CI](https://github.com/d-led/go-eventfulranges/actions/workflows/ci.yml/badge.svg)](https://github.com/d-led/go-eventfulranges/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/d-led/go-eventfulranges/branch/main/graph/badge.svg?token=NOT_REQUIRED)](https://codecov.io/gh/d-led/go-eventfulranges)
[![Go Reference](https://pkg.go.dev/badge/gitub.com/d-led/eventfulranges.svg)](https://pkg.go.dev/gitub.com/d-led/eventfulranges)
[![License: MPL-2.0](https://img.shields.io/badge/license-MPL--2.0-blue.svg)](LICENSE)

An event-sourced **CRDT for real-valued ranges**. Ranges can be added and
removed from any number of replicas, in any order, over any transport — and
every replica that has seen the same operations converges to the same set.

The default backend is a JSON Lines file and needs no network at all. A
[KurrentDB](https://kurrent.io) backend is available behind the `kurrent`
build tag.

## Quick start

```go
set, err := eventfulranges.Open(ctx, "/tmp/example", strategy.LWW)
if err != nil {
    log.Fatal(err)
}
_, _ = set.Add(ctx, 1, 10)   // [1,10]
_, _ = set.Remove(ctx, 3, 5) // cut a hole
for _, iv := range set.Ranges() {
    fmt.Println(iv) // [1,3) (5,10]
}
```

Replicas converge by exchanging operations, not by reconciling state:

```go
// replica A and B each mutated independently ...
_ = a.ApplyAll(ctx, b.Ops())
_ = b.ApplyAll(ctx, a.Ops())
// a.Ranges() == b.Ranges()
```

## Strategies

| Strategy      | Semantics                                              |
| ------------- | ------------------------------------------------------ |
| `LWW`         | Highest `(timestamp, id)` wins at each point           |
| `FWW`         | Lowest `(timestamp, id)` wins at each point            |
| `AdditiveWins`| Union of all additions minus union of all removals     |
| `GrowOnly`    | Union of all additions, removals ignored               |

## Packages

| Package    | Purpose                                              |
| ---------- | ---------------------------------------------------- |
| `interval` | 1-D open/closed intervals with canonical set algebra |
| `op`       | The append-only operation (`add` / `remove`)         |
| `clock`    | Hybrid logical clock and Lamport clock                |
| `strategy` | Conflict resolution: materializes ops to a set       |
| `engine`   | Concurrency-safe log + view, snapshotting            |
| `store`    | The `EventStore` interface (append/read/snapshot)    |
| `store/memory`, `store/jsonl` | In-memory and file backends   |
| `space`    | n-dimensional generalization (half-open boxes)       |

The public facade is the root package `eventfulranges`.

## Demos

```
go run ./demo/hello    # simplest use, no concurrency
go run ./demo/local    # goroutine replicas converge without a network
go run ./demo/network  # two HTTP peers converge
go run ./demo/web      # interactive 3D visualizer, shared live over WebSockets
```

The web demo serves an n-dimensional range-set visualizer (1–4 dimensions,
with a rotatable translucent-box 3D view and copy/pasteable CSV). Everyone
connected to the same instance shares one view: each `add`/`remove` is folded
with additive-wins semantics and broadcast over a WebSocket, so concurrent
edits converge regardless of order. Open `http://localhost:8080/ui/`.

Each demo has a smoke test; run them with `go test ./demo/...`.

## Quality

Everything is checked by `scripts/quality-gate.sh` and on CI:

- `gofumpt` formatting
- `golangci-lint` (low-complexity and duplicate-code gates)
- unit tests with the race detector and a **100% coverage** gate
- property-based tests (`pgregory.net/rapid`) checked against a **biogo
  interval-tree oracle**, plus Jepsen-style concurrent scenarios
- fuzz smoke tests (Go native fuzzing)
- mutation testing (`gremlins`, ≥80% efficacy)

Run it locally:

```bash
./scripts/test.sh        # fast: unit tests + coverage report
./scripts/quality-gate.sh # full: format, lint, tests, property, fuzz, mutation
```

## KurrentDB

```bash
./scripts/kurrent-up.sh          # docker compose up -d (needs Docker)
./scripts/itest-kurrent.sh       # integration tests (build tag kurrent)
./scripts/kurrent-down.sh
```

## License

[MPL-2.0](LICENSE)
