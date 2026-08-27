import { test, expect } from "@playwright/test";
import { readFile } from "node:fs/promises";

test("serves the UI, connects, and renders the board", async ({ page }) => {
  await page.goto("/ui/");
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator("#status")).toContainText("connected");
  await expect(page.locator("#board")).toBeAttached();
});

test("two tabs converge on the same log", async ({ browser }) => {
  const alice = await browser.newPage();
  await alice.goto("/ui/");
  await alice.waitForURL(/[?&]s=/);

  const bob = await browser.newPage();
  await bob.goto(alice.url());
  await expect(alice.locator("#status")).toContainText("connected");
  await expect(bob.locator("#status")).toContainText("connected");

  await drawRect(alice); // a 4x4 block of cells

  await expect(alice.locator("#log li.add").first()).toContainText("add");
  await expect(bob.locator("#log li.add").first()).toContainText("add");
});

test("downloads the operation log as JSONL", async ({ page }) => {
  await page.goto("/ui/");
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator("#status")).toContainText("connected");

  await drawRect(page);
  await expect(page.locator("#log li.add").first()).toContainText("add");

  const [download] = await Promise.all([
    page.waitForEvent("download"),
    page.locator("#downloadJsonl").click(),
  ]);

  const content = await readFile(await download.path(), "utf8");
  const lines = content.trim().split("\n");
  expect(lines.length).toBeGreaterThan(0);
  const first = JSON.parse(lines[0]);
  expect(first.kind).toBe("add");
  expect(first.data).toHaveProperty("min");
  expect(first.data).toHaveProperty("max");
});

test("presence separates this session from all connected", async ({
  browser,
}) => {
  const alice = await browser.newPage();
  await alice.goto("/ui/");
  await alice.waitForURL(/[?&]s=/);

  const bob = await browser.newPage();
  await bob.goto("/ui/");
  await bob.waitForURL(/[?&]s=/);

  await expect(alice.locator("#presence")).toContainText("1 here");
  await expect(bob.locator("#presence")).toContainText("1 here");

  const connected = async (page) => {
    const text = await page.locator("#presence").textContent();
    const m = text.match(/(\d+) connected/);
    return m ? Number(m[1]) : 0;
  };
  await expect.poll(() => connected(alice)).toBeGreaterThanOrEqual(2);
  await expect.poll(() => connected(bob)).toBeGreaterThanOrEqual(2);
});

test("grid controls shift the subdivision level", async ({ page }) => {
  await page.goto("/ui/");
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator("#status")).toContainText("connected");

  const levelOf = async () => {
    const text = await page.locator("#gridLabel").textContent();
    const m = text.match(/level (\d+)/);
    return m ? Number(m[1]) : NaN;
  };

  const before = await levelOf();
  await page.locator("#gridPlus").click();
  await expect.poll(() => levelOf()).toBe(before + 1);
  await page.locator("#gridDefault").click();
  await expect.poll(() => levelOf()).toBe(before);
});

test("pen paints cells along a sweep", async ({ page }) => {
  await page.goto("/ui/");
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator("#status")).toContainText("connected");

  await page.locator("#toolPen").click();
  const canvas = page.locator("#board");
  const box = await canvas.boundingBox();
  const cx = box.x + box.width / 2;
  const cy = box.y + box.height / 2;
  await page.mouse.move(cx, cy);
  await page.mouse.down();
  await page.mouse.move(cx + 24, cy, { steps: 3 });
  await page.mouse.up();

  // The swept cells arrive as a burst and collapse into one summary line.
  await expect(page.locator("#log li.summary").first()).toContainText("add");
});

test("a stroke carries its color as metadata", async ({ page }) => {
  await page.goto("/ui/");
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator("#status")).toContainText("connected");

  await page.locator("#strokeColor").evaluate((el) => {
    el.value = "#ff5500";
    el.dispatchEvent(new Event("input", { bubbles: true }));
  });
  await drawRect(page);
  await expect(page.locator("#log li.add").first()).toContainText("add");

  const [download] = await Promise.all([
    page.waitForEvent("download"),
    page.locator("#downloadJsonl").click(),
  ]);
  const content = await readFile(await download.path(), "utf8");
  const first = JSON.parse(content.trim().split("\n")[0]);
  expect(first.meta).toEqual({ color: "#ff5500" });
});

test("imports a JSONL log onto the board", async ({ page }) => {
  await page.goto("/ui/");
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator("#status")).toContainText("connected");

  await page.locator("#importJsonlInput").setInputFiles({
    name: "board.jsonl",
    mimeType: "application/x-ndjson",
    buffer: Buffer.from(
      JSON.stringify({
        kind: "add",
        data: { min: [0, 0], max: [2, 2] },
        meta: { color: "#00ff00" },
      }) + "\n",
    ),
  });

  await expect(page.locator("#log li.add").first()).toContainText("add");
});

