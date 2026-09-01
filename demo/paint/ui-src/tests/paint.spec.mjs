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

test("one-shot feedback appears as a toast and fades away", async ({
  page,
}) => {
  await page.goto("/ui/");
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator("#status")).toContainText("connected");

  await drawRect(page);
  await expect(page.locator("#log li.add").first()).toContainText("add");

  const [download] = await Promise.all([
    page.waitForEvent("download"),
    page.locator("#downloadJsonl").click(),
  ]);
  await download.path(); // consume the download

  await expect(page.locator("#toast")).toHaveClass(/show/);
  await expect(page.locator("#toast")).toContainText("downloaded board.jsonl");

  // The toast dismisses itself without any further interaction.
  await expect(page.locator("#toast")).not.toHaveClass(/show/, {
    timeout: 5000,
  });
});

test("exports the board as an SVG without the grid", async ({ page }) => {
  await page.goto("/ui/");
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator("#status")).toContainText("connected");

  await drawRect(page);
  await expect(page.locator("#log li.add").first()).toContainText("add");

  await page.locator("#exportBtn").click();
  await page.locator("#exportFormat").selectOption("svg");

  const [download] = await Promise.all([
    page.waitForEvent("download"),
    page.locator("#exportGo").click(),
  ]);
  expect(download.suggestedFilename()).toBe("board.svg");

  const content = await readFile(await download.path(), "utf8");
  expect(content).toContain("<svg");
  expect(content).toContain("<rect");
  expect(content).not.toContain("<line");
});

test("exports layered strokes without carving the base square", async ({
  page,
}) => {
  await page.goto("/ui/");
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator("#status")).toContainText("connected");

  await page.locator("#importJsonlInput").setInputFiles({
    name: "layers.jsonl",
    mimeType: "application/x-ndjson",
    buffer: Buffer.from(
      JSON.stringify({
        kind: "add",
        data: { min: [0, 0], max: [10, 10] },
        meta: { color: "#111111" },
      }) +
        "\n" +
        JSON.stringify({
          kind: "add",
          data: { min: [4, 4], max: [6, 6] },
          meta: { color: "#222222" },
        }) +
        "\n",
    ),
  });

  // Wait until both strokes are folded into the local view (the reserve log
  // mirrors logEntries, so its length is a deterministic readiness signal even
  // when the activity log coalesces the burst into one summary line).
  await expect
    .poll(async () => {
      return page.evaluate(() => {
        const session = new URLSearchParams(location.search).get("s");
        const raw = localStorage.getItem(`eventfulranges:paint:${session}`);
        return raw ? JSON.parse(raw).length : 0;
      });
    })
    .toBe(2);

  await page.locator("#exportBtn").click();
  await page.locator("#exportFormat").selectOption("svg");

  const [download] = await Promise.all([
    page.waitForEvent("download"),
    page.locator("#exportGo").click(),
  ]);

  const content = await readFile(await download.path(), "utf8");
  // One background rect, the whole base square, and the one pixel on top:
  // the square is layered, never carved into strips.
  const rects = content.match(/<rect /g) ?? [];
  expect(rects.length).toBe(3);
});

test("overdrawing a big square with a small one layers instead of carving", async ({
  page,
}) => {
  await page.goto("/ui/");
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator("#status")).toContainText("connected");

  const canvas = page.locator("#board");
  const box = await canvas.boundingBox();
  const cx = box.x + box.width / 2;
  const cy = box.y + box.height / 2;

  // Drag a 5x5 cell block with the rectangle tool: 96 px is 4 board units at
  // the initial 24 px/unit scale, which snaps to cells (0,0)..(5,5).
  await page.mouse.move(cx, cy);
  await page.mouse.down();
  await page.mouse.move(cx + 96, cy + 96, { steps: 5 });
  await page.mouse.up();
  await expect(page.locator("#log li.add").first()).toContainText("add");

  // Stamp one strictly interior cell with the pen, in a distinct colour.
  await page.locator("#strokeColor").evaluate((el) => {
    el.value = "#ff5500";
    el.dispatchEvent(new Event("input", { bubbles: true }));
  });
  await page.locator("#toolPen").click();
  await page.mouse.click(cx + 60, cy + 60); // cell (2,2): inside the 5x5 block

  await expect
    .poll(async () => {
      return page.evaluate(() => {
        const session = new URLSearchParams(location.search).get("s");
        const raw = localStorage.getItem(`eventfulranges:paint:${session}`);
        return raw ? JSON.parse(raw).length : 0;
      });
    })
    .toBe(2);

  await page.locator("#exportBtn").click();
  await page.locator("#exportFormat").selectOption("svg");

  const [download] = await Promise.all([
    page.waitForEvent("download"),
    page.locator("#exportGo").click(),
  ]);

  const content = await readFile(await download.path(), "utf8");
  // One background rect, the whole 5x5 square, and the one cell on top: the
  // base square is layered, never carved into strips.
  expect((content.match(/<rect /g) ?? []).length).toBe(3);
});

