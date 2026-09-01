// Board export: turn the materialized painted boxes into PNG, JPEG, or SVG at
// a caller-chosen size. The drawing is fit with a "contain" transform, so its
// aspect ratio is preserved and it is never stretched or clipped. The grid is
// never rendered, so the file carries only the painted cells.
import { bounds } from "./grid.js";

export const MIN_WIDTH = 640;
export const MIN_HEIGHT = 480;
export const MAX_WIDTH = 40000;
export const MAX_HEIGHT = 40000;

// maxRasterSide returns the largest width or height this browser can rasterize
// on a single canvas side. Browsers silently clamp oversized canvas sides, so
// the probe grows until the canvas stops honouring the requested size. Outside
// a browser (unit tests) it falls back to MAX_WIDTH, keeping pure validation
// deterministic.
let cachedMaxSide = null;
export function maxRasterSide() {
  if (cachedMaxSide !== null) return cachedMaxSide;
  if (typeof document === "undefined") return MAX_WIDTH;
  const c = document.createElement("canvas");
  let lo = 1;
  let hi = 65536;
  while (lo < hi) {
    const mid = Math.ceil((lo + hi) / 2);
    c.width = mid;
    c.height = 1;
    if (c.width === mid) lo = mid;
    else hi = mid - 1;
  }
  cachedMaxSide = lo;
  return lo;
}

// checkRasterSize reports whether a raster export of width × height fits inside
// this browser's canvas, so the client renders it locally and only hands off to
// the server when the size is confirmed too large. Callers may pass an explicit
// maxSide to test the validation without touching a canvas.
export function checkRasterSize(width, height, maxSide = maxRasterSide()) {
  if (width > maxSide || height > maxSide) {
    return {
      ok: false,
      error: `this browser tops out at ${maxSide} px per side — exporting via the server`,
    };
  }
  return { ok: true };
}

// BACKGROUND is the board background filled on raster exports, so a PNG or
// JPEG matches what the board shows. SVG exports stay transparent.
export const BACKGROUND = "#0e1015";

// The fallback color for entries whose metadata carries no valid color.
const DEFAULT_COLOR = "#e6e8ee";
const COLOR_RE = /^#[0-9a-fA-F]{6}$/;

// parseDimensions validates the requested size: whole numbers inside the
// allowed range.
export function parseDimensions(width, height) {
  const w = Number(width);
  const h = Number(height);
  if (!Number.isInteger(w) || !Number.isInteger(h)) {
    return { ok: false, error: "width and height must be whole numbers" };
  }
  if (w < MIN_WIDTH || h < MIN_HEIGHT) {
    return { ok: false, error: `minimum size is ${MIN_WIDTH} × ${MIN_HEIGHT}` };
  }
  if (w > MAX_WIDTH || h > MAX_HEIGHT) {
    return { ok: false, error: `maximum size is ${MAX_WIDTH} × ${MAX_HEIGHT}` };
  }
  return { ok: true, width: w, height: h };
}

// resolveDimensions returns the final pixel size. When keepRatio is true the
// drawing's bounding-box aspect ratio wins and width/height are treated as
// maximums, so the image is shrunk (never grown) to fit inside them. When
// false the requested dimensions are used exactly.
export function resolveDimensions(boxes, width, height, keepRatio) {
  if (!keepRatio) return { ok: true, width, height };
  const b = boundsOf(boxes);
  if (!b) return { ok: false, error: "nothing to export — the board is empty" };
  const scale = Math.min(width / b.bw, height / b.bh);
  return {
    ok: true,
    width: Math.max(1, Math.round(b.bw * scale)),
    height: Math.max(1, Math.round(b.bh * scale)),
  };
}

// gridExportSize returns the raster size that maps one board cell of side
// `cell` to exactly one pixel, keeping a drawing exported at the current grid
// level crisp. The output is exactly the drawing's bounds at this density, so
// it anchors to the top-left with no letterbox.
export function gridExportSize(boxes, cell) {
  const b = boundsOf(boxes);
  if (!b) return { ok: false, error: "nothing to export — the board is empty" };
  return {
    ok: true,
    width: Math.max(1, Math.round(b.bw / cell)),
    height: Math.max(1, Math.round(b.bh / cell)),
  };
}

