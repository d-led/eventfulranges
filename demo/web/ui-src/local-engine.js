// The in-page session engine: the same Go hub compiled to WebAssembly runs
// inside this page (see demo/web/wasm.go), so the UI works from any static
// host with no server at all. The engine speaks the same JSON envelopes as
// the WebSocket server, and state survives reloads through the same
// localStorage reserve copy: a fresh page replays it back into the engine,
// exactly as a reconnecting socket would be healed.
//
// The assets this engine needs (wasm_exec.js and engine.wasm) are emitted
// next to the bundle by the local build (scripts/build-local.sh); the engine
// resolves them relative to this module's own URL, so it works from any mount
// path.
export function createLocalEngine({ onMessage, onStatus, onOnline, onFirstSync }) {
  let api = null; // the __eventfulranges_wasm bridge exposed by the Go module
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

  async function start() {
    ensureSession();
    try {
      if (!api) api = await loadEngine();
      // join(session, compact, dispatch): the catch-up envelope comes back as
      // the return value, and every later broadcast arrives through dispatch.
      const hello = api.join(session, compact, (json) => onMessage(JSON.parse(json)));
      online = true;
      onOnline(true);
      onMessage(JSON.parse(hello));
      onStatus('running in this page — no server needed');
      onFirstSync();
    } catch (e) {
      online = false;
      onOnline(false);
      onStatus(`local engine error: ${e.message}`);
    }
  }

  return {
    start,
    send(op) {
      if (!api) {
        onStatus('local engine not ready yet');
        return;
      }
      api.op(JSON.stringify(op));
    },
    // A fresh start is a fresh wasm instance; there is no socket to drain, so
    // re-initialising is the whole story. The session id below is minted, so
    // the page reloads onto a new share link, like the server redirect does.
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

// loadEngine loads wasm_exec.js (which defines globalThis.Go) and then the
// engine module, instantiating it with the Go runtime.
async function loadEngine() {
  await loadGoRuntime();
  const go = new globalThis.Go();
  const resp = await fetch(new URL('engine.wasm', import.meta.url));
  if (!resp.ok) throw new Error(`engine.wasm: ${resp.status} ${resp.statusText}`);
  const bytes = await resp.arrayBuffer();
  const { instance } = await WebAssembly.instantiate(bytes, go.importObject);
  go.run(instance);
  return globalThis.__eventfulranges_wasm;
}

function loadGoRuntime() {
  if (globalThis.Go) return Promise.resolve();
  return new Promise((resolve, reject) => {
    const script = document.createElement('script');
    script.src = new URL('wasm_exec.js', import.meta.url).href;
    script.onload = () => resolve();
    script.onerror = () => reject(new Error('could not load wasm_exec.js'));
    document.head.appendChild(script);
  });
}

// mintSessionID drafts a short, URL-safe id (hex, like the base32 ids the Go
// server mints) for a fresh share link.
function mintSessionID() {
  const bytes = new Uint8Array(8);
  crypto.getRandomValues(bytes);
  return [...bytes].map((b) => b.toString(16).padStart(2, '0')).join('');
}
