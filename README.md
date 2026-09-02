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

## Use case: a shared calendar

The full program is in [`examples/calendar`](examples/calendar). A date is
just a day number (`float64`), so a date range is a plain interval:

```go
cal, _ := eventfulranges.OpenStore(ctx, memory.New(), strategy.AdditiveWins)
book, cancel := func(f, t string) { _, _ = cal.Add(ctx, days(f), days(t)) },
                func(f, t string) { _, _ = cal.Remove(ctx, days(f), days(t)) }

book("2026-07-01", "2026-07-10")   // Alice's vacation
book("2026-07-06", "2026-07-15")   // Bob's vacation (overlaps)
cancel("2026-07-08", "2026-07-10") // Alice cuts the trip short

cal.Contains(days("2026-07-01")) // true  — booked
cal.Contains(days("2026-07-08")) // false — cancelled
cal.Contains(days("2026-07-12")) // true  — Bob's still away
```

```mermaid
gantt
    title       Shared calendar — AdditiveWins
    dateFormat  YYYY-MM-DD
    axisFormat  %m-%d

    section Start (bookings)
    Alice books               :a, 2026-07-01, 10d
    Bob books                 :b, 2026-07-06, 10d

    section Operation
    Alice cancels             :crit, c, 2026-07-08, 3d

    section Result
    Busy (Alice, then Alice+Bob) :done,   r1, 2026-07-01, 7d
    Free                         :active, r2, 2026-07-08, 3d
    Busy (Bob)                   :done,   r3, 2026-07-11, 5d
```

`AdditiveWins` makes the busy set the union of all bookings minus all
cancellations, so concurrent edits converge no matter the order.

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

| Strategy       | Semantics                                          |
| -------------- | -------------------------------------------------- |
| `LWW`          | Highest `(timestamp, id)` wins at each point       |
| `FWW`          | Lowest `(timestamp, id)` wins at each point        |
| `AdditiveWins` | Union of all additions minus union of all removals |
| `GrowOnly`     | Union of all additions, removals ignored           |

## Packages

| Package                       | Purpose                                              |
| ----------------------------- | ---------------------------------------------------- |
| `interval`                    | 1-D open/closed intervals with canonical set algebra |
| `op`                          | The append-only operation (`add` / `remove`)         |
| `clock`                       | Hybrid logical clock and Lamport clock               |
| `strategy`                    | Conflict resolution: materializes ops to a set       |
| `engine`                      | Concurrency-safe log + view, snapshotting            |
| `store`                       | The `EventStore` interface (append/read/snapshot)    |
| `store/memory`, `store/jsonl` | In-memory and file backends                          |
| `space`                       | n-dimensional generalization (half-open boxes)       |

The public facade is the root package `eventfulranges`.

## Coordinates

Range endpoints are `float64`. Integer literals convert exactly while they fit
a float64's 53-bit mantissa (`|n| <= 2^53`); fractional values are stored and
compared verbatim, so no rounding error accumulates. There is no
arbitrary-precision (`math/big`) coordinate type: endpoints beyond `2^53` round
to the nearest representable value. `int64` is used only for bookkeeping —
operation timestamps and log-version counters — never as a coordinate.

## Demos

