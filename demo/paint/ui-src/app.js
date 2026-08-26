import { union, difference } from "./boxes.js";

// ---------- DOM ----------
const $ = (id) => document.getElementById(id);
const boardEl = $("board");
const statusEl = $("status");
const presenceEl = $("presence");
const logEl = $("log");
const copyShareBtn = $("copyShare");
const downloadJsonlBtn = $("downloadJsonl");
const newSessionBtn = $("newSession");
const paintBtn = $("toolPaint");
const eraseBtn = $("toolErase");
const panBtn = $("toolPan");
const toolLabel = $("toolLabel");

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

const cam = { x: 0, y: 0, scale: 12 }; // cells at the canvas centre, px per cell

let tool = "paint";
let dragging = false;
let dragStart = null;
let dragCur = null;
let panning = false;
let panLast = null;

// ---------- materialization ----------
// The browser is a replica: it folds the operation log with additive-wins
// semantics (union of additions minus union of removals) into the surviving
// boxes.
function materialize() {
  boxes = difference(adds, removes);
  draw();
}

function applyEntries(entries) {
  const newAdds = [];
  const newRemoves = [];
  for (const e of entries) {
    if (e.kind === "add") newAdds.push(e.data);
    else if (e.kind === "remove") newRemoves.push(e.data);
    logEntries.push(e);
    appendLog(e);
  }
  adds = union(adds, newAdds);
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
  drawGrid(w, h);
  const px = cam.scale;
  for (const b of boxes) {
    const sx = (b.min[0] - cam.x) * px + w / 2;
    const sy = (b.min[1] - cam.y) * px + h / 2;
    const sw = (b.max[0] - b.min[0]) * px;
    const sh = (b.max[1] - b.min[1]) * px;
    if (sx + sw < 0 || sx > w || sy + sh < 0 || sy > h) continue;
    ctx.fillStyle = "#e6e8ee";
    ctx.fillRect(sx, sy, sw, sh);
  }
  drawPreview(w, h);
}

function drawGrid(w, h) {
  if (cam.scale < 6) return;
  const left = cam.x - w / 2 / cam.scale;
  const right = cam.x + w / 2 / cam.scale;
  const top = cam.y - h / 2 / cam.scale;
  const bottom = cam.y + h / 2 / cam.scale;
  ctx.strokeStyle = "#171b24";
  ctx.lineWidth = 1;
  ctx.beginPath();
  for (let cx = Math.ceil(left); cx <= Math.floor(right); cx++) {
    const sx = (cx - cam.x) * cam.scale + w / 2;
    ctx.moveTo(sx, 0);
    ctx.lineTo(sx, h);
  }
  for (let cy = Math.ceil(top); cy <= Math.floor(bottom); cy++) {
    const sy = (cy - cam.y) * cam.scale + h / 2;
    ctx.moveTo(0, sy);
    ctx.lineTo(w, sy);
  }
  ctx.stroke();
}

function drawPreview(w, h) {
  if (!dragging) return;
  const [x0, y0, x1, y1] = normalizedRect(dragStart, dragCur);
  const sx = (x0 - cam.x) * cam.scale + w / 2;
  const sy = (y0 - cam.y) * cam.scale + h / 2;
  ctx.strokeStyle = tool === "erase" ? "#ff6b6b" : "#4f8cff";
  ctx.lineWidth = 2;
  ctx.strokeRect(sx, sy, (x1 - x0) * cam.scale, (y1 - y0) * cam.scale);
}

function normalizedRect(a, b) {
  return [
    Math.min(a.x, b.x),
    Math.min(a.y, b.y),
    Math.max(a.x, b.x) + 1,
    Math.max(a.y, b.y) + 1,
  ];
}

function cellAt(e) {
  const { w, h } = resizeCanvas();
  return {
    x: Math.floor((e.offsetX - w / 2) / cam.scale + cam.x),
    y: Math.floor((e.offsetY - h / 2) / cam.scale + cam.y),
  };
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
  dragStart = cellAt(e);
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
  if (dragging) {
    dragCur = cellAt(e);
    draw();
  }
});

window.addEventListener("mouseup", () => {
  if (panning) {
    panning = false;
    return;
  }
  if (!dragging) return;
  dragging = false;
  commitStroke();
});

boardEl.addEventListener(
  "wheel",
  (e) => {
    e.preventDefault();
    const { w, h } = resizeCanvas();
    const beforeX = (e.offsetX - w / 2) / cam.scale + cam.x;
    const beforeY = (e.offsetY - h / 2) / cam.scale + cam.y;
    cam.scale = Math.min(
      64,
      Math.max(1, cam.scale * Math.exp(-e.deltaY * 0.0015)),
    );
    cam.x = beforeX - (e.offsetX - w / 2) / cam.scale;
    cam.y = beforeY - (e.offsetY - h / 2) / cam.scale;
    draw();
  },
  { passive: false },
);

function commitStroke() {
  const [x0, y0, x1, y1] = normalizedRect(dragStart, dragCur);
  if (x1 <= x0 || y1 <= y0) return;
  sendCmd({
    kind: tool === "erase" ? "erase" : "paint",
    data: { x0, y0, x1, y1 },
  });
}

function setTool(name) {
  tool = name;
  paintBtn.classList.toggle("active", name === "paint");
  eraseBtn.classList.toggle("active", name === "erase");
  panBtn.classList.toggle("active", name === "pan");
  boardEl.style.cursor = name === "pan" ? "grab" : "crosshair";
  toolLabel.textContent = { paint: "Paint", erase: "Erase", pan: "Pan" }[name];
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

// ---------- events ----------
paintBtn.addEventListener("click", () => setTool("paint"));
eraseBtn.addEventListener("click", () => setTool("erase"));
panBtn.addEventListener("click", () => setTool("pan"));

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
  setTool("paint");
  draw();
  connect();
}

boot();
