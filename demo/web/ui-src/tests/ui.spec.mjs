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

  // Both tabs share one empty model: Alice adds a cube and carves its middle.
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

test('the chosen dimension survives a reload', async ({ page }) => {
  await page.goto('/ui/');
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator('#status')).toContainText('connected');

  await page.locator('#dims').selectOption('4');
  // The dimension is shared state: wait until the server records the change.
  await expect(page.locator('#log li')).toContainText('dims');

  await page.reload();
  await expect(page.locator('#status')).toContainText('connected');
  await expect(page.locator('#dims')).toHaveValue('4');
  await expect(page.locator('#slice')).toBeVisible();
});

test('new session starts fresh with an unfixed dimension', async ({ page }) => {
  await page.goto('/ui/');
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator('#status')).toContainText('connected');

  await page.locator('#dims').selectOption('4');
  await expect(page.locator('#log li')).toContainText('dims');
  const before = page.url();

  await page.locator('#newSession').click();
  await expect(page).not.toHaveURL(before);
  await expect(page.locator('#status')).toContainText('connected');
  await expect(page.locator('#dims')).toHaveValue('3');
  await expect(page.locator('#slice')).toBeHidden();
});

test('4D animate sweeps the w slice', async ({ page }) => {
  await page.goto('/ui/');
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator('#status')).toContainText('connected');

  await page.locator('#dims').selectOption('4');
  await page.locator('#example').click();

  // The slice position readout moves while animate is checked.
  await expect(page.locator('#wval')).not.toHaveText('2.00');
  const before = await page.locator('#wval').textContent();
  await page.waitForTimeout(300);
  const after = await page.locator('#wval').textContent();
  expect(after).not.toBe(before);
});
