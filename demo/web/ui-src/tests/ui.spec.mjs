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

test('changing the dimension does not affect the current session', async ({ page }) => {
  await page.goto('/ui/');
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator('#status')).toContainText('connected');

  // Selecting a dimension is only a preference for the next session; the
  // current (unfixed) session stays at its default 3D until a box fixes it.
  await page.locator('#dims').selectOption('4');
  await expect(page.locator('#slice')).toBeHidden();
});

test('a new session keeps its chosen dimension across reload', async ({ page }) => {
  await page.goto('/ui/');
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator('#status')).toContainText('connected');

  await page.locator('#dims').selectOption('4');
  await page.locator('#newSession').click();
  await expect(page.locator('#slice')).toBeVisible(); // the new session is 4D

  await page.reload();
  await expect(page.locator('#status')).toContainText('connected');
  await expect(page.locator('#dims')).toHaveValue('4');
  await expect(page.locator('#slice')).toBeVisible();
});

test('4D animate sweeps the w slice', async ({ page }) => {
  await page.goto('/ui/');
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator('#status')).toContainText('connected');

  await page.locator('#dims').selectOption('4');
  await page.locator('#newSession').click();
  await expect(page.locator('#slice')).toBeVisible();

  await page.locator('#example').click();

  // The slice position readout moves while animate is checked.
  await expect(page.locator('#wval')).not.toHaveText('2.00');
  const before = await page.locator('#wval').textContent();
  await page.waitForTimeout(300);
  const after = await page.locator('#wval').textContent();
  expect(after).not.toBe(before);
});

test('the 4D sweep changes what is rendered', async ({ page }) => {
  await page.goto('/ui/');
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator('#status')).toContainText('connected');

  await page.locator('#dims').selectOption('4');
  await page.locator('#newSession').click();
  await expect(page.locator('#slice')).toBeVisible();

  await page.locator('#example').click();
  await expect(page.locator('#result')).not.toHaveValue('');

  // Freeze the sweep, then sample the canvas at two different w positions.
  await page.locator('#animate').uncheck();

  const setW = async (w) => {
    await page.evaluate((val) => {
      const input = document.getElementById('w');
      input.value = String(val);
      input.dispatchEvent(new Event('input', { bubbles: true }));
    }, w);
    await page.waitForTimeout(150); // let a frame render
  };

  await setW(0.5);
  const atLow = await page.locator('#canvas').screenshot();
  await setW(3.5);
  const atHigh = await page.locator('#canvas').screenshot();

  expect(Buffer.compare(atLow, atHigh)).not.toBe(0);
});

test('random op appends a valid command in the session dimension', async ({ page }) => {
  await page.goto('/ui/');
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator('#status')).toContainText('connected');

  // The ops window is pre-filled with an example; clear it to isolate the
  // append behaviour of the random drafts.
  await page.locator('#ops').fill('');
  await page.locator('#random').click();
  await page.locator('#random').click();
  const lines = (await page.locator('#ops').inputValue()).trim().split('\n');
  expect(lines).toHaveLength(2);

  // The current (default) session is 3D, so each tuple carries 3 coordinates.
  for (const line of lines) {
    const m = line.match(/^(add|remove),\(([^)]*)\),\(([^)]*)\)$/);
    expect(m).toBeTruthy();
    expect(m[2].split(',')).toHaveLength(3);
    expect(m[3].split(',')).toHaveLength(3);
    for (const coord of [...m[2].split(','), ...m[3].split(',')]) {
      const n = Number(coord);
      expect(Number.isFinite(n)).toBe(true);
      expect(n).toBeGreaterThanOrEqual(-0.25);
      expect(n).toBeLessThanOrEqual(4.25);
    }
  }
});

test('send clears the ops window', async ({ page }) => {
  await page.goto('/ui/');
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator('#status')).toContainText('connected');

  await page.locator('#ops').fill('add,(0,0,0),(4,4,4)');
  await page.locator('#send').click();
  await expect(page.locator('#ops')).toHaveValue('');
});

test('presence separates this session from all connected', async ({ browser }) => {
  const alice = await browser.newPage();
  await alice.goto('/ui/');
  await alice.waitForURL(/[?&]s=/);

  const bob = await browser.newPage();
  await bob.goto('/ui/'); // a different session than alice's
  await bob.waitForURL(/[?&]s=/);

  // Each tab is alone in its own session ("1 here"), while the global total
  // counts at least the two of them (plus any still-connected test clients).
  await expect(alice.locator('#presence')).toContainText('1 here');
  await expect(bob.locator('#presence')).toContainText('1 here');

  const connected = async (page) => {
    const text = await page.locator('#presence').textContent();
    const m = text.match(/(\d+) connected/);
    return m ? Number(m[1]) : 0;
  };
  await expect.poll(() => connected(alice)).toBeGreaterThanOrEqual(2);
  await expect.poll(() => connected(bob)).toBeGreaterThanOrEqual(2);
});
