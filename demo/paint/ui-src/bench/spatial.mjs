// Micro-benchmark for the client-side spatial index: linear scan vs a
// median-split R-tree (the JS mirror of the Go rtree package). It answers
// whether a "places of interest" query — finding the boxes in a viewport or
// under the cursor — is worth indexing in the browser, where the paint demo
// already holds every box it draws.
//
// Run: node bench/spatial.mjs  (or: npm run bench)

const CAPACITY = 8;
const SIDE = 1 << 20; // canvas side, like the Go benchmark

// ---------- deterministic PRNG (reproducible runs) ----------
function mulberry32(seed) {
  let a = seed >>> 0;
  return function () {
    a |= 0;
    a = (a + 0x6d2b79f5) | 0;
    let t = Math.imul(a ^ (a >>> 15), 1 | a);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

function randomBoxes(n, seed, side = SIDE) {
  const rnd = mulberry32(seed);
  const boxes = new Array(n);
  for (let i = 0; i < n; i++) {
    const x = rnd() * side;
    const y = rnd() * side;
    const w = 1 + rnd() * 64;
    const h = 1 + rnd() * 64;
    boxes[i] = { min: [x, y], max: [x + w, y + h] };
  }
  return boxes;
}

// ---------- half-open overlap, as the paint demo uses ----------
function overlaps(a, b) {
  for (let i = 0; i < a.min.length; i++) {
    if (a.min[i] >= b.max[i] || a.max[i] <= b.min[i]) return false;
  }
  return true;
}

// ---------- median-split R-tree ----------
function mbr(boxes) {
  const min = boxes[0].min.slice();
  const max = boxes[0].max.slice();
  for (let i = 1; i < boxes.length; i++) {
    for (let d = 0; d < min.length; d++) {
      if (boxes[i].min[d] < min[d]) min[d] = boxes[i].min[d];
      if (boxes[i].max[d] > max[d]) max[d] = boxes[i].max[d];
    }
  }
  return { min, max };
}

function widestAxis(boxes) {
  let axis = 0;
  let width = 0;
  for (let d = 0; d < boxes[0].min.length; d++) {
    let lo = boxes[0].min[d];
    let hi = boxes[0].max[d];
    for (let i = 1; i < boxes.length; i++) {
      if (boxes[i].min[d] < lo) lo = boxes[i].min[d];
      if (boxes[i].max[d] > hi) hi = boxes[i].max[d];
    }
    if (hi - lo > width) {
      axis = d;
      width = hi - lo;
    }
  }
  return axis;
}

function buildNode(boxes) {
  if (boxes.length <= CAPACITY) {
    return { leaf: true, boxes, mbr: mbr(boxes) };
  }
  const axis = widestAxis(boxes);
  boxes.sort(
    (a, b) => a.min[axis] - b.min[axis] || a.max[axis] - b.max[axis],
  );
  const mid = boxes.length >> 1;
  const left = buildNode(boxes.slice(0, mid));
  const right = buildNode(boxes.slice(mid));
  return { leaf: false, children: [left, right], mbr: mbr([left.mbr, right.mbr]) };
}

function buildTree(boxes) {
  return boxes.length === 0 ? null : buildNode(boxes);
}

function searchNode(n, q, out) {
  if (!overlaps(n.mbr, q)) return;
  if (n.leaf) {
    for (const b of n.boxes) if (overlaps(b, q)) out.push(b);
    return;
  }
  for (const c of n.children) searchNode(c, q, out);
}

function search(tree, q) {
  const out = [];
  if (tree) searchNode(tree, q, out);
  return out;
}

// ---------- the current production path ----------
function countOverlaps(boxes, q) {
  let count = 0;
  for (const b of boxes) if (overlaps(b, q)) count++;
  return count;
}

// ---------- timing harness ----------
function bench(fn, { minTimeMs = 250, warmup = 50 } = {}) {
  for (let i = 0; i < warmup; i++) fn();
  let runs = 0;
  const t0 = performance.now();
  let t1;
  do {
    fn();
    runs++;
    t1 = performance.now();
  } while (t1 - t0 < minTimeMs);
  return ((t1 - t0) / runs) * 1e6; // ns/op
}

function fmtNs(ns) {
  if (ns < 1000) return `${ns.toFixed(0)}ns`;
  if (ns < 1e6) return `${(ns / 1e3).toFixed(1)}µs`;
  return `${(ns / 1e6).toFixed(2)}ms`;
}

// ---------- run ----------
console.log(`canvas ${SIDE}×${SIDE}, median-split R-tree (capacity ${CAPACITY})\n`);

console.log("build (ephemeral rebuild cost)");
for (const n of [1_000, 10_000, 100_000]) {
  const boxes = randomBoxes(n, 7);
  const ns = bench(() => buildTree(boxes.slice()));
  console.log(`  n=${String(n).padStart(6)}  ${fmtNs(ns).padStart(8)}`);
}

console.log("\nviewport overlap query (linear vs rtree)");
for (const n of [1_000, 10_000, 100_000]) {
  const boxes = randomBoxes(n, 11);
  const tree = buildTree(boxes.slice());
  for (const qs of [256, 8192, 2 * SIDE]) {
    const q = { min: [SIDE / 2, SIDE / 2], max: [SIDE / 2 + qs, SIDE / 2 + qs] };
    const linear = bench(() => countOverlaps(boxes, q));
    const indexed = bench(() => search(tree, q));
    const speedup = (linear / indexed).toFixed(1);
    const tag = qs === 2 * SIDE ? "all" : qs;
    console.log(
      `  n=${String(n).padStart(6)} q=${String(tag).padStart(5)}  ` +
      `linear ${fmtNs(linear).padStart(9)}  rtree ${fmtNs(indexed).padStart(9)}  (${speedup}×)`,
    );
  }
}
