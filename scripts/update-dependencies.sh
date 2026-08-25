#!/usr/bin/env bash
# Upgrades every dependency in each Go module to the latest version, tidies the
# module files, and verifies the result with the race detector. The workspace
# holds two modules: the library at the root and the demo app in ./demo.
set -euo pipefail
cd "$(dirname "$0")/.."

for mod in "." "demo"; do
  echo "== upgrade $mod"
  (
    cd "$mod"
    go get -u ./...
    go get -u -t ./...
    go mod tidy
    go test -race -count=1 ./...
  )
done

# Refresh the workspace checksums after the module files changed.
go work sync

# Upgrade the web UI's npm dependencies. The UI (three) and the Playwright test
# tooling (@playwright/test, esbuild) live in a single package.json, so one
# `taze -w major` bumps both the UI and the test dependencies.
(
  cd demo/web/ui-src
  npx -y taze -w major
  npm install --no-audit --no-fund
)

echo "dependencies upgraded"
