import { union, normalize, subtractAll } from "./boxes.js";
import { gridLevel, gridSize, gridRect, gridLine, fitCamera } from "./grid.js";

// ---------- DOM ----------
const $ = (id) => document.getElementById(id);
const boardEl = $("board");
const statusEl = $("status");
const presenceEl = $("presence");
const logEl = $("log");
const copyShareBtn = $("copyShare");
const downloadJsonlBtn = $("downloadJsonl");
const importJsonlBtn = $("importJsonl");
const importJsonlInput = $("importJsonlInput");
const newSessionBtn = $("newSession");
const rectBtn = $("toolRect");
const penBtn = $("toolPen");
const eraseBtn = $("toolErase");
const panBtn = $("toolPan");
const toolLabel = $("toolLabel");
const strokeColorEl = $("strokeColor");
const gridLabel = $("gridLabel");
const gridPlus = $("gridPlus");
const gridMinus = $("gridMinus");
const gridDefault = $("gridDefault");
const fitAllBtn = $("fitAll");

const ctx = boardEl.getContext("2d");

// ---------- state ----------
let adds = []; // normalized boxes {min, max} of painted cells
let removes = []; // normalized boxes {min, max} of erased cells
let boxes = []; // materialized view: [{min, max}]
let logEntries = []; // the full operation log, for JSONL export
let clientID = "";
let clients = 0;
let total = 0;
let socket = null;

const DEFAULT_COLOR = "#e6e8ee";
const cam = { x: 0, y: 0, scale: 12 }; // board units at the canvas centre, px per unit

let gridOffset = 0; // user shift over the zoom-chosen subdivision level
let strokeColor = DEFAULT_COLOR; // metadata attached to every painted box
let tool = "rect";
let dragging = false;
let dragStart = null;
let dragCur = null;
let penLast = null; // last painted pen cell, as {ix, iy}
let panning = false;
let panLast = null;

// ---------- materialization ----------
// The browser is a replica: it folds the operation log with additive-wins
// semantics (union of additions minus union of removals) into the surviving
// boxes.
function materialize() {
  const rems = normalize(removes);
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
  for (const e of entries) {
    if (e.kind === "add") {
      newAdds.push({
        min: e.data.min,
        max: e.data.max,
        color: e.meta?.color ?? DEFAULT_COLOR,
      });
    } else if (e.kind === "remove") {
      newRemoves.push(e.data);
    }
    logEntries.push(e);
    appendLog(e);
  }
  adds.push(...newAdds); // keep stroke order and one color per box
  removes = union(removes, newRemoves);
  materialize();
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
  const px = cell * cam.scale;
  if (px < 6) return; // too dense to read
  const left = cam.x - w / 2 / cam.scale;
  const right = cam.x + w / 2 / cam.scale;
  const top = cam.y - h / 2 / cam.scale;
  const bottom = cam.y + h / 2 / cam.scale;
  const first = Math.floor(left / cell);
  const last = Math.ceil(right / cell);
  const rowTop = Math.floor(top / cell);
  const rowBottom = Math.ceil(bottom / cell);
  // Dark halo first, so the grid stays legible over white painted cells.
  strokeGrid(cell, first, last, rowTop, rowBottom, w, h, "rgba(9, 11, 16, 0.6)", 2);
  // Light core on top, so the grid stays legible over the dark background.
  strokeGrid(cell, first, last, rowTop, rowBottom, w, h, "rgba(232, 236, 244, 0.25)", 1);
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
  if (!dragging || tool === "pen") return;
  const r = gridRect(dragStart, dragCur, gridSize(cam.scale, gridOffset));
  const sx = (r.x0 - cam.x) * cam.scale + w / 2;
  const sy = (r.y0 - cam.y) * cam.scale + h / 2;
  ctx.strokeStyle = tool === "erase" ? "#ff6b6b" : "#4f8cff";
  ctx.lineWidth = 2;
  ctx.strokeRect(sx, sy, (r.x1 - r.x0) * cam.scale, (r.y1 - r.y0) * cam.scale);
}

function pointAt(e) {
  const { w, h } = resizeCanvas();
  return {
    x: (e.offsetX - w / 2) / cam.scale + cam.x,
    y: (e.offsetY - h / 2) / cam.scale + cam.y,
  };
}

function penCell(p) {
  const cell = gridSize(cam.scale, gridOffset);
  return { ix: Math.floor(p.x / cell), iy: Math.floor(p.y / cell) };
}

function penRect(c) {
  const cell = gridSize(cam.scale, gridOffset);
  return {
    x0: c.ix * cell,
    y0: c.iy * cell,
    x1: (c.ix + 1) * cell,
    y1: (c.iy + 1) * cell,
  };
}

function sendPaint(r) {
  sendCmd({
    kind: "paint",
    data: { x0: r.x0, y0: r.y0, x1: r.x1, y1: r.y1 },
    meta: { color: strokeColor },
  });
}

function sendErase(r) {
  sendCmd({
    kind: "erase",
    data: { x0: r.x0, y0: r.y0, x1: r.x1, y1: r.y1 },
  });
}

// ---------- interaction ----------
boardEl.addEventListener("mousedown", (e) => {
  if (e.button === 1 || tool === "pan") {
    panning = true;
    panLast = { x: e.clientX, y: e.clientY };
    return;
  }
  if (e.button !== 0) return;
  dragging = true;
  if (tool === "pen") {
    penLast = penCell(pointAt(e));
    sendPaint(penRect(penLast));
    return;
  }
  dragStart = pointAt(e);
  dragCur = dragStart;
});