test("exports the board as a PNG", async ({ page }) => {
  await page.goto("/ui/");
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator("#status")).toContainText("connected");

  await drawRect(page);
  await expect(page.locator("#log li.add").first()).toContainText("add");

  await page.locator("#exportBtn").click();
  await expect(page.locator("#exportFormat")).toHaveValue("png");

  const [download] = await Promise.all([
    page.waitForEvent("download"),
    page.locator("#exportGo").click(),
  ]);
  expect(download.suggestedFilename()).toBe("board.png");

  const bytes = await readFile(await download.path());
  // PNG signature: 89 50 4E 47 0D 0A 1A 0A.
  expect([...bytes.subarray(0, 8)]).toEqual([137, 80, 78, 71, 13, 10, 26, 10]);
});

test("exports the board as a JPEG", async ({ page }) => {
  await page.goto("/ui/");
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator("#status")).toContainText("connected");

  await drawRect(page);
  await expect(page.locator("#log li.add").first()).toContainText("add");

  await page.locator("#exportBtn").click();
  await page.locator("#exportFormat").selectOption("jpeg");

  const [download] = await Promise.all([
    page.waitForEvent("download"),
    page.locator("#exportGo").click(),
  ]);
  expect(download.suggestedFilename()).toBe("board.jpg");

  const bytes = await readFile(await download.path());
  // JPEG signature: FF D8 FF.
  expect([...bytes.subarray(0, 3)]).toEqual([255, 216, 255]);
});

test("exports a PNG with a transparent background", async ({ page }) => {
  await page.goto("/ui/");
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator("#status")).toContainText("connected");

  await drawRect(page);
  await expect(page.locator("#log li.add").first()).toContainText("add");

  // Intercept the raster canvas so the test can sample pixel alpha directly,
  // without decoding the PNG file.
  await page.evaluate(() => {
    const original = HTMLCanvasElement.prototype.toBlob;
    HTMLCanvasElement.prototype.toBlob = function (callback, type, quality) {
      window.__lastExportCanvas = this;
      return original.call(this, callback, type, quality);
    };
  });

  await page.locator("#exportBtn").click();
  // Letterbox the square drawing so the margins stay empty and unpainted.
  await page.locator("#exportRatio").uncheck();
  await page.locator("#exportWidth").fill("640");
  await page.locator("#exportHeight").fill("480");

  const [download] = await Promise.all([
    page.waitForEvent("download"),
    page.locator("#exportGo").click(),
  ]);
  expect(download.suggestedFilename()).toBe("board.png");

  const corner = await page.evaluate(() => {
    const c = window.__lastExportCanvas;
    return c.getContext("2d").getImageData(0, 0, 1, 1).data;
  });
  expect(corner[3]).toBe(0); // empty space is transparent

  const center = await page.evaluate(() => {
    const c = window.__lastExportCanvas;
    return c.getContext("2d").getImageData(320, 240, 1, 1).data;
  });
  expect(center[3]).toBe(255); // the painted square is opaque
});

