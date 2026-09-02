import { front, layerOnTop } from "./boxes.js";
import { boxesFromRaster, scaleBoxes } from "./raster.js";
import {
  MIN_CELL_PX,
  MIN_LEVEL,
  MAX_LEVEL,
  gridLevel,
  gridSize,
  gridStep,
  gridRect,
  gridLine,
  bounds,
  fitCamera,
} from "./grid.js";
import { delayFor } from "./backoff.js";
import {
  parseDimensions,
  resolveDimensions,
  suggestDimensions,
  gridExportSize,
  fitExport,
  projectBox,
  renderSVG,
  sanitizeColor,
  snapRect,
  checkRasterSize,
  MAX_WIDTH,
  MAX_HEIGHT,
  BACKGROUND,
} from "./export.js";
import { startTour } from "./tour.js";

// ---------- DOM ----------
const $ = (id) => document.getElementById(id);
const boardEl = $("board");
const statusEl = $("status");
const logEl = $("log");
const copyShareBtn = $("copyShare");
const downloadJsonlBtn = $("downloadJsonl");
const importJsonlBtn = $("importJsonl");
const importJsonlInput = $("importJsonlInput");
const newSessionBtn = $("newSession");
const rectBtn = $("toolRect");
const penBtn = $("toolPen");
const eraseBtn = $("toolErase");
const eraserBtn = $("toolEraser");
const panBtn = $("toolPan");
const toolLabel = $("toolLabel");
const strokeColorEl = $("strokeColor");
const gridLabel = $("gridLabel");
const gridPlus = $("gridPlus");
const gridMinus = $("gridMinus");
const gridDefault = $("gridDefault");
const gridToggle = $("gridToggle");
const fitAllBtn = $("fitAll");
const zoomInBtn = $("zoomIn");
const zoomOutBtn = $("zoomOut");
const zoomResetBtn = $("zoomReset");
const zoomLabel = $("zoomLabel");
const reconnectBanner = $("reconnectBanner");
const reconnectBtn = $("reconnectBtn");
const reconnectText = $("reconnectText");
const rosterPanelEl = $("rosterPanel");
const rosterListEl = $("rosterList");
const rosterMoreEl = $("rosterMore");
const rosterTitleEl = $("rosterTitle");
const hereLink = $("hereLink");
const connectedLink = $("connectedLink");
const meLabel = $("meLabel");
const undoBtn = $("undoBtn");
const redoBtn = $("redoBtn");
const backlogEl = $("backlog");
const zenBtn = $("zenBtn");
const zenBacklog = $("zenBacklog");
const toastEl = $("toast");
const adminLink = $("adminLink");
const exportBtn = $("exportBtn");
const exportDialog = $("exportDialog");
const exportForm = $("exportForm");
const exportFormat = $("exportFormat");
const exportWidth = $("exportWidth");
const exportHeight = $("exportHeight");
const exportRatio = $("exportRatio");
const exportError = $("exportError");
const exportCancel = $("exportCancel");
const exportGo = $("exportGo");
const exportSizeFields = $("exportSizeFields");
const exportRatioField = $("exportRatioField");
const exportHint = $("exportHint");
const exportPixelSize = $("exportPixelSize");
const exportPixelSizeField = $("exportPixelSizeField");
const importImageBtn = $("importImageBtn");
const importDialog = $("importDialog");
const importForm = $("importForm");
const importFileInput = $("importFileInput");
const importPixelSize = $("importPixelSize");
const importClear = $("importClear");
const importFrozen = $("importFrozen");
const importHint = $("importHint");
const importError = $("importError");
const importCancel = $("importCancel");
const importGo = $("importGo");

const ctx = boardEl.getContext("2d");

// ---------- state ----------
let adds = []; // [{id, seq, min, max, color}] painted boxes, one per operation
let removes = []; // [{id, seq, min, max}] erased boxes, one per operation
let boxes = []; // materialized view: [{id, kind, min, max, color}]
let frontSeq = -1; // highest seq already folded into boxes
let needsRebuild = false; // a retraction removed ops, so rebuild the whole front
let opSeq = 0; // arrival-order counter folded by materialize()
let logEntries = []; // the full operation log, for JSONL export
let clientID = "";
let socket = null;
let connected = false; // true once a socket has opened, false after it closes
let reconnectAttempts = 0;
let reconnectTimer = null;
let roster = []; // [{user, sessions}] broadcast by the server
let rosterView = "connected"; // "here" (this session) or "connected" (all)
let pending = []; // outgoing ops not yet acknowledged by the server, in order
let pendingIDs = new Set(); // ids of pending ops, for O(1) acknowledgement
let seen = new Set(); // ids already folded into the local view
let paintBatch = []; // pen/eraser commands waiting for the debounced flush
let paintBatchTimer = null;
let drawQueued = false; // a repaint is already scheduled for the next frame

const LOG_MAX = 100; // scrolling window: at most this many rendered log items
const LOG_DEBOUNCE_MS = 150; // coalesce a burst of ops into one summary line
let logPending = []; // ops waiting for the debounced render
let logTimer = null; // debounce handle
const PAINT_BATCH_MS = 250; // flush a pen/eraser burst after this quiet window

const DEFAULT_COLOR = "#e6e8ee";
const BOARD_BACKGROUND = "#0e1015"; // the board background; an erase paints over with this
const INITIAL_SCALE = 24; // pixels per board unit at 100% zoom, matching MIN_CELL_PX
const cam = { x: 0, y: 0, scale: INITIAL_SCALE }; // board units at the canvas centre, px per unit

let gridOffset = 0; // user shift over the zoom-chosen subdivision level
let gridVisible = true; // visual only: hiding the grid never changes snapping
let strokeColor = DEFAULT_COLOR; // metadata attached to every painted box

// UI preferences remembered across sessions: the selected stroke colour and
// the last export choices (format, size, aspect lock).
const SETTINGS_KEY = "eventfulranges:paint:settings";
let settings = {};

const MAX_IMPORT_BYTES = 100 * 1024 * 1024; // largest accepted file: the first protection
const MAX_IMPORT_DIM = 1024; // largest imported image side, in pixels
const MAX_IMPORT_BOXES = 20_000; // largest accepted cell count after merging
const MAX_FROZEN_BYTES = 1 * 1024 * 1024; // largest embedded (frozen) image: its bytes travel in the log

let tool = "rect";
let dragging = false;
let dragStart = null;
let dragCur = null;
let brushLast = null; // last cell stamped by the pen or eraser brush, as {ix, iy}
let panning = false;
let panLast = null;
let pinch = null; // two-finger gesture: {dist, cx, cy}

// ---------- materialization ----------
// The browser is a replica: it folds the operation log with last-write-wins
// semantics. The view is a culled, layered front — each kept box paints in
// order, later ones on top — so a big square stays one box even when smaller
// strokes are drawn inside it, instead of being carved into strips. A remove
// paints the background, erasing whatever lies beneath it.
// allOps lifts the folded additions and removals into the flat op list the
// front is built from, carrying each op's id so uncommitted ops can be marked.
function allOps() {
  return [
    ...adds.map((a) => ({
      id: a.id,
      seq: a.seq,
      kind: "add",
      min: a.min,
      max: a.max,
      color: a.color,
      image: a.image,
      frozen: a.frozen,
    })),
    ...removes.map((r) => ({
      id: r.id,
      seq: r.seq,
      kind: "remove",
      min: r.min,
      max: r.max,
    })),
  ];
}

function materialize() {
  const ops = allOps();
  if (needsRebuild) {
    boxes = front(ops);
    needsRebuild = false;
  } else {
    // New ops always carry a higher seq than anything already folded, so the
    // front is extended in place instead of rebuilt from scratch.
    const fresh = ops.filter((o) => o.seq > frontSeq);
    if (fresh.length > 0) boxes = layerOnTop(boxes, fresh);
  }
  for (const o of ops) {
    if (o.seq > frontSeq) frontSeq = o.seq;
  }
  draw();
}

function applyEntries(entries, ack = true) {
  const fresh = [];
  const newAdds = [];
  const newRemoves = [];
  const retracted = new Set();
  for (const e of entries) {
    if (ack) ackOp(e.id);
    if (seen.has(e.id)) continue;
    seen.add(e.id);
    fresh.push(e);
    if (e.kind === "add") {
      newAdds.push({
        id: e.id,
        seq: opSeq++,
        min: e.data.min,
        max: e.data.max,
        color: e.meta?.color ?? DEFAULT_COLOR,
        image: e.meta?.image ?? null,
        frozen: e.meta?.frozen ?? false,
      });
    } else if (e.kind === "remove") {
      newRemoves.push({
        id: e.id,
        seq: opSeq++,
        min: e.data.min,
        max: e.data.max,
      });
    } else if (e.kind === "retract") {
      retracted.add(e.ref);
    }
  }
  if (fresh.length === 0) return;
  logEntries.push(...fresh);
  queueLog(fresh);
  if (retracted.size > 0) {
    adds = adds.filter((a) => !retracted.has(a.id));
    removes = removes.filter((r) => !retracted.has(r.id));
    needsRebuild = true;
  }
  adds.push(...newAdds.filter((a) => !retracted.has(a.id)));
  removes.push(...newRemoves.filter((r) => !retracted.has(r.id)));
  materialize();
  saveReserve(logEntries);
}