// suggestDimensions proposes a default export size that keeps the drawing's
// aspect ratio, with the long side near target, clamped to the allowed range.
export function suggestDimensions(boxes, target = 1600) {
  const b = boundsOf(boxes);
  if (!b) return { width: 1920, height: 1080 };
  const wide = b.bw >= b.bh;
  const width = wide ? target : (target * b.bw) / b.bh;
  const height = wide ? (target * b.bh) / b.bw : target;
  return {
    width: clampInt(width, MIN_WIDTH, MAX_WIDTH),
    height: clampInt(height, MIN_HEIGHT, MAX_HEIGHT),
  };
}

// fitExport maps the drawing onto a width × height image with a contain fit:
// the drawing keeps its aspect ratio, is centred, and the remainder is
// background. It returns the transform (offset and pixels per board unit)
// plus the bounds origin, or an error when there is nothing to export.
export function fitExport(boxes, width, height, pad = 0) {
  const b = boundsOf(boxes);
  if (!b) return { ok: false, error: "nothing to export — the board is empty" };
  const availW = Math.max(1, width - 2 * pad);
  const availH = Math.max(1, height - 2 * pad);
  const scale = Math.min(availW / b.bw, availH / b.bh);
  return {
    ok: true,
    min0: b.min0,
    min1: b.min1,
    scale,
    ox: (width - b.bw * scale) / 2,
    oy: (height - b.bh * scale) / 2,
  };
}

// projectBox maps one box to its pixel rectangle in the export view.
export function projectBox(b, view) {
  return {
    x: (b.min[0] - view.min0) * view.scale + view.ox,
    y: (b.min[1] - view.min1) * view.scale + view.oy,
    w: (b.max[0] - b.min[0]) * view.scale,
    h: (b.max[1] - b.min[1]) * view.scale,
  };
}

// snapRect rounds a pixel rectangle to whole pixels by rounding its edges
// independently, so adjacent boxes share exact boundaries and raster exports
// do not show antialiasing seams between them.
export function snapRect(r) {
  const x0 = Math.round(r.x);
  const y0 = Math.round(r.y);
  const x1 = Math.round(r.x + r.w);
  const y1 = Math.round(r.y + r.h);
  return { x: x0, y: y0, w: x1 - x0, h: y1 - y0 };
}

// sanitizeColor keeps only the #rrggbb values the browser's color input can
// produce, so an imported log cannot smuggle markup into the SVG.
export function sanitizeColor(color, fallback = DEFAULT_COLOR) {
  return typeof color === "string" && COLOR_RE.test(color) ? color : fallback;
}

// renderSVG serializes the layered front as one <rect> per layer in board
// coordinates. The viewBox carries the drawing's bounds and there is no fixed
// pixel size, so the SVG scales losslessly in any viewer and the export never
// needs a width or height. Removes erase by painting the board background, so
// the SVG matches the board exactly; the grid is never drawn.
export function renderSVG(boxes) {
  const b = boundsOf(boxes);
  if (!b) throw new Error("nothing to export — the board is empty");
  const view = { min0: b.min0, min1: b.min1, scale: 1, ox: 0, oy: 0 };
  const rects = [
    `<rect x="0" y="0" width="${fmt(b.bw)}" height="${fmt(b.bh)}" fill="${BACKGROUND}"/>`,
  ];
  for (const box of boxes) {
    const r = projectBox(box, view);
    const fill = box.kind === "remove" ? BACKGROUND : sanitizeColor(box.color);
    rects.push(
      `<rect x="${fmt(r.x)}" y="${fmt(r.y)}" width="${fmt(r.w)}" height="${fmt(r.h)}" fill="${fill}"/>`,
    );
  }
  return (
    `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${fmt(b.bw)} ${fmt(b.bh)}">` +
    rects.join("") +
    `</svg>`
  );
}

// boundsOf returns the painted drawing's bounding box — origin plus width and
// height — or null when nothing is painted or the bounds are degenerate.
// Erases (background layers) carry no paint, so they never widen the frame.
function boundsOf(boxes) {
  const painted = boxes.filter((b) => b.kind !== "remove");
  const b = bounds(painted);
  if (!b) return null;
  const bw = b.max[0] - b.min[0];
  const bh = b.max[1] - b.min[1];
  if (!Number.isFinite(bw) || !Number.isFinite(bh) || bw <= 0 || bh <= 0) {
    return null;
  }
  return { min0: b.min[0], min1: b.min[1], bw, bh };
}

// fmt renders a pixel coordinate with enough precision for sub-pixel fidelity
// and without the floating-point noise that would bloat the markup.
function fmt(v) {
  return Number(v.toPrecision(8)).toString();
}

// clampInt rounds and clamps a suggested dimension into the allowed range.
function clampInt(v, lo, hi) {
  return Math.min(hi, Math.max(lo, Math.round(v)));
}
