import { test, expect } from '@playwright/test';

const TOUR_DONE_KEY = 'eventfulranges.tour.done.v1';

test.beforeEach(async ({ page }) => {
  // The onboarding tour would otherwise appear on every fresh context and
  // cover the controls the other tests click, so mark it seen up front. The
  // tour tests below use a clean browser context or the force path instead.
  await page.addInitScript((key) => localStorage.setItem(key, '1'), TOUR_DONE_KEY);
});

test('shows the onboarding tutorial on first visit, then hides it after skip', async ({ browser }) => {
  const page = await browser.newPage();
  await page.goto('/ui/');
  await expect(page.locator('#eventfulranges-tour')).toBeVisible();

  await page.locator('#eventfulranges-tour button[data-tour="skip"]').click();
  await expect(page.locator('#eventfulranges-tour')).toHaveCount(0);

  await page.reload();
  await expect(page.locator('#eventfulranges-tour')).toHaveCount(0);
  await page.close();
});

test('reopens the tutorial from the help button', async ({ page }) => {
  await page.goto('/ui/');
  await page.locator('#helpTourBtn').click();
  await expect(page.locator('#eventfulranges-tour')).toBeVisible();
});

test('back skips a hidden step in the tutorial', async ({ page }) => {
  await page.goto('/ui/');
  await page.locator('#helpTourBtn').click();
  await expect(page.locator('#eventfulranges-tour')).toBeVisible();

  const title = page.locator('.eventfulranges-tour-title');
  const next = page.locator('#eventfulranges-tour button[data-tour="next"]');

  // Advance to Fit view; the 4th-dimension step is hidden in a 3D session.
  for (let i = 0; i < 10; i++) {
    if ((await title.textContent()) === 'Fit view') break;
    await next.click();
  }
  await expect(title).toHaveText('Fit view');

  // Back must land on Random op, skipping the hidden 4th-dimension step
  // rather than re-rendering Fit view.
  await page.locator('#eventfulranges-tour button[data-tour="back"]').click();
  await expect(title).toHaveText('Random op');
});

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
  await page.locator('#newSession').click();
  await page.locator('#dims').selectOption('4');
  await expect(page.locator('#slice')).toBeHidden();
});

test('a new session keeps its chosen dimension across reload', async ({ page }) => {
  await page.goto('/ui/');
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator('#status')).toContainText('connected');

  await page.locator('#newSession').click();
  await page.locator('#dims').selectOption('4');
  await page.locator('#startSession').click();
  await expect(page.locator('#slice')).toBeVisible(); // the new session is 4D

  await page.reload();
  await expect(page.locator('#status')).toContainText('connected');
  await page.locator('#newSession').click();
  await expect(page.locator('#dims')).toHaveValue('4');
  await expect(page.locator('#slice')).toBeVisible();
});

test('4D animate sweeps the w slice', async ({ page }) => {
  await page.goto('/ui/');
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator('#status')).toContainText('connected');

  await page.locator('#newSession').click();
  await page.locator('#dims').selectOption('4');
  await page.locator('#startSession').click();
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

  await page.locator('#newSession').click();
  await page.locator('#dims').selectOption('4');
  await page.locator('#startSession').click();
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

// The served build promises the opposite of the browser-only one: here the link
// is what puts several people on one model.
test('says the link is shared through the hub', async ({ page }) => {
  await page.goto('/ui/');
  await page.waitForURL(/[?&]s=/);

  const note = await page.locator('#sessionNote').textContent();
  expect(note).toContain('Go hub');
  expect(note).toContain('everyone who opens this link');
});

test('presence separates this session from all connected', async ({ browser }) => {
  const alice = await browser.newPage();
  await alice.goto('/ui/');
  await alice.waitForURL(/[?&]s=/);

  const bob = await browser.newPage();
  await bob.goto('/ui/'); // a different session than alice's
  await bob.waitForURL(/[?&]s=/);

  // The separation is the point of the test: two tabs, two sessions.
  const sessionOf = (page) => new URL(page.url()).searchParams.get('s');
  expect(sessionOf(alice)).not.toBe(sessionOf(bob));

  // Each tab is alone in its own session ("1 here"), while the global total
  // counts at least the two of them (plus any still-connected test clients).
  // Polling rather than a one-shot assertion: a tab that reconnects is briefly
  // counted twice, and the count settles back once the stale socket is reaped.
  const here = async (page) => {
    const text = (await page.locator('#presence').textContent()) ?? '';
    const m = text.match(/(\d+) here/);
    return m ? Number(m[1]) : null;
  };
  await expect.poll(() => here(alice), { timeout: 15_000 }).toBe(1);
  await expect.poll(() => here(bob), { timeout: 15_000 }).toBe(1);

  const connected = async (page) => {
    const text = await page.locator('#presence').textContent();
    const m = text.match(/(\d+) connected/);
    return m ? Number(m[1]) : 0;
  };
  await expect.poll(() => connected(alice)).toBeGreaterThanOrEqual(2);
  await expect.poll(() => connected(bob)).toBeGreaterThanOrEqual(2);
});

test('merge compaction merges adjacent boxes', async ({ page }) => {
  // A 2D merge session joins the two touching boxes into one. Fixing the
  // dimension first (dims=2) is what lets 2D tuples reach the engine.
  await page.goto('/ui/?dims=2&compact=merge');
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator('#status')).toContainText('connected');

  await page.locator('#ops').fill('add,(0,0),(2,4)\nadd,(2,0),(4,4)');
  await page.locator('#send').click();

  // Two adjacent boxes merge into one under merge compaction.
  await expect(async () => {
    const lines = (await page.locator('#result').inputValue()).trim().split('\n');
    expect(lines.filter(Boolean)).toHaveLength(1);
  }).toPass({ timeout: 10_000 });

  // Two operations in, one range out: what the join saved.
  await expect(page.locator('#stats')).toHaveText('2 ops → 1 range');
});

