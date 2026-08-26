import { describe, it, expect } from "vitest";
import {
  MIN_CELL_PX,
  MAX_LEVEL,
  MIN_LEVEL,
  autoLevel,
  gridLevel,
  gridSize,
  gridStep,
  gridRect,
  gridLine,
  bounds,
  fitCamera,
} from "./grid.js";

describe("grid", () => {
  it("stays at level zero while cells are smaller than the threshold", () => {
    expect(autoLevel(MIN_CELL_PX - 1)).toBe(0);
    expect(autoLevel(MIN_CELL_PX)).toBe(0);
  });

  it("subdivides once per doubling of scale", () => {
    expect(autoLevel(MIN_CELL_PX * 2)).toBe(1);
    expect(autoLevel(MIN_CELL_PX * 4)).toBe(2);
    expect(autoLevel(MIN_CELL_PX * 8)).toBe(3);
  });

  it("clamps the user offset to representable levels", () => {
    expect(gridLevel(MIN_CELL_PX, 1)).toBe(1);
    expect(gridLevel(MIN_CELL_PX, -5)).toBe(-5);
    expect(gridLevel(Number.MAX_VALUE, 0)).toBe(MAX_LEVEL);
    expect(gridLevel(MIN_CELL_PX, -1000)).toBe(MIN_LEVEL);
  });

  it("lets a negative offset coarsen below the automatic level", () => {
    expect(gridSize(MIN_CELL_PX, -1)).toBe(2);
    expect(gridSize(MIN_CELL_PX, -2)).toBe(4);
  });

  it("keeps the grid readable by stepping to a visible multiple", () => {
    expect(gridStep(1, 12)).toBe(1); // 12px is already readable
    expect(gridStep(0.125, 12)).toBe(0.5); // 1.5px steps up to 6px
    expect(gridStep(0.125, 96)).toBe(0.125); // 12px is already readable
  });

  it("derives the cell side as an exact power of two", () => {
    expect(gridSize(MIN_CELL_PX, 0)).toBe(1);
    expect(gridSize(MIN_CELL_PX * 2, 0)).toBe(0.5);
    expect(gridSize(MIN_CELL_PX * 4, 0)).toBe(0.25);
  });

  it("snaps a single click to the surrounding cell", () => {
    expect(gridRect({ x: 0.2, y: 0.2 }, { x: 0.2, y: 0.2 }, 1)).toEqual({
      x0: 0,
      y0: 0,
      x1: 1,
      y1: 1,
    });
    expect(gridRect({ x: 0.2, y: 0.2 }, { x: 0.2, y: 0.2 }, 0.5)).toEqual({
      x0: 0,
      y0: 0,
      x1: 0.5,
      y1: 0.5,
    });
  });

  it("snaps negative coordinates to the cell containing them", () => {
    expect(gridRect({ x: -0.1, y: -0.1 }, { x: -0.1, y: -0.1 }, 1)).toEqual({
      x0: -1,
      y0: -1,
      x1: 0,
      y1: 0,
    });
  });

  it("orders a reversed drag and covers every crossed cell", () => {
    expect(gridRect({ x: 2.5, y: 2.5 }, { x: 0.5, y: 0.5 }, 1)).toEqual({
      x0: 0,
      y0: 0,
      x1: 3,
      y1: 3,
    });
  });

  it("traces a pen stroke across the cells between two anchors", () => {
    const cell = (ix, iy) => ({ x0: ix, y0: iy, x1: ix + 1, y1: iy + 1 });
    expect(gridLine({ ix: 0, iy: 0 }, { ix: 3, iy: 0 }, 1)).toEqual([
      cell(1, 0),
      cell(2, 0),
      cell(3, 0),
    ]);
    expect(gridLine({ ix: 0, iy: 0 }, { ix: 0, iy: 2 }, 1)).toEqual([
      cell(0, 1),
      cell(0, 2),
    ]);
    expect(gridLine({ ix: 0, iy: 0 }, { ix: 2, iy: 2 }, 1)).toEqual([
      cell(1, 1),
      cell(2, 2),
    ]);
  });

  it("traces no cells when the anchor cell has not moved", () => {
    expect(gridLine({ ix: 1, iy: 1 }, { ix: 1, iy: 1 }, 1)).toEqual([]);
  });

  it("traces subcells at a finer grid level", () => {
    const half = (ix, iy) => ({
      x0: ix * 0.5,
      y0: iy * 0.5,
      x1: (ix + 1) * 0.5,
      y1: (iy + 1) * 0.5,
    });
    expect(gridLine({ ix: 0, iy: 0 }, { ix: 2, iy: 0 }, 0.5)).toEqual([
      half(1, 0),
      half(2, 0),
    ]);
  });

  it("bounds boxes or reports nothing to fit", () => {
    const boxes = [
      { min: [1, 2], max: [3, 5] },
      { min: [-4, 0], max: [0, 2] },
    ];
    expect(bounds(boxes)).toEqual({ min: [-4, 0], max: [3, 5] });
    expect(bounds([])).toBeNull();
  });

  it("fits every box into the viewport with padding", () => {
    const cam = fitCamera([{ min: [0, 0], max: [10, 10] }], 200, 100, 10);
    expect(cam).toEqual({ x: 5, y: 5, scale: 8 });
  });

  it("refuses to fit a degenerate viewport", () => {
    expect(fitCamera([{ min: [0, 0], max: [10, 10] }], 10, 100, 10)).toBeNull();
  });
});
