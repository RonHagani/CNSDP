import { test, expect } from "@playwright/test";

/**
 * Visual and interaction verification for the Alert Investigation
 * flagship, run against the production preview build (playwright.config.ts
 * points baseURL at `vite preview`). Captures the three required review
 * screenshots into review-artifacts/ (gitignored — never committed) and
 * exercises the interactions the milestone specifically requires.
 */

test.describe("Alert Investigation flagship", () => {
  test("wide desktop — full flagship screen renders with real evidence content", async ({
    page,
  }) => {
    await page.setViewportSize({ width: 1680, height: 1000 });
    await page.goto("/alerts/1");

    await expect(page.locator("b", { hasText: "0001" }).first()).toBeVisible();
    await expect(page.getByText("Traceability verified.")).toBeVisible();
    await expect(page.getByText("present").first()).toBeVisible();
    // The filmstrip's six tiles fade in with a per-tile stagger delay
    // (up to ~0.65s for the last one) — wait for the final tile's own
    // mount animation to settle before capturing, so the review screenshot
    // reflects steady state rather than a mid-animation frame. The opacity
    // check targets the animated <button> itself, not the text node inside
    // it — a child's computed opacity doesn't reflect an ancestor's.
    await expect(page.getByRole("button", { name: /Generated alert/ })).toHaveCSS("opacity", "1", {
      timeout: 2000,
    });

    await page.screenshot({
      path: "review-artifacts/01-wide-desktop-flagship.png",
      fullPage: true,
    });
  });

  test("field lineage — selecting a normalized field surfaces a focused provenance record", async ({
    page,
  }) => {
    await page.setViewportSize({ width: 1680, height: 1000 });
    await page.goto("/alerts/1");

    // The full normalized record is a secondary, on-demand inspector — it
    // must be opened before any of its fields are selectable.
    await page.getByRole("button", { name: "Inspect full normalized record" }).click();
    const normalizedField = page.getByRole("button", { name: "kubernetes-admin", exact: true });
    await normalizedField.click();
    await expect(normalizedField).toHaveAttribute("aria-pressed", "true");
    // The focused provenance record — raw path, behavior, normalized path —
    // is the lineage interaction's primary result now, replacing the old
    // cross-panel SVG connector.
    await expect(page.getByText("user.username", { exact: true })).toBeVisible();
    await expect(page.getByText("subject.username", { exact: true })).toBeVisible();

    await page.screenshot({
      path: "review-artifacts/02-field-lineage-focused.png",
      fullPage: false,
    });
  });

  test("narrow mobile — deliberate responsive transformation, not naive stacking", async ({
    page,
  }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/alerts/1");

    await expect(page.locator("b", { hasText: "0001" }).first()).toBeVisible();
    // Same settle wait as the wide-desktop capture: the exhibit ribbon's
    // tiles fade in on a per-tile stagger regardless of viewport, so a
    // full-page screenshot taken too early catches them mid-animation.
    await expect(page.getByRole("button", { name: /Generated alert/ })).toHaveCSS("opacity", "1", {
      timeout: 2000,
    });

    await page.screenshot({
      path: "review-artifacts/03-narrow-mobile.png",
      fullPage: true,
    });
  });

  test("command palette opens with Ctrl+K and executes a stage-focus command", async ({ page }) => {
    await page.goto("/alerts/1");
    await page.keyboard.press("Control+k");
    await expect(page.getByPlaceholder(/Jump to a stage/)).toBeVisible();

    await page.keyboard.type("Validation");
    await page.keyboard.press("Enter");
    await expect(page.getByPlaceholder(/Jump to a stage/)).toBeHidden();
  });

  test("broken traceability is rendered distinctly, never as a generic error", async ({ page }) => {
    await page.goto("/alerts/3");
    await expect(page.getByText("Traceability broken.")).toBeVisible();
    await expect(page.getByText(/raw_event_sha256/).first()).toBeVisible();
  });

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

  test("no console errors or warnings during the primary investigation flow", async ({ page }) => {
    const messages: string[] = [];
    page.on("console", (msg) => {
      if (msg.type() === "error" || msg.type() === "warning") {
        messages.push(`${msg.type()}: ${msg.text()}`);
      }
    });

    await page.goto("/alerts/1");
    await page.getByRole("button", { name: "Inspect full normalized record" }).click();
    await page.getByRole("button", { name: "kubernetes-admin", exact: true }).click();
    await page.keyboard.press("Control+k");
    await page.keyboard.press("Escape");

    expect(messages).toEqual([]);
  });
});
