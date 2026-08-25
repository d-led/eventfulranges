#!/usr/bin/env bash
# Builds the web UI and runs the Playwright end-to-end tests (demo/web).
set -euo pipefail
cd "$(dirname "$0")/.."

./scripts/build-web.sh

cd demo/web/ui-src
npm run test:unit
npx playwright install --with-deps chromium
npx playwright test
