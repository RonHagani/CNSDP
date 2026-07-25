import { test, expect } from "@playwright/test";

/**
 * Visual and interaction verification for the Dark Evidence Map — the live
 * Alert Investigation presentation as of Track B Pass 5's atomic shell
 * swap (InvestigationMap replacing the retired Forensic Case Folio). Runs
 * against the production preview build (playwright.config.ts points
 * baseURL at `vite preview`). Captures the required review screenshots
 * into review-artifacts/ (gitignored — never committed) and exercises
 * condition selection, keyboard operation, every route-level state, and
 * structural integrity (no horizontal overflow) at the UX spec's own
 * desktop viewport floor (1024px) and principal target range
 * (1440–1600px).
 */

const DESKTOP_VIEWPORTS = [
  ["1024px (viewport floor)", { width: 1024, height: 900 }],
  ["1440px (principal)", { width: 1440, height: 960 }],
  ["1600px (principal)", { width: 1600, height: 1000 }],
] as const;

test.describe("Alert Investigation — screenshots", () => {
  test("01 — scenario 1, wide desktop, default state", async ({ page }) => {
    await page.setViewportSize({ width: 1600, height: 1000 });
    await page.goto("/alerts/1");

    await expect(page.getByText("Alert #1", { exact: false }).first()).toBeVisible();
    await expect(page.getByText("Traceability intact").first()).toBeVisible();
    await expect(page.getByRole("heading", { name: "Source submission" })).toBeVisible();
    await expect(page.getByRole("heading", { name: "Detection result" })).toBeVisible();
    // No pin preselected on load.
    await expect(page.getByRole("group", { name: /Provenance record/i })).toHaveCount(0);

    await page.screenshot({
      path: "review-artifacts/01-scenario1-wide-desktop-default.png",
      fullPage: true,
    });
  });

  test("02 — scenario 1, wide desktop, Verified-provenance condition selected", async ({ page }) => {
    await page.setViewportSize({ width: 1600, height: 1000 });
    await page.goto("/alerts/1");

    await page.getByRole("button", { name: /tty_allocation/i }).click();
    const record = page.getByRole("group", { name: /Provenance record/i });
    await expect(record).toBeVisible();
    await expect(record.getByText("requestURI", { exact: true })).toBeVisible();

    await page.screenshot({
      path: "review-artifacts/02-scenario1-verified-provenance-selected.png",
      fullPage: true,
    });
  });

  test("03 — scenario 2, wide desktop, multiple Partial-provenance conditions", async ({ page }) => {
    await page.setViewportSize({ width: 1600, height: 1000 });
    await page.goto("/alerts/4");

    await expect(page.getByText("Alert #4", { exact: false }).first()).toBeVisible();
    // All five declared characteristics are satisfied and simultaneously
    // visible on the characteristic bus without any selection.
    for (const id of ["privileged_container", "host_network", "host_pid", "host_ipc", "host_path_volume"]) {
      await expect(page.getByRole("button", { name: new RegExp(id) })).toBeVisible();
    }

    await page.getByRole("button", { name: /privileged_container/i }).click();
    const record = page.getByRole("group", { name: /Provenance record/i });
    await expect(record).toBeVisible();
    await expect(record.getByText(/Partial/)).toBeVisible();

    await page.screenshot({
      path: "review-artifacts/03-scenario2-partial-provenance.png",
      fullPage: true,
    });
  });

  test("04 — scenario 3, wide desktop", async ({ page }) => {
    await page.setViewportSize({ width: 1600, height: 1000 });
    await page.goto("/alerts/5");

    await expect(page.getByText("Alert #5", { exact: false }).first()).toBeVisible();
    await expect(page.getByRole("button", { name: /role_ref_cluster_admin/i })).toBeVisible();

    await page.screenshot({
      path: "review-artifacts/04-scenario3-wide-desktop.png",
      fullPage: true,
    });
  });

  test("05 — scenario 1, below-1024px readable fallback", async ({ page }) => {
    await page.setViewportSize({ width: 768, height: 1024 });
    await page.goto("/alerts/1");

    await expect(page.getByText("Alert #1", { exact: false }).first()).toBeVisible();
    await expect(page.getByRole("heading", { name: "Source submission" })).toBeVisible();
    await expect(page.getByText("Traceability intact").first()).toBeVisible();

    const hasHorizontalOverflow = await page.evaluate(
      () => document.documentElement.scrollWidth > document.documentElement.clientWidth,
    );
    expect(hasHorizontalOverflow).toBe(false);

    await page.screenshot({
      path: "review-artifacts/05-scenario1-below-1024-fallback.png",
      fullPage: true,
    });
  });

  test("06 — broken traceability (raw_event_sha256), wide desktop", async ({ page }) => {
    await page.setViewportSize({ width: 1600, height: 1000 });
    await page.goto("/alerts/3");

    await expect(page.getByText("Alert #3", { exact: false }).first()).toBeVisible();
    await expect(page.getByText("Traceability broken").first()).toBeVisible();
    await expect(page.getByText(/recorded integrity digest/)).toBeVisible();

    await page.screenshot({
      path: "review-artifacts/06-broken-traceability-raw-event-sha256.png",
      fullPage: true,
    });
  });

  test("07 — broken traceability (source_key), wide desktop", async ({ page }) => {
    await page.setViewportSize({ width: 1600, height: 1000 });
    await page.goto("/alerts/6");

    await expect(page.getByText("Alert #6", { exact: false }).first()).toBeVisible();
    await expect(page.getByText("Traceability broken").first()).toBeVisible();
    await expect(page.getByText(/source identity/)).toBeVisible();

    await page.screenshot({
      path: "review-artifacts/07-broken-traceability-source-key.png",
      fullPage: true,
    });
  });
});

