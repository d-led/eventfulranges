import { defineConfig } from "@playwright/test";

// Playwright launches the Go server itself, so a single `npx playwright test`
// (or `npm test`) exercises the full stack: gin server, WebSocket, and UI.
export default defineConfig({
  testDir: "./tests",
  timeout: 30_000,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? "list" : "html",
  use: {
    baseURL: "http://127.0.0.1:18083",
  },
  webServer: {
    cwd: "..",
    command: "go run . -addr 127.0.0.1:18083",
    url: "http://127.0.0.1:18083/ui/app.js",
    reuseExistingServer: !process.env.CI,
    timeout: 60_000,
  },
});
