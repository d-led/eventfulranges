// Set algebra over half-open integer intervals [a, b). The browser
// materializes the whiteboard by folding the operation log with the same
// additive-wins rule as the library: union of additions minus union of
// removals.

export function normalize(list) {
  const sorted = [...list]
    .filter(([a, b]) => a < b)
    .sort((p, q) => p[0] - q[0] || p[1] - q[1]);
  const out = [];
  for (const [a, b] of sorted) {
    if (out.length && a <= out[out.length - 1][1]) {
      out[out.length - 1][1] = Math.max(out[out.length - 1][1], b);
    } else {
      out.push([a, b]);
    }
  }
  return out;
}

export function union(a, b) {
  return normalize([...a, ...b]);
}

export function difference(a, b) {
  let result = normalize(a);
  for (const [lo, hi] of normalize(b)) {
    const next = [];
    for (const [x, y] of result) {
      if (hi <= x || lo >= y) {
        next.push([x, y]);
        continue;
      }
      if (lo > x) next.push([x, lo]);
      if (hi < y) next.push([hi, y]);
    }
    result = next;
  }
  return result;
}
