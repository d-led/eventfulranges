import { normalize, subtractAll } from "./boxes.js";
import {
  gridLevel,
  gridSize,
  gridStep,
  gridRect,
  gridLine,
  fitCamera,
} from "./grid.js";
import { delayFor } from "./backoff.js";

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

const ctx = boardEl.getContext("2d");

// ---------- state ----------
let adds = []; // [{id, min, max, color}] painted boxes, one per operation
let removes = []; // [{id, min, max}] erased boxes, one per operation
let boxes = []; // materialized view: [{min, max}]
let logEntries = []; // the full operation log, for JSONL export
let clientID = "";
let socket = null;
let connected = false; // true once a socket has opened, false after it closes
let reconnectAttempts = 0;
let reconnectTimer = null;
let roster = []; // [{user, sessions}] broadcast by the server
let rosterView = "connected"; // "here" (this session) or "connected" (all)

const LOG_MAX = 100; // scrolling window: at most this many rendered log items
const LOG_DEBOUNCE_MS = 150; // coalesce a burst of ops into one summary line
let logPending = []; // ops waiting for the debounced render
let logTimer = null; // debounce handle

const DEFAULT_COLOR = "#e6e8ee";
const INITIAL_SCALE = 24; // pixels per board unit at 100% zoom, matching MIN_CELL_PX
const cam = { x: 0, y: 0, scale: INITIAL_SCALE }; // board units at the canvas centre, px per unit

let gridOffset = 0; // user shift over the zoom-chosen subdivision level
let strokeColor = DEFAULT_COLOR; // metadata attached to every painted box
let tool = "rect";
let dragging = false;
let dragStart = null;
let dragCur = null;
let brushLast = null; // last cell stamped by the pen or eraser brush, as {ix, iy}
let panning = false;
let panLast = null;
let pinch = null; // two-finger gesture: {dist, cx, cy}

// ---------- materialization ----------
// The browser is a replica: it folds the operation log with additive-wins
// semantics (union of additions minus union of removals) into the surviving
// boxes.
function materialize() {
  const rems = normalize(removes.map((r) => ({ min: r.min, max: r.max })));
  const pieces = [];
  for (const a of adds) {
    for (const p of subtractAll({ min: a.min, max: a.max }, rems)) {
      pieces.push({ min: p.min, max: p.max, color: a.color });
    }
  }
  boxes = pieces;
  draw();
}