// ---------- canvas ----------
function resizeCanvas() {
  const dpr = window.devicePixelRatio || 1;
  const w = boardEl.clientWidth;
  const h = boardEl.clientHeight;
  if (
    boardEl.width !== Math.round(w * dpr) ||
    boardEl.height !== Math.round(h * dpr)
  ) {
    boardEl.width = Math.round(w * dpr);
    boardEl.height = Math.round(h * dpr);
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
  }
  return { w, h };
}

// draw schedules one repaint on the next animation frame, coalescing the many
// synchronous call sites (mouse moves, zooms, batch flushes) into a single
// canvas pass per frame so a busy stroke never paints more than it must.
function draw() {
  if (drawQueued) return;
  drawQueued = true;
  requestAnimationFrame(() => {
    drawQueued = false;
    render();
  });
}

function render() {
  const { w, h } = resizeCanvas();
  ctx.fillStyle = BOARD_BACKGROUND;
  ctx.fillRect(0, 0, w, h);
  drawBoxes(w, h);
  if (gridVisible) drawGrid(w, h);
  drawPreview(w, h);
  drawPendingStroke(w, h);
}

// drawPendingStroke paints the cells of the stroke in progress, before they are
// folded into the view, so the pen and eraser give instant feedback under the
// cursor instead of waiting for the next batch flush.
function drawPendingStroke(w, h) {
  if (paintBatch.length === 0) return;
  const px = cam.scale;
  for (const cmd of paintBatch) {
    if (!cmd.data) continue;
    const sx = (cmd.data.x0 - cam.x) * px + w / 2;
    const sy = (cmd.data.y0 - cam.y) * px + h / 2;
    const sw = (cmd.data.x1 - cmd.data.x0) * px;
    const sh = (cmd.data.y1 - cmd.data.y0) * px;
    if (sx + sw < 0 || sx > w || sy + sh < 0 || sy > h) continue;
    ctx.fillStyle =
      cmd.kind === "erase"
        ? BOARD_BACKGROUND
        : (cmd.meta?.color ?? DEFAULT_COLOR);
    ctx.fillRect(sx, sy, sw, sh);
    ctx.strokeStyle = "#f5c542";
    ctx.lineWidth = 1.5;
    ctx.setLineDash([4, 3]);
    ctx.strokeRect(
      sx + 0.75,
      sy + 0.75,
      Math.max(0, sw - 1.5),
      Math.max(0, sh - 1.5),
    );
    ctx.setLineDash([]);
  }
}

function drawBoxes(w, h) {
  const px = cam.scale;
  for (const b of boxes) {
    const sx = (b.min[0] - cam.x) * px + w / 2;
    const sy = (b.min[1] - cam.y) * px + h / 2;
    const sw = (b.max[0] - b.min[0]) * px;
    const sh = (b.max[1] - b.min[1]) * px;
    if (sx + sw < 0 || sx > w || sy + sh < 0 || sy > h) continue;
    if (b.image) {
      const rec = imageFor(b.image);
      if (rec.el.complete && rec.el.naturalWidth > 0) {
        ctx.drawImage(rec.el, sx, sy, sw, sh);
      } else {
        ctx.fillStyle = b.color;
        ctx.fillRect(sx, sy, sw, sh);
        rec.promise.then(draw); // repaint once the bytes decode
      }
    } else {
      ctx.fillStyle = b.kind === "remove" ? BOARD_BACKGROUND : b.color;
      ctx.fillRect(sx, sy, sw, sh);
    }
    if (pendingIDs.has(b.id)) {
      // Speculative pixels: drawn at full strength, but the dashed amber
      // frame marks the cell as not yet acknowledged until the server echoes
      // the operation back.
      ctx.strokeStyle = "#f5c542";
      ctx.lineWidth = 1.5;
      ctx.setLineDash([4, 3]);
      ctx.strokeRect(
        sx + 0.75,
        sy + 0.75,
        Math.max(0, sw - 1.5),
        Math.max(0, sh - 1.5),
      );
      ctx.setLineDash([]);
    }
  }
}

function drawGrid(w, h) {
  const cell = gridSize(cam.scale, gridOffset);
  // Draw the coarsest power-of-two multiple of the cell that is still readable;
  // strokes still snap to the fine cell, but the grid never vanishes.
  const step = gridStep(cell, cam.scale);
  // Iterate lines relative to the camera, so their indices stay small no
  // matter how far the camera has panned from the origin at extreme zoom.
  const centerX = Math.round(cam.x / step);
  const centerY = Math.round(cam.y / step);
  const offX = cam.x - centerX * step;
  const offY = cam.y - centerY * step;
  const cols = Math.ceil(w / (step * cam.scale) / 2) + 1;
  const rows = Math.ceil(h / (step * cam.scale) / 2) + 1;
  // Dark halo first, so the grid stays legible over white painted cells.
  strokeGrid(step, offX, offY, cols, rows, w, h, "rgba(9, 11, 16, 0.6)", 2);
  // Light core on top, so the grid stays legible over the dark background.
  strokeGrid(
    step,
    offX,
    offY,
    cols,
    rows,
    w,
    h,
    "rgba(232, 236, 244, 0.25)",
    1,
  );
}

function strokeGrid(step, offX, offY, cols, rows, w, h, style, width) {
  ctx.strokeStyle = style;
  ctx.lineWidth = width;
  ctx.beginPath();
  for (let i = -cols; i <= cols; i++) {
    const sx = (i * step - offX) * cam.scale + w / 2;
    ctx.moveTo(sx, 0);
    ctx.lineTo(sx, h);
  }
  for (let j = -rows; j <= rows; j++) {
    const sy = (j * step - offY) * cam.scale + h / 2;
    ctx.moveTo(0, sy);
    ctx.lineTo(w, sy);
  }
  ctx.stroke();
}

function drawPreview(w, h) {
  if (!dragging || tool === "pen" || tool === "eraser") return;
  const r = gridRect(dragStart, dragCur, gridSize(cam.scale, gridOffset));
  const sx = (r.x0 - cam.x) * cam.scale + w / 2;
  const sy = (r.y0 - cam.y) * cam.scale + h / 2;
  ctx.strokeStyle = tool === "erase" ? "#ff6b6b" : "#4f8cff";
  ctx.lineWidth = 2;
  ctx.strokeRect(sx, sy, (r.x1 - r.x0) * cam.scale, (r.y1 - r.y0) * cam.scale);
}

function pointAt(e) {
  return boardPoint(e.offsetX, e.offsetY);
}

// boardPoint maps canvas-local pixels to board units.
function boardPoint(x, y) {
  const { w, h } = resizeCanvas();
  return {
    x: (x - w / 2) / cam.scale + cam.x,
    y: (y - h / 2) / cam.scale + cam.y,
  };
}

// touchPoint maps a touch event to canvas-local pixels.
function touchPoint(t) {
  const rect = boardEl.getBoundingClientRect();
  return { x: t.clientX - rect.left, y: t.clientY - rect.top };
}

function boardPointAtTouch(t) {
  const p = touchPoint(t);
  return boardPoint(p.x, p.y);
}

// cellAt returns the grid cell containing the board point p.
function cellAt(p) {
  const cell = gridSize(cam.scale, gridOffset);
  return { ix: Math.floor(p.x / cell), iy: Math.floor(p.y / cell) };
}

// cellRect returns the half-open cell rectangle of the grid cell c.
function cellRect(c) {
  const cell = gridSize(cam.scale, gridOffset);
  return {
    x0: c.ix * cell,
    y0: c.iy * cell,
    x1: (c.ix + 1) * cell,
    y1: (c.iy + 1) * cell,
  };
}

// newOpID mints the client-side operation identifier, so an undo can retract
// exactly the ops this browser issued.
function newOpID() {
  return crypto.randomUUID();
}

function sendPaint(r) {
  const cmd = paintCmd(r);
  trackStroke(cmd);
  sendCmd(cmd);
}

function sendErase(r) {
  const cmd = eraseCmd(r);
  trackStroke(cmd);
  sendCmd(cmd);
}

// paintCmd and eraseCmd build the command for one rectangle. The pen and the
// eraser stamp these one cell at a time; the rectangle tools send a single one.
function paintCmd(r) {
  return {
    id: newOpID(),
    kind: "paint",
    data: { x0: r.x0, y0: r.y0, x1: r.x1, y1: r.y1 },
    meta: { color: strokeColor },
  };
}

