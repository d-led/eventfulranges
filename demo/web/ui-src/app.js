import * as THREE from 'three';
import { OrbitControls } from 'three/addons/controls/OrbitControls.js';
import { fadeOpacity } from './slice.js';
import { orthoHalf, orthoFrustum, perspDistance } from './camera.js';
import { createServerEngine } from './server-engine.js';
import { createLocalEngine } from './local-engine.js';

// ---------- DOM ----------
const $ = (id) => document.getElementById(id);
const dimsSel = $('dims');
const compactSel = $('compact');
const modal = $('modal');
const startSessionBtn = $('startSession');
const cancelSessionBtn = $('cancelSession');
const opsEl = $('ops');
const sendBtn = $('send');
const exampleBtn = $('example');
const randomBtn = $('random');
const newSessionBtn = $('newSession');
const sliceEl = $('slice');
const wInput = $('w');
const wVal = $('wval');
const animateChk = $('animate');
const statusEl = $('status');
const resultEl = $('result');
const copyBtn = $('copy');
const copyLinkBtn = $('copyLink');
const presenceEl = $('presence');
const logEl = $('log');
const fitViewBtn = $('fitView');
const compactionEl = $('compaction');
const reconnectBanner = $('reconnectBanner');
const reconnectBtn = $('reconnectBtn');

// ---------- three.js scene ----------
const canvasHost = $('canvas');
const scene = new THREE.Scene();
scene.background = new THREE.Color(0x111318);

// 1D and 2D sessions render flat in an orthographic top-down view; 3D and 4D
// sessions use perspective. setViewMode switches the active camera. Near/far
// are kept tight to avoid depth-buffer z-fighting between coplanar helpers.
const perspCamera = new THREE.PerspectiveCamera(50, 1, 0.1, 1000);
const orthoCamera = new THREE.OrthographicCamera(-1, 1, 1, -1, 0.1, 100);
let camera = perspCamera;

const renderer = new THREE.WebGLRenderer({ antialias: true });
renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
canvasHost.appendChild(renderer.domElement);

const controls = new OrbitControls(camera, renderer.domElement);
controls.enableDamping = true;

const boxesGroup = new THREE.Group();
scene.add(boxesGroup);

scene.add(new THREE.AmbientLight(0xffffff, 0.7));
const key = new THREE.DirectionalLight(0xffffff, 1.1);
key.position.set(6, 9, 7);
scene.add(key);
const rim = new THREE.DirectionalLight(0x6688cc, 0.5);
rim.position.set(-6, -3, -6);
scene.add(rim);

const grid = new THREE.GridHelper(20, 20, 0x2a2f3a, 0x1c2029);
grid.position.y = -0.02; // a hair below the axes so they never share a plane and z-fight
scene.add(grid);
scene.add(new THREE.AxesHelper(6));

const palette = [
  0x4f8cff, 0xff6b6b, 0x51cf66, 0xffd43b,
  0xcc5de8, 0x22b8cf, 0xff922b, 0x74c0fc,
];

let currentBoxes = []; // [{min, max}] as returned by the server
let currentDims = 3;
let sliceW = 2;
let sliceDir = 1; // animation sweep direction (ping-pong, no wrap teleport)
let startDimsSent = false; // the ?dims= URL preference is applied once
let needsFit = true;
let clientID = '';
let clients = 0; // viewers of this session
let total = 0;   // viewers connected across all sessions
let sessionOps = []; // the full operation log, for the local reserve copy

// ---------- settings ----------
// The dimension and compaction chosen for the next session are remembered in
// localStorage, so a reload keeps them (like the paint demo's preferences).
// Storage may be unavailable (private mode) or full; the choices then survive
// only for this page load.
const SETTINGS_KEY = 'eventfulranges:web:settings';
const settings = loadSettings();

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
    // best effort: the in-memory choices still hold for this page load
  }
}

function resize() {
  const w = canvasHost.clientWidth;
  const h = canvasHost.clientHeight;
  renderer.setSize(w, h);
  perspCamera.aspect = w / h;
  perspCamera.updateProjectionMatrix();
  needsFit = true; // the orthographic frustum is recomputed on the next frame
}
window.addEventListener('resize', resize);