test("coalesces a burst of ops into one summary line", async ({ page }) => {
  await page.goto("/ui/");
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator("#status")).toContainText("connected");

  const lines = Array.from({ length: 5 }, (_, i) =>
    JSON.stringify({
      kind: "add",
      data: { min: [i, 0], max: [i + 1, 1] },
      meta: { color: "#00ff00" },
    }),
  ).join("\n");
  await page.locator("#importJsonlInput").setInputFiles({
    name: "burst.jsonl",
    mimeType: "application/x-ndjson",
    buffer: Buffer.from(lines + "\n"),
  });

  await expect(page.locator("#log li.summary").first()).toContainText("add ×5");
  await expect(page.locator("#log li.add")).toHaveCount(0);
});

test("grid offset snaps painting to the chosen level", async ({ page }) => {
  await page.goto("/ui/");
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator("#status")).toContainText("connected");

  await page.locator("#gridPlus").click(); // level 1 → half-unit cells
  await expect(page.locator("#gridLabel")).toContainText("level 1");

  // A single click paints one cell of the current grid.
  const canvas = page.locator("#board");
  const box = await canvas.boundingBox();
  await page.mouse.click(box.x + box.width / 2 + 3, box.y + box.height / 2 + 3);
  await expect(page.locator("#log li.add").first()).toContainText("add");

  const [download] = await Promise.all([
    page.waitForEvent("download"),
    page.locator("#downloadJsonl").click(),
  ]);
  const content = await readFile(await download.path(), "utf8");
  const first = JSON.parse(content.trim().split("\n")[0]);
  expect(first.data.min).toEqual([0, 0]);
  expect(first.data.max).toEqual([0.5, 0.5]);
});

test("grid plus to level three still paints at the fine level", async ({
  page,
}) => {
  await page.goto("/ui/");
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator("#status")).toContainText("connected");

  await page.locator("#gridPlus").click();
  await page.locator("#gridPlus").click();
  await page.locator("#gridPlus").click(); // level 3 → eighth-unit cells
  await expect(page.locator("#gridLabel")).toContainText("level 3");

  const canvas = page.locator("#board");
  const box = await canvas.boundingBox();
  await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
  await expect(page.locator("#log li.add").first()).toContainText("add");

  const [download] = await Promise.all([
    page.waitForEvent("download"),
    page.locator("#downloadJsonl").click(),
  ]);
  const content = await readFile(await download.path(), "utf8");
  const first = JSON.parse(content.trim().split("\n")[0]);
  expect(first.data.min).toEqual([0, 0]);
  expect(first.data.max).toEqual([0.125, 0.125]);
});

test("grid minus coarsens painting beyond the automatic level", async ({
  page,
}) => {
  await page.goto("/ui/");
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator("#status")).toContainText("connected");

  await page.locator("#gridMinus").click(); // level -1 → two-unit cells
  await expect(page.locator("#gridLabel")).toContainText("level -1");

  const canvas = page.locator("#board");
  const box = await canvas.boundingBox();
  await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
  await expect(page.locator("#log li.add").first()).toContainText("add");

  const [download] = await Promise.all([
    page.waitForEvent("download"),
    page.locator("#downloadJsonl").click(),
  ]);
  const content = await readFile(await download.path(), "utf8");
  const first = JSON.parse(content.trim().split("\n")[0]);
  expect(first.data.min).toEqual([0, 0]);
  expect(first.data.max).toEqual([2, 2]);
});

test("import fits the loaded picture into view", async ({ page }) => {
  await page.goto("/ui/");
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator("#status")).toContainText("connected");
  await expect(page.locator("#gridLabel")).toContainText("level 0");

  await page.locator("#importJsonlInput").setInputFiles({
    name: "far.jsonl",
    mimeType: "application/x-ndjson",
    buffer: Buffer.from(
      JSON.stringify({
        kind: "add",
        data: { min: [1000, 1000], max: [1002, 1002] },
      }) + "\n",
    ),
  });

  await expect(page.locator("#log li.add").first()).toContainText("add");
  await expect(page.locator("#gridLabel")).not.toContainText("level 0");
});

test("round-trips a picture through JSONL export and import", async ({
  browser,
}) => {
  const alice = await browser.newPage();
  await alice.goto("/ui/");
  await alice.waitForURL(/[?&]s=/);
  await expect(alice.locator("#status")).toContainText("connected");

  await drawRect(alice);
  await expect(alice.locator("#log li.add").first()).toContainText("add");

  const [download] = await Promise.all([
    alice.waitForEvent("download"),
    alice.locator("#downloadJsonl").click(),
  ]);
  const content = await readFile(await download.path(), "utf8");

  const bob = await browser.newPage();
  await bob.goto("/ui/");
  await bob.waitForURL(/[?&]s=/);
  await expect(bob.locator("#status")).toContainText("connected");

  await bob.locator("#importJsonlInput").setInputFiles({
    name: "board.jsonl",
    mimeType: "application/x-ndjson",
    buffer: Buffer.from(content),
  });

  await expect(bob.locator("#log li.add").first()).toContainText("add");
  await expect(bob.locator("#log li.add")).toHaveCount(1);
});

