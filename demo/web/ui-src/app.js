import * as THREE from 'three';
import { OrbitControls } from 'three/addons/controls/OrbitControls.js';

// ---------- DOM ----------
const $ = (id) => document.getElementById(id);
const dimsSel = $('dims');
const opsEl = $('ops');
const sendBtn = $('send');
const exampleBtn = $('example');
const resetBtn = $('reset');
const sliceEl = $('slice');
const wInput = $('w');
const wVal = $('wval');
const animateChk = $('animate');
const statusEl = $('status');
const resultEl = $('result');
const copyBtn = $('copy');

// ---------- three.js scene ----------
const canvasHost = $('canvas');
const scene = new THREE.Scene();
scene.background = new THREE.Color(0x111318);

const camera = new THREE.PerspectiveCamera(50, 1, 0.01, 1000);
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

scene.add(new THREE.GridHelper(20, 20, 0x2a2f3a, 0x1c2029));
scene.add(new THREE.AxesHelper(6));

const palette = [
  0x4f8cff, 0xff6b6b, 0x51cf66, 0xffd43b,
  0xcc5de8, 0x22b8cf, 0xff922b, 0x74c0fc,
];

let currentBoxes = []; // [{min, max}] as returned by the server
let currentDims = 3;
let sliceW = 0.5;
let needsFit = true;

function resize() {
  const w = canvasHost.clientWidth;
  const h = canvasHost.clientHeight;
  renderer.setSize(w, h, false);
  camera.aspect = w / h;
  camera.updateProjectionMatrix();
}
window.addEventListener('resize', resize);

function makeBox(lo, hi, color, opacity) {
  const size = lo.map((v, i) => Math.max(hi[i] - v, 1e-4));
  const geo = new THREE.BoxGeometry(size[0], size[1], size[2]);
  const fill = new THREE.MeshStandardMaterial({
    color,
    transparent: true,
    opacity,
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
    new THREE.LineBasicMaterial({ color: 0xffffff, transparent: true, opacity: 0.85 }),
  );
  mesh.add(edges);
  return mesh;
}

// Project an n-dimensional box onto 3D. Missing dimensions are padded to thin
// slabs; 4D boxes are sliced at the current w value.
function project(box, dims, w) {
  const { min, max } = box;
  if (dims === 4 && !(min[3] <= w && w < max[3])) return null;
  const lo = min.slice(0, 3);
  const hi = max.slice(0, 3);
  for (let d = dims; d < 3; d++) {
    lo.push(-0.35);
    hi.push(0.35);
  }
  return { lo, hi };
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
    boxesGroup.add(makeBox(p.lo, p.hi, palette[i % palette.length], 0.4));
  }
}

function fitCamera() {
  const boxes = projectedBoxes();
  if (boxes.length === 0) {
    controls.target.set(0, 0, 0);
    camera.position.set(6, 5, 8);
    controls.update();
    return;
  }
  const min = [Infinity, Infinity, Infinity];
  const max = [-Infinity, -Infinity, -Infinity];
  for (const { lo, hi } of boxes) {
    for (let i = 0; i < 3; i++) {
      min[i] = Math.min(min[i], lo[i]);
      max[i] = Math.max(max[i], hi[i]);
    }
  }
  const center = min.map((v, i) => (v + max[i]) / 2);
  controls.target.set(...center);
  const radius = Math.max(1, max[0] - min[0], max[1] - min[1], max[2] - min[2]);
  camera.position.set(
    center[0] + radius * 0.8,
    center[1] + radius * 0.8,
    center[2] + radius * 1.1,
  );
  camera.near = radius / 100;
  camera.far = radius * 100;
  camera.updateProjectionMatrix();
  controls.update();
}

function tick() {
  requestAnimationFrame(tick);
  if (currentDims === 4 && animateChk.checked) {
    sliceW += 0.004;
    if (sliceW >= 1) sliceW = 0;
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
function parseOps(text, dims) {
  const ops = [];
  for (const raw of text.split('\n')) {
    const line = raw.trim();
    if (!line || line.startsWith('#')) continue;
    const parts = line.split(',').map((s) => s.trim());
    const kind = parts[0].toLowerCase();
    if (kind !== 'add' && kind !== 'remove') {
      throw new Error(`unknown op "${parts[0]}" (use add or remove)`);
    }
    const nums = parts.slice(1).map(Number);
    if (nums.length !== dims * 2) {
      throw new Error(`"${line}": expected ${dims * 2} numbers, got ${nums.length}`);
    }
    if (nums.some(Number.isNaN)) {
      throw new Error(`"${line}": non-numeric value`);
    }
    ops.push({ kind, min: nums.slice(0, dims), max: nums.slice(dims) });
  }
  return ops;
}

function boxesToCSV(boxes) {
  return boxes.map((b) => [...b.min, ...b.max].join(',')).join('\n');
}

function exampleFor(dims) {
  const examples = {
    1: 'add,0,8\nremove,2,4',
    2: 'add,0,0,4,4\nremove,1,1,3,3',
    3: 'add,0,0,0,4,4,4\nremove,1,1,1,3,3,3',
    4: 'add,0,0,0,0,3,3,3,3\nadd,1,1,1,1,4,4,4,4\nremove,1.5,1.5,1.5,1.5,2.5,2.5,2.5,2.5',
  };
  return examples[dims] ?? examples[3];
}

// ---------- websocket ----------
let socket = null;

function setStatus(text) {
  statusEl.textContent = text;
}

function connect() {
  const proto = location.protocol === 'https:' ? 'wss' : 'ws';
  const ws = new WebSocket(`${proto}://${location.host}/ws`);
  socket = ws;

  ws.onopen = () => setStatus('connected — edits are shared live');
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
    if (msg.type === 'state' && msg.state) {
      currentBoxes = msg.state.boxes || [];
      const dims = msg.state.dims;
      if (dims > 0 && dims !== currentDims) {
        currentDims = dims;
        dimsSel.value = String(Math.min(dims, 4));
        sliceEl.hidden = dims !== 4;
      }
      resultEl.value = boxesToCSV(currentBoxes);
      rebuild();
      needsFit = true;
    } else if (msg.type === 'error') {
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

// ---------- events ----------
sendBtn.addEventListener('click', () => {
  const dims = Number(dimsSel.value);
  try {
    const ops = parseOps(opsEl.value, dims);
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
  opsEl.value = exampleFor(Number(dimsSel.value));
});

resetBtn.addEventListener('click', () => {
  sendOp({ kind: 'clear' });
  opsEl.value = '';
});

dimsSel.addEventListener('change', () => {
  sliceEl.hidden = Number(dimsSel.value) !== 4;
  opsEl.value = exampleFor(Number(dimsSel.value));
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

// ---------- boot ----------
resize();
opsEl.value = exampleFor(3);
wVal.textContent = sliceW.toFixed(2);
connect();
tick();