function applyEntries(entries) {
  const newAdds = [];
  const newRemoves = [];
  const retracted = new Set();
  for (const e of entries) {
    if (e.kind === "add") {
      newAdds.push({
        id: e.id,
        min: e.data.min,
        max: e.data.max,
        color: e.meta?.color ?? DEFAULT_COLOR,
      });
    } else if (e.kind === "remove") {
      newRemoves.push({ id: e.id, min: e.data.min, max: e.data.max });
    } else if (e.kind === "retract") {
      retracted.add(e.ref);
    }
    logEntries.push(e);
  }
  queueLog(entries);
  if (retracted.size > 0) {
    adds = adds.filter((a) => !retracted.has(a.id));
    removes = removes.filter((r) => !retracted.has(r.id));
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

function draw() {
  const { w, h } = resizeCanvas();
  ctx.fillStyle = "#0e1015";
  ctx.fillRect(0, 0, w, h);
  drawBoxes(w, h);
  drawGrid(w, h);
  drawPreview(w, h);
}

function drawBoxes(w, h) {
  const px = cam.scale;
  for (const b of boxes) {
    const sx = (b.min[0] - cam.x) * px + w / 2;
    const sy = (b.min[1] - cam.y) * px + h / 2;
    const sw = (b.max[0] - b.min[0]) * px;
    const sh = (b.max[1] - b.min[1]) * px;
    if (sx + sw < 0 || sx > w || sy + sh < 0 || sy > h) continue;
    ctx.fillStyle = b.color;
    ctx.fillRect(sx, sy, sw, sh);
  }
}

function drawGrid(w, h) {
  const cell = gridSize(cam.scale, gridOffset);
  // Draw the coarsest power-of-two multiple of the cell that is still readable;
  // strokes still snap to the fine cell, but the grid never vanishes.
  const step = gridStep(cell, cam.scale);
  const left = cam.x - w / 2 / cam.scale;
  const right = cam.x + w / 2 / cam.scale;
  const top = cam.y - h / 2 / cam.scale;
  const bottom = cam.y + h / 2 / cam.scale;
  const first = Math.floor(left / step);
  const last = Math.ceil(right / step);
  const rowTop = Math.floor(top / step);
  const rowBottom = Math.ceil(bottom / step);
  // Dark halo first, so the grid stays legible over white painted cells.
  strokeGrid(
    step,
    first,
    last,
    rowTop,
    rowBottom,
    w,
    h,
    "rgba(9, 11, 16, 0.6)",
    2,
  );
  // Light core on top, so the grid stays legible over the dark background.
  strokeGrid(
    step,
    first,
    last,
    rowTop,
    rowBottom,
    w,
    h,
    "rgba(232, 236, 244, 0.25)",
    1,
  );
}

function strokeGrid(cell, first, last, rowTop, rowBottom, w, h, style, width) {
  ctx.strokeStyle = style;
  ctx.lineWidth = width;
  ctx.beginPath();
  for (let k = first; k <= last; k++) {
    const sx = (k * cell - cam.x) * cam.scale + w / 2;
    ctx.moveTo(sx, 0);
    ctx.lineTo(sx, h);
  }
  for (let k = rowTop; k <= rowBottom; k++) {
    const sy = (k * cell - cam.y) * cam.scale + h / 2;
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
  const cmd = {
    id: newOpID(),
    kind: "paint",
    data: { x0: r.x0, y0: r.y0, x1: r.x1, y1: r.y1 },
    meta: { color: strokeColor },
  };
  trackStroke(cmd);
  sendCmd(cmd);
}

function sendErase(r) {
  const cmd = {
    id: newOpID(),
    kind: "erase",
    data: { x0: r.x0, y0: r.y0, x1: r.x1, y1: r.y1 },
  };
  trackStroke(cmd);
  sendCmd(cmd);
}

// brushSend stamps one cell with the active brush tool: the pen paints it and
// the eraser erases it. Both cover exactly one grid cell, so the eraser is as
// precise as the pen.
function brushSend(r) {
  if (tool === "eraser") sendErase(r);
  else sendPaint(r);
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
      cam.scale = next;
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

// zoomBy scales the camera around the canvas centre by the given factor.
function zoomBy(factor) {
  const next = cam.scale * factor;
  if (!Number.isFinite(next) || next <= 0) return;
  cam.scale = next;
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
    cam.scale = next;
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

function fitAll() {
  const { w, h } = resizeCanvas();
  const view = fitCamera(boxes, w, h);
  if (!view) {
    setStatus("nothing to fit");
    return;
  }
  cam.x = view.x;
  cam.y = view.y;
  cam.scale = view.scale;
  updateGridLabel();
  draw();
}

// ---------- websocket ----------
function setConnected(online) {
  connected = online;
  document.body.classList.toggle("disconnected", !online);
  reconnectBanner.hidden = online;
  rectBtn.disabled = !online;
  penBtn.disabled = !online;
  eraseBtn.disabled = !online;
  eraserBtn.disabled = !online;
  importJsonlBtn.disabled = !online;
  if (online) {
    reconnectAttempts = 0;
    clearReconnectTimer();
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
  setStatus(`disconnected — retrying in ${secs}s`);
  reconnectText.textContent = `Disconnected — retrying in ${secs}s`;
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
    setStatus("connected — strokes are shared live");
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
    } else if (local.length > 0) {
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
  logEntries = [];
  logPending = [];
  if (logTimer !== null) {
    clearTimeout(logTimer);
    logTimer = null;
  }
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

function replayLocalLog(log) {
  for (const entry of log) {
    const cmd = entryToCmd(entry);
    if (cmd) sendCmd(cmd);
  }
}

function sendCmd(cmd) {
  if (!connected || !socket || socket.readyState !== WebSocket.OPEN) {
    setStatus("disconnected — reconnect to paint");
    return;
  }
  socket.send(JSON.stringify(cmd));
}

function setStatus(text) {
  statusEl.textContent = text;
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
  a.download = "board.jsonl";
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(a.href);
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
  setStatus(`imported ${imported} ops${skipped ? `, skipped ${skipped}` : ""}`);
  input.value = "";
  draw();
}

function entryToCmd(entry) {
  if (entry.kind === "retract") {
    return entry.ref ? { kind: "retract", ref: entry.ref } : null;
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

strokeColorEl.addEventListener("input", () => {
  strokeColor = strokeColorEl.value;
});

importJsonlBtn.addEventListener("click", () => importJsonlInput.click());
importJsonlInput.addEventListener("change", () =>
  importJSONL(importJsonlInput),
);

gridPlus.addEventListener("click", () => {
  gridOffset += 1;
  updateGridLabel();
  draw();
});
gridMinus.addEventListener("click", () => {
  gridOffset -= 1;
  updateGridLabel();
  draw();
});
gridDefault.addEventListener("click", () => {
  gridOffset = 0;
  updateGridLabel();
  draw();
});
fitAllBtn.addEventListener("click", fitAll);

zoomInBtn.addEventListener("click", () => zoomBy(2));
zoomOutBtn.addEventListener("click", () => zoomBy(0.5));
zoomResetBtn.addEventListener("click", zoomReset);

copyShareBtn.addEventListener("click", async () => {
  try {
    await navigator.clipboard.writeText(location.href);
    setStatus("share link copied");
  } catch {
    setStatus("could not copy link");
  }
});

downloadJsonlBtn.addEventListener("click", () => {
  downloadJSONL();
  setStatus(`downloaded board.jsonl (${logEntries.length} ops)`);
});

newSessionBtn.addEventListener("click", () => location.replace("/ui/"));

window.addEventListener("resize", draw);

reconnectBtn.addEventListener("click", reconnectNow);

// ---------- boot ----------
function boot() {
  resizeCanvas();
  setTool("rect");
  updateGridLabel();
  draw();
  connect();
}

boot();

// Test seam: Playwright closes the live socket through this to exercise the
// reconnection flow. It is inert in normal use.
window.__eventfulranges = { closeSocket: () => socket && socket.close() };
