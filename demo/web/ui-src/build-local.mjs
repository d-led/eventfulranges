// Bundles the same UI for the browser-only build (dist-local/): app.js (with
// Three.js), the static assets, and an index.html that preselects the in-page
// WebAssembly engine. The Go engine itself (engine.wasm) and its runtime
// (wasm_exec.js) are added by scripts/build-local.sh.
import { build } from 'esbuild';
import { cpSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs';

const out = '../dist-local';
mkdirSync(out, { recursive: true });

const options = {
  entryPoints: ['app.js'],
  bundle: true,
  minify: true,
  format: 'esm',
  outfile: `${out}/app.js`,
  logLevel: 'info',
};
await build(options);

cpSync('style.css', `${out}/style.css`);

// The local page preselects the wasm engine. A module script runs before the
// app bundle, so the flag is set by the time app.js reads it (module scripts
// execute in document order).
let html = readFileSync('index.html', 'utf8');
html = html.replace(
  '<script type="module" src="app.js"></script>',
  '<script type="module">window.__EVENTFULRANGES_STATIC__ = true;</script>\n  <script type="module" src="app.js"></script>',
);
writeFileSync(`${out}/index.html`, html);
console.log(`built ${out}`);
