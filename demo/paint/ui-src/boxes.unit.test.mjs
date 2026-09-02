import { describe, it, expect } from "vitest";
import {
  normalize,
  union,
  difference,
  subtractAll,
  front,
  layerOnTop,
} from "./boxes.js";

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

  it("subtracts every box in a list from one box", () => {
    const pieces = subtractAll(box(0, 0, 4, 4), [
      box(1, 1, 2, 2),
      box(2, 2, 3, 3),
    ]);
    expect(covers(pieces, 0.5, 0.5)).toBe(true);
    expect(covers(pieces, 1.5, 1.5)).toBe(false);
    expect(covers(pieces, 2.5, 2.5)).toBe(false);
    expect(covers(pieces, 3.5, 3.5)).toBe(true);
  });
});

describe("front", () => {
  const add = (seq, x0, y0, x1, y1, color = "#ffffff") => ({
    seq,
    kind: "add",
    min: [x0, y0],
    max: [x1, y1],
    color,
  });
  const remove = (seq, x0, y0, x1, y1) => ({
    seq,
    kind: "remove",
    min: [x0, y0],
    max: [x1, y1],
  });

  it("layers a small stroke on a big square without carving it", () => {
    expect(
      front([add(0, 0, 0, 10, 10, "#111111"), add(1, 4, 4, 6, 6, "#222222")]),
    ).toEqual([add(0, 0, 0, 10, 10, "#111111"), add(1, 4, 4, 6, 6, "#222222")]);
  });

  it("keeps image metadata on a big image layered under a smaller one", () => {
    const big = {
      ...add(0, 0, 0, 8, 8, "#ff0000"),
      image: "data:A",
      frozen: true,
    };
    const small = {
      ...add(1, 1, 1, 3, 3, "#0000ff"),
      image: "data:B",
      frozen: true,
    };
    expect(front([big, small])).toEqual([big, small]);
    expect(layerOnTop([big], [small])).toEqual([big, small]);
  });

  it("culls layers fully covered by a later one", () => {
    expect(
      front([add(0, 0, 0, 2, 2), add(1, 4, 4, 6, 6), add(2, 0, 0, 10, 10)]),
    ).toEqual([add(2, 0, 0, 10, 10)]);
  });

  it("keeps an erase that cuts into a lower layer", () => {
    expect(front([add(0, 0, 0, 10, 10), remove(1, 4, 4, 6, 6)])).toEqual([
      add(0, 0, 0, 10, 10),
      remove(1, 4, 4, 6, 6),
    ]);
  });

  it("culls an erase that a higher add repaints over", () => {
    expect(
      front([add(0, 0, 0, 10, 10), remove(1, 4, 4, 6, 6), add(2, 4, 4, 6, 6)]),
    ).toEqual([add(0, 0, 0, 10, 10), add(2, 4, 4, 6, 6)]);
  });

  it("culls a layer that is fully erased", () => {
    expect(front([add(0, 0, 0, 10, 10), remove(1, 0, 0, 10, 10)])).toEqual([
      remove(1, 0, 0, 10, 10),
    ]);
  });

  it("layers new ops on top without a full rebuild", () => {
    const base = front([add(0, 0, 0, 10, 10, "#111111")]);
    expect(layerOnTop(base, [add(1, 4, 4, 6, 6, "#222222")])).toEqual([
      add(0, 0, 0, 10, 10, "#111111"),
      add(1, 4, 4, 6, 6, "#222222"),
    ]);
  });

  it("culls an existing box that a new op fully covers", () => {
    const base = front([add(0, 4, 4, 6, 6, "#111111")]);
    expect(layerOnTop(base, [add(1, 0, 0, 10, 10, "#222222")])).toEqual([
      add(1, 0, 0, 10, 10, "#222222"),
    ]);
  });

  it("culls a new op that a newer new op fully covers", () => {
    expect(layerOnTop([], [add(0, 0, 0, 2, 2), add(1, 0, 0, 10, 10)])).toEqual([
      add(1, 0, 0, 10, 10),
    ]);
  });
});
