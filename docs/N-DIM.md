# n-dimensional ranges

The `space` package generalizes the one-dimensional `interval` set to **n
dimensions**. It is the geometry layer for the n-dimensional CRDT engine.

## Boxes

A `space.Box` is an axis-aligned, **half-open** rectangle:

```
Box = { Min []float64, Max []float64 }
point p is inside Box b iff  ∀i:  Min[i] <= p[i] < Max[i]
```

Half-open bounds were chosen deliberately: subtracting one box from another
always yields a small, exact set of boxes, with no special cases around
shared edges. With closed bounds, subtraction is only clean on interiors and
degenerates at boundaries.

## Set operations

- `Normalize` removes empty boxes and any box subsumed by another, then sorts
  the survivors — a canonical cover that preserves the covered point set
  exactly. Partially overlapping boxes are kept (subdividing them into
  disjoint pieces is not needed for correctness).
- `Union(a, b)` is the normalized concatenation.
- `Difference(a, b)` is **exact**: it subtracts each `b` box from each `a`
  box by splitting along the first dimension where the boxes clip, recursing
  on the middle slab for the remaining dimensions.
- `Contains` and `OverlapsSet` are point and box queries over a cover.

The property tests verify every operation against a **point-membership
oracle**: for dense sample grids, a point is in `Normalize(x)` /
`Union(a,b)` / `Difference(a,b)` exactly when the defining predicate says so.
Because the oracle is membership, not the implementation, a geometry bug
cannot slip through.

## The n-D CRDT engine

The n-dimensional engine mirrors the 1-D stack, one package per layer, all
typed to `space.Box` instead of `interval.Interval`:

| Layer            | 1-D                    | n-D                     |
| ---------------- | ---------------------- | ----------------------- |
| Operation        | `op`                   | `space/op`              |
| Conflict policy  | `strategy`             | `space/strategy`        |
| Event log        | `store`, `store/memory`, `store/jsonl` | `space/store`, `space/store/memory`, `space/store/jsonl` |
| Engine           | `engine`               | `space/engine`          |
| Facade           | `eventfulranges.RangeSet` | `eventfulranges.BoxSet` |

Every 1-D strategy (`LWW`, `FWW`, `AdditiveWins`, `GrowOnly`) has an n-D
counterpart with the same semantics, materializing `[]space.Box` instead of
`[]interval.Interval`. `AdditiveWins` and `GrowOnly` reduce to `space.Union`
and `space.Difference`; `LWW` and `FWW` layer operations in priority order,
carving each box by the region still undecided. Because the n-D cover is not
uniquely decomposable (partially overlapping boxes are kept, not subdivided),
the engine materializes `LWW`/`FWW` from the full operation list — never
incrementally — so two replicas that have seen the same operations always
yield the identical cover.

For rendering, `space/strategy.Layers` (exposed as `BoxSet.Layers`) is the
painter's-algorithm counterpart of the cover: it returns the effective
operations as an ordered, culled front of overlapping boxes (`Layer{Box,
Kind}`) in bottom-to-top paint order. Each box stays whole — a small stroke
inside a big box layers on top instead of carving the big box into strips —
and any box fully covered by a higher layer is dropped. It is a paint recipe,
not a set cover, so it backs rendering rather than the set queries
(`Contains`, `Overlaps`, `Crossed`, `Traverse`).
