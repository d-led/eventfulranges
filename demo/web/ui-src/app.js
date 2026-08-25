import * as THREE from 'three';
import { OrbitControls } from 'three/addons/controls/OrbitControls.js';
import { fadeOpacity } from './slice.js';

// ---------- DOM ----------
const $ = (id) => document.getElementById(id);
const dimsSel = $('dims');
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

// setOrthoFrustum frames the orthographic camera around a centered box.
function setOrthoFrustum(center, half) {
  const aspect = (canvasHost.clientWidth / canvasHost.clientHeight) || 1;
  orthoCamera.left = center[0] - half * aspect;
  orthoCamera.right = center[0] + half * aspect;
  orthoCamera.top = center[1] + half;
  orthoCamera.bottom = center[1] - half;
  orthoCamera.updateProjectionMatrix();
}

function fitCamera() {
  const boxes = projectedBoxes();
  const center = [0, 0, 0];
  let half = 4;
  let radius = 8;
  if (boxes.length > 0) {
    const min = [Infinity, Infinity, Infinity];
    const max = [-Infinity, -Infinity, -Infinity];
    for (const { lo, hi } of boxes) {
      for (let i = 0; i < 3; i++) {
        min[i] = Math.min(min[i], lo[i]);
        max[i] = Math.max(max[i], hi[i]);
      }
    }
    center[0] = (min[0] + max[0]) / 2;
    center[1] = (min[1] + max[1]) / 2;
    center[2] = (min[2] + max[2]) / 2;
    half = Math.max(1, (max[0] - min[0]) / 2, (max[1] - min[1]) / 2);
    radius = Math.max(1, max[0] - min[0], max[1] - min[1], max[2] - min[2]);
  }
  controls.target.set(...center);
  if (camera === orthoCamera) {
    orthoCamera.near = 0.1;
    orthoCamera.far = 100;
    setOrthoFrustum(center, half);
    orthoCamera.position.set(center[0], center[1], 10);
  } else {
    camera.position.set(
      center[0] + radius * 0.8,
      center[1] + radius * 0.8,
      center[2] + radius * 1.1,
    );
    camera.near = radius / 10;
    camera.far = radius * 20;
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

function exampleFor(dims) {
  // 1D–3D: a centred hollow shell (two segments, a square frame, a cube's six
  // faces). 4D: two hypercubes sliding past each other with their overlap
  // carved out — their symmetric difference. Sweeping w shows one cube slide
  // into the other, open into a six-face shell through the overlap, and slide
  // on through.
  const examples = {
    1: 'add,(0),(8)\nremove,(2),(6)',
    2: 'add,(0,0),(4,4)\nremove,(1,1),(3,3)',
    3: 'add,(0,0,0),(4,4,4)\nremove,(1,1,1),(3,3,3)',
    4: 'add,(0,0,0,0),(3,3,3,3)\nadd,(1,1,1,1),(4,4,4,4)\nremove,(1,1,1,1),(3,3,3,3)',
  };
  return examples[dims] ?? examples[3];
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

// ---------- websocket ----------
let socket = null;

function setStatus(text) {
  statusEl.textContent = text;
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

// applyState folds a server view into the scene and the result textarea. It
// deliberately never refits the camera: an incoming edit must not yank the view.
function applyState(state) {
  currentBoxes = state.boxes || [];
  const dims = state.dims;
  if (dims > 0 && dims !== currentDims) {
    currentDims = dims;
    dimsSel.value = String(Math.min(dims, 4));
    sliceEl.hidden = dims !== 4;
    opsEl.value = exampleFor(dims);
    setViewMode(dims);
  }
  resultEl.value = boxesToCSV(currentBoxes);
  rebuild();
}

function connect() {
  const proto = location.protocol === 'https:' ? 'wss' : 'ws';
  const session = new URLSearchParams(location.search).get('s');
  if (!session) {
    // A page without a session id (e.g. an old bookmark) mints one instead of
    // opening a socket the server would have to reject.
    location.replace('/ui/');
    return;
  }
  const ws = new WebSocket(`${proto}://${location.host}/ws?s=${encodeURIComponent(session)}`);
  socket = ws;

  ws.onopen = () => {
    setStatus('connected — edits are shared live');
    const startDims = Number(new URLSearchParams(location.search).get('dims'));
    if (!startDimsSent && startDims >= 1 && startDims <= 4) {
      startDimsSent = true;
      sendOp({ kind: 'dims', dims: startDims });
    }
  };
  ws.onclose = () => {
    setStatus('disconnected — reconnecting…');
    setTimeout(connect, 1000);
  };
  ws.onerror = () => setStatus('connection error');

  ws.onmessage = (ev) => {
    let msg;
    try {
      msg = JSON.parse(ev.data);
    } catch {
      return;
    }
    if (msg.state) applyState(msg.state);
    if (msg.clientID) {
      clientID = msg.clientID;
      updatePresence();
      needsFit = true; // fit once on join, not on every later edit
    }
    if (msg.ops) {
      logEl.innerHTML = '';
      for (const op of msg.ops) appendLog(op);
    }
    if (msg.op) {
      appendLog(msg.op);
    }
    if (msg.clients !== undefined || msg.total !== undefined) updatePresence(msg.clients, msg.total);
    if (msg.type === 'error') {
      setStatus(`error: ${msg.error}`);
    }
  };
}

function sendOp(op) {
  if (socket && socket.readyState === WebSocket.OPEN) {
    socket.send(JSON.stringify(op));
  } else {
    setStatus('not connected — cannot send');
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
  } catch (e) {
    setStatus(`error: ${e.message}`);
  }
});

exampleBtn.addEventListener('click', () => {
  opsEl.value = exampleFor(currentDims);
  applyScenario(currentDims, opsEl.value);
});

randomBtn.addEventListener('click', () => {
  opsEl.value = randomOp(currentDims);
  setStatus('drafted a random op — review and send');
});

newSessionBtn.addEventListener('click', () => {
  // A session's dimension is fixed; start a fresh one with the chosen dimension.
  location.replace(`/ui/?dims=${Number(dimsSel.value)}`);
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

// ---------- boot ----------
resize();
wVal.textContent = sliceW.toFixed(2);
opsEl.value = exampleFor(3);
setViewMode(currentDims);
connect();
tick();
