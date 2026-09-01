// Raster import: turn an RGBA image into the board boxes that paint it. One
// pixel is one board unit, and contiguous runs of the same colour merge into
// rectangles, so a flat region becomes one box instead of one per pixel.
// Pixels whose alpha falls below the threshold paint nothing.

export const ALPHA_THRESHOLD = 128;

// boxesFromRaster returns the half-open board boxes covering every opaque
// pixel of a width × height RGBA image. data is the flat Uint8ClampedArray of
// an ImageData (four bytes per pixel: r, g, b, a).
export function boxesFromRaster(data, width, height) {
  const boxes = [];
  let active = new Map(); // x -> rectangle continued from the previous row
  for (let y = 0; y < height; y++) {
    const runs = rowRuns(data, width, y);
    const next = new Map();
    for (const run of runs) {
      const above = active.get(run.x);
      if (above && above.w === run.w && above.color === run.color) {
        next.set(run.x, above); // extend the rectangle down one row
      } else {
        next.set(run.x, { x: run.x, w: run.w, y0: y, color: run.color });
      }
    }
    for (const [x, rect] of active) {
      if (next.get(x) !== rect) {
        boxes.push({
          min: [rect.x, rect.y0],
          max: [rect.x + rect.w, y],
          color: rect.color,
        });
      }
    }
    active = next;
  }
  for (const rect of active.values()) {
    boxes.push({
      min: [rect.x, rect.y0],
      max: [rect.x + rect.w, height],
      color: rect.color,
    });
  }
  return boxes;
}

// rowRuns returns the horizontal runs of opaque pixels in one row, as
// {x, w, color}. Transparent pixels break runs and produce nothing.
function rowRuns(data, width, y) {
  const runs = [];
  let x = 0;
  while (x < width) {
    const color = colorAt(data, width, y, x);
    let end = x + 1;
    while (end < width && colorAt(data, width, y, end) === color) end++;
    if (color !== null) runs.push({ x, w: end - x, color });
    x = end;
  }
  return runs;
}

// colorAt returns the hex colour of one pixel, or null when it is transparent.
function colorAt(data, width, y, x) {
  const i = (y * width + x) * 4;
  if (data[i + 3] < ALPHA_THRESHOLD) return null;
  return rgbToHex(data[i], data[i + 1], data[i + 2]);
}

// rgbToHex renders a colour as #rrggbb.
function rgbToHex(r, g, b) {
  return `#${hex(r)}${hex(g)}${hex(b)}`;
}

function hex(v) {
  return v.toString(16).padStart(2, "0");
}
