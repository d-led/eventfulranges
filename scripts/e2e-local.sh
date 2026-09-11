#!/usr/bin/env bash
# Builds the browser-only (WebAssembly) build of the web demo and runs the
# local-engine Playwright tests against it, served statically (no Go server).
set -euo pipefail
cd "$(dirname "$0")/.."

./scripts/build-local.sh

cd demo/web/ui-src
npx playwright install --with-deps chromium
# line reporter: the html reporter serves a report and waits, so a failing test
# would hang the gate instead of failing it.
npm run test:local -- --reporter=line
