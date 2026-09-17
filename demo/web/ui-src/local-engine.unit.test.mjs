// The local engine talks to a worker, so the page never has to wait on the
// engine thread. These tests drive that protocol with a stand-in worker and
// check what the engine sends, and what the page is told in return.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createLocalEngine } from './local-engine.js';

test('joining the session hands the worker the session and compaction', () => {
  const page = connected();
  page.engine.start();
  assert.deepEqual(page.worker().sent[0], { kind: 'join', session: 'abc123', compact: 'partition-merge' });
});

test('the engine comes online with the catch-up envelope already delivered', () => {
  const page = connected();
  page.engine.start();
  page.report({ kind: 'message', envelope: JSON.stringify({ type: 'state', state: { boxes: [], dims: 3 } }) });
  page.report({ kind: 'ready' });

  assert.deepEqual(page.seen.messages, [{ type: 'state', state: { boxes: [], dims: 3 } }]);
  assert.deepEqual(page.seen.online, [true]);
  assert.equal(page.seen.firstSync, 1, 'the ?dims= preference is applied once the engine is up');
});

test('an operation is handed to the worker as a command', () => {
  const page = connected();
  page.engine.start();
  page.report({ kind: 'ready' });
  page.engine.send({ kind: 'add', min: [0, 0], max: [1, 1] });

  assert.deepEqual(page.worker().sent.at(-1), { kind: 'op', op: { kind: 'add', min: [0, 0], max: [1, 1] } });
});

test('the page is told when the engine starts and stops working', () => {
  const page = connected();
  page.engine.start();
  page.report({ kind: 'ready' });
  page.engine.send({ kind: 'add', min: [0], max: [1] });
  page.report({ kind: 'busy' });
  page.report({ kind: 'idle' });

  assert.deepEqual(page.seen.busy, [true, false], 'a fold brackets its work with busy and idle');
});

test('several queued operations each report their own busy round', () => {
  const page = connected();
  page.engine.start();
  page.report({ kind: 'ready' });
  page.engine.send({ kind: 'add', min: [0], max: [1] });
  page.engine.send({ kind: 'remove', min: [0], max: [1] });
  for (const kind of ['busy', 'idle', 'busy', 'idle']) page.report({ kind });

  assert.deepEqual(page.seen.busy, [true, false, true, false]);
  assert.equal(page.worker().sent.filter((m) => m.kind === 'op').length, 2);
});

test('a failed engine reports it instead of pretending to be online', () => {
  const page = connected();
  page.engine.start();
  page.report({ kind: 'error', message: 'engine.wasm: 404 Not Found' });

  assert.deepEqual(page.seen.status, ['local engine error: engine.wasm: 404 Not Found']);
  assert.deepEqual(page.seen.online, [], 'the page is never told it is online');
});

test('a reconnect re-joins the session on the worker already running', () => {
  const page = connected();
  page.engine.start();
  page.report({ kind: 'ready' });
  page.engine.reconnect();

  assert.equal(page.spawned(), 1, 'the engine instance is reused, not started again');
  assert.deepEqual(page.worker().sent.at(-1), { kind: 'join', session: 'abc123', compact: 'partition-merge' });
});

// connected returns an engine wired to a stand-in worker, and records what the
// page was told. The worker only appears once the engine starts, so tests reach
// it through worker().
function connected() {
  const seen = { messages: [], status: [], online: [], busy: [], firstSync: 0 };
  const workers = [];
  globalThis.location = { search: '?s=abc123&compact=partition-merge', pathname: '/', href: 'http://localhost/' };
  globalThis.history = { replaceState() {} };
  globalThis.Worker = class {
    constructor() {
      this.sent = [];
      this.listeners = {};
      workers.push(this);
    }
    addEventListener(kind, listener) {
      this.listeners[kind] = listener;
    }
    postMessage(message) {
      this.sent.push(message);
    }
  };
  const engine = createLocalEngine({
    onMessage: (message) => seen.messages.push(message),
    onStatus: (text) => seen.status.push(text),
    onOnline: (online) => seen.online.push(online),
    onBusy: (busy) => seen.busy.push(busy),
    onFirstSync: () => seen.firstSync++,
  });
  return {
    engine,
    seen,
    worker: () => workers[0],
    spawned: () => workers.length,
    report: (message) => workers[0].listeners.message({ data: message }),
  };
}
