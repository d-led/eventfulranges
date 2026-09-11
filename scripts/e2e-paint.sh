#!/usr/bin/env bash
# Builds the whiteboard UI, runs the JS unit tests (vitest), and runs the
# Playwright end-to-end tests (demo/paint).
set -euo pipefail
cd "$(dirname "$0")/.."

./scripts/build-paint.sh

cd demo/paint/ui-src
npm run test:unit
npx playwright install --with-deps chromium
# line reporter: the html reporter serves a report and waits, so a failing test
# would hang the gate instead of failing it.
npx playwright test --reporter=line
