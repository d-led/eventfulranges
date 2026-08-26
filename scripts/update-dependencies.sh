#!/usr/bin/env bash
# Upgrades every dependency in each Go module to the latest version, tidies the
# module files, and verifies the result with the race detector.
#
# Go modules:
#   - "." and "./demo" live in the root go.work workspace.
#   - "./examples/calendar" is deliberately standalone: it resolves the
#     published library by version (no replace directive), so it must be
#     upgraded with GOWORK=off.
#
# npm packages: each demo UI (web, paint) keeps its UI and Playwright test
# tooling in a single package.json.
set -euo pipefail
cd "$(dirname "$0")/.."

# Modules that live in the root go.work.
workspace_modules=("." "demo")

# Modules that are deliberately standalone (own go.mod, not in go.work).
standalone_modules=("examples/calendar")

for mod in "${workspace_modules[@]}"; do
  echo "== upgrade Go module $mod"
  (
    cd "$mod"
    go get -u ./...
    go get -u -t ./...
    go mod tidy
    go test -race -count=1 ./...
  )
done

for mod in "${standalone_modules[@]}"; do
  echo "== upgrade Go module $mod (standalone)"
  (
    cd "$mod"
    GOWORK=off go get -u ./...
    GOWORK=off go get -u -t ./...
    GOWORK=off go mod tidy
    GOWORK=off go test -race -count=1 ./...
  )
done

# Refresh the workspace checksums after the module files changed.
go work sync

# Upgrade each demo UI's npm dependencies. A demo's UI and its Playwright test
# tooling (@playwright/test, esbuild) live in a single package.json, so one
# `taze -w major` bumps both the UI and the test dependencies.
ui_dirs=("demo/web/ui-src" "demo/paint/ui-src")
for ui in "${ui_dirs[@]}"; do
  echo "== upgrade $ui"
  (
    cd "$ui"
    npx -y taze -w major
    npm install --no-audit --no-fund
  )
done

echo "dependencies upgraded"