function eraseCmd(r) {
  return {
    id: newOpID(),
    kind: "erase",
    data: { x0: r.x0, y0: r.y0, x1: r.x1, y1: r.y1 },
  };
}

// brushSend stamps one cell with the active brush tool: the pen paints it and
// the eraser erases it. Both cover exactly one grid cell, so the eraser is as
// precise as the pen. Cells are queued locally and flushed in a batch, so one
// materialize folds a whole burst instead of one per cell.
function brushSend(r) {
  const cmd = tool === "eraser" ? eraseCmd(r) : paintCmd(r);
  trackStroke(cmd);
  queuePaint(cmd);
}

// queuePaint buffers brush cells and flushes them together after a short quiet
// window, so a fast pen sweep does not re-materialize per cell.
function queuePaint(cmd) {
  paintBatch.push(cmd);
  draw(); // schedule a repaint so the in-progress cell appears immediately
  if (paintBatchTimer === null) {
    paintBatchTimer = setTimeout(flushPaintBatch, PAINT_BATCH_MS);
  }
}

// flushPaintBatch folds every queued brush cell into the local view and the
// outgoing queue in one pass.
function flushPaintBatch() {
  paintBatchTimer = null;
  const cmds = paintBatch;
  paintBatch = [];
  if (cmds.length === 0) return;
  applyEntries(cmds.map(cmdToEntry), false);
  pending.push(...cmds);
  for (const cmd of cmds) pendingIDs.add(cmd.id);
  savePending();
  updateBacklog();
  flushPending();
}

// ---------- interaction ----------
function startDraw(boardPt) {
  dragging = true;
  beginStroke();
  if (tool === "pen" || tool === "eraser") {
    brushLast = cellAt(boardPt);
    brushSend(cellRect(brushLast));
    return;
  }
  dragStart = boardPt;
  dragCur = boardPt;
}

function moveDraw(boardPt) {
  if (tool === "pen" || tool === "eraser") {
    const cur = cellAt(boardPt);
    const cell = gridSize(cam.scale, gridOffset);
    for (const r of gridLine(brushLast, cur, cell)) brushSend(r);
    brushLast = cur;
    return;
  }
  dragCur = boardPt;
}

function endDraw() {
  if (!dragging) return;
  dragging = false;
  if (tool !== "pen" && tool !== "eraser") commitStroke();
  endStroke();
  flushPaintBatch();
}

function startPan(clientX, clientY) {
  panning = true;
  panLast = { x: clientX, y: clientY };
}

function movePan(clientX, clientY) {
  cam.x -= (clientX - panLast.x) / cam.scale;
  cam.y -= (clientY - panLast.y) / cam.scale;
  panLast = { x: clientX, y: clientY };
  draw();
}

function endPan() {
  panning = false;
}

boardEl.addEventListener("mousedown", (e) => {
  // Middle button, right button, and the pan tool all pan.
  if (e.button === 1 || e.button === 2 || tool === "pan") {
    startPan(e.clientX, e.clientY);
    return;
  }
  if (e.button !== 0) return;
  startDraw(pointAt(e));
});

window.addEventListener("mousemove", (e) => {
  if (panning) {
    movePan(e.clientX, e.clientY);
    return;
  }
  if (!dragging) return;
  moveDraw(pointAt(e));
  draw();
});

window.addEventListener("mouseup", () => {
  if (panning) {
    endPan();
    return;
  }
  endDraw();
});

// A right-drag is a pan, not a context menu.
boardEl.addEventListener("contextmenu", (e) => e.preventDefault());

// Two-finger pinch pans and zooms; one finger draws with the active tool.
boardEl.addEventListener(
  "touchstart",
  (e) => {
    e.preventDefault();
    if (e.touches.length === 2) {
      if (dragging) endDraw();
      if (panning) endPan();
      pinch = beginPinch(e.touches);
      return;
    }
    if (e.touches.length === 1 && !pinch) {
      const t = e.touches[0];
      if (tool === "pan") startPan(t.clientX, t.clientY);
      else startDraw(boardPointAtTouch(t));
    }
  },
  { passive: false },
);

boardEl.addEventListener(
  "touchmove",
  (e) => {
    e.preventDefault();
    if (e.touches.length === 2 && pinch) {
      movePinch(e.touches);
      return;
    }
    if (e.touches.length === 1) {
      const t = e.touches[0];
      if (panning) movePan(t.clientX, t.clientY);
      else if (dragging) {
        moveDraw(boardPointAtTouch(t));
        draw();
      }
    }
  },
  { passive: false },
);

boardEl.addEventListener(
  "touchend",
  (e) => {
    e.preventDefault();
    if (e.touches.length < 2) pinch = null;
    if (e.touches.length === 0) {
      if (dragging) endDraw();
      if (panning) endPan();
    }
  },
  { passive: false },
);

boardEl.addEventListener("touchcancel", () => {
  pinch = null;
  if (dragging) endDraw();
  if (panning) endPan();
});

function beginPinch(touches) {
  const a = touchPoint(touches[0]);
  const b = touchPoint(touches[1]);
  return {
    dist: Math.hypot(b.x - a.x, b.y - a.y),
    cx: (a.x + b.x) / 2,
    cy: (a.y + b.y) / 2,
  };
}

function movePinch(touches) {
  const a = touchPoint(touches[0]);
  const b = touchPoint(touches[1]);
  const dist = Math.hypot(b.x - a.x, b.y - a.y);
  const cx = (a.x + b.x) / 2;
  const cy = (a.y + b.y) / 2;
  // Pan by the midpoint movement, then zoom around the new midpoint.
  cam.x -= (cx - pinch.cx) / cam.scale;
  cam.y -= (cy - pinch.cy) / cam.scale;
  if (pinch.dist > 0 && dist > 0) {
    const before = boardPoint(cx, cy);
    const next = cam.scale * (dist / pinch.dist);
    if (Number.isFinite(next) && next > 0) {
      cam.scale = clampScale(next);
      cam.x = before.x - (cx - resizeCanvas().w / 2) / cam.scale;
      cam.y = before.y - (cy - resizeCanvas().h / 2) / cam.scale;
    }
  }
  pinch.dist = dist;
  pinch.cx = cx;
  pinch.cy = cy;
  updateGridLabel();
  draw();
}

// maxSafeLevel caps the grid level so every cell index stays below 2^53,
// float64's exact-integer range. Far from the origin the finest usable cell
// grows with the distance — the same precision trick used for
// astrophysics-scale coordinates, so no stroke is ever lost to rounding.
function maxSafeLevel() {
  const far = Math.max(1, Math.abs(cam.x), Math.abs(cam.y));
  return Math.max(0, Math.min(MAX_LEVEL, 52 - Math.ceil(Math.log2(far))));
}

// clampScale keeps the camera scale inside the grid's representable range.
function clampScale(next) {
  const lo = MIN_CELL_PX * 2 ** MIN_LEVEL;
  const hi = MIN_CELL_PX * 2 ** maxSafeLevel();
  return Math.min(hi, Math.max(lo, next));
}

// setGridOffset shifts the user's fine/coarse grid adjustment, clamped so the
// effective level stays inside the precision budget at the current position.
function setGridOffset(next) {
  const base = gridLevel(cam.scale, 0);
  gridOffset = Math.max(
    MIN_LEVEL - base,
    Math.min(next, maxSafeLevel() - base),
  );
  updateGridLabel();
  draw();
}

// zoomBy scales the camera around the canvas centre by the given factor.
function zoomBy(factor) {
  const next = cam.scale * factor;
  if (!Number.isFinite(next) || next <= 0) return;
  cam.scale = clampScale(next);
  updateGridLabel();
  draw();
}

function zoomReset() {
  cam.x = 0;
  cam.y = 0;
  cam.scale = INITIAL_SCALE;
  updateGridLabel();
  draw();
}

boardEl.addEventListener(
  "wheel",
  (e) => {
    e.preventDefault();
    const { w, h } = resizeCanvas();
    const beforeX = (e.offsetX - w / 2) / cam.scale + cam.x;
    const beforeY = (e.offsetY - h / 2) / cam.scale + cam.y;
    const next = cam.scale * Math.exp(-e.deltaY * 0.0015);
    if (!Number.isFinite(next) || next <= 0) return;
    cam.scale = clampScale(next);
    cam.x = beforeX - (e.offsetX - w / 2) / cam.scale;
    cam.y = beforeY - (e.offsetY - h / 2) / cam.scale;
    updateGridLabel();
    draw();
  },
  { passive: false },
);

function commitStroke() {
  const r = gridRect(dragStart, dragCur, gridSize(cam.scale, gridOffset));
  if (r.x1 <= r.x0 || r.y1 <= r.y0) return;
  if (tool === "erase") sendErase(r);
  else sendPaint(r);
}