test.describe("Alert Investigation — interaction", () => {
  test("selecting a satisfied characteristic pin reveals its provenance record", async ({ page }) => {
    await page.goto("/alerts/1");
    await page.getByRole("button", { name: /stdin_streaming/i }).click();
    await expect(page.getByRole("group", { name: /Provenance record/i })).toBeVisible();
  });

  test("keyboard selection: focusing a pin and activating with Enter", async ({ page }) => {
    await page.goto("/alerts/1");
    const pin = page.getByRole("button", { name: /tty_allocation/i });
    await pin.focus();
    await page.keyboard.press("Enter");
    await expect(page.getByRole("group", { name: /Provenance record/i })).toBeVisible();
  });

  test("re-selecting the same pin closes its provenance record", async ({ page }) => {
    await page.goto("/alerts/1");
    const pin = page.getByRole("button", { name: /tty_allocation/i });
    await pin.click();
    await expect(page.getByRole("group", { name: /Provenance record/i })).toBeVisible();
    await pin.click();
    await expect(page.getByRole("group", { name: /Provenance record/i })).toHaveCount(0);
  });
});

test.describe("Alert Investigation — route-level states", () => {
  test("not-found, unauthorized, and unavailable states each render distinctly", async ({
    page,
  }) => {
    await page.goto("/alerts/999");
    await expect(page.getByText(/No alert exists with id 999/)).toBeVisible();

    await page.goto("/alerts/1?demo=unauthorized");
    await expect(page.getByText("Authentication required")).toBeVisible();

    await page.goto("/alerts/1?demo=unavailable");
    await expect(page.getByText("The investigation backend could not be reached")).toBeVisible();
    await expect(page.getByRole("button", { name: "Retry" })).toBeVisible();
  });

  test("partial artifact availability renders the gap explicitly, other artifacts unaffected", async ({
    page,
  }) => {
    await page.goto("/alerts/2");
    await expect(page.getByText("Alert #2", { exact: false }).first()).toBeVisible();
    await expect(page.getByText("Detection definition unavailable")).toBeVisible();
    await expect(page.getByRole("heading", { name: "Source submission" })).toBeVisible();
  });
});

test.describe("Alert Investigation — structural integrity", () => {
  for (const [label, viewport] of DESKTOP_VIEWPORTS) {
    test(`functions correctly at ${label}: no overflow, key regions visible, selection traces correctly`, async ({
      page,
    }) => {
      await page.setViewportSize(viewport);
      await page.goto("/alerts/1");

      // All six evidence artifacts remain reachable and visible, not just
      // a sample of them, at this exact viewport.
      await expect(page.getByRole("heading", { name: "Source submission" })).toBeVisible();
      await expect(page.getByRole("group", { name: "Validation outcome" })).toBeVisible();
      await expect(page.getByRole("heading", { name: "Normalized event" })).toBeVisible();
      await expect(page.getByRole("heading", { name: "Detection definition" })).toBeVisible();
      await expect(page.getByRole("heading", { name: "Detection result" })).toBeVisible();
      await expect(page.getByRole("heading", { name: "Generated alert" })).toBeVisible();
      await expect(page.getByText("Traceability intact").first()).toBeVisible();

      const hasHorizontalOverflow = await page.evaluate(
        () => document.documentElement.scrollWidth > document.documentElement.clientWidth,
      );
      expect(hasHorizontalOverflow).toBe(false);

      // The selected condition's trace lands on the correct normalized
      // field and its provenance annotation, at this exact viewport.
      await page.getByRole("button", { name: /tty_allocation/i }).click();
      const record = page.getByRole("group", { name: /Provenance record/i });
      await expect(record).toBeVisible();
      await expect(record.getByText("exec.tty", { exact: true })).toBeVisible();
      const highlightedRow = page.locator('[data-highlighted="true"]');
      await expect(highlightedRow).toHaveCount(1);
      await expect(highlightedRow).toContainText("exec.tty");
    });
  }

  test("no page-level horizontal overflow below 1024px (readable fallback, not a dedicated phone experience)", async ({
    page,
  }) => {
    await page.setViewportSize({ width: 768, height: 1024 });
    await page.goto("/alerts/1");
    await expect(page.getByRole("heading", { name: "Source submission" })).toBeVisible();

    const hasHorizontalOverflow = await page.evaluate(
      () => document.documentElement.scrollWidth > document.documentElement.clientWidth,
    );
    expect(hasHorizontalOverflow).toBe(false);
  });

  test("no console errors or warnings during the primary investigation flow", async ({ page }) => {
    const messages: string[] = [];
    page.on("console", (msg) => {
      if (msg.type() === "error" || msg.type() === "warning") {
        messages.push(`${msg.type()}: ${msg.text()}`);
      }
    });

    await page.goto("/alerts/1");
    await page.getByRole("button", { name: /tty_allocation/i }).click();
    await page.getByRole("button", { name: /tty_allocation/i }).click();

    expect(messages).toEqual([]);
  });
});