function makeBox(lo, hi, color, alpha) {
  const size = lo.map((v, i) => Math.max(hi[i] - v, 1e-4));
  const geo = new THREE.BoxGeometry(size[0], size[1], size[2]);
  const fill = new THREE.MeshStandardMaterial({
    color,
    transparent: true,
    opacity: 0.4 * alpha,
    depthWrite: false,
    side: THREE.DoubleSide,
  });
  const mesh = new THREE.Mesh(geo, fill);
  mesh.position.set(
    (lo[0] + hi[0]) / 2,
    (lo[1] + hi[1]) / 2,
    (lo[2] + hi[2]) / 2,
  );
  const edges = new THREE.LineSegments(
    new THREE.EdgesGeometry(geo),
    new THREE.LineBasicMaterial({ color: 0xffffff, transparent: true, opacity: 0.85 * alpha }),
  );
  mesh.add(edges);
  return mesh;
}

// Project an n-dimensional box onto 3D. Missing dimensions are padded to thin
// slabs; 4D boxes are sliced at the current w value with a soft falloff near
// their w-boundaries, so sweeping w fades boxes in and out instead of popping
// them in at the hyperplane.
function project(box, dims, w) {
  const { min, max } = box;
  let opacity = 1;
  if (dims === 4) {
    opacity = fadeOpacity(min[3], max[3], w);
    if (opacity <= 0) return null;
  }
  const lo = min.slice(0, 3);
  const hi = max.slice(0, 3);
  for (let d = dims; d < 3; d++) {
    // Missing dimensions become flat slabs so the shape still reads: 1D
    // intervals get a fixed height, and 2D rectangles lie in the z = 0 plane.
    lo.push(d === 1 ? -0.5 : 0);
    hi.push(d === 1 ? 0.5 : 0.02);
  }
  return { lo, hi, opacity };
}

function projectedBoxes() {
  const out = [];
  for (const b of currentBoxes) {
    const p = project(b, currentDims, sliceW);
    if (p) out.push(p);
  }
  return out;
}

function rebuild() {
  while (boxesGroup.children.length) boxesGroup.remove(boxesGroup.children[0]);
  for (let i = 0; i < currentBoxes.length; i++) {
    const p = project(currentBoxes[i], currentDims, sliceW);
    if (!p) continue;
    boxesGroup.add(makeBox(p.lo, p.hi, palette[i % palette.length], p.opacity));
  }
}

// setViewMode picks the camera and interaction model for the materialized
// dimension: 1D/2D are drawn from above and only pan/zoom, 3D/4D rotate freely.
function setViewMode(dims) {
  const flat = dims <= 2;
  camera = flat ? orthoCamera : perspCamera;
  controls.object = camera;
  controls.enableRotate = !flat;
  needsFit = true;
}

// applyOrthoFrustum frames the orthographic camera around its own local
// origin: the camera is positioned above the material's centre, so the
// frustum planes are symmetric around (0,0) in camera space.
function applyOrthoFrustum(half) {
  const aspect = (canvasHost.clientWidth / canvasHost.clientHeight) || 1;
  const { left, right, top, bottom } = orthoFrustum(half, aspect);
  orthoCamera.left = left;
  orthoCamera.right = right;
  orthoCamera.top = top;
  orthoCamera.bottom = bottom;
  orthoCamera.updateProjectionMatrix();
}

// boundsOf returns the axis-aligned bounds of the projected boxes, or null
// when there is nothing to frame.
function boundsOf(boxes) {
  if (boxes.length === 0) return null;
  const min = [Infinity, Infinity, Infinity];
  const max = [-Infinity, -Infinity, -Infinity];
  for (const { lo, hi } of boxes) {
    for (let i = 0; i < 3; i++) {
      min[i] = Math.min(min[i], lo[i]);
      max[i] = Math.max(max[i], hi[i]);
    }
  }
  return { min, max };
}

// centroidOf returns the volume-weighted center (barycenter) of the boxes, so
// the camera orbits the material rather than the origin.
function centroidOf(boxes) {
  const c = [0, 0, 0];
  let volume = 0;
  for (const { lo, hi } of boxes) {
    const v = (hi[0] - lo[0]) * (hi[1] - lo[1]) * (hi[2] - lo[2]);
    for (let i = 0; i < 3; i++) c[i] += v * (lo[i] + hi[i]) / 2;
    volume += v;
  }
  return volume > 0 ? c.map((x) => x / volume) : [0, 0, 0];
}

