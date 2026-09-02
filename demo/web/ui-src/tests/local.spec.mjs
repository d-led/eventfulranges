import { test, expect } from '@playwright/test';

// Browser-only (WebAssembly) end-to-end tests: the same UI runs against the
// Go engine compiled to wasm and hosted by a plain static server — no Go
// server process exists. Run with `npm run test:local` (see
// playwright.local.config.mjs), which serves demo/web/dist-local.

// connect loads the page and waits for the in-page engine to come up.
async function connect(page) {
  await page.goto('/');
  await expect(page.locator('#status')).toContainText('running in this page', { timeout: 20_000 });
  await expect(page.locator('#presence')).toContainText('1 here');
}

test('runs entirely in the page and renders the canvas', async ({ page }) => {
  await connect(page);
  await expect(page.locator('#canvas canvas')).toBeAttached();
  // The page mints its own shareable session id, like the server would.
  await expect(page).toHaveURL(/[?&]s=/);
});

test('folds add/remove into the hollow cube inside the page', async ({ page }) => {
  await connect(page);
  await page.locator('#ops').fill('add,(0,0,0),(4,4,4)\nremove,(1,1,1),(3,3,3)');
  await page.locator('#send').click();
  await expect(async () => {
    const lines = (await page.locator('#result').inputValue()).trim().split('\n');
    expect(lines.filter(Boolean)).toHaveLength(6);
  }).toPass({ timeout: 10_000 });
});

test('keeps the model across reloads from the local reserve', async ({ page }) => {
  await connect(page);
  await page.locator('#ops').fill('add,(0,0,0),(4,4,4)');
  await page.locator('#send').click();
  await expect(page.locator('#result')).not.toHaveValue('');

  // A reload starts a fresh wasm instance (empty hub); the page heals it by
  // replaying its localStorage reserve, so the cube comes straight back.
  await page.reload();
  await expect(page.locator('#status')).toContainText('running in this page', { timeout: 20_000 });
  await expect(async () => {
    const lines = (await page.locator('#result').inputValue()).trim().split('\n');
    expect(lines.filter(Boolean)).toHaveLength(1);
  }).toPass({ timeout: 10_000 });
});

test('merge compaction merges adjacent boxes', async ({ page }) => {
  // A compact=merge 2D session joins touching boxes into one, as the
  // library's MergeAdjacent canonicalizer does in the Go server build too.
  await page.goto('/?dims=2&compact=merge');
  await expect(page.locator('#status')).toContainText('running in this page', { timeout: 20_000 });
  await page.locator('#ops').fill('add,(0,0),(2,4)\nadd,(2,0),(4,4)');
  await page.locator('#send').click();
  await expect(async () => {
    const lines = (await page.locator('#result').inputValue()).trim().split('\n');
    expect(lines.filter(Boolean)).toHaveLength(1);
  }).toPass({ timeout: 10_000 });
});

test('partition compaction splits overlaps into disjoint boxes', async ({ page }) => {
  await page.goto('/?dims=2&compact=partition');
  await expect(page.locator('#status')).toContainText('running in this page', { timeout: 20_000 });

  // Two corner-overlapping boxes split into five non-overlapping rectangles.
  await page.locator('#ops').fill('add,(0,0),(2,2)\nadd,(1,1),(3,3)');
  await page.locator('#send').click();

  await expect(async () => {
    const lines = (await page.locator('#result').inputValue()).trim().split('\n');
    expect(lines.filter(Boolean)).toHaveLength(5);
  }).toPass({ timeout: 10_000 });
});

test('a new 4D session keeps its dimension across reload', async ({ page }) => {
  await connect(page);
  await page.locator('#newSession').click();
  await page.locator('#dims').selectOption('4');
  await page.locator('#startSession').click();
  await expect(page.locator('#slice')).toBeVisible({ timeout: 10_000 });

  await page.reload();
  await expect(page.locator('#status')).toContainText('running in this page', { timeout: 20_000 });
  await expect(page.locator('#slice')).toBeVisible();
});