test("freezes mutations while disconnected but keeps local download", async ({
  page,
}) => {
  await page.goto("/ui/");
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator("#status")).toContainText("connected");

  await drawRect(page); // paint while connected so the log has a real entry
  await expect(page.locator("#log li.add").first()).toContainText("add");

  // Drop the connection as if the server vanished: the board freezes, mutation
  // tools disable, and the local JSONL export stays available.
  await page.evaluate(() => window.__eventfulranges.closeSocket());
  await expect(page.locator("#reconnectBanner")).toBeVisible();
  await expect(page.locator("#toolRect")).toBeDisabled();
  await expect(page.locator("#downloadJsonl")).toBeEnabled();

  const [download] = await Promise.all([
    page.waitForEvent("download"),
    page.locator("#downloadJsonl").click(),
  ]);
  const content = await readFile(await download.path(), "utf8");
  const first = JSON.parse(content.trim().split("\n")[0]);
  expect(first.kind).toBe("add");
});

test("reconnect button reconnects immediately", async ({ page }) => {
  await page.goto("/ui/");
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator("#status")).toContainText("connected");

  await page.evaluate(() => window.__eventfulranges.closeSocket());
  await expect(page.locator("#reconnectBanner")).toBeVisible();

  await page.locator("#reconnectBtn").click();
  await expect(page.locator("#status")).toContainText("connected");
  await expect(page.locator("#reconnectBanner")).toBeHidden();
});

test("eraser removes one cell like the pen", async ({ page }) => {
  await page.goto("/ui/");
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator("#status")).toContainText("connected");

  await page.locator("#toolEraser").click();
  const canvas = page.locator("#board");
  const box = await canvas.boundingBox();
  const cx = box.x + box.width / 2;
  const cy = box.y + box.height / 2;
  await page.mouse.move(cx, cy);
  await page.mouse.down();
  await page.mouse.up();

  // A single click stamps exactly one cell, the same footprint as the pen.
  await expect(page.locator("#log li.remove").first()).toContainText("remove");
  await expect(page.locator("#log li.remove")).toHaveCount(1);
});

test("zoom buttons zoom in, out, and reset", async ({ page }) => {
  await page.goto("/ui/");
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator("#status")).toContainText("connected");

  await expect(page.locator("#zoomLabel")).toHaveText("100%");
  await page.locator("#zoomIn").click();
  await expect(page.locator("#zoomLabel")).toHaveText("200%");
  await page.locator("#zoomOut").click();
  await expect(page.locator("#zoomLabel")).toHaveText("100%");
  await page.locator("#zoomIn").click();
  await page.locator("#zoomIn").click(); // 400%
  await expect(page.locator("#zoomLabel")).toHaveText("400%");
  await page.locator("#zoomReset").click();
  await expect(page.locator("#zoomLabel")).toHaveText("100%");
});

test("right-drag pans without drawing", async ({ page }) => {
  await page.goto("/ui/");
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator("#status")).toContainText("connected");

  const canvas = page.locator("#board");
  const box = await canvas.boundingBox();
  const cx = box.x + box.width / 2;
  const cy = box.y + box.height / 2;
  await page.mouse.move(cx, cy);
  await page.mouse.down({ button: "right" });
  await page.mouse.move(cx + 40, cy + 40, { steps: 3 });
  await page.mouse.up({ button: "right" });

  // A right-drag pans the camera: nothing is painted.
  await expect(page.locator("#log li")).toHaveCount(0);
});

test("draws with a single touch on touch devices", async ({ browser }) => {
  const context = await browser.newContext({ hasTouch: true });
  const page = await context.newPage();
  await page.goto("/ui/");
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator("#status")).toContainText("connected");

  await page.locator("#toolPen").click();
  const canvas = page.locator("#board");
  const box = await canvas.boundingBox();
  await page.touchscreen.tap(box.x + box.width / 2, box.y + box.height / 2);

  await expect(page.locator("#log li.add").first()).toContainText("add");
  await context.close();
});

test("keeps a local reserve copy of the log in the browser", async ({
  page,
}) => {
  await page.goto("/ui/");
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator("#status")).toContainText("connected");

  await drawRect(page);
  await expect(page.locator("#log li.add").first()).toContainText("add");

  const reserve = await page.evaluate(() => {
    const session = new URLSearchParams(location.search).get("s");
    const raw = localStorage.getItem(`eventfulranges:paint:${session}`);
    return raw ? JSON.parse(raw) : [];
  });
  expect(reserve.length).toBeGreaterThan(0);
  expect(reserve[0].kind).toBe("add");
});