// norm3 scales a direction vector to unit length, so the camera distance does
// not depend on the viewing angle.
function norm3(x, y, z) {
  const len = Math.hypot(x, y, z);
  return [x / len, y / len, z / len];
}

function fitCamera() {
  const boxes = projectedBoxes();
  const bounds = boundsOf(boxes);

  if (camera === orthoCamera) {
    // Flat views are centered on the material and framed so both extents fit,
    // scaling for the canvas aspect so nothing is clipped.
    const aspect = (canvasHost.clientWidth / canvasHost.clientHeight) || 1;
    const center = bounds
      ? [(bounds.min[0] + bounds.max[0]) / 2, (bounds.min[1] + bounds.max[1]) / 2, 0]
      : [0, 0, 0];
    const half = orthoHalf(bounds, aspect);
    controls.target.set(center[0], center[1], 0);
    orthoCamera.near = 0.1;
    orthoCamera.far = 100;
    applyOrthoFrustum(half);
    orthoCamera.position.set(center[0], center[1], 10);
  } else {
    // Perspective views orbit the barycenter and step back far enough that the
    // material's bounding sphere fits the narrower of the two fields of view.
    const center = bounds ? centroidOf(boxes) : [0, 0, 0];
    let radius = 8; // empty-scene fallback
    if (bounds) {
      let r2 = 0;
      for (const x of [bounds.min[0], bounds.max[0]]) {
        for (const y of [bounds.min[1], bounds.max[1]]) {
          for (const z of [bounds.min[2], bounds.max[2]]) {
            const dx = x - center[0];
            const dy = y - center[1];
            const dz = z - center[2];
            r2 = Math.max(r2, dx * dx + dy * dy + dz * dz);
          }
        }
      }
      radius = Math.max(1, Math.sqrt(r2));
    }
    camera.aspect = (canvasHost.clientWidth / canvasHost.clientHeight) || 1;
    const dist = perspDistance(radius, camera.fov, camera.aspect);
    const dir = norm3(0.8, 0.8, 1.1);
    controls.target.set(...center);
    camera.position.set(
      center[0] + dir[0] * dist,
      center[1] + dir[1] * dist,
      center[2] + dir[2] * dist,
    );
    camera.near = dist / 100;
    camera.far = dist * 100;
    camera.updateProjectionMatrix();
  }
  controls.update();
}

function tick() {
  requestAnimationFrame(tick);
  if (currentDims === 4 && animateChk.checked) {
    sliceW += sliceDir * 0.008;
    if (sliceW >= 4) { sliceW = 4; sliceDir = -1; }
    else if (sliceW <= 0) { sliceW = 0; sliceDir = 1; }
    wInput.value = sliceW;
    wVal.textContent = sliceW.toFixed(2);
    rebuild();
  }
  if (needsFit) {
    fitCamera();
    needsFit = false;
  }
  controls.update();
  renderer.render(scene, camera);
}

// ---------- CSV ----------
// Each op is `kind,(min…),(max…)`, e.g. `add,(0,0,0),(4,4,4)`.
function parseOps(text, dims) {
  const ops = [];
  for (const raw of text.split('\n')) {
    const line = raw.trim();
    if (!line || line.startsWith('#')) continue;
    const match = line.match(/^(add|remove)\s*,\s*\(([^)]*)\)\s*,\s*\(([^)]*)\)$/i);
    if (!match) {
      throw new Error(`"${line}": expected kind,(min…),(max…)`);
    }
    const min = tuple(match[2], line);
    const max = tuple(match[3], line);
    if (min.length !== dims || max.length !== dims) {
      throw new Error(`"${line}": expected ${dims} numbers in each tuple`);
    }
    ops.push({ kind: match[1].toLowerCase(), min, max });
  }
  return ops;
}

function tuple(text, line) {
  const parts = text.split(',');
  if (parts.some((s) => s.trim() === '')) {
    throw new Error(`"${line}": empty coordinate`);
  }
  const nums = parts.map((s) => Number(s.trim()));
  if (nums.some(Number.isNaN)) {
    throw new Error(`"${line}": non-numeric value`);
  }
  return nums;
}

function boxesToCSV(boxes) {
  return boxes.map((b) => `(${b.min.join(',')}),(${b.max.join(',')})`).join('\n');
}

