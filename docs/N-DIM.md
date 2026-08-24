# n-dimensional ranges

The `space` package generalizes the one-dimensional `interval` set to **n
dimensions**. It is the geometry layer for a future n-dimensional engine.

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

## Relationship to the 1-D CRDT

The 1-D package and the n-D package share the same shape:

| 1-D (`interval`)        | n-D (`space`)          |
| ----------------------- | ---------------------- |
| `Interval` (open/closed)| `Box` (half-open)      |
| `Normalize` (merge)     | `Normalize` (subsume)  |
| `Union` / `Difference`  | `Union` / `Difference` |
| `Contains` / `Overlaps` | `Contains` / `OverlapsSet` |

An n-D CRDT engine would apply the same conflict-resolution strategies to
operations over `space.Box` instead of `interval.Interval`; the strategy
layer is shape-agnostic in principle. That engine is future work — today
`space` ships as a fully tested geometry package.