test("clicking here vs connected filters the roster", async ({ page }) => {
  await page.goto("/ui/");
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator("#status")).toContainText("connected");

  const session = new URL(page.url()).searchParams.get("s");
  await expect(page.locator("#presence")).toContainText(`you are ${session}`);

  // "connected" lists everyone across all sessions, with their session ids.
  await page.locator("#connectedLink").click();
  const list = page.locator("#rosterList");
  await expect(list).toBeVisible();
  await expect(page.locator("#rosterTitle")).toHaveText("all sessions");
  await expect(list).toContainText(session);

  // "here" narrows the same roster to the current session.
  await page.locator("#hereLink").click();
  await expect(page.locator("#rosterTitle")).toHaveText("in this session");
  await expect(list).toContainText(session);
});

test("roster groups one user's sessions together", async ({ context }) => {
  const a = await context.newPage();
  await a.goto("/ui/");
  await a.waitForURL(/[?&]s=/);
  await expect(a.locator("#status")).toContainText("connected");

  const b = await context.newPage();
  await b.goto("/ui/");
  await b.waitForURL(/[?&]s=/);
  await expect(b.locator("#status")).toContainText("connected");

  const idA = new URL(a.url()).searchParams.get("s");
  const idB = new URL(b.url()).searchParams.get("s");

  await a.locator("#connectedLink").click();
  const list = a.locator("#rosterList");
  await expect(list).toBeVisible();
  await expect(list).toContainText(idA);
  await expect(list).toContainText(idB);

  // One user with two sessions appears in a single entry.
  await expect(a.locator("#rosterList li", { hasText: idA })).toHaveCount(1);
  await expect(a.locator("#rosterList li", { hasText: idA })).toContainText(idB);
});

test("undo retracts the last stroke and redo restores it", async ({ page }) => {
  await page.goto("/ui/");
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator("#status")).toContainText("connected");

  await drawRect(page);
  await expect(page.locator("#log li.add").first()).toContainText("add");

  await page.locator("#undoBtn").click();
  await expect(page.locator("#log li.retract").first()).toContainText(
    "retract",
  );

  await page.locator("#redoBtn").click();
  await expect(page.locator("#log li.add").last()).toContainText("add");
});

test("undoing an erase restores the erased cell", async ({ page }) => {
  await page.goto("/ui/");
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator("#status")).toContainText("connected");

  await drawRect(page); // paint a 2x2 block
  await expect(page.locator("#log li.add").first()).toContainText("add");

  await page.locator("#toolEraser").click();
  const canvas = page.locator("#board");
  const box = await canvas.boundingBox();
  await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
  await expect(page.locator("#log li.remove").first()).toContainText("remove");

  await page.locator("#undoBtn").click();
  await expect(page.locator("#log li.retract").first()).toContainText(
    "retract",
  );
});

test("undo retracts only the undoing client's own stroke", async ({ browser }) => {
  const alice = await browser.newPage();
  await alice.goto("/ui/");
  await alice.waitForURL(/[?&]s=/);

  const bob = await browser.newPage();
  await bob.goto(alice.url());
  await expect(alice.locator("#status")).toContainText("connected");
  await expect(bob.locator("#status")).toContainText("connected");

  await drawRect(alice); // alice paints first
  await expect(alice.locator("#log li.add")).toHaveCount(1);
  await drawRect(bob); // bob paints the same cells second
  await expect(alice.locator("#log li.add")).toHaveCount(2);

  await bob.locator("#undoBtn").click();
  await expect(alice.locator("#log li.retract").first()).toContainText(
    "retract",
  );

  // The retract names bob's op, so alice's identical stroke survives.
  const [download] = await Promise.all([
    alice.waitForEvent("download"),
    alice.locator("#downloadJsonl").click(),
  ]);
  const content = await readFile(await download.path(), "utf8");
  const entries = content.trim().split("\n").map((line) => JSON.parse(line));
  const adds = entries.filter((e) => e.kind === "add");
  const retract = entries.find((e) => e.kind === "retract");
  expect(adds).toHaveLength(2);
  expect(retract.ref).toBe(adds[1].id);
  expect(retract.ref).not.toBe(adds[0].id);
});

// drawRect drags a 2x2 cell rectangle starting from the board centre, which is
// cell (0,0) at the initial camera (scale 24 px per cell).
async function drawRect(page) {
  const canvas = page.locator("#board");
  const box = await canvas.boundingBox();
  const cx = box.x + box.width / 2;
  const cy = box.y + box.height / 2;
  await page.mouse.move(cx, cy);
  await page.mouse.down();
  await page.mouse.move(cx + 36, cy + 36);
  await page.mouse.up();
}