// exampleFor builds the same hollow shell in every dimension: a hypercube
// tiled into unit cubes (six in 1D, three per axis otherwise) with an aligned
// central block carved out. Canonical compaction keeps every surviving tile,
// so the shell appears as a fine grid; merge-adjacent collapses each side
// into a single box, so the same shell appears as a few clean slabs.
function exampleFor(dims) {
  const side = dims === 1 ? 6 : 3;
  const carveLo = Array(dims).fill(dims === 1 ? 2 : 1);
  const carveHi = Array(dims).fill(dims === 1 ? 4 : 2);
  const lines = [];
  const total = side ** dims;
  for (let code = 0; code < total; code++) {
    const min = [];
    const max = [];
    let c = code;
    for (let d = 0; d < dims; d++) {
      const v = c % side;
      c = Math.floor(c / side);
      min.push(v);
      max.push(v + 1);
    }
    lines.push(`add,(${min.join(',')}),(${max.join(',')})`);
  }
  lines.push(`remove,(${carveLo.join(',')}),(${carveHi.join(',')})`);
  return lines.join('\n');
}

// boundingBoxOf returns the axis-aligned box spanning every materialized box,
// falling back to a default [0,4]^dims cube when the set is empty.
function boundingBoxOf(dims) {
  const lo = Array(dims).fill(Infinity);
  const hi = Array(dims).fill(-Infinity);
  for (const b of currentBoxes) {
    for (let d = 0; d < dims; d++) {
      if (b.min[d] < lo[d]) lo[d] = b.min[d];
      if (b.max[d] > hi[d]) hi[d] = b.max[d];
    }
  }
  for (let d = 0; d < dims; d++) {
    if (!Number.isFinite(lo[d])) {
      lo[d] = 0;
      hi[d] = 4;
    }
  }
  return { lo, hi };
}

// randomOp drafts one add or remove whose values fall inside a box 1.1x the
// current set's bounding box, so random edits stay around the model.
function randomOp(dims) {
  const { lo, hi } = boundingBoxOf(dims);
  const min = [];
  const max = [];
  for (let d = 0; d < dims; d++) {
    const extent = hi[d] - lo[d];
    const low = lo[d] - extent * 0.05;
    const high = hi[d] + extent * 0.05;
    const a = Math.round((low + Math.random() * (high - low)) * 100) / 100;
    let b = Math.round((low + Math.random() * (high - low)) * 100) / 100;
    if (a === b) b = Math.round((a + 0.01) * 100) / 100; // never degenerate
    min.push(Math.min(a, b));
    max.push(Math.max(a, b));
  }
  const kind = Math.random() < 0.5 ? 'add' : 'remove';
  return `${kind},(${min.join(',')}),(${max.join(',')})`;
}

// ---------- session engine (transport switch) ----------
// A "session engine" is where the envelopes the page renders come from. Two
// engines speak the same protocol, so the rest of the UI is identical:
//   * server — the Go visualizer over a WebSocket (the default when the UI is
//     served by it): every browser that opens the same share link converges
//     on one shared model.
//   * local  — the same Go hub compiled to WebAssembly and running inside this
//     page: the UI works from any static host, with no server at all.
// The switch lives here, in one place: ?engine=local forces the local engine,
// and the local build (dist-local/) preselects it through __EVENTFULRANGES_STATIC__.
const ENGINE_IS_LOCAL =
  new URLSearchParams(location.search).get('engine') === 'local' ||
  window.__EVENTFULRANGES_STATIC__ === true;

function setStatus(text) {
  statusEl.textContent = text;
}

// setConnected flips the whole UI between live and frozen: the reconnect
// banner appears, mutation controls disable, and the canvas stops taking
// input while the engine is down. Local-only actions (copy, download) stay on.
function setConnected(online) {
  document.body.classList.toggle('disconnected', !online);
  reconnectBanner.hidden = online;
  sendBtn.disabled = !online;
  exampleBtn.disabled = !online;
}

// COMPACTION_LABELS names the two session compaction modes so the panel can
// explain, in words, what the active strategy does to the materialized boxes.
const COMPACTION_LABELS = {
  canonical: 'Compaction: canonical — every box is kept exactly as materialized.',
  merge: 'Compaction: merge adjacent — touching boxes are joined into larger ones.',
  partition: 'Compaction: partition — overlaps are split so each point appears in exactly one rectangle.',
  'partition-merge': 'Compaction: partition + merge — overlaps are split, then touching boxes are joined (disjoint and compact).',
};