function setTool(name) {
  tool = name;
  rectBtn.classList.toggle("active", name === "rect");
  penBtn.classList.toggle("active", name === "pen");
  eraseBtn.classList.toggle("active", name === "erase");
  eraserBtn.classList.toggle("active", name === "eraser");
  panBtn.classList.toggle("active", name === "pan");
  boardEl.style.cursor = name === "pan" ? "grab" : "crosshair";
  toolLabel.textContent = {
    rect: "Rect",
    pen: "Pen",
    erase: "Erase",
    eraser: "Eraser",
    pan: "Pan",
  }[name];
}

function updateGridLabel() {
  const n = gridLevel(cam.scale, gridOffset);
  const cell = gridSize(cam.scale, gridOffset);
  const offset =
    gridOffset === 0
      ? ""
      : ` · offset ${gridOffset > 0 ? "+" : ""}${gridOffset}`;
  gridLabel.textContent = `${cell} × ${cell} · level ${n}${offset}`;
  const pct = Math.round((cam.scale / INITIAL_SCALE) * 100);
  zoomLabel.textContent = `${pct}%`;
}

// fitBoxes centres the camera on the given boxes, reporting whether there was
// anything to fit.
function fitBoxes(painted) {
  const { w, h } = resizeCanvas();
  const view = fitCamera(painted, w, h);
  if (!view) return false;
  cam.x = view.x;
  cam.y = view.y;
  cam.scale = clampScale(view.scale);
  updateGridLabel();
  draw();
  return true;
}

function fitAll() {
  if (!fitBoxes(boxes.filter((b) => b.kind !== "remove"))) {
    notify("nothing to fit");
  }
}

// ---------- websocket ----------
function setConnected(online) {
  connected = online;
  document.body.classList.toggle("disconnected", !online);
  reconnectBanner.hidden = online;
  importJsonlBtn.disabled = !online; // bulk import still needs the server
  if (online) {
    reconnectAttempts = 0;
    clearReconnectTimer();
    flushPending();
  }
}

function clearReconnectTimer() {
  if (reconnectTimer !== null) {
    clearTimeout(reconnectTimer);
    reconnectTimer = null;
  }
}

// scheduleReconnect queues one retry with exponential back-off: 1s, 2s, 4s,
// … capped at 30s, so a dead server is retried at most twice a minute once
// the cap is reached.
function scheduleReconnect() {
  if (reconnectTimer !== null) return;
  const delay = delayFor(reconnectAttempts);
  reconnectAttempts += 1;
  const secs = Math.round(delay / 1000);
  setStatus(`disconnected — edits queued, retrying in ${secs}s`);
  reconnectText.textContent = `Disconnected — edits queued, retrying in ${secs}s`;
  reconnectTimer = setTimeout(() => {
    reconnectTimer = null;
    connect();
  }, delay);
}

// reconnectNow skips the back-off: the banner button forces an immediate
// fresh connection attempt.
function reconnectNow() {
  clearReconnectTimer();
  reconnectAttempts = 0;
  setStatus("reconnecting…");
  reconnectText.textContent = "Reconnecting…";
  if (socket && socket.readyState !== WebSocket.CLOSED) {
    socket.onclose = null; // this close is ours; connect() below opens a fresh socket
    socket.close();
  }
  socket = null;
  connect();
}

function connect() {
  if (socket && socket.readyState === WebSocket.CONNECTING) return;
  const proto = location.protocol === "https:" ? "wss" : "ws";
  const session = new URLSearchParams(location.search).get("s");
  if (!session) {
    location.replace("/ui/");
    return;
  }
  const ws = new WebSocket(
    `${proto}://${location.host}/ws?s=${encodeURIComponent(session)}&u=${encodeURIComponent(localIdentity())}`,
  );
  socket = ws;
  ws.onopen = () => {
    setConnected(true);
    setStatus(
      pending.length > 0
        ? `connected — syncing ${pending.length} queued edit${pending.length === 1 ? "" : "s"}…`
        : "connected — strokes are shared live",
    );
  };
  ws.onclose = () => {
    setConnected(false);
    scheduleReconnect();
  };
  ws.onerror = () => setStatus("connection error");
  ws.onmessage = (ev) => {
    let msg;
    try {
      msg = JSON.parse(ev.data);
    } catch {
      return;
    }
    handleMessage(msg);
  };
}

function handleMessage(msg) {
  if (msg.clientID) {
    clientID = msg.clientID;
  }

  if (msg.type === "state") {
    // A live server re-sends the full log; an empty one means the session was
    // lost (e.g. the server restarted), so restore from the local reserve.
    const full = msg.ops || [];
    const local = logEntries.length > 0 ? logEntries.slice() : loadReserve();
    resetLocal();
    if (full.length > 0) {
      applyEntries(full);
      // Re-apply and re-send the edits the server has not acknowledged yet.
      for (const cmd of pending) applyLocalCmd(cmd);
      flushPending();
    } else if (local.length > 0) {
      pending = [];
      pendingIDs = new Set();
      replayLocalLog(local);
      setStatus("restored from local copy — syncing…");
    } else {
      materialize();
    }
  } else if (msg.type === "op") {
    if (msg.op) applyEntries([msg.op]);
    if (msg.ops) applyEntries(msg.ops);
  } else if (msg.type === "roster") {
    roster = msg.roster || [];
    renderRoster();
    updatePresence();
  } else if (msg.type === "error") {
    setStatus(`error: ${msg.error}`);
  }
}

function resetLocal() {
  adds = [];
  removes = [];
  boxes = [];
  frontSeq = -1;
  needsRebuild = false;
  opSeq = 0;
  logEntries = [];
  logPending = [];
  seen = new Set();
  if (logTimer !== null) {
    clearTimeout(logTimer);
    logTimer = null;
  }
  if (paintBatchTimer !== null) {
    clearTimeout(paintBatchTimer);
    paintBatchTimer = null;
  }
  paintBatch = [];
  logEl.innerHTML = "";
}

// The browser keeps a reserve copy of the operation log in localStorage, so a
// server restart does not lose the picture: on reconnect, if the server's
// session is empty, the local log is replayed back into it.
function reserveKey() {
  const session = new URLSearchParams(location.search).get("s");
  return session ? `eventfulranges:paint:${session}` : null;
}

function saveReserve(log) {
  const key = reserveKey();
  if (!key) return;
  try {
    localStorage.setItem(key, JSON.stringify(log));
  } catch {
    // Storage may be unavailable (private mode) or full: the in-memory log
    // still keeps the session alive for as long as the page does.
  }
}

function loadReserve() {
  const key = reserveKey();
  if (!key) return [];
  try {
    const raw = localStorage.getItem(key);
    return raw ? JSON.parse(raw) : [];
  } catch {
    return [];
  }
}

// pendingKey names the localStorage slot holding the not-yet-acked outbox.
function pendingKey() {
  const session = new URLSearchParams(location.search).get("s");
  return session ? `eventfulranges:paint:pending:${session}` : null;
}

// savePending persists the outbox so a reload does not drop unsynced edits.
// A large import acknowledges thousands of ops in a burst, so the write is
// debounced: re-serializing the whole outbox per acknowledgement would be
// quadratic. Duplicate re-sends after a reload are harmless — the server
// ignores operations it already knows by ID.
let pendingSaveTimer = null;
const PENDING_SAVE_MS = 300;

function savePending() {
  if (pendingSaveTimer !== null) return;
  pendingSaveTimer = setTimeout(() => {
    pendingSaveTimer = null;
    const key = pendingKey();
    if (!key) return;
    try {
      localStorage.setItem(key, JSON.stringify(pending));
    } catch {
      // Storage may be unavailable (private mode) or full: the in-memory queue
      // still keeps the edits alive for as long as the page does.
    }
  }, PENDING_SAVE_MS);
}

function loadPending() {
  const key = pendingKey();
  if (!key) return [];
  try {
    const raw = localStorage.getItem(key);
    return raw ? JSON.parse(raw) : [];
  } catch {
    return [];
  }
}

function replayLocalLog(log) {
  for (const entry of log) {
    const cmd = entryToCmd(entry);
    if (cmd) {
      applyLocalCmd(cmd);
      enqueue(cmd);
    }
  }
  flushPending();
}

// sendCmd applies one client command to the local view immediately, queues it
// for the server, and tries to flush the queue. It is the local-first write
// path: the server catches up in the background.
function sendCmd(cmd) {
  applyLocalCmd(cmd);
  enqueue(cmd);
  flushPending();
}

// cmdToEntry folds one client command into the log-entry shape.
function cmdToEntry(cmd) {
  const kind =
    cmd.kind === "paint" ? "add" : cmd.kind === "erase" ? "remove" : "retract";
  const entry = {
    id: cmd.id,
    kind,
    ref: cmd.ref,
    client: clientID,
    at: new Date().toISOString(),
  };
  if (cmd.data) {
    entry.data = {
      min: [cmd.data.x0, cmd.data.y0],
      max: [cmd.data.x1, cmd.data.y1],
    };
  }
  if (cmd.meta) entry.meta = cmd.meta;
  return entry;
}

