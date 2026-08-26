// Fractal grid: the board plane subdivides into four squares per level, so a
// cell of level n has side 2^-n board units. The browser picks the level from
// the zoom (one subdivision step for every doubling of scale), and a user
// offset shifts that automatic choice coarser or finer.

export const MIN_CELL_PX = 24; // a cell subdivides once its quarters would be this size
export const MAX_LEVEL = 48; // 2^-48 is still exact in a float64
export const MIN_LEVEL = -16; // 2^16-unit cells are coarse enough for any view
export const MIN_GRID_PX = 6; // below this the lines blur into noise

// autoLevel picks the subdivision level whose on-screen cell is at least
// MIN_CELL_PX and less than 2 * MIN_CELL_PX.
export function autoLevel(scale) {
  if (!Number.isFinite(scale) || scale <= 0) return 0;
  return Math.max(0, Math.floor(Math.log2(scale / MIN_CELL_PX)));
}

// gridLevel clamps the automatic level plus the user offset.
export function gridLevel(scale, offset = 0) {
  return Math.max(MIN_LEVEL, Math.min(MAX_LEVEL, autoLevel(scale) + offset));
}

// gridSize is the side length, in board units, of the current cell.
export function gridSize(scale, offset = 0) {
  return 2 ** -gridLevel(scale, offset);
}

// gridStep returns the coarsest power-of-two multiple of cell whose on-screen
// side is at least minPx. Strokes still snap to cell, but the grid is drawn at
// step so fine cells never vanish into sub-pixel noise.
export function gridStep(cell, scale, minPx = MIN_GRID_PX) {
  let step = cell;
  while (step * scale < minPx) step *= 2;
  return step;
}

// gridRect snaps two board points onto the current grid and returns the
// half-open cell rectangle [x0,x1) x [y0,y1) they cover, in board units.
export function gridRect(a, b, cell) {
  const ix0 = Math.floor(a.x / cell);
  const iy0 = Math.floor(a.y / cell);
  const ix1 = Math.floor(b.x / cell);
  const iy1 = Math.floor(b.y / cell);
  const loX = Math.min(ix0, ix1);
  const hiX = Math.max(ix0, ix1);
  const loY = Math.min(iy0, iy1);
  const hiY = Math.max(iy0, iy1);
  return {
    x0: loX * cell,
    y0: loY * cell,
    x1: (hiX + 1) * cell,
    y1: (hiY + 1) * cell,
  };
}

// gridLine returns the half-open cell rectangles, in board units, of the cells
// crossed by the line from cell a to cell b, excluding a and including b.
// Consecutive calls tile a pen stroke without repainting the anchor cell.
export function gridLine(a, b, cell) {
  const rects = [];
  let x0 = a.ix;
  let y0 = a.iy;
  const x1 = b.ix;
  const y1 = b.iy;
  const dx = Math.abs(x1 - x0);
  const dy = -Math.abs(y1 - y0);
  const sx = x0 < x1 ? 1 : -1;
  const sy = y0 < y1 ? 1 : -1;
  let err = dx + dy;
  while (x0 !== x1 || y0 !== y1) {
    const e2 = 2 * err;
    if (e2 >= dy) {
      err += dy;
      x0 += sx;
    }
    if (e2 <= dx) {
      err += dx;
      y0 += sy;
    }
    rects.push({
      x0: x0 * cell,
      y0: y0 * cell,
      x1: (x0 + 1) * cell,
      y1: (y0 + 1) * cell,
    });
  }
  return rects;
}

// bounds returns the tight axis-aligned bounds of the boxes, or null when
// there is nothing to fit.
export function bounds(boxes) {
  if (!boxes.length) return null;
  let min0 = Infinity;
  let min1 = Infinity;
  let max0 = -Infinity;
  let max1 = -Infinity;
  for (const b of boxes) {
    min0 = Math.min(min0, b.min[0]);
    min1 = Math.min(min1, b.min[1]);
    max0 = Math.max(max0, b.max[0]);
    max1 = Math.max(max1, b.max[1]);
  }
  return { min: [min0, min1], max: [max0, max1] };
}

// fitCamera returns a camera (centre and scale) that shows every box inside
// the viewport with the given padding, or null when there is nothing to fit.
export function fitCamera(boxes, w, h, pad = 48) {
  const b = bounds(boxes);
  if (!b) return null;
  const bw = b.max[0] - b.min[0];
  const bh = b.max[1] - b.min[1];
  if (bw <= 0 || bh <= 0) return null;
  const availW = w - 2 * pad;
  const availH = h - 2 * pad;
  if (availW <= 0 || availH <= 0) return null;
  return {
    x: (b.min[0] + b.max[0]) / 2,
    y: (b.min[1] + b.max[1]) / 2,
    scale: Math.min(availW / bw, availH / bh),
  };
}
