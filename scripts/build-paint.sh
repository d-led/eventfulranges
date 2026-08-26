#!/usr/bin/env bash
# Rebuilds the embedded whiteboard UI: installs the JS dependencies and bundles
# demo/paint/ui-src into demo/paint/dist (generated; go:embedded with -tags
# embed). dist/ is gitignored except for the directory marker, so rebuilding
# never dirties tracked files.
set -euo pipefail
cd "$(dirname "$0")/.."

cd demo/paint
go generate .
