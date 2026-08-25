#!/usr/bin/env bash
# Starts the interactive web range-set visualizer (demo/web).
# Open the printed URL in a browser; Ctrl+C stops it.
set -euo pipefail
cd "$(dirname "$0")/.."

cd demo/web
exec go run . "$@"
