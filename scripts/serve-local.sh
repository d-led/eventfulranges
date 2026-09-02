#!/usr/bin/env bash
# Builds and serves the browser-only (WebAssembly) visualizer. No Go server
# runs: a static file server hands out the bundle, and the Go engine runs
# inside the page. Open the printed URL in a browser; Ctrl+C stops it.
set -euo pipefail
cd "$(dirname "$0")/.."

port="${1:-8082}"
./scripts/build-local.sh

cd demo/web/dist-local
echo "open http://127.0.0.1:${port}/ in a browser"
exec python3 -m http.server "$port"