function setCompaction(mode) {
  compactionEl.textContent = COMPACTION_LABELS[mode] || COMPACTION_LABELS.canonical;
  compactionEl.dataset.mode = mode;
}

// updatePresence renders the connected-client count and this client's own id.
function updatePresence(n, t) {
  if (n !== undefined) clients = n;
  if (t !== undefined) total = t;
  const me = clientID ? ` · you are ${clientID}` : '';
  presenceEl.textContent = `${clients} here · ${total} connected${me}`;
}

// appendLog renders one activity entry, newest last, highlighting this client.
function appendLog(op) {
  const li = document.createElement('li');
  li.className = op.kind;
  const when = new Date(op.at).toLocaleTimeString();
  const detail = op.kind === 'dims'
    ? ` → ${op.dims}D`
    : (op.min && op.min.length ? ` (${op.min.join(',')})→(${op.max.join(',')})` : '');
  li.textContent = `${when}  ${op.client}  ${op.kind}${detail}`;
  if (op.client === clientID) li.classList.add('me');
  logEl.appendChild(li);
  logEl.scrollTop = logEl.scrollHeight;
  while (logEl.children.length > 200) logEl.removeChild(logEl.firstChild);
}

// applyState folds an engine view into the scene and the result textarea. It
// frames the first content that appears, but later edits never yank the view.
function applyState(state) {
  const hadBoxes = currentBoxes.length > 0;
  currentBoxes = state.boxes || [];
  if (state.compact) setCompaction(state.compact);
  const dims = state.dims;
  if (dims > 0 && dims !== currentDims) {
    currentDims = dims;
    sliceEl.hidden = dims !== 4;
    opsEl.value = exampleFor(dims);
    setViewMode(dims);
  }
  resultEl.value = boxesToCSV(currentBoxes);
  rebuild();
  if (!hadBoxes && currentBoxes.length > 0) needsFit = true;
}

// The two engine implementations live next to this file; each takes the page
// callbacks it drives and returns the one object the rest of the UI uses.
const engine = ENGINE_IS_LOCAL
  ? createLocalEngine({
      onMessage: handleMessage,
      onStatus: setStatus,
      onOnline: setConnected,
      onFirstSync: sendDimsPreference,
    })
  : createServerEngine({
      onMessage: handleMessage,
      onStatus: setStatus,
      onOnline: setConnected,
      onFirstSync: sendDimsPreference,
    });

// handleMessage folds one engine envelope into the scene, the activity log,
// the local reserve copy, and the presence readout. Both engines deliver the
// same envelopes, so the handling below is transport-agnostic.
function handleMessage(msg) {
  if (msg.type === 'state') {
    // A live server re-sends the full log; an empty one means the session was
    // lost (e.g. the server restarted), so restore from the local reserve.
    const serverOps = msg.ops || [];
    const serverBoxes = (msg.state && msg.state.boxes) || [];
    if (serverOps.length === 0 && serverBoxes.length === 0) {
      const reserve = sessionOps.length > 0 ? sessionOps.slice() : loadReserve();
      sessionOps = [];
      logEl.innerHTML = '';
      if (reserve.length > 0) {
        for (const op of reserve) replayOp(op);
        setStatus('restored from local copy — syncing…');
      } else {
        applyState(msg.state);
      }
    } else {
      applyState(msg.state);
      sessionOps = serverOps.slice();
      saveReserve(sessionOps);
      logEl.innerHTML = '';
      for (const op of serverOps) appendLog(op);
    }
  } else if (msg.type === 'op') {
    if (msg.op) {
      sessionOps.push(msg.op);
      saveReserve(sessionOps);
      appendLog(msg.op);
    }
    if (msg.state) applyState(msg.state);
  }

  if (msg.clientID) {
    clientID = msg.clientID;
    updatePresence();
    needsFit = true; // fit once on join, not on every later edit
  }
  if (msg.clients !== undefined || msg.total !== undefined) updatePresence(msg.clients, msg.total);
  if (msg.type === 'error') {
    setStatus(`error: ${msg.error}`);
  }
}

// connect starts the selected engine; the engine reports connectivity and
// envelopes back through the callbacks wired above.
function connect() {
  engine.start();
}

// sendDimsPreference applies the ?dims= URL preference once, as the first dims
// op after the engine comes online — the local engine's analogue of the dims
// command the socket mode used to send on open.
function sendDimsPreference() {
  const startDims = Number(new URLSearchParams(location.search).get('dims'));
  if (!startDimsSent && startDims >= 1 && startDims <= 4) {
    startDimsSent = true;
    sendOp({ kind: 'dims', dims: startDims });
  }
}