// applyLocalCmd folds one client command into the local view before the server
// echoes it back, so edits render instantly.
function applyLocalCmd(cmd) {
  applyEntries([cmdToEntry(cmd)], false);
}

// enqueue records one outgoing op that the server has not yet acknowledged.
function enqueue(cmd) {
  pending.push(cmd);
  pendingIDs.add(cmd.id);
  savePending();
  updateBacklog();
}

// ackOp drops an acknowledged op from the outgoing queue.
function ackOp(id) {
  if (!pendingIDs.has(id)) return;
  pendingIDs.delete(id);
  pending = pending.filter((c) => c.id !== id);
  savePending();
  updateBacklog();
}

// flushPending sends every queued op to the server; duplicates are ignored
// server-side by operation ID, so re-sending the whole queue is safe.
function flushPending() {
  if (!connected || !socket || socket.readyState !== WebSocket.OPEN) return;
  for (const cmd of pending) {
    socket.send(JSON.stringify(cmd));
  }
}

// updateBacklog shows how many local ops are still waiting for the server.
function updateBacklog() {
  backlogEl.hidden = pending.length === 0;
  backlogEl.textContent = `↻ ${pending.length} op${pending.length === 1 ? "" : "s"} to sync`;
  zenBacklog.hidden = pending.length === 0;
  zenBacklog.textContent = `↻ ${pending.length}`;
}

function setStatus(text) {
  statusEl.textContent = text;
}

// notify flashes a transient toast for one-shot feedback (copy, download,
// import) that would otherwise be invisible at the top of a phone screen.
let toastTimer = null;

function notify(text) {
  toastEl.textContent = text;
  toastEl.classList.add("show");
  if (toastTimer !== null) clearTimeout(toastTimer);
  toastTimer = setTimeout(() => {
    toastEl.classList.remove("show");
    toastTimer = null;
  }, 2400);
}

// updatePresence derives the "here" and "connected" counts from the roster, so
// one user with several tabs counts once, not once per connection.
function updatePresence() {
  const session = currentSession();
  const here = session
    ? roster.filter((e) => e.sessions.includes(session)).length
    : 0;
  const connected = roster.length;
  hereLink.textContent = `${here} here`;
  connectedLink.textContent = `${connected} connected`;
  meLabel.textContent = session ? ` · you are ${shortID(session)}` : "";
}

// currentSession returns the session id from the share URL, the identifier the
// roster lists, so "you are …" matches what collaborators see.
function currentSession() {
  return new URLSearchParams(location.search).get("s");
}

// shortID shortens an identifier to its first five characters, matching the
// length of the client IDs the activity log already shows.
function shortID(id) {
  return id.slice(0, 5);
}

// ---------- who's here ----------
const ROSTER_CAP = 20; // at most this many users are listed
const IDENTITY_KEY = "eventfulranges:paint:me";

// localIdentity is a stable, self-chosen guest email kept in localStorage, so
// runs without oauth2-proxy still group one browser's sessions together.
function localIdentity() {
  let me = null;
  try {
    me = localStorage.getItem(IDENTITY_KEY);
  } catch {
    // storage unavailable: fall through and mint an in-memory one
  }
  if (!me) {
    me = `anon-${randomToken()}@local`;
    try {
      localStorage.setItem(IDENTITY_KEY, me);
    } catch {
      // best effort; the in-memory value still keeps this page coherent
    }
  }
  return me;
}

// randomToken mints a short, URL-safe hex token for the local guest identity.
function randomToken() {
  const bytes = new Uint8Array(8);
  crypto.getRandomValues(bytes);
  return Array.from(bytes, (b) => b.toString(16).padStart(2, "0")).join("");
}

// renderRoster draws the capped "who's here" list for the active view.
function renderRoster() {
  const entries = rosterEntries();
  rosterTitleEl.textContent =
    rosterView === "here" ? "in this session" : "all sessions";
  rosterListEl.innerHTML = "";
  if (entries.length === 0) {
    const li = document.createElement("li");
    li.textContent = "nobody else";
    rosterListEl.appendChild(li);
    return;
  }
  const shown = entries.slice(0, ROSTER_CAP);
  for (const entry of shown) {
    const li = document.createElement("li");
    li.textContent = `${entry.user} — ${entry.sessions.map(shortID).join(", ")}`;
    rosterListEl.appendChild(li);
  }
  const more = entries.length - shown.length;
  rosterMoreEl.hidden = more <= 0;
  if (more > 0) rosterMoreEl.textContent = `and ${more} more…`;
}

// rosterEntries filters the global roster for the active view: the current
// session for "here", everything for "connected".
function rosterEntries() {
  const session = currentSession();
  if (rosterView === "here" && session) {
    return roster.filter((e) => e.sessions.includes(session));
  }
  return roster;
}

function showRoster(view) {
  if (!rosterPanelEl.hidden && rosterView === view) {
    rosterPanelEl.hidden = true; // clicking the active badge closes it
    return;
  }
  rosterView = view;
  rosterPanelEl.hidden = false;
  renderRoster();
}

hereLink.addEventListener("click", () => showRoster("here"));
connectedLink.addEventListener("click", () => showRoster("connected"));
statusEl.addEventListener("click", () => showRoster("connected"));

// ---------- undo / redo ----------
const UNDO_LIMIT = 100; // at most this many strokes can be undone
let undoStack = []; // each entry is one stroke: an array of paint/erase commands
let redoStack = [];
let stroke = []; // the commands of the stroke currently being drawn

// beginStroke starts collecting a stroke; endStroke commits it to the undo
// stack once the pointer is lifted.
function beginStroke() {
  stroke = [];
}

function trackStroke(cmd) {
  stroke.push(cmd);
}

function endStroke() {
  if (stroke.length === 0) return;
  undoStack.push(stroke);
  if (undoStack.length > UNDO_LIMIT) undoStack.shift();
  redoStack = [];
  updateUndoRedo();
  stroke = [];
}

// undo retracts the last stroke's operations by ID, so it undoes only this
// browser's own edits; redo re-issues them under fresh IDs.
function undo() {
  const s = undoStack.pop();
  if (!s) return;
  redoStack.push(s);
  for (let i = s.length - 1; i >= 0; i--) {
    sendCmd({ id: newOpID(), kind: "retract", ref: s[i].id });
  }
  updateUndoRedo();
}

function redo() {
  const s = redoStack.pop();
  if (!s) return;
  const reissued = s.map((cmd) => ({ ...cmd, id: newOpID() }));
  undoStack.push(reissued);
  for (const cmd of reissued) sendCmd(cmd);
  updateUndoRedo();
}

function updateUndoRedo() {
  undoBtn.disabled = undoStack.length === 0;
  redoBtn.disabled = redoStack.length === 0;
}

undoBtn.addEventListener("click", undo);
redoBtn.addEventListener("click", redo);

// queueLog batches incoming ops and renders them after a short quiet period:
// a single op keeps its full line, a burst collapses into one summary line.
function queueLog(entries) {
  logPending.push(...entries);
  if (logTimer !== null) clearTimeout(logTimer);
  logTimer = setTimeout(flushLog, LOG_DEBOUNCE_MS);
}

function flushLog() {
  logTimer = null;
  const batch = logPending;
  logPending = [];
  if (batch.length === 0) return;
  appendLog(batch.length === 1 ? entryLine(batch[0]) : summaryLine(batch));
}

function entryLine(entry) {
  const li = document.createElement("li");
  li.className = entry.kind;
  const when = new Date(entry.at).toLocaleTimeString();
  li.textContent = `${when}  ${entry.client}  ${entry.kind} ${entry.detail ?? ""}`;
  if (entry.client === clientID) li.classList.add("me");
  return li;
}

function summaryLine(batch) {
  const groups = new Map(); // `${client}\u0000${kind}` -> {client, kind, n}
  for (const e of batch) {
    const key = `${e.client}\u0000${e.kind}`;
    const g = groups.get(key) ?? { client: e.client, kind: e.kind, n: 0 };
    g.n += 1;
    groups.set(key, g);
  }
  const byClient = new Map();
  for (const g of groups.values()) {
    const parts = byClient.get(g.client) ?? [];
    parts.push(`${g.kind} ×${g.n}`);
    byClient.set(g.client, parts);
  }
  const when = new Date(batch[batch.length - 1].at).toLocaleTimeString();
  const text = [...byClient.entries()]
    .map(([client, parts]) =>
      byClient.size === 1 ? parts.join(", ") : `${client}: ${parts.join(", ")}`,
    )
    .join(" · ");
  const li = document.createElement("li");
  li.className = "summary";
  if (batch.every((e) => e.client === clientID)) li.classList.add("me");
  li.textContent = `${when}  ${text}`;
  return li;
}

function appendLog(li) {
  logEl.appendChild(li);
  while (logEl.children.length > LOG_MAX) logEl.removeChild(logEl.firstChild);
}

