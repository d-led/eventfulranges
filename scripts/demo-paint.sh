#!/usr/bin/env bash
# Builds the whiteboard UI on first use, then starts the interactive server.
# Open the printed URL in a browser; Ctrl+C stops it.
set -euo pipefail
cd "$(dirname "$0")/.."

echo "(re-)generating the whiteboard UI"
./scripts/build-paint.sh

cd demo/paint
exec go run . "$@"
