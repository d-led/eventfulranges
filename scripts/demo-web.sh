#!/usr/bin/env bash
# Builds the web UI on first use, then starts the interactive visualizer.
# Open the printed URL in a browser; Ctrl+C stops it.
set -euo pipefail
cd "$(dirname "$0")/.."

echo "(re-)generating the web UI"
./scripts/build-web.sh

cd demo/web
exec go run . "$@"