function sendOp(op) {
  engine.send(op);
}

// The browser keeps a reserve copy of the operation log in localStorage, so a
// lost model does not lose the picture: on (re)connect, if the engine reports
// an empty session, the local log is replayed back into it. This heals a
// restarted Go server and a freshly loaded in-page wasm engine alike.
function reserveKey() {
  const session = new URLSearchParams(location.search).get('s');
  return session ? `eventfulranges:web:${session}` : null;
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

// replayOp turns one logged operation back into a command and re-sends it, so
// an emptied server session is healed from the browser's local copy.
function replayOp(op) {
  if (op.kind === 'dims') {
    sendOp({ kind: 'dims', dims: op.dims });
  } else if (op.min && op.max) {
    sendOp({ kind: op.kind, min: op.min, max: op.max });
  }
}

// Apply a scenario: fold the given ops into the shared view in order. The
// ops are unioned/removed, so loading the same example twice is idempotent.
function applyScenario(dims, text) {
  try {
    const ops = parseOps(text, dims);
    if (ops.length === 0) return;
    for (const op of ops) sendOp(op);
    setStatus(`sent ${ops.length} op(s)`);
  } catch (e) {
    setStatus(`error: ${e.message}`);
  }
}

// shareURL is simply this page's own URL: the session ID in the query string
// is what makes it unique, so anyone who opens it joins the same model.
function shareURL() {
  return location.href;
}

// ---------- events ----------
sendBtn.addEventListener('click', () => {
  try {
    const ops = parseOps(opsEl.value, currentDims);
    if (ops.length === 0) {
      setStatus('nothing to send');
      return;
    }
    for (const op of ops) sendOp(op);
    setStatus(`sent ${ops.length} op(s)`);
    opsEl.value = ''; // clear the window like a chat input
  } catch (e) {
    setStatus(`error: ${e.message}`);
  }
});

exampleBtn.addEventListener('click', () => {
  opsEl.value = exampleFor(currentDims);
  applyScenario(currentDims, opsEl.value);
});

randomBtn.addEventListener('click', () => {
  const op = randomOp(currentDims);
  opsEl.value = opsEl.value ? `${opsEl.value}\n${op}` : op;
  setStatus('drafted a random op — review and send');
});

newSessionBtn.addEventListener('click', () => {
  modal.hidden = false;
});

cancelSessionBtn.addEventListener('click', () => {
  modal.hidden = true;
});

// Remember the next-session choices so a reload keeps them.
dimsSel.addEventListener('change', () => {
  settings.dims = Number(dimsSel.value);
  saveSettings();
});
compactSel.addEventListener('change', () => {
  settings.compact = compactSel.value;
  saveSettings();
});

startSessionBtn.addEventListener('click', () => {
  const dims = Number(dimsSel.value);
  const compact = compactSel.value;
  location.replace(engine.newSessionURL(dims, compact));
});

fitViewBtn.addEventListener('click', () => {
  needsFit = true;
});

wInput.addEventListener('input', () => {
  sliceW = Number(wInput.value);
  wVal.textContent = sliceW.toFixed(2);
  rebuild();
});

copyBtn.addEventListener('click', async () => {
  try {
    await navigator.clipboard.writeText(resultEl.value);
  } catch {
    resultEl.select();
    document.execCommand('copy');
  }
  setStatus('copied');
});

copyLinkBtn.addEventListener('click', async () => {
  try {
    await navigator.clipboard.writeText(shareURL());
  } catch {
    setStatus('could not copy link');
    return;
  }
  setStatus('share link copied');
});

reconnectBtn.addEventListener('click', () => engine.reconnect());

// ---------- boot ----------
if (settings.dims >= 1 && settings.dims <= 4) dimsSel.value = String(settings.dims);
if (['merge', 'partition', 'partition-merge'].includes(settings.compact)) compactSel.value = settings.compact;
resize();
wVal.textContent = sliceW.toFixed(2);
opsEl.value = exampleFor(3);
setViewMode(currentDims);
connect();
tick();

// Test seam: Playwright closes the live socket through this to exercise the
// reconnection flow. It is inert in normal use.
window.__eventfulranges = { closeSocket: () => engine.close() };
