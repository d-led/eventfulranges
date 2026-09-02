#!/usr/bin/env bash
# Builds the browser-only (WebAssembly) edition of the web visualizer into
# demo/web/dist-local: the same UI, plus the Go engine compiled to WebAssembly
# and its runtime glue. Serve the folder with any static host — no Go server
# runs; the engine executes inside the page.
set -euo pipefail
cd "$(dirname "$0")/.."

web=demo/web
ui="$web/ui-src"

echo "(bundling the UI into dist-local)"
(cd "$ui" && npm run build:local)

echo "(compiling the Go engine for js/wasm)"
(cd "$web" && GOOS=js GOARCH=wasm go build -trimpath -ldflags="-s -w" -o dist-local/engine.wasm .)

echo "(copying the Go wasm runtime next to it)"
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" "$web/dist-local/wasm_exec.js"

echo "browser-only build ready: $web/dist-local"
