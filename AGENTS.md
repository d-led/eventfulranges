# AGENTS.md

`eventfulranges` is a Go CRDT over real-valued ranges, with an n-dimensional box
counterpart under `space`. History is an append-only operation log; the
materialized view is a cache.

## Build, test, verify

```shell
./scripts/test.sh          # library tests with coverage, then the demo module's tests
./scripts/quality-gate.sh  # format, lint, tests, property, fuzz, mutation, UI builds, Playwright
./scripts/fmt.sh           # gofumpt
./scripts/lint.sh          # go vet, staticcheck, golangci-lint, gocyclo, revive, gosec, govulncheck, jscpd
./scripts/property.sh      # rapid properties against the biogo oracle
./scripts/fuzz.sh          # Go native fuzz smoke tests
./scripts/mutation.sh      # gremlins, at least 80% efficacy
./scripts/itest-kurrent.sh # KurrentDB integration tests; needs ./scripts/kurrent-up.sh
```

`go.work` holds two modules: the root library and `demo`. The 100% coverage bar
covers library packages only; the demos have smoke tests.

## Layout

| Path | Contents |
| --- | --- |
| `eventfulranges.go`, `boxes.go` | the facades: `RangeSet` and `BoxSet` |
| `interval/` | 1-D interval algebra and the doubled-line sweep |
| `op/`, `clock/`, `strategy/`, `engine/`, `store/` | the 1-D CRDT stack |
| `space/` | boxes, covers, canonicalizers, path queries, components, and its own stack |
| `rtree/`, `meta/` | spatial index over `space.Box`; metadata join |
| `demo/` | one module holding every runnable demo |
| `examples/calendar` | a standalone consumer that imports the published version |
| `docs/` | the deep dives |
| `scripts/` | gate, lint, test, demo and release scripts |

## Invariants

- **History is the truth; the view is a cache.** Replicas exchange `Ops()` and
  call `ApplyAll`; materialized state never travels.
- **Endpoints are compared, never combined.** No arithmetic on coordinates, so
  no rounding error accumulates. `int64` carries timestamps and versions only.
- **1-D defaults to closed `[a,b]`; `space.Box` is half-open.** The asymmetry is
  deliberate; both facades expose bounds-explicit variants.
- **The n-D cover is not unique.** `space/engine` materializes `LWW`/`FWW` from
  the full operation list, never incrementally, so replicas agree on the cover.
- **`AdditiveWins` and `GrowOnly` fold incrementally** in both dimensions.
- **The oracle is test-only.** `github.com/biogo/store` appears in the property
  tests and nowhere in production code.
- **Wire names and JSON tags are public API.** Change the type and its `*Names`
  helper together.
- **KurrentDB stays behind the `kurrent` build tag.** The default backend is a
  local JSONL file.
- **License stays MPL-2.0.**

## Documentation

- Sentence-case headings, US spelling. `.harper-dictionary.txt` lists the project
  vocabulary for `harper-cli`.
- When a public type or member changes, update the matching `docs/` page and the
  README tables in the same change.
- Keep the quadrants apart: tutorials walk to a working result, how-tos solve one
  task, reference states exact behavior, explanation says why.