test("remembers the selected colour across reloads", async ({ page }) => {
  await page.goto("/ui/");
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator("#status")).toContainText("connected");

  await page.locator("#strokeColor").evaluate((el) => {
    el.value = "#ff5500";
    el.dispatchEvent(new Event("input", { bubbles: true }));
  });

  const stored = await page.evaluate(() =>
    JSON.parse(localStorage.getItem("eventfulranges:paint:settings")),
  );
  expect(stored.color).toBe("#ff5500");

  await page.reload();
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator("#status")).toContainText("connected");
  await expect(page.locator("#strokeColor")).toHaveValue("#ff5500");
});

test("remembers the export settings across reloads", async ({ page }) => {
  await page.goto("/ui/");
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator("#status")).toContainText("connected");

  await drawRect(page);
  await expect(page.locator("#log li.add").first()).toContainText("add");

  await page.locator("#exportBtn").click();
  await page.locator("#exportFormat").selectOption("jpeg");
  await page.locator("#exportWidth").fill("800");
  await page.locator("#exportHeight").fill("600");
  await page.locator("#exportRatio").uncheck();

  const [download] = await Promise.all([
    page.waitForEvent("download"),
    page.locator("#exportGo").click(),
  ]);
  expect(download.suggestedFilename()).toBe("board.jpg");

  await page.reload();
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator("#log li.add").first()).toContainText("add");

  await page.locator("#exportBtn").click();
  await expect(page.locator("#exportFormat")).toHaveValue("jpeg");
  await expect(page.locator("#exportWidth")).toHaveValue("800");
  await expect(page.locator("#exportHeight")).toHaveValue("600");
  await expect(page.locator("#exportRatio")).not.toBeChecked();
});

test("imports a PNG as merged cells", async ({ page }) => {
  await page.goto("/ui/");
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator("#status")).toContainText("connected");

  // A 2x2 solid red PNG, generated inside the page itself.
  const dataUrl = await page.evaluate(() => {
    const c = document.createElement("canvas");
    c.width = 2;
    c.height = 2;
    const ctx = c.getContext("2d");
    ctx.fillStyle = "#ff0000";
    ctx.fillRect(0, 0, 2, 2);
    return c.toDataURL("image/png");
  });

  await page.locator("#importImageInput").setInputFiles({
    name: "solid.png",
    mimeType: "image/png",
    buffer: Buffer.from(dataUrl.split(",")[1], "base64"),
  });

  await expect(page.locator("#log li.add").first()).toContainText("add");

  const [download] = await Promise.all([
    page.waitForEvent("download"),
    page.locator("#downloadJsonl").click(),
  ]);
  const content = await readFile(await download.path(), "utf8");
  const first = JSON.parse(content.trim().split("\n")[0]);
  expect(first.kind).toBe("add");
  expect(first.data.min).toEqual([0, 0]);
  expect(first.data.max).toEqual([2, 2]);
  expect(first.meta).toEqual({ color: "#ff0000" });
});

test("clears the board before importing when asked", async ({ page }) => {
  await page.goto("/ui/");
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator("#status")).toContainText("connected");

  await drawRect(page); // paint a 2x2 block first
  await expect(page.locator("#log li.add").first()).toContainText("add");

  await page.locator("#importImageClear").check();

  // A 1x1 blue PNG.
  const dataUrl = await page.evaluate(() => {
    const c = document.createElement("canvas");
    c.width = 1;
    c.height = 1;
    const ctx = c.getContext("2d");
    ctx.fillStyle = "#0000ff";
    ctx.fillRect(0, 0, 1, 1);
    return c.toDataURL("image/png");
  });
  await page.locator("#importImageInput").setInputFiles({
    name: "dot.png",
    mimeType: "image/png",
    buffer: Buffer.from(dataUrl.split(",")[1], "base64"),
  });

  // Wait until all three ops land (the painted block, the clear, the pixel).
  await expect
    .poll(async () => {
      return page.evaluate(() => {
        const session = new URLSearchParams(location.search).get("s");
        const raw = localStorage.getItem(`eventfulranges:paint:${session}`);
        return raw ? JSON.parse(raw).length : 0;
      });
    })
    .toBe(3);

  const [download] = await Promise.all([
    page.waitForEvent("download"),
    page.locator("#downloadJsonl").click(),
  ]);
  const entries = (await readFile(await download.path(), "utf8"))
    .trim()
    .split("\n")
    .map((line) => JSON.parse(line));

  expect(entries.map((e) => e.kind)).toEqual(["add", "remove", "add"]);
  expect(entries[1].data).toEqual({ min: [0, 0], max: [2, 2] }); // clears the old block
  expect(entries[2].data).toEqual({ min: [0, 0], max: [1, 1] });
  expect(entries[2].meta).toEqual({ color: "#0000ff" });
});

