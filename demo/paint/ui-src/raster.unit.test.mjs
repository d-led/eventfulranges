import { describe, it, expect } from "vitest";
import { boxesFromRaster, scaleBoxes } from "./raster.js";

const RED = "#ff0000";
const BLUE = "#0000ff";

// pixels builds a flat RGBA buffer from rows of "#rrggbb" colours, where null
// is a transparent pixel.
function pixels(rows) {
  const height = rows.length;
  const width = rows[0].length;
  const data = new Uint8ClampedArray(width * height * 4);
  for (let y = 0; y < height; y++) {
    for (let x = 0; x < width; x++) {
      const c = rows[y][x];
      if (!c) continue;
      const i = (y * width + x) * 4;
      data[i] = parseInt(c.slice(1, 3), 16);
      data[i + 1] = parseInt(c.slice(3, 5), 16);
      data[i + 2] = parseInt(c.slice(5, 7), 16);
      data[i + 3] = 255;
    }
  }
  return { data, width, height };
}

describe("boxesFromRaster", () => {
  it("merges a flat image into one box", () => {
    const img = pixels([
      [RED, RED],
      [RED, RED],
    ]);
    expect(boxesFromRaster(img.data, img.width, img.height)).toEqual([
      { min: [0, 0], max: [2, 2], color: RED },
    ]);
  });

  it("merges contiguous runs horizontally and vertically", () => {
    const img = pixels([
      [RED, RED, BLUE],
      [RED, RED, BLUE],
    ]);
    expect(boxesFromRaster(img.data, img.width, img.height)).toEqual([
      { min: [0, 0], max: [2, 2], color: RED },
      { min: [2, 0], max: [3, 2], color: BLUE },
    ]);
  });

  it("splits a rectangle when the colour changes", () => {
    const img = pixels([[RED], [BLUE]]);
    expect(boxesFromRaster(img.data, img.width, img.height)).toEqual([
      { min: [0, 0], max: [1, 1], color: RED },
      { min: [0, 1], max: [1, 2], color: BLUE },
    ]);
  });

  it("skips transparent pixels", () => {
    const img = pixels([
      [RED, null],
      [null, BLUE],
    ]);
    expect(boxesFromRaster(img.data, img.width, img.height)).toEqual([
      { min: [0, 0], max: [1, 1], color: RED },
      { min: [1, 1], max: [2, 2], color: BLUE },
    ]);
  });

  it("returns nothing for a fully transparent image", () => {
    const img = pixels([
      [null, null],
      [null, null],
    ]);
    expect(boxesFromRaster(img.data, img.width, img.height)).toEqual([]);
  });

  it("imports a semi-transparent pixel as its opaque RGB", () => {
    const data = new Uint8ClampedArray([255, 0, 0, 100]); // red, alpha 100
    expect(boxesFromRaster(data, 1, 1)).toEqual([
      { min: [0, 0], max: [1, 1], color: "#ff0000" },
    ]);
  });
});

describe("scaleBoxes", () => {
  const boxes = [
    { min: [0, 0], max: [2, 1], color: "#ff0000" },
    { min: [2, 0], max: [3, 2], color: "#0000ff" },
  ];

  it("leaves boxes untouched at identity scale and origin", () => {
    expect(scaleBoxes(boxes, 1, 0, 0)).toEqual(boxes);
  });

  it("scales every edge by the cell size", () => {
    expect(scaleBoxes(boxes, 0.5, 0, 0)).toEqual([
      { min: [0, 0], max: [1, 0.5], color: "#ff0000" },
      { min: [1, 0], max: [1.5, 1], color: "#0000ff" },
    ]);
  });

  it("shifts the drawing to the given origin", () => {
    expect(scaleBoxes(boxes, 1, 10, -5)).toEqual([
      { min: [10, -5], max: [12, -4], color: "#ff0000" },
      { min: [12, -5], max: [13, -3], color: "#0000ff" },
    ]);
  });
});
