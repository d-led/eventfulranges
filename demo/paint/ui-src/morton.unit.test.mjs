import { describe, it, expect } from 'vitest';
import { encode, decode, intervalToBoxes } from './morton.js';

describe('morton', () => {
  it('round-trips cells', () => {
    const cells = [
      [0, 0],
      [3, -7],
      [-33554432, -33554432],
      [33554431, 33554431],
      [-1, 1],
    ];
    for (const [x, y] of cells) {
      expect(decode(encode(x, y))).toEqual([x, y]);
    }
  });

  it('decodes an aligned block into one box', () => {
    const lo = encode(0, 0);
    expect(intervalToBoxes(lo, lo + 16n)).toEqual([{ x: 0, y: 0, size: 4 }]);
  });

  it('tiles a rectangle exactly', () => {
    const [x0, y0, x1, y1] = [2, 1, 5, 4]; // a 3x3 rectangle at (2,1)
    for (const [lo, hi] of rectRanges(x0, y0, x1, y1)) {
      let covered = 0;
      for (const box of intervalToBoxes(lo, hi)) {
        expect(box.x).toBeGreaterThanOrEqual(x0);
        expect(box.y).toBeGreaterThanOrEqual(y0);
        expect(box.x + box.size).toBeLessThanOrEqual(x1);
        expect(box.y + box.size).toBeLessThanOrEqual(y1);
        expect(box.x % box.size).toBe(0);
        expect(box.y % box.size).toBe(0);
        covered += box.size * box.size;
      }
      expect(covered).toBe(Number(hi - lo));
    }
  });

  it('splits a non-aligned single cell into itself', () => {
    const code = encode(-3, 5);
    expect(intervalToBoxes(code, code + 1n)).toEqual([{ x: -3, y: 5, size: 1 }]);
  });
});

// rectRanges enumerates a rectangle's cells and merges their codes into
// contiguous runs, matching the server's decomposition.
function rectRanges(x0, y0, x1, y1) {
  const codes = [];
  for (let x = x0; x < x1; x++) {
    for (let y = y0; y < y1; y++) codes.push(encode(x, y));
  }
  codes.sort((a, b) => (a < b ? -1 : a > b ? 1 : 0));
  const ranges = [];
  for (const c of codes) {
    const last = ranges[ranges.length - 1];
    if (last && c === last[1]) last[1] += 1n;
    else ranges.push([c, c + 1n]);
  }
  return ranges;
}