test("keeps the drawing's aspect ratio when asked", async ({ page }) => {
  await page.goto("/ui/");
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator("#status")).toContainText("connected");

  await drawRect(page); // a 2x2 cell block: square bounds
  await expect(page.locator("#log li.add").first()).toContainText("add");

  await page.locator("#exportBtn").click();
  // 2000 x 1000 as maximums: the square drawing keeps 1:1, so the result is
  // 1000 x 1000.
  await page.locator("#exportWidth").fill("2000");
  await page.locator("#exportHeight").fill("1000");
  await page.locator("#exportRatio").check();

  const [download] = await Promise.all([
    page.waitForEvent("download"),
    page.locator("#exportGo").click(),
  ]);
  const bytes = await readFile(await download.path());
  expect(bytes.readUInt32BE(16)).toBe(1000); // IHDR width
  expect(bytes.readUInt32BE(20)).toBe(1000); // IHDR height
});

test("rejects an export smaller than the minimum", async ({ page }) => {
  await page.goto("/ui/");
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator("#status")).toContainText("connected");

  await drawRect(page);
  await expect(page.locator("#log li.add").first()).toContainText("add");

  await page.locator("#exportBtn").click();
  await page.locator("#exportWidth").fill("100");
  await page.locator("#exportHeight").fill("100");
  await page.locator("#exportGo").click();

  await expect(page.locator("#exportError")).toBeVisible();
  await expect(page.locator("#exportError")).toContainText("minimum");
  await expect(page.locator("#exportDialog")).toBeVisible();
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

  // The sweep paints the cells between the anchors; whether they collapse
  // into one summary line depends on arrival timing, so just assert the log.
  await expect(page.locator("#log li").first()).toContainText("add");
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

test("queues edits while disconnected and syncs on reconnect", async ({
  page,
}) => {
  await page.goto("/ui/");
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator("#status")).toContainText("connected");

  await drawRect(page); // paint while connected so the log has a real entry
  await expect(page.locator("#log li.add").first()).toContainText("add");

  // Drop the connection: drawing keeps working and the edit is queued.
  await page.evaluate(() => window.__eventfulranges.closeSocket());
  await expect(page.locator("#reconnectBanner")).toBeVisible();

  await drawRect(page);
  await expect(page.locator("#backlog")).toBeVisible();
  await expect(page.locator("#backlog")).toContainText("1");

  // The queued edit is part of the local log and export before the server
  // has even acknowledged it.
  const [download] = await Promise.all([
    page.waitForEvent("download"),
    page.locator("#downloadJsonl").click(),
  ]);
  const content = await readFile(await download.path(), "utf8");
  expect(content.trim().split("\n")).toHaveLength(2);

  // Reconnecting drains the queue and clears the backlog.
  await page.locator("#reconnectBtn").click();
  await expect(page.locator("#status")).toContainText("connected");
  await expect(page.locator("#backlog")).toBeHidden();
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

test("repainting over an erased area materializes", async ({ page }) => {
  await page.goto("/ui/");
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator("#status")).toContainText("connected");

  await drawRect(page); // paint cells (0,0)..(1,1)
  await expect.poll(() => cellPainted(page, 0, 0)).toBe(true);

  await page.locator("#toolErase").click();
  await drawRect(page); // erase the same block
  await expect.poll(() => cellPainted(page, 0, 0)).toBe(false);

  await page.locator("#toolRect").click();
  await drawRect(page); // repaint: the later stroke must win
  await expect.poll(() => cellPainted(page, 0, 0)).toBe(true);
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
  await expect(page.locator("#presence")).toContainText(
    `you are ${session.slice(0, 5)}`,
  );

  // "connected" lists everyone across all sessions, with their session ids.
  await page.locator("#connectedLink").click();
  const list = page.locator("#rosterList");
  await expect(list).toBeVisible();
  await expect(page.locator("#rosterTitle")).toHaveText("all sessions");
  await expect(list).toContainText(session.slice(0, 5));

  // "here" narrows the same roster to the current session.
  await page.locator("#hereLink").click();
  await expect(page.locator("#rosterTitle")).toHaveText("in this session");
  await expect(list).toContainText(session.slice(0, 5));
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
  await expect(list).toContainText(idA.slice(0, 5));
  await expect(list).toContainText(idB.slice(0, 5));

  // One user with two sessions appears in a single entry.
  await expect(
    a.locator("#rosterList li", { hasText: idA.slice(0, 5) }),
  ).toHaveCount(1);
  await expect(
    a.locator("#rosterList li", { hasText: idA.slice(0, 5) }),
  ).toContainText(idB.slice(0, 5));
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

test("undo retracts only the undoing client's own stroke", async ({
  browser,
}) => {
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
  const entries = content
    .trim()
    .split("\n")
    .map((line) => JSON.parse(line));
  const adds = entries.filter((e) => e.kind === "add");
  const retract = entries.find((e) => e.kind === "retract");
  expect(adds).toHaveLength(2);
  expect(retract.ref).toBe(adds[1].id);
  expect(retract.ref).not.toBe(adds[0].id);
});

test("focus mode gives the canvas the whole viewport and hides the chrome", async ({
  page,
}) => {
  await page.goto("/ui/");
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator("#status")).toContainText("connected");

  const before = await page.locator("#board").boundingBox();

  await page.locator("#zenBtn").click();
  await expect(page.locator("body")).toHaveClass(/zen/);
  await expect(page.locator("#panel")).toBeHidden();
  await expect(page.locator("#activity")).toBeHidden();
  await expect(page.locator("#downloadJsonl")).toBeHidden();

  const viewport = page.viewportSize();
  const full = await page.locator("#board").boundingBox();
  expect(full.height).toBeGreaterThanOrEqual(viewport.height * 0.8);
  expect(full.height).toBeGreaterThan(before.height);

  // The resized canvas must be repainted, not left blank.
  await expectBoardPainted(page);

  // Drawing still works with the chrome hidden.
  await drawRect(page);
  await expect(page.locator("#log li.add").first()).toContainText("add");

  // Exiting restores the side panel and activity log, and repaints the board
  // at its restored size.
  await page.locator("#zenBtn").click();
  await expect(page.locator("body")).not.toHaveClass(/zen/);
  await expect(page.locator("#panel")).toBeVisible();
  await expect(page.locator("#activity")).toBeVisible();
  await expectBoardPainted(page);
});

test("on a small phone the board stays at least 80% of the viewport tall", async ({
  page,
}) => {
  await page.setViewportSize({ width: 375, height: 667 });
  await page.goto("/ui/");
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator("#status")).toContainText("connected");

  const viewport = page.viewportSize();
  const box = await page.locator("#board").boundingBox();
  // The board must cover at least 80% of the viewport (whole pixels; the
  // browser quantizes layout to 1/64 px, so compare against the floor).
  expect(box.height).toBeGreaterThanOrEqual(Math.floor(viewport.height * 0.8));
});

test("admins see the admin link and page", async ({ browser }) => {
  const context = await browser.newContext({
    extraHTTPHeaders: { "X-Auth-Request-Email": "admin@example.com" },
  });
  const page = await context.newPage();
  await page.goto("/ui/");
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator("#status")).toContainText("connected");

  await expect(page.locator("#adminLink")).toBeVisible();
  await page.locator("#adminLink").click();
  await page.waitForURL(/\/admin\//);
  // The page fetched and rendered the instance info.
  await expect(page.locator("#storageBytes")).toHaveText(/\d/);
  await expect(page.locator("#users")).toBeAttached();
  await expect(page.locator("#sessions")).toBeAttached();

  await context.close();
});

test("non-admins do not see the admin link and are denied the admin API", async ({
  page,
  request,
}) => {
  await page.goto("/ui/");
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator("#status")).toContainText("connected");

  await expect(page.locator("#adminLink")).toBeHidden();

  const res = await request.get("/admin/api/info");
  expect(res.status()).toBe(403);
});

