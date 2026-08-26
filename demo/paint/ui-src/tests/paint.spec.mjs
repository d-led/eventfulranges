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

// drawRect drags a 4x4 cell rectangle starting from the board centre, which is
// cell (0,0) at the initial camera (scale 12 px per cell).
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