window.addEventListener("mousemove", (e) => {
  if (panning) {
    cam.x -= (e.clientX - panLast.x) / cam.scale;
    cam.y -= (e.clientY - panLast.y) / cam.scale;
    panLast = { x: e.clientX, y: e.clientY };
    draw();
    return;
  }
  if (!dragging) return;
  if (tool === "pen") {
    const cur = penCell(pointAt(e));
    const cell = gridSize(cam.scale, gridOffset);
    for (const r of gridLine(penLast, cur, cell)) sendPaint(r);
    penLast = cur;
    return;
  }
  dragCur = pointAt(e);
  draw();
});

window.addEventListener("mouseup", () => {
  if (panning) {
    panning = false;
    return;
  }
  if (!dragging) return;
  dragging = false;
  if (tool !== "pen") commitStroke();
});

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
  panBtn.classList.toggle("active", name === "pan");
  boardEl.style.cursor = name === "pan" ? "grab" : "crosshair";
  toolLabel.textContent = { rect: "Rect", pen: "Pen", erase: "Erase", pan: "Pan" }[name];
}

function updateGridLabel() {
  const n = gridLevel(cam.scale, gridOffset);
  const cell = gridSize(cam.scale, gridOffset);
  const offset =
    gridOffset === 0
      ? ""
      : ` · offset ${gridOffset > 0 ? "+" : ""}${gridOffset}`;
  gridLabel.textContent = `${cell} × ${cell} · level ${n}${offset}`;
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
function connect() {
  const proto = location.protocol === "https:" ? "wss" : "ws";
  const session = new URLSearchParams(location.search).get("s");
  if (!session) {
    location.replace("/ui/");
    return;
  }
  const ws = new WebSocket(
    `${proto}://${location.host}/ws?s=${encodeURIComponent(session)}`,
  );
  socket = ws;
  ws.onopen = () => setStatus("connected — strokes are shared live");
  ws.onclose = () => {
    setStatus("disconnected — reconnecting…");
    setTimeout(connect, 1000);
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
    updatePresence();
  }
  if (msg.clients !== undefined || msg.total !== undefined)
    updatePresence(msg.clients, msg.total);

  if (msg.type === "state") {
    adds = [];
    removes = [];
    logEntries = [];
    logEl.innerHTML = "";
    const full = msg.ops || [];
    if (full.length) applyEntries(full);
    else materialize();
  } else if (msg.type === "op") {
    if (msg.op) applyEntries([msg.op]);
    if (msg.ops) applyEntries(msg.ops);
  } else if (msg.type === "error") {
    setStatus(`error: ${msg.error}`);
  }
}

function sendCmd(cmd) {
  if (socket && socket.readyState === WebSocket.OPEN) {
    socket.send(JSON.stringify(cmd));
  } else {
    setStatus("not connected — cannot send");
  }
}

function setStatus(text) {
  statusEl.textContent = text;
}

function updatePresence(n, t) {
  if (n !== undefined) clients = n;
  if (t !== undefined) total = t;
  const me = clientID ? ` · you are ${clientID}` : "";
  presenceEl.textContent = `${clients} here · ${total} connected${me}`;
}

function appendLog(entry) {
  const li = document.createElement("li");
  li.className = entry.kind;
  const when = new Date(entry.at).toLocaleTimeString();
  li.textContent = `${when}  ${entry.client}  ${entry.kind} ${entry.detail ?? ""}`;
  if (entry.client === clientID) li.classList.add("me");
  logEl.appendChild(li);
  logEl.scrollTop = logEl.scrollHeight;
  while (logEl.children.length > 200) logEl.removeChild(logEl.firstChild);
}

function downloadJSONL() {
  const lines = logEntries.map((e) =>
    JSON.stringify({
      id: e.id,
      kind: e.kind,
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
  const file = input.files && input.files[0];
  if (!file) return;
  const text = await file.text();
  let imported = 0;
  let skipped = 0;
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
    if (socket && socket.readyState === WebSocket.OPEN) {
      socket.send(JSON.stringify(cmd));
      imported++;
    } else {
      skipped++;
    }
  }
  setStatus(`imported ${imported} ops${skipped ? `, skipped ${skipped}` : ""}`);
  input.value = "";
}

function entryToCmd(entry) {
  const kind =
    entry.kind === "add" ? "paint" : entry.kind === "remove" ? "erase" : null;
  if (!kind || !entry.data) return null;
  const { min, max } = entry.data;
  if (!Array.isArray(min) || !Array.isArray(max)) return null;
  const cmd = {
    kind,
    data: { x0: min[0], y0: min[1], x1: max[0], y1: max[1] },
  };
  if (entry.meta) cmd.meta = entry.meta;
  return cmd;
}

// ---------- events ----------
rectBtn.addEventListener("click", () => setTool("rect"));
penBtn.addEventListener("click", () => setTool("pen"));
eraseBtn.addEventListener("click", () => setTool("erase"));
panBtn.addEventListener("click", () => setTool("pan"));

strokeColorEl.addEventListener("input", () => {
  strokeColor = strokeColorEl.value;
});

importJsonlBtn.addEventListener("click", () => importJsonlInput.click());
importJsonlInput.addEventListener("change", () => importJSONL(importJsonlInput));

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

// ---------- boot ----------
function boot() {
  resizeCanvas();
  setTool("rect");
  updateGridLabel();
  draw();
  connect();
}

boot();