- [hello](#hello) — simplest use, no concurrency
- [local](#local) — goroutine replicas converge over channels
- [pubsub](#pubsub) — replicas converge over an in-process pub/sub bus
- [network](#network) — two HTTP peers converge
- [web](#web) — interactive 3D visualizer, shared live over WebSockets

### hello

```shell
go run ./demo/hello
```

`demo/hello` opens an in-memory set and prints what a single add/remove leaves
behind — the smallest possible program.

### local

```shell
go run ./demo/local
```

`demo/local` opens three in-memory replicas, lets each mutate its own copy from
a goroutine, then floods every replica's `Ops()` to every other replica until
they agree. The transport is Go channels; there is no network.

### pubsub

```shell
go run ./demo/pubsub
```

`demo/pubsub` is the same idea through a bus: each replica subscribes to a
topic on a `github.com/cskr/pubsub/v2` bus, mutates locally, and publishes its
operations. Every replica applies every broadcast it receives, so they converge
without talking to each other directly.

### network

```shell
go run ./demo/network
```

`demo/network` runs two replicas, each behind its own HTTP server. There is no
CRDT-specific protocol — a peer exports its log as `GET /ops` (JSON) and folds
someone else's log in with `POST /ops`. Each peer mutates its own copy, then
the two exchange logs and converge; ports come from `-ports 18080,18081`.

### web

```shell
go run ./demo/web
```

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

#### Running purely in the browser (WebAssembly)

The same visualizer also runs with **no Go server at all**: the session hub is
compiled to WebAssembly (`GOOS=js GOARCH=wasm`) and executes inside the page,
so the UI works from any static host. The page cannot tell the difference —
both builds speak the same JSON envelopes and reuse the same rendering code.
The seam is two switches in the code:

- **A session-engine interface** (`demo/web/ui-src/server-engine.js` vs
  `local-engine.js`): the WebSocket transport, or a direct call into the
  in-page Go engine (`demo/web/wasm.go`). Pick one with `?engine=local`, or by
  opening the local build, whose page preselects it.
- **A log repository** (`demo/web/persist.go` vs the browser's `localStorage`
  reserve): the server appends each operation to a JSONL file; the in-page
  engine is replayed from the browser's local copy on reload, exactly as a
  reconnecting socket would be healed.

```bash
./scripts/build-local.sh  # build demo/web/dist-local: UI + engine.wasm + wasm runtime
./scripts/serve-local.sh  # build and serve it statically (any static host works)
./scripts/e2e-local.sh    # Playwright tests against the in-page wasm engine
```

Each demo has a smoke test; run them with `go test ./demo/...`.

![web demo 2d](./docs/img/eventfulranges-2d-demo.gif)

### paint

```shell
go run ./demo/paint
```

`demo/paint` is an infinite, shared pixel whiteboard built on the library's
n-dimensional range CRDT. Each stroke is one half-open `add`/`remove` box, so
a filled rectangle of cells is a single operation. Browsers receive the
operation log and materialize the view themselves — pure event sourcing — and
concurrent strokes converge regardless of arrival order. The share link is the
session URL, and the raw operation log is one click away as JSONL. Open
`http://localhost:8081/ui/`.

```bash
./scripts/demo-paint.sh     # start the whiteboard (open the printed URL)
./scripts/build-paint.sh    # (re)build the embedded UI (npm install + esbuild)
./scripts/itest-paint.sh    # smoke test: Go tests + server serves the UI
./scripts/e2e-paint.sh      # vitest unit tests + Playwright end-to-end tests
```

#### Admin area

The server exposes an admin area (`/admin/`, see `demo/paint/ui-src/admin.html`)
for inspecting instance storage and deleting inactive sessions. In the deployed
build the area is gated by the `ADMIN_EMAILS` reverse-proxy allow-list, so only
listed users reach it. In a development build (no `embed` build tag) the admin
gate opens to direct requests when `ADMIN_EMAILS` is unset, so the area is
usable locally without an oauth2-proxy; when `ADMIN_EMAILS` is set it is gated
exactly as in deployment. The dev-only gate is compiled out of embedded
artifacts, so it can never leak into a deployed binary.

#### Related — infinite canvases and zoom-first editors

- [tldraw](https://tldraw.dev) — open-source infinite canvas, real-time collaboration
- [Excalidraw](https://excalidraw.com) — infinite canvas, CRDT (Yjs) collaboration ([source](https://github.com/excalidraw/excalidraw))
- [Miro](https://miro.com) — infinite canvas, real-time collaboration
- [FigJam](https://www.figma.com/figjam/) — infinite canvas, real-time collaboration
- [InfiniPaint](https://infinipaint.com) — collaborative canvas with no zoom limit
- [Endless Paper](https://www.endlesspaper.app) — single-user infinite canvas
- [Prezi](https://prezi.com) — the zoomable-canvas presentation paradigm

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
