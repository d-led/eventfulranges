// The in-page session engine: the same Go hub compiled to WebAssembly runs
// inside this page (see demo/web/wasm.go), so the UI works from any static host
// with no server at all. The engine speaks the same JSON envelopes as the
// WebSocket server, and state survives reloads through the same localStorage
// reserve copy: a fresh page replays it back into the engine, exactly as a
// reconnecting socket would be healed.
//
// The engine runs in a worker (engine-worker.js) rather than on this thread,
// because a fold is unbounded work — merging a partitioned 3D session is the
// expensive case — and Go's wasm runtime cannot yield in the middle of one.
// Off this thread, the page keeps painting, orbiting and reporting progress
// while the engine is busy. The worker's busy and idle reports become onBusy,
// which is how the page knows to say that the engine is working.
//
// The assets it needs (wasm_exec.js and engine.wasm, and now the worker itself)
// are emitted next to the bundle by the local build (scripts/build-local.sh);
// everything is resolved relative to this module's own URL, so the folder works
// from any mount path.
export function createLocalEngine({ onMessage, onStatus, onOnline, onFirstSync, onBusy }) {
  let worker = null;
  let online = false;
  let session = null;
  let compact = '';

  // Mint a shareable session id on the spot (the Go server mints one for the
  // socket mode) so the URL stays a working share link.
  function ensureSession() {
    const params = new URLSearchParams(location.search);
    session = params.get('s');
    compact = params.get('compact') || '';
    if (session) return;
    params.set('s', mintSessionID());
    history.replaceState(null, '', `${location.pathname}?${params}`);
    session = params.get('s');
  }

  function spawn() {
    const spawned = new Worker(new URL('engine-worker.js', import.meta.url));
    spawned.addEventListener('message', handleMessage);
    spawned.addEventListener('error', (event) => {
      online = false;
      onOnline(false);
      onStatus(`local engine error: ${event.message || 'the engine worker failed to start'}`);
    });
    return spawned;
  }

  // handleMessage folds one worker report into the page callbacks. The worker
  // keeps these in order, so a 'message' always belongs to the fold whose
  // 'busy' came before it.
  function handleMessage(event) {
    const report = event.data;
    switch (report.kind) {
      case 'busy':
        onBusy(true);
        break;
      case 'idle':
        onBusy(false);
        break;
      case 'ready':
        // The engine has caught up, so commands can be accepted: this is the
        // local engine's equivalent of the socket opening.
        online = true;
        onOnline(true);
        onStatus('running in this page — no server needed');
        onFirstSync();
        break;
      case 'message':
        onMessage(JSON.parse(report.envelope));
        break;
      case 'error':
        onStatus(`local engine error: ${report.message}`);
        break;
    }
  }

  function start() {
    ensureSession();
    worker = worker || spawn();
    worker.postMessage({ kind: 'join', session, compact });
  }

  return {
    start,
    send(op) {
      if (!worker) {
        onStatus('local engine not ready yet');
        return;
      }
      worker.postMessage({ kind: 'op', op });
    },
    // The engine is a worker with no socket to drain: re-joining the session
    // re-reads the hub, which is the whole story for a reload or a reconnect.
    reconnect() {
      start();
    },
    close() {
      // The local engine has no socket to drop: it is never "disconnected".
    },
    // A fresh session is a fresh share link too, but no server mints the id —
    // this page does, so the URL stays shareable in static hosting.
    newSessionURL(dims, compact) {
      const p = new URLSearchParams();
      p.set('s', mintSessionID());
      p.set('dims', String(dims));
      if (compact) p.set('compact', compact);
      return `${location.pathname}?${p}`;
    },
  };
}

// mintSessionID drafts a short, URL-safe id (hex, like the base32 ids the Go
// server mints) for a fresh share link.
function mintSessionID() {
  const bytes = new Uint8Array(8);
  crypto.getRandomValues(bytes);
  return [...bytes].map((b) => b.toString(16).padStart(2, '0')).join('');
}
