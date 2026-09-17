import { test, expect } from '@playwright/test';

const TOUR_DONE_KEY = 'eventfulranges.tour.done.v1';

test.beforeEach(async ({ page }) => {
  // Suppress the onboarding tour so it never covers controls these tests
  // click; the tour itself is covered by the server suite's tour tests.
  await page.addInitScript((key) => localStorage.setItem(key, '1'), TOUR_DONE_KEY);
});

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

test('canonical compaction keeps every tile', async ({ page }) => {
  // The default mode is the reference the others are compared against: it
  // neither joins touching boxes nor splits overlaps, so two boxes stay two.
  await page.goto('/?dims=2&compact=canonical');
  await expect(page.locator('#status')).toContainText('running in this page', { timeout: 20_000 });
  await page.locator('#ops').fill('add,(0,0),(2,4)\nadd,(2,0),(4,4)');
  await page.locator('#send').click();

  await expect(async () => {
    const lines = (await page.locator('#result').inputValue()).trim().split('\n');
    expect(lines.filter(Boolean)).toHaveLength(2);
  }).toPass({ timeout: 10_000 });

  await expect(page.locator('#stats')).toHaveText('2 ops → 2 ranges');
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

  // Two operations in, one range out: what the join saved.
  await expect(page.locator('#stats')).toHaveText('2 ops → 1 range');
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

  // The cells are the result here, so the readout names the two ends: two
  // operations in, five boxes out.
  await expect(page.locator('#stats')).toHaveText('2 ops → 5 ranges');
});

test('partition + merge combines after splitting', async ({ page }) => {
  await page.goto('/?dims=2&compact=partition-merge');
  await expect(page.locator('#status')).toContainText('running in this page', { timeout: 20_000 });

  // Two corner-overlapping boxes split, then rejoin, into three rectangles.
  await page.locator('#ops').fill('add,(0,0),(2,2)\nadd,(1,1),(3,3)');
  await page.locator('#send').click();

  await expect(async () => {
    const lines = (await page.locator('#result').inputValue()).trim().split('\n');
    expect(lines.filter(Boolean)).toHaveLength(3);
  }).toPass({ timeout: 10_000 });

  // The only mode that shows all three counts: two operations cut into five
  // cells, joined back into three ranges.
  await expect(page.locator('#stats')).toHaveText('2 ops → 5 partition cells → 3 ranges');
});

// The consolidation a session runs is chosen when it starts, and the boxes
// alone do not reveal it, so the view names it.
test('names the session compaction in the view', async ({ page }) => {
  const cases = [
    ['canonical', 'Compaction: canonical — every box kept'],
    ['merge', 'Compaction: merge adjacent — touching boxes joined'],
    ['partition', 'Compaction: partition — overlaps split apart'],
    ['partition-merge', 'Compaction: partition + merge — split, then joined'],
  ];

  for (const [mode, label] of cases) {
    await page.goto(`/?dims=2&compact=${mode}`);
    await expect(page.locator('#status')).toContainText('running in this page', { timeout: 20_000 });
    await expect(page.locator('#mode'), `a compact=${mode} session`).toHaveText(label);
  }
});

