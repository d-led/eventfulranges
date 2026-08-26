# Extensions: canonicalization, metadata, and merge verification

This document records additive extensions to `eventfulranges`. Every
extension is opt-in: the existing 1-D and n-D interfaces, their
materialization, and their convergence guarantees are unchanged unless a new
option is explicitly enabled. Defaults reproduce today's behavior exactly.

## 1. Canonicalization (compaction) seam — *implemented*

**Why.** The n-D `space` canonical cover keeps partially-overlapping boxes; it
drops only empty and subsumed boxes and never subdivides. The materialized
view is therefore not a unique *decomposition* of the point set: equivalent
sets can be represented by different covers, so `space.Equal` is only
meaningful between covers produced by the same deterministic pipeline. There
is no way to request a minimal disjoint cover, or to plug in a spatial index.

**Seam.** A `Canonicalizer` is a pure function applied as the final step of
materialization:

```go
// space
type Canonicalizer func([]Box) []Box
```

The engine stores one (default: none) and applies it wherever the cached view
is set — `applyToView`, `materializeAll`, and `restore` — so every observable
view is canonicalized identically.

A `Canonicalizer` must be:

- **Cover-preserving** — a point is covered by the output exactly when it is
  covered by the input, so `Contains`/`Overlaps` answers cannot change.
- **Deterministic** — the same input yields the same output, so replicas that
  have seen the same operations converge to the same canonicalized view.

Because both hold, a canonicalizer cannot break convergence or membership.

**Where simulation of simplicity belongs.** This seam is the place for SoS.
The default pipeline never needs it — endpoints are compared, never combined,
and half-open bounds resolve boundary ownership by convention. A custom
`Canonicalizer` that *subdivides* into a disjoint cover reintroduces the
degenerate cases SoS exists for (coincident or adjacent endpoints deciding a
split). SoS is therefore an optional ingredient of an optional
`Canonicalizer`, not a change to the core: it supplies the deterministic
tie-break that keeps the canonicalizer deterministic and canonical.

**Built-ins.** Two canonicalizers ship with the library:

- `space.MergeAdjacent` — merges boxes that touch edge-to-edge in exactly one
  dimension and agree in every other dimension. It is a deterministic,
  cover-preserving greedy fixpoint (not a provably-minimal rectangle cover).
- `space.Chain(...)` — composes canonicalizers left to right (chain of
  responsibility), so a pipeline such as `Chain(Normalize, MergeAdjacent)` can
  be supplied as a single canonicalizer.

## 2. Optional metadata + custom merge — *proposed*

Generalize `Op.Origin` into an optional payload with a user-supplied merge.
The strategy produces `[]AnnotatedBox{Box, Meta}` instead of `[]Box`, where
`Meta` is the merge of the operations deciding that region:

- `LWW`/`FWW` — one winner per point → that op's metadata (merge only on
  exact `(ts, id)` ties).
- `AdditiveWins`/`GrowOnly` — every covering add contributes → the merge of
  all of them.

The merge **must be a semilattice join** — commutative, associative,
idempotent — or convergence is lost. Valid: LWW-register, G-Set, G-Counter,
OR-Set, LWW-map. Invalid: last-arrival-wins, subtraction, anything
order-sensitive. The existing `Origin` string is effectively a LWW-register on
the op timestamp.

**Open type-shape decision:**

- **Generic** `Op[M]` — type-safe, but ripples through `op`, `strategy`,
  `engine`, `store`, and the facades.
- **Opaque** `Meta` (`any` / `json.RawMessage`) with a merge registered on the
  engine — far less invasive; metadata stays truly optional (zero value means
  "none").

## 3. Merge verification tooling — *proposed*

A reusable verifier that checks a user-supplied merge against the CRDT
properties, reusing the existing rapid/oracle/Jepsen infrastructure:

```go
crdt.Verify(t, merge, gen) // semilattice laws + order independence + convergence
```

It checks:

1. **Commutativity, associativity, idempotency** — fuzz + `rapid`.
2. **Order independence** — merging a shuffled history yields the same value.
3. **Convergence** — two replicas mutate concurrently then gossip (the
   `scenario_test.go` pattern).
4. **Oracle agreement** — where a sequential reference semantics exists.

## Non-breaking guarantee

- Default options reproduce current behavior exactly.
- New types and functions are additive; no existing symbol changes meaning.
- The 1-D path is untouched; the seam lands first on the n-D engine, where the
  non-unique-decomposition issue actually exists (1-D `interval.Normalize`
  already merges into a unique canonical form).
