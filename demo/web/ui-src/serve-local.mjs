// The development server for the browser-only (WebAssembly) build: it rebuilds
// demo/web/dist-local whenever a source file changes, serves the folder over
// HTTP with caching disabled (a stale engine.wasm is the one failure mode this
// loop exists to prevent), and prints the URL to open.
//
// The build itself is always delegated to scripts/build-local.sh, so the
// one-shot build, the e2e tests and this loop can never drift apart.
import { spawn } from 'node:child_process';
import { createServer } from 'node:http';
import { watch } from 'node:fs';
import { readFile, stat } from 'node:fs/promises';
import { extname, join, normalize, resolve } from 'node:path';

const repo = resolve(import.meta.dirname, '../../..');
const dist = join(repo, 'demo/web/dist-local');
const port = Number(process.env.PORT || process.argv[2] || 8082);

// The bundle is out of date when a UI source or any Go source the engine is
// built from changes — hence one recursive watch over the repository, filtered
// to those files so build output and dependencies never trigger a rebuild.
const RELEVANT = /\.(js|mjs|css|html)$|(?<!_test)\.go$/;
const IGNORED = /(^|\/)(node_modules|\.git|build|dist|dist-local|test-results|playwright-report)(\/|$)/;

const CONTENT_TYPES = {
  '.html': 'text/html; charset=utf-8',
  '.js': 'text/javascript; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.wasm': 'application/wasm',
  '.json': 'application/json',
  '.svg': 'image/svg+xml',
};

// rebuild runs the shared build script and reports how it went. Builds are
// serialised: a change arriving mid-build queues exactly one more run.
let building = false;
let queued = false;

async function rebuild(reason) {
  if (building) {
    queued = true;
    return;
  }
  building = true;
  console.log(`(rebuilding — ${reason})`);
  const child = spawn(join(repo, 'scripts/build-local.sh'), [], { cwd: repo, stdio: 'inherit' });
  child.on('exit', (code) => {
    building = false;
    console.log(code === 0 ? 'rebuilt — reload the page' : `build failed (exit ${code})`);
    if (queued) {
      queued = false;
      rebuild('changes during the last build');
    }
  });
}

function watchForChanges() {
  let timer = null;
  watch(repo, { recursive: true }, (_event, file) => {
    if (!file || !RELEVANT.test(file) || IGNORED.test(file)) return;
    clearTimeout(timer);
    timer = setTimeout(() => rebuild(file), 200);
  });
}

// serveFile answers one request from dist-local, refusing to escape the folder
// and disabling caching so a reload always picks up the newest build.
async function serveFile(path, res) {
  const target = join(dist, normalize(path));
  if (!target.startsWith(dist)) {
    res.writeHead(403).end('forbidden');
    return;
  }
  try {
    const info = await stat(target);
    const file = info.isDirectory() ? join(target, 'index.html') : target;
    const body = await readFile(file);
    res.writeHead(200, {
      'content-type': CONTENT_TYPES[extname(file)] || 'application/octet-stream',
      'cache-control': 'no-store',
    });
    res.end(body);
  } catch {
    res.writeHead(404).end('not found');
  }
}

await rebuild('initial build');
watchForChanges();

createServer((req, res) => serveFile(decodeURIComponent(new URL(req.url, 'http://localhost').pathname), res))
  .listen(port, '127.0.0.1', () => {
    console.log(`browser-only build served at http://127.0.0.1:${port}/`);
    console.log('watching for changes — Ctrl+C stops it');
  });