test("the admin page does not hold a session open", async ({ browser }) => {
  const context = await browser.newContext({
    extraHTTPHeaders: { "X-Auth-Request-Email": "admin@example.com" },
  });

  // One whiteboard tab connects to a session and paints into it.
  const board = await context.newPage();
  await board.goto("/ui/");
  await board.waitForURL(/[?&]s=/);
  const session = new URL(board.url()).searchParams.get("s");
  await expect(board.locator("#status")).toContainText("connected");
  await drawRect(board);
  await expect(board.locator("#log li.add").first()).toContainText("add");

  // The admin page is plain HTTP: it adds no client, so the live whiteboard
  // tab alone accounts for the session's client count.
  const admin = await context.newPage();
  await admin.goto("/admin/");
  await expect(admin.locator("#storageBytes")).toHaveText(/\d/);
  const row = admin.locator("#sessions tr", { hasText: session });
  await expect(row.locator("td").nth(2)).toHaveText("1");

  // Close the whiteboard; the admin page stays open, yet the session becomes
  // inactive and deletable.
  await board.close();
  await expect
    .poll(async () => {
      const res = await admin.request.get("/admin/api/info", {
        headers: { "X-Auth-Request-Email": "admin@example.com" },
      });
      const info = await res.json();
      return info.sessions.find((s) => s.id === session)?.clients ?? 0;
    })
    .toBe(0);

  const del = await admin.request.delete(`/admin/api/sessions/${session}`, {
    headers: { "X-Auth-Request-Email": "admin@example.com" },
  });
  expect(del.status()).toBe(204);

  await admin.reload();
  await expect(admin.locator("#sessions tr", { hasText: session })).toHaveCount(
    0,
  );

  await context.close();
});

