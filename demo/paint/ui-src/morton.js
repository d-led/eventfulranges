// Morton encoding over the browser's integers. Codes are up to 52 bits, so
// everything uses BigInt for bit arithmetic while the rest of the app treats
// codes as plain JSON numbers (exact up to 2^53).
//
// The layout matches demo/internal/morton in the Go module: 26 bits per axis,
// with a power-of-two bias shifting signed coordinates into the non-negative
// range so aligned rectangles stay contiguous along the curve.

const COORD_BITS = 26;
const BIAS = 2n ** 25n;

export function encode(x, y) {
  return spread(BigInt(x) + BIAS) | (spread(BigInt(y) + BIAS) << 1n);
}

export function decode(code) {
  return [Number(compact(code) - BIAS), Number(compact(code >> 1n) - BIAS)];
}

// intervalToBoxes splits a contiguous half-open Morton range [lo, hi) into
// maximal aligned boxes. Each box {x, y, size} covers the cells
// [x, x+size) x [y, y+size); boxes tile the range exactly.
export function intervalToBoxes(lo, hi) {
  const boxes = [];
  let start = BigInt(lo);
  const end = BigInt(hi);
  while (start < end) {
    const [x, y] = decode(start);
    let k = Math.min(trailingZeros(x), trailingZeros(y), COORD_BITS);
    let span = 1n << BigInt(2 * k);
    while (start + span > end) {
      k -= 1;
      span = 1n << BigInt(2 * k);
    }
    boxes.push({ x, y, size: 1 << k });
    start += span;
  }
  return boxes;
}

function trailingZeros(n) {
  let v = BigInt(n);
  if (v === 0n) return COORD_BITS;
  let tz = 0;
  while ((v & 1n) === 0n && tz < COORD_BITS) {
    v >>= 1n;
    tz += 1;
  }
  return tz;
}

function spread(v) {
  let x = BigInt(v);
  x = (x | (x << 16n)) & 0x0000FFFF0000FFFFn;
  x = (x | (x << 8n)) & 0x00FF00FF00FF00FFn;
  x = (x | (x << 4n)) & 0x0F0F0F0F0F0F0F0Fn;
  x = (x | (x << 2n)) & 0x3333333333333333n;
  x = (x | (x << 1n)) & 0x5555555555555555n;
  return x;
}

function compact(v) {
  let x = BigInt(v) & 0x5555555555555555n;
  x = (x | (x >> 1n)) & 0x3333333333333333n;
  x = (x | (x >> 2n)) & 0x0F0F0F0F0F0F0F0Fn;
  x = (x | (x >> 4n)) & 0x00FF00FF00FF00FFn;
  x = (x | (x >> 8n)) & 0x0000FFFF0000FFFFn;
  x = (x | (x >> 16n)) & 0x00000000FFFFFFFFn;
  return x;
}