// timestampedName returns a filesystem-safe export name stamped with the
// local time, so repeated exports never overwrite each other, e.g.
// board-20260902-073500.png.
function timestampedName(base, ext) {
  const d = new Date();
  const pad = (n) => String(n).padStart(2, "0");
  const stamp =
    `${d.getFullYear()}${pad(d.getMonth() + 1)}${pad(d.getDate())}` +
    `-${pad(d.getHours())}${pad(d.getMinutes())}${pad(d.getSeconds())}`;
  return `${base}-${stamp}.${ext}`;
}

function downloadJSONL() {
  const lines = logEntries.map((e) =>
    JSON.stringify({
      id: e.id,
      kind: e.kind,
      ref: e.ref,
      data: e.data,
      meta: e.meta,
      client: e.client,
      at: e.at,
    }),
  );
  const blob = new Blob([lines.join("\n") + (lines.length ? "\n" : "")], {
    type: "application/x-ndjson",
  });
  const a = document.createElement("a");
  a.href = URL.createObjectURL(blob);
  a.download = timestampedName("board", "infj");
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(a.href);
}

// ---------- settings ----------
// The selected colour and the last export choices are remembered in
// localStorage, so a reload keeps them. Storage may be unavailable (private
// mode) or full, in which case the preferences simply do not survive.
function loadSettings() {
  try {
    const raw = localStorage.getItem(SETTINGS_KEY);
    return raw ? JSON.parse(raw) : {};
  } catch {
    return {};
  }
}

function saveSettings() {
  try {
    localStorage.setItem(SETTINGS_KEY, JSON.stringify(settings));
  } catch {
    // best effort: the in-memory preferences still hold for this page load
  }
}

// ---------- export ----------
// The board can be saved as PNG, JPEG, or SVG at a caller-chosen size. The
// drawing is fit with a "contain" transform — never stretched or clipped —
// and the grid is never drawn, so the file carries only the painted cells.
function openExportDialog() {
  if (!boxes.some((b) => b.kind !== "remove")) {
    notify("nothing to export — draw something first");
    return;
  }
  const size = suggestDimensions(boxes);
  exportWidth.value = Number.isFinite(settings.width)
    ? settings.width
    : size.width;
  exportHeight.value = Number.isFinite(settings.height)
    ? settings.height
    : size.height;
  exportFormat.value = ["png", "jpeg", "svg"].includes(settings.format)
    ? settings.format
    : "png";
  exportRatio.checked = settings.keepRatio ?? true;
  exportPixelSize.value = ["1:1", "grid", "pixels"].includes(settings.pixelSize)
    ? settings.pixelSize
    : "1:1";
  syncExportFields();
  showExportError(null);
  if (!exportDialog.open) exportDialog.showModal();
}

function closeExportDialog() {
  exportDialog.close();
}

function showExportError(message) {
  exportError.textContent = message ?? "";
  exportError.hidden = !message;
}

async function submitExport() {
  const format = exportFormat.value;

  // SVG is vector: it needs no size, so it skips the dimension fields entirely.
  if (format === "svg") {
    setExportBusy(true);
    try {
      await exportImage(format, 0, 0);
      settings.format = format;
      saveSettings();
      closeExportDialog();
      notify("exported board.svg (vector — any size)");
    } catch (err) {
      showExportError(err.message || "export failed");
    } finally {
      setExportBusy(false);
    }
    return;
  }

  const { size, custom } = rasterExportSize(exportPixelSize.value);
  if (!size.ok) {
    showExportError(size.error);
    return;
  }
  if (size.width > MAX_WIDTH || size.height > MAX_HEIGHT) {
    showExportError(`too large for export — max ${MAX_WIDTH} px per side`);
    return;
  }
  setExportBusy(true);
  try {
    await exportImage(format, size.width, size.height);
    settings.format = format;
    settings.pixelSize = exportPixelSize.value;
    if (custom) {
      settings.width = custom.width;
      settings.height = custom.height;
      settings.keepRatio = custom.keepRatio;
    }
    saveSettings();
    closeExportDialog();
    notify(
      `exported board.${extension(format)} (${size.width} × ${size.height})`,
    );
  } catch (err) {
    showExportError(err.message || "export failed");
  } finally {
    setExportBusy(false);
  }
}

// rasterExportSize resolves the pixel size for one raster export. The custom
// mode uses the width/height fields; "grid" maps one current grid cell to one
// pixel; "pixels" fits the drawing into the current viewport. `custom` carries
// the raw fields to persist, only when they were used.
function rasterExportSize(pixelSize) {
  if (pixelSize === "grid") {
    return { size: gridExportSize(boxes, gridSize(cam.scale, gridOffset)) };
  }
  if (pixelSize === "pixels") {
    return { size: { ok: true, width: boardEl.width, height: boardEl.height } };
  }
  const parsed = parseDimensions(exportWidth.value, exportHeight.value);
  if (!parsed.ok) return { size: parsed };
  return {
    size: resolveDimensions(
      boxes,
      parsed.width,
      parsed.height,
      exportRatio.checked,
    ),
    custom: {
      width: parsed.width,
      height: parsed.height,
      keepRatio: exportRatio.checked,
    },
  };
}

// setExportBusy disables the export button and relabels it while a request is
// in flight, so a slow server-side render is visibly acknowledged.
function setExportBusy(busy) {
  exportGo.disabled = busy;
  exportGo.textContent = busy ? "Exporting…" : "Export";
}

// syncExportFields hides the pixel size controls when the chosen format has no
// fixed resolution, leaving only the format selector.
function syncExportFields() {
  const svg = exportFormat.value === "svg";
  const custom = !svg && exportPixelSize.value === "1:1";
  exportPixelSizeField.hidden = svg;
  exportSizeFields.hidden = !custom;
  exportRatioField.hidden = !custom;
  exportHint.hidden = !custom;
}

function extension(format) {
  return format === "jpeg" ? "jpg" : format;
}

async function exportImage(format, width, height) {
  if (format === "svg") {
    downloadBlob(
      new Blob([renderSVG(boxes)], { type: "image/svg+xml" }),
      timestampedName("board", "svg"),
    );
    return;
  }

  const view = fitExport(boxes, width, height);
  if (!view.ok) throw new Error(view.error);

  // A raster small enough for the browser canvas renders locally; a larger one
  // (or one the browser cannot encode after all) goes to the server, which
  // materializes the authoritative view and has no canvas size limit.
  if (checkRasterSize(width, height).ok) {
    const blob = await exportClientRaster(format, width, height, view);
    if (blob) {
      downloadBlob(
        blob,
        timestampedName("board", format === "jpeg" ? "jpg" : "png"),
      );
      return;
    }
  }
  await exportServerRaster(format, width, height);
}

// exportClientRaster draws the board on a browser canvas and encodes it,
// resolving to null when the browser cannot encode the requested size.
async function exportClientRaster(format, width, height, view) {
  const canvas = document.createElement("canvas");
  canvas.width = width;
  canvas.height = height;
  // Browsers cap the canvas backing store; a silently clamped size means the
  // client cannot rasterize it.
  if (canvas.width !== width || canvas.height !== height) {
    return null;
  }
  const jpeg = format === "jpeg";
  const c = canvas.getContext("2d");
  // PNG keeps empty space transparent; JPEG has no alpha, so it fills the
  // flat board background instead.
  if (jpeg) {
    c.fillStyle = BACKGROUND;
    c.fillRect(0, 0, width, height);
  }
  for (const b of boxes) {
    const r = snapRect(projectBox(b, view));
    if (b.image) {
      const img = await imageFor(b.image).promise;
      c.drawImage(img, r.x, r.y, r.w, r.h);
      continue;
    }
    if (b.kind === "remove") {
      // An erase clears to transparent for PNG; JPEG repaints the background.
      if (jpeg) {
        c.fillStyle = BACKGROUND;
        c.fillRect(r.x, r.y, r.w, r.h);
      } else {
        c.clearRect(r.x, r.y, r.w, r.h);
      }
      continue;
    }
    c.fillStyle = sanitizeColor(b.color);
    c.fillRect(r.x, r.y, r.w, r.h);
  }
  return new Promise((resolve) =>
    canvas.toBlob(
      resolve,
      jpeg ? "image/jpeg" : "image/png",
      jpeg ? 0.92 : undefined,
    ),
  );
}

async function exportServerRaster(format, width, height) {
  const session = new URLSearchParams(location.search).get("s");
  const url = `/api/export?s=${encodeURIComponent(session)}&format=${format}&w=${width}&h=${height}`;
  const res = await fetch(url);
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || "export failed");
  }
  const blob = await res.blob();
  downloadBlob(
    blob,
    timestampedName("board", format === "jpeg" ? "jpg" : "png"),
  );
}

function downloadBlob(blob, filename) {
  const a = document.createElement("a");
  a.href = URL.createObjectURL(blob);
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(a.href);
}

