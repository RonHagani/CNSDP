import { test, expect } from "@playwright/test";

/**
 * Visual and interaction verification for the Causal Evidence Dossier,
 * run against the production preview build (playwright.config.ts points
 * baseURL at `vite preview`). Captures the six required review screenshots
 * into review-artifacts/ (gitignored — never committed) and exercises the
 * interactions the Alert Investigation shell replacement specifically
 * requires: register/concordance selection, the command palette, every
 * route-level state, and structural integrity (no horizontal overflow).
 */

test.describe("Alert Investigation — screenshots", () => {
  test("01 — scenario 1, wide desktop, default state", async ({ page }) => {
    await page.setViewportSize({ width: 1680, height: 1000 });
    await page.goto("/alerts/1");

    await expect(page.getByText("Alert #1", { exact: false }).first()).toBeVisible();
    await expect(page.getByText("Traceability verified.")).toBeVisible();
    await expect(page.getByRole("heading", { name: "Evidence register" })).toBeVisible();
    // Detection Result is selected by default — no prior interaction.
    await expect(page.getByRole("heading", { name: /Inspection: Detection result/i })).toBeVisible();

    await page.screenshot({
      path: "review-artifacts/01-scenario1-wide-desktop-default.png",
      fullPage: true,
    });
  });

  test("02 — scenario 1, wide desktop, Verified-provenance condition selected", async ({ page }) => {
    await page.setViewportSize({ width: 1680, height: 1000 });
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
    await page.setViewportSize({ width: 1680, height: 1000 });
    await page.goto("/alerts/4");

    await expect(page.getByText("Alert #4", { exact: false }).first()).toBeVisible();
    // All five declared characteristics are Partial provenance and are
    // simultaneously visible in the concordance without any selection.
    await expect(page.getByText(/^Partial —/).first()).toBeVisible();
    expect(await page.getByText(/^Partial —/).count()).toBe(5);

    await page.screenshot({
      path: "review-artifacts/03-scenario2-partial-provenance.png",
      fullPage: true,
    });
  });

  test("04 — scenario 3, wide desktop", async ({ page }) => {
    await page.setViewportSize({ width: 1680, height: 1000 });
    await page.goto("/alerts/5");

    await expect(page.getByText("Alert #5", { exact: false }).first()).toBeVisible();
    await expect(page.getByText(/scenario-3/).first()).toBeVisible();
    await expect(page.getByText("role_ref_cluster_admin").first()).toBeVisible();

    await page.screenshot({
      path: "review-artifacts/04-scenario3-wide-desktop.png",
      fullPage: true,
    });
  });

  test("05 — scenario 1, narrow mobile", async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/alerts/1");

    await expect(page.getByText("Alert #1", { exact: false }).first()).toBeVisible();
    await expect(page.getByRole("heading", { name: "Evidence register" })).toBeVisible();

    await page.screenshot({
      path: "review-artifacts/05-scenario1-narrow-mobile.png",
      fullPage: true,
    });
  });

  test("06 — broken traceability, wide desktop", async ({ page }) => {
    await page.setViewportSize({ width: 1680, height: 1000 });
    await page.goto("/alerts/3");

    await expect(page.getByText("Alert #3", { exact: false }).first()).toBeVisible();
    await expect(page.getByText("Traceability broken.")).toBeVisible();
    await expect(page.getByText("Failed link: raw_event_sha256")).toBeVisible();

    await page.screenshot({
      path: "review-artifacts/06-broken-traceability-wide-desktop.png",
      fullPage: true,
    });
  });
});

test.describe("Alert Investigation — interaction", () => {
  test("command palette opens with Ctrl+K and an Inspect command selects the artifact", async ({
    page,
  }) => {
    await page.goto("/alerts/1");
    await expect(page.getByRole("heading", { name: "Evidence register" })).toBeVisible();
    await page.keyboard.press("Control+k");
    await expect(page.getByPlaceholder(/Jump to/)).toBeVisible();

    await page.keyboard.type("Inspect normalized event");
    await page.keyboard.press("Enter");

    await expect(page.getByPlaceholder(/Jump to/)).toBeHidden();
    await expect(page.getByRole("heading", { name: /Inspection: Normalized event/i })).toBeVisible();
    await expect(page.getByRole("button", { name: /Normalized event/i }).first()).toHaveAttribute(
      "aria-pressed",
      "true",
    );
  });

  test("selecting a register entry and a concordance condition are independent axes", async ({
    page,
  }) => {
    await page.goto("/alerts/1");

    const registerSection = page.locator("section", { has: page.getByRole("heading", { name: "Evidence register" }) });
    await registerSection.getByRole("button", { name: /Source submission/i }).click();
    await expect(page.getByRole("heading", { name: /Inspection: Source event/i })).toBeVisible();

    await page.getByRole("button", { name: /tty_allocation/i }).click();
    await expect(page.getByRole("group", { name: /Provenance record/i })).toBeVisible();
    // The register selection made moments ago is unaffected.
    await expect(
      registerSection.getByRole("button", { name: /Source submission/i }),
    ).toHaveAttribute("aria-pressed", "true");
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
});

test.describe("Alert Investigation — structural integrity", () => {
  for (const [label, viewport] of [
    ["desktop", { width: 1680, height: 1000 }],
    ["mobile", { width: 390, height: 844 }],
  ] as const) {
    test(`no page-level horizontal overflow at ${label} viewport`, async ({ page }) => {
      await page.setViewportSize(viewport);
      await page.goto("/alerts/1");
      await expect(page.getByRole("heading", { name: "Evidence register" })).toBeVisible();

      const hasHorizontalOverflow = await page.evaluate(
        () => document.documentElement.scrollWidth > document.documentElement.clientWidth,
      );
      expect(hasHorizontalOverflow).toBe(false);
    });
  }

  test("no console errors or warnings during the primary investigation flow", async ({ page }) => {
    const messages: string[] = [];
    page.on("console", (msg) => {
      if (msg.type() === "error" || msg.type() === "warning") {
        messages.push(`${msg.type()}: ${msg.text()}`);
      }
    });

    await page.goto("/alerts/1");
    const registerSection = page.locator("section", { has: page.getByRole("heading", { name: "Evidence register" }) });
    await registerSection.getByRole("button", { name: /Normalized event/i }).click();
    await page.getByRole("button", { name: /tty_allocation/i }).click();
    await page.keyboard.press("Control+k");
    await page.keyboard.press("Escape");

    expect(messages).toEqual([]);
  });
});
