import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { createRequire } from "node:module";
import { dirname, resolve } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const frontend = resolve(
  process.env.FRONTEND_PACKAGE ?? resolve(root, "../../frontend/package.json"),
);
const frontendRoot = dirname(frontend);
const require = createRequire(frontend);
const { chromium, expect } = require("@playwright/test");
const { createServer } = await import(
  pathToFileURL(require.resolve("vite")).href
);
const { default: react } = await import(
  pathToFileURL(require.resolve("@vitejs/plugin-react")).href
);
const { default: tailwind } = await import(
  pathToFileURL(require.resolve("@tailwindcss/vite")).href
);
const aliases = Object.entries({
  "@fixture/sdk": "src/app/module-sdk.ts",
  "@fixture/registry": "src/app/module-registry.ts",
  "@fixture/i18n": "src/i18n/index.ts",
  "@fixture/interaction": "src/shared/interaction.tsx",
  "@fixture/style": "src/style.css",
  "@fixture/design": "src/design-system.css",
  "@fixture/home": "src/home/components.css",
}).map(([find, path]) => ({ find, replacement: resolve(frontendRoot, path) }));
for (const name of [
  "react/jsx-runtime",
  "react-dom/client",
  "react",
  "react-router-dom",
  "@tanstack/react-query",
])
  aliases.push({
    find: new RegExp("^" + name + "$"),
    replacement: require.resolve(name),
  });