// ---------- image import ----------
// A PNG or JPEG is rasterized into board boxes (one pixel per board unit) and
// then scaled onto the board: 1:1 keeps one pixel per cell, "grid" makes one
// pixel one current grid cell, and "pixels" fits the image into the viewport.
// Contiguous runs of the same colour merge into rectangles, so a flat region
// becomes a single box. When "new canvas" is checked the board is cleared first.

// rasterize decodes a file into merged board boxes at a resolution whose box
// count fits the board's budget, halving the size until it does. A photo that
// would explode into millions of cells is imported at a smaller size instead of
// being rejected as too detailed. It returns the boxes, the effective pixel
// size, and the downscale factor (1 means full resolution).
async function rasterize(file) {
  // The file size is the first, cheapest protection: reject before decoding.
  if (file.size > MAX_IMPORT_BYTES) {
    throw new Error(
      `file too large — max ${MAX_IMPORT_BYTES / 1024 / 1024} MB`,
    );
  }
  let bitmap;
  try {
    bitmap = await createImageBitmap(file);
  } catch {
    throw new Error("could not read the image");
  }
  if (bitmap.width > MAX_IMPORT_DIM || bitmap.height > MAX_IMPORT_DIM) {
    bitmap.close();
    throw new Error(
      `image too large — max ${MAX_IMPORT_DIM} × ${MAX_IMPORT_DIM} px`,
    );
  }

  let factor = 1;
  let width = bitmap.width;
  let height = bitmap.height;
  let px = rasterPixels(bitmap, width, height);
  let cells = boxesFromRaster(px.data, px.width, px.height);
  while (cells.length > MAX_IMPORT_BOXES && (width > 1 || height > 1)) {
    factor /= 2;
    width = Math.max(1, Math.round(bitmap.width * factor));
    height = Math.max(1, Math.round(bitmap.height * factor));
    px = rasterPixels(bitmap, width, height);
    cells = boxesFromRaster(px.data, px.width, px.height);
  }
  bitmap.close();
  return { cells, width, height, factor };
}

// rasterPixels draws a bitmap at width × height and returns its flat RGBA data
// and dimensions, releasing the scratch canvas's backing store.
function rasterPixels(bitmap, width, height) {
  const canvas = document.createElement("canvas");
  canvas.width = width;
  canvas.height = height;
  const c = canvas.getContext("2d");
  c.drawImage(bitmap, 0, 0, width, height);
  const data = c.getImageData(0, 0, width, height).data;
  canvas.width = 0; // release the decoded pixels
  return { data, width, height };
}

async function importImage(file, pixelSize, clear, frozen) {
  if (!connected) throw new Error("disconnected — reconnect to import");
  if (frozen) {
    if (file.size > MAX_FROZEN_BYTES) {
      throw new Error(
        `background image too large — max ${MAX_FROZEN_BYTES / 1024 / 1024} MB`,
      );
    }
    const dataUrl = await readAsDataURL(file);
    const { width, height } = await imageSize(file);
    const t = importTransform(pixelSize, width, height);
    const cmds = [];
    if (clear) {
      const erase = clearCmd();
      if (erase) cmds.push(erase);
    }
    cmds.push({
      id: newOpID(),
      kind: "paint",
      data: {
        x0: t.ox,
        y0: t.oy,
        x1: t.ox + width * t.cell,
        y1: t.oy + height * t.cell,
      },
      meta: { color: DEFAULT_COLOR, image: dataUrl, frozen: true },
    });
    commitImport(cmds);
    notify(`imported image (${width} × ${height} px, frozen)`);
    return;
  }
  const { cells: raw, width, height, factor } = await rasterize(file);
  if (raw.length === 0) throw new Error("image has no opaque pixels");

  const t = importTransform(pixelSize, width, height);
  const cells =
    t.cell === 1 && t.ox === 0 && t.oy === 0
      ? raw
      : scaleBoxes(raw, t.cell, t.ox, t.oy);

  const cmds = [];
  if (clear) {
    const erase = clearCmd();
    if (erase) cmds.push(erase);
  }
  for (const cell of cells) {
    cmds.push({
      id: newOpID(),
      kind: "paint",
      data: {
        x0: cell.min[0],
        y0: cell.min[1],
        x1: cell.max[0],
        y1: cell.max[1],
      },
      meta: { color: cell.color },
    });
  }

  commitImport(cmds);

  if (pixelSize === "1:1") {
    fitBoxes([{ min: [0, 0], max: [width, height] }]);
  }
  const downscaled = factor < 1 ? ` at ${width} × ${height} px` : "";
  notify(
    `imported image (${cells.length} cell${cells.length === 1 ? "" : "s"}${downscaled})`,
  );
}

// commitImport folds the import's commands into the local view, queues them for
// the server, and records the whole import as one undoable stroke.
function commitImport(cmds) {
  applyEntries(cmds.map(cmdToEntry), false);
  pending.push(...cmds);
  for (const cmd of cmds) pendingIDs.add(cmd.id);
  savePending();
  updateBacklog();
  flushPending();
  undoStack.push(cmds);
  if (undoStack.length > UNDO_LIMIT) undoStack.shift();
  redoStack = [];
  updateUndoRedo();
}

// importTransform returns the board scale (board units per image pixel) and
// origin for one import: 1:1 starts at the origin, "grid" uses the current
// grid cell at the viewport's top-left, and "pixels" fits the image into the
// viewport at its top-left.
function importTransform(pixelSize, imgW, imgH) {
  if (pixelSize === "grid") {
    return { cell: gridSize(cam.scale, gridOffset), ...viewTopLeft() };
  }
  if (pixelSize === "pixels") {
    const viewW = boardEl.clientWidth / cam.scale;
    const viewH = boardEl.clientHeight / cam.scale;
    return { cell: Math.min(viewW / imgW, viewH / imgH), ...viewTopLeft() };
  }
  return { cell: 1, ox: 0, oy: 0 };
}

// viewTopLeft returns the board coordinates of the viewport's top-left corner,
// snapped down to the current grid so an import lands on a grid line. It uses
// the canvas's CSS size (clientWidth/Height), not its device-pixel backing
// store, so the import lands at the visible canvas edge rather than under the
// sidebar on hi-dpi screens.
function viewTopLeft() {
  const cell = gridSize(cam.scale, gridOffset);
  return {
    ox: Math.floor((cam.x - boardEl.clientWidth / cam.scale / 2) / cell) * cell,
    oy:
      Math.floor((cam.y - boardEl.clientHeight / cam.scale / 2) / cell) * cell,
  };
}

// ---------- embedded images ----------
// A frozen import stores its source bytes as a data URL in the box metadata;
// decode once and cache the image for the renderer and the exporters.
const imageCache = new Map(); // dataUrl -> {el, promise}

function imageFor(src) {
  let rec = imageCache.get(src);
  if (!rec) {
    const el = new Image();
    const promise = new Promise((resolve, reject) => {
      el.onload = () => resolve(el);
      el.onerror = () => reject(new Error("could not decode the image"));
    });
    el.src = src;
    rec = { el, promise };
    imageCache.set(src, rec);
  }
  return rec;
}

// readAsDataURL returns the file's bytes as a data URL, so the original image
// travels in the log without re-encoding.
function readAsDataURL(file) {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(reader.result);
    reader.onerror = () => reject(new Error("could not read the image"));
    reader.readAsDataURL(file);
  });
}

// imageSize decodes just enough of the file to read its pixel dimensions.
async function imageSize(file) {
  let bitmap;
  try {
    bitmap = await createImageBitmap(file);
  } catch {
    throw new Error("could not read the image");
  }
  const { width, height } = bitmap;
  bitmap.close();
  return { width, height };
}

// clearCmd erases the union of everything painted so far, or null when the
// board is already empty.
function clearCmd() {
  const b = bounds(boxes.filter((box) => box.kind !== "remove"));
  if (!b) return null;
  return {
    id: newOpID(),
    kind: "erase",
    data: { x0: b.min[0], y0: b.min[1], x1: b.max[0], y1: b.max[1] },
  };
}

// ---------- import dialog ----------
function openImportDialog() {
  importClear.checked = settings.importClear ?? false;
  importFrozen.checked = settings.importFrozen ?? false;
  importPixelSize.value = ["1:1", "grid", "pixels"].includes(
    settings.importPixelSize,
  )
    ? settings.importPixelSize
    : "1:1";
  syncImportHint();
  showImportError(null);
  if (!importDialog.open) importDialog.showModal();
}

function closeImportDialog() {
  importDialog.close();
  importFileInput.value = "";
}

function showImportError(message) {
  importError.textContent = message ?? "";
  importError.hidden = !message;
}

// syncImportHint states the limit that applies to the chosen import mode: a
// frozen background embeds the original bytes, so it is capped by size; the
// editable mode rasterizes to cells, so it is capped by pixel dimensions.
function syncImportHint() {
  importHint.textContent = importFrozen.checked
    ? `background image — up to ${MAX_FROZEN_BYTES / 1024 / 1024} MB each`
    : `painted image — up to ${MAX_IMPORT_DIM} × ${MAX_IMPORT_DIM} px, converted to cells`;
}