test('partition compaction splits overlaps into disjoint boxes', async ({ page }) => {
  await page.goto('/ui/?dims=2&compact=partition');
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator('#status')).toContainText('connected');

  // Two corner-overlapping boxes: the later one wins where they overlap, and
  // the partition splits what is left into four disjoint rectangles.
  await page.locator('#ops').fill('add,(0,0),(2,2)\nadd,(1,1),(3,3)');
  await page.locator('#send').click();

  await expect(async () => {
    const lines = (await page.locator('#result').inputValue()).trim().split('\n');
    expect(lines.filter(Boolean)).toHaveLength(4);
  }).toPass({ timeout: 10_000 });

  // The cells are the result here, so the readout names the two ends: two
  // operations in, four boxes out.
  await expect(page.locator('#stats')).toHaveText('2 ops → 4 ranges');
});

// The newest operation at a point decides it, so a hole is only a hole until
// something is added over it: cutting a band out of a square and adding it back
// leaves one range, not two.
test('an add paints over a removed hole', async ({ page }) => {
  await page.goto('/ui/?dims=2&compact=merge');
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator('#status')).toContainText('connected');

  await page.locator('#ops').fill('add,(0,0),(4,4)\nremove,(1,0),(3,4)');
  await page.locator('#send').click();
  await expect(page.locator('#stats'), 'the cut leaves the two sides')
    .toHaveText('2 ops → 2 ranges', { timeout: 10_000 });

  await page.locator('#ops').fill('add,(1,0),(3,4)');
  await page.locator('#send').click();
  await expect(page.locator('#stats'), 'the square is whole again')
    .toHaveText('3 ops → 1 range', { timeout: 10_000 });
});

test('partition + merge combines after splitting', async ({ page }) => {
  await page.goto('/ui/?dims=2&compact=partition-merge');
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator('#status')).toContainText('connected');

  // Two corner-overlapping boxes split, then rejoin, into three rectangles.
  await page.locator('#ops').fill('add,(0,0),(2,2)\nadd,(1,1),(3,3)');
  await page.locator('#send').click();

  await expect(async () => {
    const lines = (await page.locator('#result').inputValue()).trim().split('\n');
    expect(lines.filter(Boolean)).toHaveLength(3);
  }).toPass({ timeout: 10_000 });

  // The only mode that shows all three counts: two operations cut into four
  // cells, joined back into three ranges.
  await expect(page.locator('#stats')).toHaveText('2 ops → 4 partition cells → 3 ranges');
});

// The consolidation a session runs is chosen when it starts, and the boxes
// alone do not reveal it, so the view names it.
test('names the session compaction in the view', async ({ page }) => {
  await page.goto('/ui/?dims=2&compact=partition-merge');
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator('#mode')).toHaveText('Compaction: partition + merge — split, then joined');

  await page.goto('/ui/?dims=2&compact=canonical');
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator('#mode')).toHaveText('Compaction: canonical — every box kept');
});

// The readout is the whole story of a session's consolidation: the operations it
// was given, the partition cells those became, and the ranges on screen now.
test('reports the operations, the cells they became, and the ranges on screen', async ({ page }) => {
  await page.goto('/ui/?dims=3&compact=partition-merge');
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator('#status')).toContainText('connected');
  await page.locator('#example').click();

  await expect(async () => {
    const stats = await page.locator('#stats').textContent();
    const ranges = (await page.locator('#result').inputValue()).trim().split('\n').filter(Boolean).length;
    const shown = stats.match(/^(\d+) ops → (\d+) partition cells → (\d+) ranges$/);
    expect(shown, `readout was ${JSON.stringify(stats)}`).not.toBeNull();
    expect(Number(shown[1]), 'the operations the session was given').toBe(28);
    expect(Number(shown[2]), 'the cells were consolidated into fewer ranges').toBeGreaterThan(ranges);
    expect(Number(shown[3]), 'the readout agrees with the model on screen').toBe(ranges);
  }).toPass({ timeout: 20_000 });
});

