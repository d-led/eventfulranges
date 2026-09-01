// Set algebra over half-open 2-D boxes {min:[x0,y0], max:[x1,y1]}. The browser
// materializes the whiteboard by folding the operation log with the same
// additive-wins rule as the library: union of additions minus union of removals.

export function normalize(boxes) {
  const kept = [];
  for (const b of boxes) {
    if (empty(b)) continue;
    if (kept.some((k) => subsumes(k, b))) continue;
    for (let i = kept.length - 1; i >= 0; i--) {
      if (subsumes(b, kept[i])) kept.splice(i, 1);
    }
    kept.push(b);
  }
  return kept.sort(compare);
}

export function union(a, b) {
  return normalize([...a, ...b]);
}

export function difference(a, b) {
  return normalize(a).flatMap((p) => subtractAll(p, normalize(b)));
}

// front returns the culled, layered front of the operations: the ops sorted
// bottom-to-top by seq, minus any op whose box a single higher op already
// covers. The kept ops paint in order (painter's algorithm), so a later
// stroke layers over an earlier one instead of carving it into strips; a
// remove erases by painting the background, exactly like an add layered over
// an add. Culling only needs a conservative test: the painter's algorithm
// renders a kept-but-covered box identically to a culled one, so an op that
// is covered only collectively is kept and simply overdraws.
export function front(ops) {
  const sorted = [...ops].sort((a, b) => a.seq - b.seq);
  let covered = [];
  const kept = [];
  for (let i = sorted.length - 1; i >= 0; i--) {
    const op = sorted[i];
    const box = { min: op.min, max: op.max };
    if (isCoveredBy(box, covered)) continue; // cull
    covered = insert(covered, box);
    kept.push(op); // top-down order; reversed below
  }
  kept.reverse(); // bottom-to-top paint order
  return kept;
}

// layerOnTop layers ops that are all newer than every box in the front onto
// it: the new ops are culled among themselves, then any existing box fully
// covered by a new op is dropped, and the survivors are appended on top. When
// every op in fresh has a higher seq than every box in boxes, the result is
// exactly front(boxes ++ fresh), so it is the incremental form of front.
export function layerOnTop(boxes, fresh) {
  const top = front(fresh);
  const keep = boxes.filter((b) => !top.some((t) => subsumes(t, b)));
  keep.push(...top);
  return keep;
}

// isCoveredBy reports whether some box in the sorted, subsumption-free set
// covers box entirely. The set is sorted by its lower corner, so once a box
// starts beyond box's own lower corner, none of the rest can contain box.
function isCoveredBy(box, set) {
  for (const b of set) {
    if (b.min[0] > box.min[0]) return false;
    if (subsumes(b, box)) return true;
  }
  return false;
}

// insert returns the normalized union of a sorted, subsumption-free set and
// one box, preserving the sort, in one linear pass.
function insert(set, box) {
  if (empty(box)) return set;
  const kept = [];
  let subsumed = false;
  let placed = false;
  for (const b of set) {
    if (subsumes(box, b)) continue; // box covers b: drop b
    if (subsumes(b, box)) {
      subsumed = true;
      kept.push(b);
      continue;
    }
    if (!subsumed && !placed && compare(box, b) < 0) {
      kept.push(box);
      placed = true;
    }
    kept.push(b);
  }
  if (!subsumed && !placed) kept.push(box);
  return kept;
}

// subtractAll carves every box in list out of p, returning the surviving
// pieces. The caller can pre-normalize list once and reuse it across many
// boxes, which keeps per-stroke materialization from re-normalizing the whole
// removal set for each painted box.
export function subtractAll(p, list) {
  let result = [p];
  for (const q of list) {
    const next = [];
    for (const r of result) next.push(...subtract(r, q));
    result = normalize(next);
  }
  return result;
}

function empty(b) {
  return b.min[0] >= b.max[0] || b.min[1] >= b.max[1];
}

function subsumes(b, a) {
  return (
    b.min[0] <= a.min[0] &&
    b.max[0] >= a.max[0] &&
    b.min[1] <= a.min[1] &&
    b.max[1] >= a.max[1]
  );
}

function compare(a, b) {
  for (let d = 0; d < 2; d++) {
    if (a.min[d] !== b.min[d]) return a.min[d] - b.min[d];
    if (a.max[d] !== b.max[d]) return a.max[d] - b.max[d];
  }
  return 0;
}

// subtract returns the boxes covering p without q. Two dimensions keep it
// simple: the remainder is at most four strips (bottom, top, left, right).
function subtract(p, q) {
  const ox0 = Math.max(p.min[0], q.min[0]);
  const oy0 = Math.max(p.min[1], q.min[1]);
  const ox1 = Math.min(p.max[0], q.max[0]);
  const oy1 = Math.min(p.max[1], q.max[1]);
  if (ox1 <= ox0 || oy1 <= oy0) return [p];
  const out = [];
  if (oy0 > p.min[1]) out.push(box(p.min[0], p.min[1], p.max[0], oy0));
  if (oy1 < p.max[1]) out.push(box(p.min[0], oy1, p.max[0], p.max[1]));
  if (ox0 > p.min[0]) out.push(box(p.min[0], oy0, ox0, oy1));
  if (ox1 < p.max[0]) out.push(box(ox1, oy0, p.max[0], oy1));
  return out;
}

function box(x0, y0, x1, y1) {
  return { min: [x0, y0], max: [x1, y1] };
}
