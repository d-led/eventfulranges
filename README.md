# eventfulranges

[![CI](https://github.com/d-led/eventfulranges/actions/workflows/ci.yml/badge.svg)](https://github.com/d-led/eventfulranges/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/d-led/eventfulranges/branch/main/graph/badge.svg?token=NOT_REQUIRED)](https://codecov.io/gh/d-led/eventfulranges)
[![Go Reference](https://pkg.go.dev/badge/github.com/d-led/eventfulranges.svg)](https://pkg.go.dev/github.com/d-led/eventfulranges)
[![License: MPL-2.0](https://img.shields.io/badge/license-MPL--2.0-blue.svg)](LICENSE)

An event-sourced **CRDT for real-valued ranges**. Ranges can be added and
removed from any number of replicas, in any order, over any transport — and
every replica that has seen the same operations converges to the same set.

Storage and transport are both swappable. A replica can keep its operations in
a JSON Lines file, in memory, or in any backend you write against the
`store.Log` interface. Replicas converge by exchanging operations over whatever
transport you already have — goroutine channels, an in-process pub/sub bus,
plain HTTP, or an event database such as
[KurrentDB](https://kurrent.io) (behind the `kurrent` build tag).

## Quick start

```go
set, _ := eventfulranges.Open(ctx, "./example", strategy.LWW) // ./example/ranges.stream.jsonl
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

## Storage & transport

A replica is a `store.Log` plus a strategy. Three backends ship in the repo,
and you can plug in your own:

```go
set, _ := eventfulranges.Open(ctx, "./example", strategy.LWW)         // JSON Lines stream (default)
set, _ := eventfulranges.OpenStore(ctx, memory.New(), strategy.LWW)   // in memory
set, _ := eventfulranges.OpenStore(ctx, myBackend, strategy.LWW)      // your own store.Log
```

`Open` keeps the append-only event stream as JSON Lines
(`./example/ranges.stream.jsonl`) and caches the materialized view in a
sidecar snapshot (`./example/ranges.snapshot.json`). The stream is the source
of truth; the snapshot only fast-forwards a restart.

Transport is yours to choose, too: convergence is just shipping `Ops()` and
calling `ApplyAll`. See [Demos](#demos) for channels, a pub/sub bus, and HTTP,
and [KurrentDB](#kurrentdb) for the event-database backend.

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

## Coordinates

Range endpoints are `float64`. Integer literals convert exactly while they fit
a float64's 53-bit mantissa (`|n| <= 2^53`); fractional values are stored and
compared verbatim, so no rounding error accumulates. There is no
arbitrary-precision (`math/big`) coordinate type: endpoints beyond `2^53` round
to the nearest representable value. `int64` appears only as the operation
timestamp, never as a coordinate.

## Demos

```
go run ./demo/hello    # simplest use, no concurrency
go run ./demo/local    # goroutine replicas converge over channels
go run ./demo/pubsub   # replicas converge over an in-process pub/sub bus
go run ./demo/network  # two HTTP peers converge
go run ./demo/web      # interactive 3D visualizer, shared live over WebSockets
```

`demo/hello` opens an in-memory set and prints what a single add/remove leaves
behind — the smallest possible program.

`demo/local` opens three in-memory replicas, lets each mutate its own copy from
a goroutine, then floods every replica's `Ops()` to every other replica until
they agree. The transport is Go channels; there is no network.

`demo/pubsub` is the same idea through a bus: each replica subscribes to a
topic on a `github.com/cskr/pubsub/v2` bus, mutates locally, and publishes its
operations. Every replica applies every broadcast it receives, so they converge
without talking to each other directly.

`demo/network` runs two replicas, each behind its own HTTP server. There is no
CRDT-specific protocol — a peer exports its log as `GET /ops` (JSON) and folds
someone else's log in with `POST /ops`. Each peer mutates its own copy, then
the two exchange logs and converge; ports come from `-ports 18080,18081`.

`demo/web` serves an n-dimensional range-set visualizer (1–4 dimensions, with a
rotatable translucent-box 3D view and copy/pasteable CSV). Everyone connected
to the same instance shares one view: each `add`/`remove` is folded with
additive-wins semantics and broadcast over a WebSocket, so concurrent edits
converge regardless of order. Open `http://localhost:8080/ui/`.

One command starts it, and the other scripts cover the rest:

```bash
./scripts/demo-web.sh     # start the web visualizer (open the printed URL)
./scripts/build-web.sh    # (re)build the embedded UI (npm install + esbuild)
./scripts/itest-web.sh    # smoke test: unit tests + server serves the UI
./scripts/e2e-web.sh      # Playwright end-to-end tests
```

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
./scripts/update-dependencies.sh # bump every module to its latest deps
```

## KurrentDB

```bash
./scripts/kurrent-up.sh          # docker compose up -d (needs Docker)
./scripts/itest-kurrent.sh       # integration tests (build tag kurrent)
./scripts/kurrent-down.sh
```

## License

[MPL-2.0](LICENSE)
