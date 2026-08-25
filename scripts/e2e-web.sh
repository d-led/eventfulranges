#!/usr/bin/env bash
# Runs the Playwright end-to-end tests for the web demo (demo/web).
set -euo pipefail
cd "$(dirname "$0")/.."

cd demo/web/ui-src
npm install --no-audit --no-fund
npx playwright install --with-deps chromium
npx playwright test
