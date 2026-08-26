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
  let result = normalize(a);
  for (const q of normalize(b)) {
    const next = [];
    for (const p of result) next.push(...subtract(p, q));
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
