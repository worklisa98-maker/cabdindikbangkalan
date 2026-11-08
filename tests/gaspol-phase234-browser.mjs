import assert from "node:assert/strict";
import fs from "node:fs/promises";
import { pathToFileURL } from "node:url";
const runtime = process.env.GASPOL_PLAYWRIGHT_MODULE;
if (!runtime)
  throw new Error(
    "Set GASPOL_PLAYWRIGHT_MODULE to the available Playwright module path"
  );
const { chromium } = await import(pathToFileURL(runtime));
const browser = await chromium.launch({ headless: true, channel: "msedge" });
const base = process.env.GASPOL_UI_TEST_URL ?? "http://127.0.0.1:3012";
const candidates = Array.from({ length: 5 }, (_, i) => ({
  schoolId: i + 1,
  schoolName: `Sekolah Kandidat ${i + 1}`,
  npsn: `2050000${i + 1}`,
  shortCode: `SCHOOL00000${i + 1}`,
}));
const messages = [];
try {
  const context = await browser.newContext({
    viewport: { width: 390, height: 844 },
  });
  const page = await context.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(e.message));
  let voted = false;
  let role = "GASPOL_VOTER";
  await page.route("**/api/**", async (route) => {
    const url = new URL(route.request().url());
    let data = {
      halo: { is_active: false },
      sinom: { is_active: false },
      pusat_tautan: { categories: [], links: [] },
    };
    if (url.pathname === "/api/auth/login")
      data = {
        token: "qa-token-only",
        user: { id: 3, username: "qa-voter", role },
      };
    else if (url.pathname === "/api/auth/me") {
      if (!route.request().headers().authorization) {
        await route.fulfill({ json: { error: "UNAUTHORIZED" }, status: 401 });
        return;
      }
      data = { id: 3, username: "qa-voter", role };
    } else if (url.pathname === "/api/gaspol/voting/vote") {
      const input = route.request().postDataJSON();
      assert.equal(input.sessionId, 1);
      assert.equal(input.schoolId, 2);
      assert.equal(
        route.request().headers().authorization,
        "Bearer qa-token-only"
      );
      voted = true;
      data = { voted: true };
    } else if (url.pathname === "/api/gaspol/voting")
      data = {
        id: 1,
        year: 2026,
        status: "OPEN",
        candidates,
        voted,
        canVote: !voted,
        registered: 33,
        present: 33,
        votedCount: 1,
        notVoted: 32,
        participation: 100 / 33,
        roster: [],
      };
    else if (url.pathname === "/api/service-links")
      data = {
        halo: { is_active: false },
        sinom: { is_active: false },
        pusat_tautan: { categories: [], links: [] },
      };
    else if (url.pathname.includes("system-time"))
      data = { now: new Date().toISOString() };
    await route.fulfill({ json: { data }, status: 200 });
  });
  await page.goto(`${base}/gaspol/voting?year=2026&schoolId=2&sessionId=1`);
  await page.getByLabel("Username", { exact: true }).fill("qa-voter");
  await page.getByLabel("Password", { exact: true }).fill("private-test-input");
  await page
    .getByRole("button", { name: "Login dan lanjutkan pilihan" })
    .click();
  await page.getByLabel("Sekolah pilihan").waitFor();
  assert.equal(await page.getByLabel("Sekolah pilihan").inputValue(), "2");
  await page.getByLabel("Fallback kode pendek").fill("school000002");
  await page.getByRole("button", { name: "Tinjau pilihan" }).click();
  await page
    .getByRole("button", { name: "Konfirmasi suara", exact: true })
    .waitFor();
  await page.screenshot({
    path: "docs/qa/gaspol-phase234-vote-mobile.png",
    fullPage: true,
  });
  await page
    .getByRole("button", { name: "Konfirmasi suara", exact: true })
    .click();
  await page
    .getByText("Suara Anda telah tercatat.", { exact: false })
    .waitFor();
  assert.equal(
    await page
      .getByRole("button", { name: "Konfirmasi suara", exact: true })
      .count(),
    0
  );
  assert.equal(
    await page.evaluate(
      () => document.documentElement.scrollWidth > window.innerWidth
    ),
    false
  );
  messages.push(
    "PASS: QR target persists through login, authenticated confirmation, receipt, no repeat vote UI, mobile no overflow"
  );
  messages.push(
    "PASS: confirmed candidate is resolved from the current session"
  );
  await context.close();
  const masterContext = await browser.newContext({
    viewport: { width: 390, height: 844 },
  });
  const master = await masterContext.newPage();
  master.on("pageerror", (e) => errors.push(e.message));
  await master.route("**/api/**", async (route) => {
    const path = new URL(route.request().url()).pathname;
    let data = {
      halo: { is_active: false },
      sinom: { is_active: false },
      pusat_tautan: { categories: [], links: [] },
    };
    if (path === "/api/auth/me") {
      if (!route.request().headers().authorization) {
        await route.fulfill({ json: { error: "UNAUTHORIZED" }, status: 401 });
        return;
      }
      data = { id: 1, username: "qa-master", role: "ADMIN_MASTER" };
    }
    if (path === "/api/auth/login")
      data = {
        token: "qa-token-only",
        user: { id: 1, username: "qa-master", role: "ADMIN_MASTER" },
      };
    if (path === "/api/gaspol/voting")
      data = {
        id: 1,
        year: 2026,
        status: "OPEN",
        candidates,
        voted: false,
        canVote: false,
        registered: 33,
        present: 33,
        votedCount: 1,
        notVoted: 32,
        participation: 100 / 33,
        roster: [],
      };
    await route.fulfill({ json: { data } });
  });
  await master.goto(`${base}/gaspol/voting?year=2026`);
  await master.getByLabel("Username", { exact: true }).fill("qa-master");
  await master
    .getByLabel("Password", { exact: true })
    .fill("private-test-input");
  await master
    .getByRole("button", { name: "Login dan lanjutkan pilihan" })
    .click();
  await master.getByRole("link", { name: "Download QR" }).first().waitFor();
  assert.equal(
    await master.getByRole("link", { name: "Download QR" }).count(),
    5
  );
  assert.equal(
    await master.getByRole("button", { name: "Cetak QR" }).count(),
    5
  );
  const download = master.waitForEvent("download");
  await master.getByRole("link", { name: "Download QR" }).first().click();
  const file = await download;
  const path = await file.path();
  assert.ok((await fs.stat(path)).size > 1000);
  assert.match(file.suggestedFilename(), /QR-Ter-Favorit-2026-1/);
  await master.screenshot({
    path: "docs/qa/gaspol-phase234-qr-mobile.png",
    fullPage: true,
  });
  assert.equal(
    await master.evaluate(
      () => document.documentElement.scrollWidth > window.innerWidth
    ),
    false
  );
  messages.push(
    "PASS: five QR previews, downloaded PNG, print controls, identity, mobile no overflow"
  );
  assert.deepEqual(errors, []);
  messages.push("PASS: no new page runtime errors in tested scenarios");
  await master.goto(`${base}/gaspol/voting?year=2026&schoolId=1&sessionId=999`);
  await master
    .getByText("QR berasal dari sesi lain. Gunakan QR resmi sesi berjalan.")
    .waitFor();
  assert.equal(
    await master.getByRole("link", { name: "Download QR" }).count(),
    0
  );
  messages.push("PASS: stale session QR is rejected");
  await masterContext.close();
  console.log(messages.join("\n"));
  console.log(
    "UI uses explicit API simulations; database/security checks are separate integration tests."
  );
} finally {
  await browser.close();
}
