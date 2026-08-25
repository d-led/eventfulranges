// Bundles app.js (with Three.js) into ../dist/app.js and copies the static
// assets. Run with `npm run build`, or `npm run watch` during development.
import { build, context } from 'esbuild';
import { cpSync, mkdirSync } from 'node:fs';

const watch = process.argv.includes('--watch');

const options = {
  entryPoints: ['app.js'],
  bundle: true,
  minify: true,
  format: 'esm',
  outfile: '../dist/app.js',
  logLevel: 'info',
};

mkdirSync('../dist', { recursive: true });
cpSync('index.html', '../dist/index.html');
cpSync('style.css', '../dist/style.css');

if (watch) {
  const ctx = await context(options);
  await ctx.watch();
  console.log('watching ui-src for changes…');
} else {
  await build(options);
  console.log('built ../dist');
}
