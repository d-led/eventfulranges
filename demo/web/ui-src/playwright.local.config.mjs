import { defineConfig } from '@playwright/test';

// Playwright launches a static server over the browser-only (WebAssembly)
// build (demo/web/dist-local) and runs local.spec.mjs against it: no Go
// server is involved — the engine executes inside the page.
export default defineConfig({
  testDir: './tests',
  testMatch: 'local.spec.mjs',
  timeout: 45_000,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? 'list' : 'html',
  use: {
    baseURL: 'http://127.0.0.1:18083',
  },
  webServer: {
    cwd: '../../..',
    command: './scripts/serve-local.sh 18083',
    url: 'http://127.0.0.1:18083/',
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
  },
});
