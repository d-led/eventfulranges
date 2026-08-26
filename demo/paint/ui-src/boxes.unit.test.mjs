import { describe, it, expect } from "vitest";
import { normalize, union, difference } from "./boxes.js";

const box = (x0, y0, x1, y1) => ({ min: [x0, y0], max: [x1, y1] });
const covers = (boxes, x, y) =>
  boxes.some(
    (b) => x >= b.min[0] && x < b.max[0] && y >= b.min[1] && y < b.max[1],
  );

describe("boxes", () => {
  it("normalizes subsumed and empty boxes", () => {
    expect(normalize([box(0, 0, 4, 4), box(1, 1, 3, 3)])).toEqual([
      box(0, 0, 4, 4),
    ]);
    expect(normalize([box(0, 0, 0, 4)])).toEqual([]);
  });

  it("subtracts a hole into a frame", () => {
    const frame = difference([box(0, 0, 4, 4)], [box(1, 1, 3, 3)]);
    expect(frame).toHaveLength(4);
    expect(covers(frame, 0.5, 2)).toBe(true);
    expect(covers(frame, 2, 2)).toBe(false);
  });

  it("applies additive wins", () => {
    const adds = union([box(0, 0, 4, 4)], [box(8, 0, 12, 4)]);
    const view = difference(adds, [box(2, 1, 3, 3), box(8, 0, 10, 4)]);
    expect(covers(view, 0.5, 0.5)).toBe(true);
    expect(covers(view, 2, 2)).toBe(false); // erased
    expect(covers(view, 9, 2)).toBe(false); // erased
    expect(covers(view, 11, 2)).toBe(true); // kept
  });

  it("difference with no overlap is the identity", () => {
    expect(difference([box(0, 0, 4, 4)], [box(10, 10, 12, 12)])).toEqual([
      box(0, 0, 4, 4),
    ]);
  });
});