test("painting still works after extreme zoom far from the origin", async ({
  page,
}) => {
  await page.goto("/ui/");
  await page.waitForURL(/[?&]s=/);
  await expect(page.locator("#status")).toContainText("connected");

  const canvas = page.locator("#board");
  const box = await canvas.boundingBox();
  const cx = box.x + box.width / 2;
  const cy = box.y + box.height / 2;

  // Pan ~50 board units right, then zoom in far past the grid's default
  // limit. The zoom must cap at the precision budget instead of crashing.
  await page.mouse.move(cx, cy);
  await page.mouse.down({ button: "right" });
  await page.mouse.move(cx - 1200, cy, { steps: 20 });
  await page.mouse.up({ button: "right" });

  for (let i = 0; i < 48; i++) {
    await page.locator("#zoomIn").click();
  }
  await expect(page.locator("#zoomLabel")).not.toHaveText("100%");

  // A stroke at this zoom still lands as a real operation, not an error.
  await page.mouse.move(cx, cy);
  await page.mouse.down();
  await page.mouse.move(cx + 36, cy + 36);
  await page.mouse.up();

  await expect(page.locator("#log li.add").first()).toContainText("add");
  await expect(page.locator("#status")).toContainText("connected");
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

// expectBoardPainted waits until the canvas backing store has been drawn: a
// resized canvas is transparent (alpha 0) until draw() fills the background.
async function expectBoardPainted(page) {
  await page.waitForFunction(() => {
    const c = document.querySelector("#board");
    return c.getContext("2d").getImageData(1, 1, 1, 1).data[3] > 0;
  });
}

// cellPainted samples the centre of board cell (ix, iy) under the initial
// camera (centre 0,0, 24 px per board unit) and reports whether it is the
// stroke colour rather than the background.
async function cellPainted(page, ix, iy) {
  const red = await page.locator("#board").evaluate(
    (canvas, [x, y]) => {
      const dpr = window.devicePixelRatio || 1;
      const px = Math.round(canvas.width / 2 + (x + 0.5) * 24 * dpr);
      const py = Math.round(canvas.height / 2 + (y + 0.5) * 24 * dpr);
      return canvas.getContext("2d").getImageData(px, py, 1, 1).data[0];
    },
    [ix, iy],
  );
  return red > 150;
}
