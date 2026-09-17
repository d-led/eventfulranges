// The engine's own thread.
//
// The Go hub compiled to WebAssembly (demo/web/wasm.go) runs here instead of in
// the page: a fold can take hundreds of milliseconds — seconds on a heavily
// partitioned 3D session — and Go's wasm runtime cannot yield part-way through
// one, so on the main thread it would freeze the canvas, the buttons, and the
// very feedback that is supposed to explain the wait.
//
// The worker speaks the same envelopes as the WebSocket server. join answers
// with the catch-up envelope a late joiner would receive; every op pushes the
// broadcast envelopes that result from it back through the dispatch that join
// registered. Around each fold it reports busy and idle, which is the page's
// cue to say that the engine is working.
importScripts('wasm_exec.js');

const engine = load();

self.addEventListener('message', async (event) => {
  const { kind } = event.data;
  try {
    const api = await engine;
    if (kind === 'join') {
      fold(() => {
        const hello = api.join(event.data.session, event.data.compact, post);
        self.postMessage({ kind: 'message', envelope: hello });
      });
      self.postMessage({ kind: 'ready' });
      return;
    }
    if (kind === 'op') {
      fold(() => api.op(JSON.stringify(event.data.op)));
    }
  } catch (error) {
    self.postMessage({ kind: 'error', message: error.message });
  }
});

// fold runs one engine call between a busy and an idle report. The busy report
// is posted before the call, so the page receives it while this thread is still
// inside it — the work itself starts only after.
function fold(work) {
  self.postMessage({ kind: 'busy' });
  try {
    work();
  } finally {
    self.postMessage({ kind: 'idle' });
  }
}

// post sends one engine envelope to the page.
function post(envelope) {
  self.postMessage({ kind: 'message', envelope });
}

// load fetches the Go runtime glue and the engine module, starts the Go
// program, and returns the bridge it publishes on the worker's global object.
async function load() {
  const go = new globalThis.Go();
  const resp = await fetch('engine.wasm');
  if (!resp.ok) throw new Error(`engine.wasm: ${resp.status} ${resp.statusText}`);
  const { instance } = await WebAssembly.instantiate(await resp.arrayBuffer(), go.importObject);
  go.run(instance);
  return globalThis.__eventfulranges_wasm;
}
