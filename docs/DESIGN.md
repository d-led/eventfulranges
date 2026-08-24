# Design

`eventfulranges` is an **operation-based CRDT** over real-valued ranges. This
document explains the model, the conflict-resolution rules, and the design
decisions behind each package.

## Model

The state is a set of disjoint intervals over `float64`. The *history* is an
append-only log of operations:

```
Op = { id, kind (add | remove), interval, ts, origin? }
```

Replicas never reconcile *state*; they reconcile *history*. Two replicas that
have applied the same set of operations — in any order — reach the same set.
This is the defining property, verified by property tests.

## Interval geometry

Intervals are defined in `interval` with **per-side inclusivity**:

- `[a, b]`, `[a, b)`, `(a, b]`, `(a, b)`

All set operations compare endpoints and never do arithmetic on them, so
floating-point values cannot accumulate error: an endpoint is used exactly as
given. `NaN` and infinities are rejected; `Validate` enforces finiteness,
`start <= end`, and that at least one point is contained.

## Conflict resolution

`strategy` materializes a history into a canonical interval set:

- **LWW** — the op with the highest `(ts, id)` wins at each point.
- **FWW** — the op with the lowest `(ts, id)` wins.
- **AdditiveWins** — union of additions minus union of removals.
- **GrowOnly** — union of additions; removals ignored.

`(ts, id)` is a total order: the id breaks timestamp ties. LWW and FWW first
project the ops into winner-annotated *segments* (in priority order, later
ops fill the gaps left by earlier ones), then extract the add-decided
segments. AdditiveWins and GrowOnly keep running unions.

The reference implementation of these semantics is an independent **oracle**
built on `github.com/biogo/store` interval trees, used by the property tests
and nothing else.

## Engine

`engine` binds a `strategy` to a `store` and is safe for concurrent use:

- `Apply` / `ApplyAll` validate ops, stamp any missing timestamp, append to
  the store with an **expected version**, and fold the new ops into the
  cached view.
- On a version conflict (another writer appended first) it reads the missing
  suffix (`catchUp`) and retries, so multiple writers sharing one store do
  not lose updates.
- The view is cached and rebuilt incrementally; `Materialize`, `Contains`,
  `Overlaps` and `Ops` take read locks, writes take the write lock.
- Periodic snapshots persist the materialized view; `reload` restores from the
  newest compatible snapshot and replays any suffix.

The cached view is an optimization only — the log is the source of truth, so
restarting never changes the result.

## Stores

`store.EventStore` is the only storage contract:

```go
Append(ctx, expectedVersion, events) error
Read(ctx, fromVersion) (events, version, error)
SaveSnapshot(ctx, data, version) error
LoadSnapshot(ctx) (data, version, error)
```

Backends:

- `store/memory` — for tests and single-process use.
- `store/jsonl` — one JSON object per line, no server required.
- `store/kurrent` — KurrentDB, behind the `kurrent` build tag.

## Clocks

`op` timestamps are `int64`. Two clocks are provided: a **hybrid logical
clock** (default) and a **Lamport clock**. `Observe` keeps a replica's clock
ahead of every timestamp it has seen, so freshly stamped ops sort after
delivered ones.

## Testing

The tests are behavior- and oracle-driven, not implementation-mirroring:

- **Unit** — `testify`, every package at 100% coverage.
- **Property** — `pgregory.net/rapid` compares each strategy against the
  biogo-tree oracle over random histories, checks order-independence, and
  runs Jepsen-style concurrent scenarios (replicas mutate concurrently, then
  gossip, and must converge to the oracle).
- **Fuzz** — Go native fuzzing over the same invariants.
- **Mutation** — `gremlins`; the efficacy threshold (≥0.8) is the quality
  bar rather than demanding 100%, because a few mutants are provably
  equivalent (capacity-hint arithmetic and strict comparisons guarded by
  `!=`).

## Notable decisions

- **`float64`, not `big.Rat`** — endpoints are compared, never combined, so
  exact rational arithmetic buys nothing for correctness and hurts speed and
  readability. A `big.Rat` variant is a possible future extension.
- **No third-party CRDT or interval library in production code** — the
  semantics are small enough to keep in-repo and fully testable. biogo is a
  test-only oracle, so a bug in *our* code cannot hide behind a shared
  dependency.
- **KurrentDB is optional** — the default backend is a local file, so the
  library and most tests need no network.
