import { defineConfig } from '@playwright/test';

// Playwright launches the Go server itself, so a single `npx playwright test`
// (or `npm test`) exercises the full stack: gin server, WebSocket, and UI.
// The browser-only (WebAssembly) edition has its own project/config:
// playwright.local.config.mjs (or `npm run test:local`).
export default defineConfig({
  testDir: './tests',
  testIgnore: ['**/local.spec.mjs'], // run separately against the wasm build (npm run test:local)
  timeout: 30_000,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? 'list' : 'html',
  use: {
    baseURL: 'http://127.0.0.1:18081',
  },
  webServer: {
    cwd: '..',
    command: 'go run . -addr 127.0.0.1:18081',
    url: 'http://127.0.0.1:18081/ui/app.js',
    reuseExistingServer: !process.env.CI,
    timeout: 60_000,
  },
});