test('names the compaction a session was started with', async ({ page }) => {
  await connect(page);
  await page.locator('#newSession').click();
  await page.locator('#compact').selectOption('partition');
  await page.locator('#startSession').click();

  await expect(page.locator('#mode')).toHaveText('Compaction: partition — overlaps split apart');
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

// The readout is the whole story of a session's consolidation: the operations it
// was given, the partition cells those became, and the ranges on screen now.
test('reports the operations, the cells they became, and the ranges on screen', async ({ page }) => {
  await page.goto('/?dims=3&compact=partition-merge');
  await expect(page.locator('#status')).toContainText('running in this page', { timeout: 20_000 });
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
  await page.goto('/?dims=2&compact=canonical');
  await expect(page.locator('#status')).toContainText('running in this page', { timeout: 20_000 });
  await page.locator('#ops').fill('add,(0,0),(2,4)\nadd,(2,0),(4,4)');
  await page.locator('#send').click();

  await expect(page.locator('#stats')).toHaveText('2 ops → 2 ranges', { timeout: 10_000 });
});

// A 3D partition-merge session is the expensive case: every operation
// re-partitions and re-merges the whole cover. The engine runs in a worker
// precisely so that this happens without the page — canvas, buttons and the
// "merging" feedback — seizing up while it does.
test('a heavy 3D merge runs off the main thread and reports the wait', async ({ page }) => {
  await page.goto('/?dims=3&compact=partition-merge');
  await expect(page.locator('#status')).toContainText('running in this page', { timeout: 20_000 });

  // The built-in example first: 27 tiles with the middle one carved out. Its
  // operations land asynchronously, so wait for the engine to go idle — the
  // result panel fills in, and the send button comes back when the fold is
  // over.
  await page.locator('#example').click();
  await expect(page.locator('#result')).not.toHaveValue('', { timeout: 30_000 });
  await expect(page.locator('#send')).toBeEnabled({ timeout: 30_000 });

  await page.locator('#ops').fill(MIXED_3D_OPS);
  const merge = await page.evaluate(async (opCount) => {
    const logLines = () => document.querySelectorAll('#log li').length;
    const before = logLines();
    const busy = document.getElementById('busy');
    const send = document.getElementById('send');

    let ticks = 0;
    let sawFeedback = false;
    const tick = () => {
      ticks++;
      if (!busy.hidden) sawFeedback = true;
      requestAnimationFrame(tick);
    };
    requestAnimationFrame(tick);

    send.click();
    await new Promise((done) => {
      const watch = () => {
        if (!busy.hidden) sawFeedback = true;
        if (logLines() >= before + opCount && busy.hidden) done();
        else requestAnimationFrame(watch);
      };
      requestAnimationFrame(watch);
    });
    return { ticks, sawFeedback, sendEnabled: !send.disabled };
  }, MIXED_3D_OPS.split('\n').length);

  expect(merge.ticks, 'the page keeps painting while the engine folds').toBeGreaterThan(3);
  expect(merge.sawFeedback, 'the wait is shown while the engine works').toBe(true);
  expect(merge.sendEnabled, 'the send button is released once the fold is done').toBe(true);

  await expect(async () => {
    const lines = (await page.locator('#result').inputValue()).trim().split('\n');
    expect(lines.filter(Boolean).length).toBeGreaterThan(0);
  }).toPass({ timeout: 10_000 });
});

// MIXED_3D_OPS edits the example's hollow shell the way a viewer's random ops
// do: overlapping adds and removes that force the partition to split and the
// merge to rejoin. The lattice at the end is what makes the batch heavy — 24
// more folds, each over a cover the previous one grew — so the busy window
// spans many frames on any machine, however fast a single fold is.
const MIXED_3D_OPS = [
  'remove,(1.1,1.1,1.1),(2.4,2.4,2.4)',
  'add,(0.2,0.2,0.2),(2.6,1.4,2.6)',
  'remove,(0.4,0.4,0.4),(1.6,1.6,1.6)',
  'add,(1.3,0.6,0.6),(2.9,2.7,2.7)',
  'remove,(0.5,1.2,0.9),(2.2,2.2,1.9)',
  'add,(0.9,0.9,0.9),(2.1,2.1,2.1)',
  'remove,(1.5,0.3,0.3),(2.8,1.1,1.1)',
  'add,(0.3,1.3,1.3),(1.8,2.8,2.6)',
  ...Array.from({ length: 24 }, (_, i) => {
    const lo = [0, 2, 4].map((offset) => (((i + offset) % 6) * 0.4).toFixed(2));
    const hi = lo.map((value) => (Number(value) + 0.9).toFixed(2));
    return `add,(${lo.join(',')}),(${hi.join(',')})`;
  }),
].join('\n');
