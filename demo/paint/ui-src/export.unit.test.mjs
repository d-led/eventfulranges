import { describe, it, expect } from "vitest";
import {
  MIN_WIDTH,
  MIN_HEIGHT,
  MAX_WIDTH,
  MAX_HEIGHT,
  parseDimensions,
  resolveDimensions,
  suggestDimensions,
  fitExport,
  projectBox,
  snapRect,
  sanitizeColor,
  renderSVG,
  checkRasterSize,
  maxRasterSide,
} from "./export.js";

const box = (x0, y0, x1, y1) => ({ min: [x0, y0], max: [x1, y1] });

describe("parseDimensions", () => {
  it("accepts sizes inside the allowed range", () => {
    expect(parseDimensions("1920", "1080")).toEqual({
      ok: true,
      width: 1920,
      height: 1080,
    });
    expect(parseDimensions(MIN_WIDTH, MIN_HEIGHT)).toEqual({
      ok: true,
      width: MIN_WIDTH,
      height: MIN_HEIGHT,
    });
    expect(parseDimensions(MAX_WIDTH, MAX_HEIGHT)).toEqual({
      ok: true,
      width: MAX_WIDTH,
      height: MAX_HEIGHT,
    });
  });

  it("rejects sizes below the minimum", () => {
    expect(parseDimensions(639, 480).ok).toBe(false);
    expect(parseDimensions(640, 479).ok).toBe(false);
  });

  it("rejects sizes above the maximum", () => {
    expect(parseDimensions(40001, 480).ok).toBe(false);
    expect(parseDimensions(640, 40001).ok).toBe(false);
  });

  it("rejects fractional or non-numeric sizes", () => {
    expect(parseDimensions("800.5", "600").ok).toBe(false);
    expect(parseDimensions("wide", "600").ok).toBe(false);
  });
});

describe("checkRasterSize", () => {
  it("accepts sizes inside the browser's canvas limit", () => {
    expect(checkRasterSize(1000, 1000, 16384)).toEqual({ ok: true });
    expect(checkRasterSize(16384, 16384, 16384)).toEqual({ ok: true });
  });

  it("rejects sizes whose side exceeds the limit", () => {
    const res = checkRasterSize(20000, 20000, 16384);
    expect(res.ok).toBe(false);
    expect(res.error).toContain("16384");
    expect(res.error).toContain("SVG");
  });

  it("falls back to MAX_WIDTH without a document", () => {
    expect(maxRasterSide()).toBe(MAX_WIDTH);
    expect(checkRasterSize(MAX_WIDTH, MAX_HEIGHT)).toEqual({ ok: true });
    expect(checkRasterSize(MAX_WIDTH + 1, MAX_HEIGHT).ok).toBe(false);
  });
});

describe("resolveDimensions", () => {
  it("uses the requested size exactly when the ratio is not kept", () => {
    expect(resolveDimensions([box(0, 0, 2, 1)], 800, 600, false)).toEqual({
      ok: true,
      width: 800,
      height: 600,
    });
  });

  it("keeps the drawing ratio, shrinking the size into the maximums", () => {
    expect(resolveDimensions([box(0, 0, 20, 10)], 2000, 2000, true)).toEqual({
      ok: true,
      width: 2000,
      height: 1000,
    });
    expect(resolveDimensions([box(0, 0, 10, 20)], 1000, 1000, true)).toEqual({
      ok: true,
      width: 500,
      height: 1000,
    });
  });

  it("never grows past the maximums when keeping the ratio", () => {
    expect(resolveDimensions([box(0, 0, 4, 2)], 640, 480, true)).toEqual({
      ok: true,
      width: 640,
      height: 320,
    });
  });

  it("reports an empty board when keeping the ratio", () => {
    expect(resolveDimensions([], 800, 600, true).ok).toBe(false);
  });
});

