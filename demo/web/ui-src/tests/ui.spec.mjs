import { test, expect } from '@playwright/test';

test('serves the UI, connects, and renders the canvas', async ({ page }) => {
  await page.goto('/ui/');
  await expect(page.locator('#status')).toContainText('connected');
  await expect(page.locator('#canvas canvas')).toBeAttached();
});

test('two tabs converge on the same view', async ({ browser }) => {
  const alice = await browser.newPage();
  await alice.goto('/ui/');
  // The bare URL redirects to a unique, shareable session URL.
  await alice.waitForURL(/[?&]s=/);

  const bob = await browser.newPage();
  await bob.goto(alice.url()); // both tabs join the same shared model
  await expect(alice.locator('#status')).toContainText('connected');
  await expect(bob.locator('#status')).toContainText('connected');

  // Start from a clean shared view, then add a cube and carve out its middle.
  await alice.locator('#reset').click();
  await alice.locator('#ops').fill('add,(0,0,0),(4,4,4)\nremove,(1,1,1),(3,3,3)');
  await alice.locator('#send').click();

  // Both browsers observe the same materialization (a six-face hollow cube).
  await expect(async () => {
    const a = (await alice.locator('#result').inputValue()).trim();
    const b = (await bob.locator('#result').inputValue()).trim();
    expect(a).not.toBe('');
    expect(a).toBe(b);
  }).toPass({ timeout: 10_000 });

  const lines = (await alice.locator('#result').inputValue()).trim().split('\n');
  expect(lines).toHaveLength(6);
});