const server = await createServer({
  configFile: false,
  root,
  plugins: [react(), tailwind()],
  resolve: { alias: aliases },
  server: {
    host: "127.0.0.1",
    port: 15179,
    strictPort: true,
    fs: { allow: [root, frontendRoot] },
  },
});
await server.listen();
const browser = await chromium.launch({
  executablePath: process.env.CHROME_PATH || "/opt/google/chrome/chrome",
  headless: true,
  args: ["--no-sandbox"],
});
const dictionaries = Object.fromEntries(
  await Promise.all(
    ["en", "ru", "uk"].map(async (lang) => [
      lang,
      JSON.parse(
        await readFile(resolve(root, `frontend/locales/${lang}.json`), "utf8"),
      ),
    ]),
  ),
);
const offer = (id = "fixture-offer-a", duration = 60000) => ({
  id,
  pin: 4321,
  expires: new Date(Date.now() + duration).toISOString(),
  files: [
    {
      Name:
        "family-archive-with-a-long-translated-file-name-".repeat(3) +
        "2026.zip",
      Size: 2048,
    },
  ],
});
async function setup(
  lang = "en",
  theme = "light",
  surface = "solid",
  width = 1440,
  initial = offer(),
) {
  const page = await browser.newPage({ viewport: { width, height: 800 } });
  const state = {
    history: [],
    inbox: "/home/fixture/Inbox",
    defaultInbox: true,
    busy: false,
    offer: initial,
  };
  const writes = [];
  const errors = [];
  const fonts = [];
  let mode = "success",
    release;
  page.on("pageerror", (e) => errors.push(e.message));
  page.on("console", (m) => {
    if (m.type() === "error" && !m.text().includes("409 (Conflict)"))
      errors.push(m.text());
  });
  page.on("response", (r) => {
    if (r.url().includes("manrope") && r.url().includes(".woff") && r.ok())
      fonts.push(r.url());
  });
  await page.route("**/api/v1/**", async (route) => {
    const url = new URL(route.request().url());
    if (url.pathname.endsWith("/file-drop/state"))
      return route.fulfill({ json: state });
    if (
      url.pathname === "/api/v1/manage" &&
      url.searchParams.get("view") === "personal-folders"
    )
      return route.fulfill({
        json: {
          roots: [{ name: "Home", path: "/home/fixture" }],
          folders: [],
          path: "/home/fixture/Inbox",
          allowed: true,
        },
      });
    if (url.pathname.endsWith("/file-drop/offer")) {
      const body = route.request().postDataJSON();
      writes.push(body);
      if (mode === "hold")
        await new Promise((done) => {
          release = done;
        });
      if (mode === "fail")
        return route.fulfill({
          status: 409,
          json: { error: "Fixture decision rejected" },
        });
      state.offer = undefined;
      return route.fulfill({ json: { accepted: body.decision === "accept" } });
    }
    return route.fulfill({
      status: 404,
      json: { error: "Unexpected fixture API " + url.pathname },
    });
  });
  await page.goto(
    "http://127.0.0.1:15179/tests/offers.html?" +
      new URLSearchParams({ lang, theme, surface }),
  );
  await expect(
    page.getByRole("heading", { name: dictionaries[lang].title, exact: true }),
  ).toBeVisible();
  await page.evaluate(() => document.fonts.ready);
  const loaded = await page.evaluate(async () => {
    const faces = await document.fonts.load(
      '16px "Manrope Variable"',
      "File Drop",
    );
    return (
      faces.some(
        (f) => f.family === "Manrope Variable" && f.status === "loaded",
      ) &&
      getComputedStyle(document.body).fontFamily.includes("Manrope Variable")
    );
  });
  assert.ok(loaded, "actual Manrope face must load");
  return {
    page,
    state,
    writes,
    errors,
    fonts,
    text: dictionaries[lang],
    refresh: () => page.evaluate(() => window.offerFixture.refresh()),
    setMode: (value) => {
      mode = value;
    },
    release: () => release?.(),
    hasRelease: () => !!release,
  };
}
async function inspect(page, text) {
  const dialog = page.getByRole("dialog");
  await expect(dialog).toBeVisible();
  await expect(
    dialog
      .locator(".modal-footer")
      .getByRole("button", { name: text.later, exact: true }),
  ).toBeFocused();
  await expect(async () => {
    const shape = await dialog.evaluate((el) => {
      const rect = el.getBoundingClientRect();
      return {
        left: rect.left,
        right: rect.right,
        top: rect.top,
        bottom: rect.bottom,
        w: innerWidth,
        h: innerHeight,
        overflow: el.scrollWidth > el.clientWidth + 1,
        buttons: [...el.querySelectorAll(".modal-footer button")].map((b) => {
          const r = b.getBoundingClientRect();
          return {
            left: r.left,
            right: r.right,
            top: r.top,
            bottom: r.bottom,
            width: r.width,
            height: r.height,
            hits: [
              [0.5, 0.5],
              [0.1, 0.5],
              [0.9, 0.5],
              [0.5, 0.1],
              [0.5, 0.9],
            ].every(([x, y]) =>
              b.contains(
                document.elementFromPoint(
                  r.left + r.width * x,
                  r.top + r.height * y,
                ),
              ),
            ),
          };
        }),
      };
    });
    assert.ok(
      shape.left >= -1 &&
        shape.right <= shape.w + 1 &&
        shape.top >= -1 &&
        shape.bottom <= shape.h + 1 &&
        !shape.overflow,
    );
    assert.ok(
      shape.buttons.every(
        (b) =>
          b.left >= -1 &&
          b.right <= shape.w + 1 &&
          b.top >= 0 &&
          b.bottom <= shape.h + 1 &&
          b.width >= 43.9 &&
          b.height >= 43.9 &&
          b.hits,
      ),
    );
  }).toPass({ timeout: 5000 });
  return dialog;
}
try {
  for (const lang of ["en", "ru", "uk"])
    for (const theme of ["light", "dark"])
      for (const surface of ["solid", "glass"])
        for (const width of [320, 1440]) {
          const t = await setup(lang, theme, surface, width);
          await expect(t.page.getByRole("dialog")).toHaveCount(0);
          const trigger = t.page.getByRole("button", {
            name: t.text.reviewRequest,
            exact: true,
          });
          await trigger.click();
          const dialog = await inspect(t.page, t.text);
          await expect(dialog).toContainText("4321");
          await expect(dialog).toContainText(t.state.offer.files[0].Name);
          await t.page.keyboard.press("Escape");
          await expect(dialog).toHaveCount(0);
          await expect(trigger).toBeFocused();
          assert.equal(t.writes.length, 0);
          assert.ok(t.fonts.length > 0);
          assert.deepEqual(t.errors, []);
          await t.page.close();
        }
  const t = await setup("en", "light", "solid", 1440, undefined);
  t.state.offer = undefined;
  await t.refresh();
  await t.page.evaluate(() =>
    window.offerFixture.navigate("/file-drop/folder"),
  );
  await t.page
    .getByRole("button", { name: t.text.changeFolder, exact: true })
    .click();
  await expect(
    t.page.getByRole("dialog", { name: t.text.myFolder, exact: true }),
  ).toBeVisible();
  t.state.offer = offer();
  await t.refresh();
  await expect(t.page.getByRole("dialog")).toHaveCount(1);
  await expect(
    t.page.getByRole("dialog", { name: t.text.myFolder, exact: true }),
  ).toBeVisible();
  await t.page.keyboard.press("Escape");
  await t.page
    .getByRole("button", { name: t.text.reviewRequest, exact: true })
    .click();
  await inspect(t.page, t.text);
  await t.page
    .getByRole("dialog")
    .locator(".modal-footer")
    .getByRole("button", { name: t.text.later, exact: true })
    .click();
  assert.equal(
    t.writes.length,
    0,
    "folder and offer dismissal must not decide",
  );
  await t.page
    .getByRole("button", { name: t.text.reviewRequest, exact: true })
    .click();
  t.state.offer = offer("fixture-offer-b");
  await t.refresh();
  await expect(t.page.getByRole("dialog")).toContainText(
    t.text.requestUnavailable,
  );
  await expect(
    t.page
      .getByRole("dialog")
      .getByRole("button", { name: t.text.accept, exact: true }),
  ).toBeDisabled();
  await expect(
    t.page
      .getByRole("dialog")
      .getByRole("button", { name: t.text.decline, exact: true }),
  ).toBeDisabled();
  assert.equal(t.writes.length, 0);
  await t.page.keyboard.press("Escape");
  await t.page
    .getByRole("button", { name: t.text.reviewRequest, exact: true })
    .click();
  await t.page
    .getByRole("dialog")
    .getByRole("button", { name: t.text.decline, exact: true })
    .click();
  await expect(t.page.getByRole("dialog")).toHaveCount(0);
  assert.deepEqual(t.writes, [{ id: "fixture-offer-b", decision: "decline" }]);
  t.state.offer = offer("fixture-offer-c");
  await t.refresh();
  t.setMode("fail");
  await t.page
    .getByRole("button", { name: t.text.reviewRequest, exact: true })
    .click();
  await t.page
    .getByRole("dialog")
    .getByRole("button", { name: t.text.accept, exact: true })
    .click();
  await expect(t.page.getByRole("dialog")).toContainText(
    "Fixture decision rejected",
  );
  t.setMode("hold");
  await t.page
    .getByRole("dialog")
    .getByRole("button", { name: t.text.accept, exact: true })
    .click();
  await expect.poll(t.hasRelease).toBe(true);
  await expect(t.page.getByRole("dialog")).toHaveAttribute("aria-busy", "true");
  await t.page.keyboard.press("Escape");
  await expect(t.page.getByRole("dialog")).toBeVisible();
  await expect(
    t.page
      .getByRole("dialog")
      .locator(".modal-footer")
      .getByRole("button", { name: t.text.later, exact: true }),
  ).toBeDisabled();
  t.setMode("success");
  t.release();
  await expect(t.page.getByRole("dialog")).toHaveCount(0);
  assert.deepEqual(t.writes.slice(1), [
    { id: "fixture-offer-c", decision: "accept" },
    { id: "fixture-offer-c", decision: "accept" },
  ]);
  t.state.offer = offer("fixture-offer-expiring", 1200);
  await t.refresh();
  await t.page
    .getByRole("button", { name: t.text.reviewRequest, exact: true })
    .click();
  await expect(t.page.getByRole("dialog")).toContainText(t.text.requestExpired);
  await expect(
    t.page
      .getByRole("dialog")
      .getByRole("button", { name: t.text.accept, exact: true }),
  ).toBeDisabled();
  await t.page.keyboard.press("Escape");
  assert.equal(t.writes.length, 3);
  assert.deepEqual(t.errors, []);
  await t.page.close();
  console.log(
    "PASS built File Drop: 24 locale/theme/width cases, pending offer during folder choice, dismiss/reopen without refusal, selected IDs, expiry/change guard, pending/error recovery, focus, hit targets and loaded fonts",
  );
} finally {
  await browser.close();
  await server.close();
}