describe("fitExport", () => {
  it("fits a square drawing into a square image exactly", () => {
    expect(fitExport([box(0, 0, 10, 10)], 100, 100)).toEqual({
      ok: true,
      min0: 0,
      min1: 0,
      scale: 10,
      ox: 0,
      oy: 0,
    });
  });

  it("letterboxes a wide drawing into a square image", () => {
    const view = fitExport([box(0, 0, 20, 10)], 100, 100);
    expect(view.ok).toBe(true);
    expect(view.scale).toBe(5);
    expect(view.ox).toBe(0);
    expect(view.oy).toBe(25);
  });

  it("frames the painted layers and ignores far-away erases", () => {
    const view = fitExport(
      [
        { kind: "add", min: [0, 0], max: [10, 10], color: "#112233" },
        { kind: "remove", min: [100, 100], max: [110, 110] },
      ],
      100,
      100,
    );
    expect(view.ok).toBe(true);
    expect(view.scale).toBe(10);
    expect(view.ox).toBe(0);
    expect(view.oy).toBe(0);
  });

  it("reports an empty board", () => {
    expect(fitExport([], 100, 100).ok).toBe(false);
  });
});

describe("projectBox", () => {
  it("maps the bounding box onto the full image", () => {
    const view = fitExport([box(1, 2, 5, 6)], 200, 200);
    expect(projectBox(box(1, 2, 5, 6), view)).toEqual({
      x: 0,
      y: 0,
      w: 200,
      h: 200,
    });
  });
});

describe("snapRect", () => {
  it("rounds to whole pixels", () => {
    expect(snapRect({ x: 0.4, y: 1.5, w: 9.6, h: 4.4 })).toEqual({
      x: 0,
      y: 2,
      w: 10,
      h: 4,
    });
  });

  it("keeps adjacent boxes sharing a boundary seam-free", () => {
    const left = snapRect({ x: 0, y: 0, w: 10.4, h: 10 });
    const right = snapRect({ x: 10.4, y: 0, w: 9.6, h: 10 });
    expect(left.x + left.w).toBe(right.x); // identical rounded edge, no gap or overlap
  });
});

describe("sanitizeColor", () => {
  it("passes valid colors through", () => {
    expect(sanitizeColor("#ff5500")).toBe("#ff5500");
  });

  it("replaces invalid or missing colors with the fallback", () => {
    expect(sanitizeColor('"><script>')).toBe("#e6e8ee");
    expect(sanitizeColor("red")).toBe("#e6e8ee");
    expect(sanitizeColor(undefined)).toBe("#e6e8ee");
  });
});

describe("renderSVG", () => {
  it("renders one rect per painted piece and no grid", () => {
    const view = fitExport([box(0, 0, 10, 10)], 100, 100);
    const svg = renderSVG([box(0, 0, 10, 10)], 100, 100, view);
    expect(svg).toContain(
      '<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100" viewBox="0 0 100 100">',
    );
    expect(svg).toContain(
      '<rect x="0" y="0" width="100" height="100" fill="#e6e8ee"/>',
    );
    expect(svg).not.toContain("<line");
    expect(svg).not.toContain("<path");
    expect(svg).not.toContain("stroke");
  });

  it("replaces markup smuggled in via imported colors", () => {
    const view = fitExport([box(0, 0, 1, 1)], 10, 10);
    const svg = renderSVG(
      [{ min: [0, 0], max: [1, 1], color: '"><script>' }],
      10,
      10,
      view,
    );
    expect(svg).not.toContain("<script");
    expect(svg).toContain('fill="#e6e8ee"');
  });

  it("paints layers in order over an opaque background", () => {
    const view = fitExport([box(0, 0, 10, 10)], 100, 100);
    const svg = renderSVG(
      [
        { kind: "add", min: [0, 0], max: [10, 10], color: "#112233" },
        { kind: "remove", min: [4, 4], max: [6, 6] },
      ],
      100,
      100,
      view,
    );
    const fills = [...svg.matchAll(/<rect [^>]*fill="([^"]+)"[^>]*\/>/g)].map(
      (m) => m[1],
    );
    expect(fills).toEqual(["#0e1015", "#112233", "#0e1015"]);
  });
});

describe("suggestDimensions", () => {
  it("keeps the drawing ratio with the long side near the target", () => {
    expect(suggestDimensions([box(0, 0, 20, 10)])).toEqual({
      width: 1600,
      height: 800,
    });
    expect(suggestDimensions([box(0, 0, 10, 20)])).toEqual({
      width: 800,
      height: 1600,
    });
  });
});