test('a session with no partition reports its operations and its ranges', async ({ page }) => {
  await page.goto('/ui/?dims=2&compact=canonical');
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator('#status')).toContainText('connected');
  await page.locator('#ops').fill('add,(0,0),(2,4)\nadd,(2,0),(4,4)');
  await page.locator('#send').click();

  await expect(page.locator('#stats')).toHaveText('2 ops → 2 ranges', { timeout: 10_000 });
});

test('remembers the new-session selections across reloads', async ({ page }) => {
  await page.goto('/ui/');
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator('#status')).toContainText('connected');

  await page.locator('#newSession').click();
  await page.locator('#dims').selectOption('4');
  await page.locator('#compact').selectOption('partition-merge');
  await page.locator('#startSession').click();
  await expect(page.locator('#slice')).toBeVisible();

  await page.reload();
  await expect(page.locator('#status')).toContainText('connected');
  await page.locator('#newSession').click();
  await expect(page.locator('#dims')).toHaveValue('4');
  await expect(page.locator('#compact')).toHaveValue('partition-merge');
});

test('fit view frames material far from the origin', async ({ page }) => {
  await page.goto('/ui/?dims=2');
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator('#status')).toContainText('connected');

  // A box far from the origin: the orthographic fit must centre the camera on
  // the material, not offset the frustum by its centre (which clips it away).
  await page.locator('#ops').fill('add,(100,100),(104,104)');
  await page.locator('#send').click();
  await expect(page.locator('#result')).not.toHaveValue('');
  await page.waitForTimeout(250); // let the fit and a render frame settle

  const withBox = await page.locator('#canvas canvas').screenshot();

  // Removing the same box leaves the camera where the fit put it (empty views
  // do not refit), so the only difference between these two frames is the box.
  await page.locator('#ops').fill('remove,(100,100),(104,104)');
  await page.locator('#send').click();
  await expect.poll(() => page.locator('#result').inputValue()).toBe('');
  await page.waitForTimeout(250);

  const withoutBox = await page.locator('#canvas canvas').screenshot();

  expect(Buffer.compare(withBox, withoutBox)).not.toBe(0);
});

test('detects disconnection and freezes edits', async ({ page }) => {
  await page.goto('/ui/');
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator('#status')).toContainText('connected');

  // Drop the connection as if the server vanished: the banner appears and the
  // mutation controls freeze, while local actions stay available.
  await page.evaluate(() => window.__eventfulranges.closeSocket());
  await expect(page.locator('#reconnectBanner')).toBeVisible();
  await expect(page.locator('#send')).toBeDisabled();
  await expect(page.locator('#copyLink')).toBeEnabled();
});

test('reconnect button reconnects immediately', async ({ page }) => {
  await page.goto('/ui/');
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator('#status')).toContainText('connected');

  await page.evaluate(() => window.__eventfulranges.closeSocket());
  await expect(page.locator('#reconnectBanner')).toBeVisible();

  // The tab also retries on its own backoff, so the banner can clear before the
  // click lands. Race the two legitimate paths — the button and the automatic
  // retry — and let the assertions below decide whether the tab came back. The
  // click's rejection is ignored only when it lost that race, which the banner
  // being hidden already proves.
  await Promise.race([
    page.locator('#reconnectBtn').click().catch(() => {}),
    page.locator('#reconnectBanner').waitFor({ state: 'hidden', timeout: 10_000 }),
  ]);

  await expect(page.locator('#status')).toContainText('connected');
  await expect(page.locator('#reconnectBanner')).toBeHidden();
});

test('keeps a local reserve copy of the log in the browser', async ({ page }) => {
  await page.goto('/ui/');
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator('#status')).toContainText('connected');

  await page.locator('#ops').fill('add,(0,0,0),(4,4,4)');
  await page.locator('#send').click();
  await expect(page.locator('#result')).not.toHaveValue('');

  const reserve = await page.evaluate(() => {
    const session = new URLSearchParams(location.search).get('s');
    const raw = localStorage.getItem(`eventfulranges:web:${session}`);
    return raw ? JSON.parse(raw) : [];
  });
  expect(reserve.length).toBeGreaterThan(0);
  expect(reserve[0].kind).toBe('add');
});
