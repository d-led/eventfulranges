import { defineConfig } from 'vitest/config';

// Unit tests are the pure modules; Playwright owns the browser integration
// tests in ./tests, which vitest must not pick up.
export default defineConfig({
  test: {
    include: ['*.unit.test.mjs'],
    environment: 'node',
  },
});