async function submitImport() {
  const file = importFileInput.files && importFileInput.files[0];
  if (!file) {
    showImportError("choose an image first");
    return;
  }
  setImportBusy(true);
  try {
    await importImage(
      file,
      importPixelSize.value,
      importClear.checked,
      importFrozen.checked,
    );
    settings.importPixelSize = importPixelSize.value;
    settings.importClear = importClear.checked;
    settings.importFrozen = importFrozen.checked;
    saveSettings();
    closeImportDialog();
  } catch (err) {
    showImportError(err.message || "import failed");
  } finally {
    setImportBusy(false);
  }
}

// setImportBusy disables the import button and relabels it while a decode is
// in flight, mirroring the export dialog's feedback.
function setImportBusy(busy) {
  importGo.disabled = busy;
  importGo.textContent = busy ? "Importing…" : "Import";
}

async function importJSONL(input) {
  if (!connected) {
    setStatus("disconnected — reconnect to import");
    input.value = "";
    return;
  }
  const file = input.files && input.files[0];
  if (!file) return;
  const text = await file.text();
  let imported = 0;
  let skipped = 0;
  const boxes = [];
  for (const line of text.split("\n")) {
    if (!line.trim()) continue;
    let entry;
    try {
      entry = JSON.parse(line);
    } catch {
      skipped++;
      continue;
    }
    const cmd = entryToCmd(entry);
    if (!cmd) {
      skipped++;
      continue;
    }
    if (entry.kind === "add") {
      boxes.push({ min: entry.data.min, max: entry.data.max });
    }
    if (socket && socket.readyState === WebSocket.OPEN) {
      socket.send(JSON.stringify(cmd));
      imported++;
    } else {
      skipped++;
    }
  }
  if (boxes.length) {
    const { w, h } = resizeCanvas();
    const view = fitCamera(boxes, w, h);
    if (view) {
      cam.x = view.x;
      cam.y = view.y;
      cam.scale = view.scale;
      updateGridLabel();
    }
  }
  notify(`imported ${imported} ops${skipped ? `, skipped ${skipped}` : ""}`);
  input.value = "";
  draw();
}

function entryToCmd(entry) {
  if (entry.kind === "retract") {
    if (!entry.ref) return null;
    const cmd = { kind: "retract", ref: entry.ref };
    if (entry.id) cmd.id = entry.id;
    return cmd;
  }
  const kind =
    entry.kind === "add" ? "paint" : entry.kind === "remove" ? "erase" : null;
  if (!kind || !entry.data) return null;
  const { min, max } = entry.data;
  if (!Array.isArray(min) || !Array.isArray(max)) return null;
  const cmd = {
    kind,
    data: { x0: min[0], y0: min[1], x1: max[0], y1: max[1] },
  };
  if (entry.id) cmd.id = entry.id;
  if (entry.meta) cmd.meta = entry.meta;
  return cmd;
}

// ---------- events ----------
rectBtn.addEventListener("click", () => setTool("rect"));
penBtn.addEventListener("click", () => setTool("pen"));
eraseBtn.addEventListener("click", () => setTool("erase"));
eraserBtn.addEventListener("click", () => setTool("eraser"));
panBtn.addEventListener("click", () => setTool("pan"));

// zenBtn toggles focus mode: the canvas takes the whole viewport and the
// toolbar floats over it, so only the drawing controls stay visible.
zenBtn.addEventListener("click", () => {
  const zen = document.body.classList.toggle("zen");
  zenBtn.classList.toggle("active", zen);
  zenBtn.setAttribute("aria-pressed", String(zen));
  zenBtn.title = zen ? "Exit focus mode" : "Focus mode: full-screen editing";
  // The layout changes on the next frame; redraw then, because resizing the
  // canvas clears it and only draw() repaints the grid and boxes.
  requestAnimationFrame(draw);
});

strokeColorEl.addEventListener("input", () => {
  strokeColor = strokeColorEl.value;
  settings.color = strokeColor;
  saveSettings();
});

importJsonlBtn.addEventListener("click", () => importJsonlInput.click());
importJsonlInput.addEventListener("change", () =>
  importJSONL(importJsonlInput),
);

importImageBtn.addEventListener("click", openImportDialog);
importFrozen.addEventListener("change", syncImportHint);
importCancel.addEventListener("click", closeImportDialog);
importForm.addEventListener("submit", (e) => {
  e.preventDefault();
  submitImport();
});

gridPlus.addEventListener("click", () => setGridOffset(gridOffset + 1));
gridMinus.addEventListener("click", () => setGridOffset(gridOffset - 1));
gridDefault.addEventListener("click", () => {
  gridOffset = 0;
  updateGridLabel();
  draw();
});
gridToggle.addEventListener("click", () => {
  gridVisible = !gridVisible;
  gridToggle.classList.toggle("active", !gridVisible);
  gridToggle.setAttribute("aria-pressed", String(!gridVisible));
  gridToggle.title = gridVisible ? "Hide the grid" : "Show the grid";
  draw();
});
fitAllBtn.addEventListener("click", fitAll);

zoomInBtn.addEventListener("click", () => zoomBy(2));
zoomOutBtn.addEventListener("click", () => zoomBy(0.5));
zoomResetBtn.addEventListener("click", zoomReset);

copyShareBtn.addEventListener("click", async () => {
  try {
    await navigator.clipboard.writeText(location.href);
    notify("share link copied");
  } catch {
    notify("could not copy link");
  }
});

downloadJsonlBtn.addEventListener("click", () => {
  downloadJSONL();
  notify(`downloaded board (${logEntries.length} ops)`);
});

exportBtn.addEventListener("click", openExportDialog);
exportCancel.addEventListener("click", closeExportDialog);
exportFormat.addEventListener("change", syncExportFields);
exportPixelSize.addEventListener("change", syncExportFields);
exportForm.addEventListener("submit", (e) => {
  e.preventDefault();
  submitExport();
});

newSessionBtn.addEventListener("click", () => location.replace("/ui/"));

window.addEventListener("resize", draw);

reconnectBtn.addEventListener("click", reconnectNow);

// ---------- onboarding tour ----------
// Shown once per browser, and reopenable from the help button.
const TOUR_STEPS = [
  { targetSelectors: ["#toolbar", "#board"], title: "Welcome", body: "Welcome to ± infinite paint. Every stroke is an event in an append-only log, merged by a CRDT, so every browser that opens this link converges on the same canvas." },
  { targetSelector: "#toolRect", title: "Tools", body: "Draw with the rectangle, pen, and erase tools; the eraser brush and pan complete the set." },
  { targetSelector: "#strokeColor", title: "Color", body: "Pick the stroke color before drawing — it travels with each box as metadata." },
  { targetSelectors: ["#gridMinus", "#gridDefault", "#gridPlus", "#gridToggle"], title: "Grid", body: "The fractal grid snaps every stroke to cells; coarsen, refine, or hide it." },
  { targetSelectors: ["#zoomOut", "#zoomReset", "#zoomIn"], title: "Zoom & pan", body: "Zoom with these buttons, scroll, or pinch; drag to pan across the infinite board." },
  { targetSelector: "#fitAll", title: "Fit all", body: "Re-frame the whole painted area in view." },
  { targetSelector: "#copyShare", title: "Share", body: "Copy the share link so everyone joins the same canvas and converges on the same log." },
  { targetSelector: "#exportBtn", title: "Export", body: "Save the board as PNG, JPEG, or SVG — or download the raw JSONL log." },
  { targetSelector: "#helpTourBtn", title: "Need a refresher?", body: "You can always reopen this tutorial from the help button." },
];

// ---------- boot ----------
function boot() {
  settings = loadSettings();
  strokeColor = sanitizeColor(settings.color);
  strokeColorEl.value = strokeColor;
  resizeCanvas();
  setTool("rect");
  updateGridLabel();
  pending = loadPending();
  pendingIDs = new Set(pending.map((c) => c.id));
  updateBacklog();
  draw();
  connect();
  startTour(TOUR_STEPS);
  const helpTourBtn = document.getElementById("helpTourBtn");
  if (helpTourBtn) helpTourBtn.addEventListener("click", () => startTour(TOUR_STEPS, { force: true }));
  checkAdmin();
}

// checkAdmin reveals the admin link when the server recognizes this user as an
// admin (the server gates /admin by the reverse-proxy email, so a 403 hides it).
async function checkAdmin() {
  try {
    const res = await fetch("/admin/api/info");
    adminLink.hidden = !res.ok;
  } catch {
    adminLink.hidden = true;
  }
}

boot();

// Test seam: Playwright closes the live socket through this to exercise the
// reconnection flow. It is inert in normal use.
window.__eventfulranges = { closeSocket: () => socket && socket.close() };
