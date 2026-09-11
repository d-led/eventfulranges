#!/usr/bin/env bash
# Builds the web UI and runs the Playwright end-to-end tests (demo/web).
set -euo pipefail
cd "$(dirname "$0")/.."

./scripts/build-web.sh

cd demo/web/ui-src
npm run test:unit
npx playwright install --with-deps chromium
# line reporter: the html reporter serves a report and waits, so a failing test
# would hang the gate instead of failing it.
npx playwright test --reporter=line
