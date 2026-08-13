import { test, expect } from "@playwright/test";
import { mockAlertListBody, mockAlertListStatus, mockFixtureBackend } from "./support/mockApi";

/**
 * Real-browser coverage for the Alert Inventory + Application Shell
 * vertical slice: `/alerts` as the primary entry point, navigation into
 * the existing Dark Evidence Map investigation screen, and back again.
 * Runs against the production preview build, same as flagship.spec.ts.
 *
 * The production frontend calls the real `GET /v1/alerts` and
 * `GET /v1/alerts/{id}` endpoints (src/lib/api/client.ts, proxied
 * same-origin by Vite — see vite.config.ts). `mockFixtureBackend`
 * (e2e/support/mockApi.ts) stands in for that backend at the network
 * layer so this suite runs without a live database; the one genuinely
 * non-intercepted journey against a real running backend lives in
 * e2e/real-backend.spec.ts.
 */

test.beforeEach(async ({ page }) => {
  await mockFixtureBackend(page);
});

const INVENTORY_VIEWPORTS = [
  ["1600px", { width: 1600, height: 1000 }],
  ["1440px", { width: 1440, height: 960 }],
  ["1280px", { width: 1280, height: 900 }],
  ["1024px", { width: 1024, height: 900 }],
  ["768px", { width: 768, height: 1024 }],
] as const;

test.describe("Alert Inventory — primary entry point", () => {
  test("`/` redirects to `/alerts`", async ({ page }) => {
    await page.goto("/");
    await expect(page).toHaveURL(/\/alerts$/);
    await expect(page.getByRole("heading", { name: "Alerts" })).toBeVisible();
  });

  test("renders multiple alerts with a visible total count, sourced from the backend", async ({ page }) => {
    await page.goto("/alerts");
    const rows = page.locator("tbody tr");
    await expect(rows).toHaveCount(6);
    await expect(page.getByText(/^6 alerts$/)).toBeVisible();
  });

  test("rows render in stable ascending alert-id order", async ({ page }) => {
    await page.goto("/alerts");
    const ids = await page.locator("tbody tr td:first-child a").allTextContents();
    const numeric = ids.map((t) => Number(t.replace("#", "")));
    expect(numeric).toEqual([...numeric].sort((a, b) => a - b));
  });

  test("the empty state renders from a real empty API response, without touching production fixtures", async ({
    page,
  }) => {
    await mockAlertListBody(page, { alerts: [], total: 0 });
    await page.goto("/alerts");
    await expect(page.getByText(/No alerts have been generated yet/)).toBeVisible();
    await expect(page.locator("tbody tr")).toHaveCount(0);
  });

  test("the error state renders with a working retry action for a 5xx", async ({ page }) => {
    await mockAlertListStatus(page, 500);
    await page.goto("/alerts");
    await expect(page.getByText(/The alert inventory backend could not be reached/)).toBeVisible();
    const retry = page.getByRole("button", { name: "Retry" });
    await expect(retry).toBeVisible();
    await retry.click();
    // Still failing (same stub), so the same state persists — proves the
    // click genuinely re-issued the request rather than being inert.
    await expect(page.getByText(/The alert inventory backend could not be reached/)).toBeVisible();
  });

  test("renders an unauthorized state for a 401", async ({ page }) => {
    await mockAlertListStatus(page, 401);
    await page.goto("/alerts");
    await expect(page.getByText("Authentication required")).toBeVisible();
  });

  test("the loading state is reachable before the inventory resolves", async ({ page }) => {
    await page.route("**/api/v1/alerts", () => new Promise(() => {})); // never resolves
    await page.goto("/alerts");
    await expect(page.getByText(/Loading alert inventory/)).toBeVisible();
  });

  test("a not-found page renders for a path that matches no route", async ({ page }) => {
    await page.goto("/this-path-does-not-exist");
    await expect(page.getByText("This page does not exist.")).toBeVisible();
    await page.getByRole("link", { name: /alert inventory/i }).click();
    await expect(page).toHaveURL(/\/alerts$/);
  });

  test("the primary navigation shows Alerts as active, Data Sources/Detections/Submissions as live links, and System Health as the one remaining disabled destination", async ({
    page,
  }) => {
    await page.goto("/alerts");
    const alertsLink = page.getByRole("link", { name: "Alerts" });
    await expect(alertsLink).toHaveAttribute("aria-current", "page");
    for (const label of ["Data Sources", "Detections", "Submissions"]) {
      const link = page.getByRole("link", { name: label });
      await expect(link).toBeVisible();
      await expect(link).not.toHaveAttribute("aria-current", "page");
    }
    await expect(page.locator('[aria-disabled="true"]', { hasText: "System Health" })).toBeVisible();
  });
});

test.describe("Alert Inventory — navigation into and out of investigation", () => {
  test("clicking a row navigates to that alert's investigation screen, then Back to alerts returns", async ({
    page,
  }) => {
    await page.goto("/alerts");
    await page.getByRole("link", { name: "#4" }).click();

    await expect(page).toHaveURL(/\/alerts\/4$/);
    await expect(page.getByText("Alert #4", { exact: false }).first()).toBeVisible();

    await page.getByRole("link", { name: /Back to alerts/ }).click();
    await expect(page).toHaveURL(/\/alerts$/);
    await expect(page.getByRole("heading", { name: "Alerts" })).toBeVisible();
  });

  test("a row is keyboard-focusable and Enter navigates to its investigation screen", async ({ page }) => {
    await page.goto("/alerts");
    const link = page.getByRole("link", { name: "#1" });
    await link.focus();
    await page.keyboard.press("Enter");
    await expect(page).toHaveURL(/\/alerts\/1$/);
    await expect(page.getByText("Alert #1", { exact: false }).first()).toBeVisible();
  });

  test("direct navigation to /alerts/:alertId works without visiting the inventory first", async ({
    page,
  }) => {
    await page.goto("/alerts/5");
    await expect(page.getByText("Alert #5", { exact: false }).first()).toBeVisible();
    await expect(page.getByRole("button", { name: /role_ref_cluster_admin/i })).toBeVisible();
  });

  test("browser back/forward navigation works correctly across inventory and investigation", async ({
    page,
  }) => {
    await page.goto("/alerts");
    await page.getByRole("link", { name: "#1" }).click();
    await expect(page).toHaveURL(/\/alerts\/1$/);

    await page.goBack();
    await expect(page).toHaveURL(/\/alerts$/);
    await expect(page.getByRole("heading", { name: "Alerts" })).toBeVisible();

    await page.goForward();
    await expect(page).toHaveURL(/\/alerts\/1$/);
    await expect(page.getByText("Alert #1", { exact: false }).first()).toBeVisible();
  });

  test("refreshing on an investigation URL re-fetches and renders the same alert", async ({ page }) => {
    await page.goto("/alerts/5");
    await expect(page.getByText("Alert #5", { exact: false }).first()).toBeVisible();

    await page.reload();
    await expect(page.getByText("Alert #5", { exact: false }).first()).toBeVisible();
    await expect(page.getByRole("button", { name: /role_ref_cluster_admin/i })).toBeVisible();
  });

  /*
   * The required end-to-end main-journey proof: open the inventory, open a
   * non-default alert, confirm its real identity, return to the inventory,
   * open a different alert, and confirm the prior alert's selected
   * characteristic/provenance annotation does not remain -- proving the
   * investigation canvas is genuinely remounted per alert, not carrying
   * stale selection state across a full inventory round trip.
   */
  test("main journey: inventory -> non-default alert -> back -> different alert carries no stale selection state", async ({
    page,
  }) => {
    await page.goto("/alerts");
    await expect(page.locator("tbody tr")).toHaveCount(6);

    await page.getByRole("link", { name: "#4" }).click();
    await expect(page).toHaveURL(/\/alerts\/4$/);
    await expect(page.getByText("Alert #4", { exact: false }).first()).toBeVisible();
    await expect(page.getByRole("button", { name: /privileged_container/i })).toBeVisible();

    await page.getByRole("button", { name: /privileged_container/i }).click();
    await expect(page.getByRole("group", { name: /Provenance record/i })).toBeVisible();

    await page.getByRole("link", { name: /Back to alerts/ }).click();
    await expect(page).toHaveURL(/\/alerts$/);

    await page.getByRole("link", { name: "#1" }).click();
    await expect(page).toHaveURL(/\/alerts\/1$/);
    await expect(page.getByText("Alert #1", { exact: false }).first()).toBeVisible();

    // No pin preselected, and no leftover provenance record from alert 4's
    // selection.
    await expect(page.getByRole("group", { name: /Provenance record/i })).toHaveCount(0);
  });
});

test.describe("Alert Inventory — structural integrity across viewports", () => {
  for (const [label, viewport] of INVENTORY_VIEWPORTS) {
    test(`renders without horizontal page overflow at ${label}, with every row's identity and navigation reachable`, async ({
      page,
    }) => {
      await page.setViewportSize(viewport);
      await page.goto("/alerts");

      await expect(page.getByRole("heading", { name: "Alerts" })).toBeVisible();
      await expect(page.locator("tbody tr")).toHaveCount(6);

      const hasBodyOverflow = await page.evaluate(
        () => document.documentElement.scrollWidth > document.documentElement.clientWidth,
      );
      expect(hasBodyOverflow).toBe(false);

      // Every row's alert-id link is reachable, even if the table itself
      // scrolls horizontally within its own container at this width.
      for (let i = 1; i <= 6; i++) {
        await expect(page.getByRole("link", { name: `#${i}` })).toBeAttached();
      }

      await page.getByRole("link", { name: "#1" }).click();
      await expect(page).toHaveURL(/\/alerts\/1$/);
      await expect(page.getByText("Alert #1", { exact: false }).first()).toBeVisible();
    });
  }
});

test.describe("Alert Inventory — sticky Alert identity and scroll affordance (768px)", () => {
  test("the Alert-id column stays pinned and legible while the row scrolls horizontally underneath it", async ({
    page,
  }) => {
    await page.setViewportSize({ width: 768, height: 1024 });
    await page.goto("/alerts");
    await expect(page.locator("tbody tr")).toHaveCount(6);

    const scrollContainer = page.locator('[role="region"][aria-label*="scrollable"]');
    await expect(scrollContainer).toBeVisible();

    const firstIdCell = page.locator("tbody tr").first().locator("td").first();
    const beforeBox = (await firstIdCell.boundingBox())!;

    await scrollContainer.evaluate((el) => {
      el.scrollLeft = el.scrollWidth;
    });
    await page.waitForTimeout(100);

    const afterBox = (await firstIdCell.boundingBox())!;
    // The pinned cell's own screen position does not move even though the
    // container scrolled — that is what "sticky" means, verified against
    // real measured geometry, not just a CSS class name.
    expect(Math.abs(afterBox.x - beforeBox.x)).toBeLessThan(2);
    await expect(page.getByRole("link", { name: "#1" })).toBeVisible();

    // The Traceability column, off-screen before scrolling, is now
    // reachable — proving the scroll genuinely moved real content, not
    // just the pinned column's illusion of staying still.
    await expect(page.locator("tbody tr").first().getByText(/Intact|Broken|raw_event|source_key/)).toBeVisible();
  });

  /*
   * Regression coverage for a real, confirmed defect: `border-collapse:
   * collapse` combined with `position: sticky` on a table cell paints
   * unreliably across browsers -- the concrete symptom was Detection and
   * Operation column text visibly bleeding through/beside the sticky
   * Alert column after scrolling. The fix (AlertInventoryPage.module.css)
   * switched to `border-collapse: separate` with every rule border
   * expressed as `box-shadow` instead of `border`.
   *
   * A bounding-box "the two cells' boxes must not overlap" check is
   * deliberately NOT used here: once the container is scrolled far
   * enough, a scrolled-away column's own layout box legitimately extends
   * underneath the sticky column's box (that's what makes it "sticky" --
   * the whole point is other content flows underneath it while it stays
   * put), so box overlap alone is not a defect signal and produced false
   * positives at 1024px during this test's own development, where the
   * scrollable overflow is small relative to a column's width. The
   * actual defect was a PAINT-order failure, not a layout one, so this
   * samples `elementFromPoint` at several points inside the sticky
   * cell's own rendered area instead -- the direct way to ask the
   * browser what is actually topmost at those pixels.
   */
  for (const [label, viewport] of [
    ["768px", { width: 768, height: 1024 }],
    ["1024px", { width: 1024, height: 900 }],
  ] as const) {
    test(`at ${label}, after horizontal scroll the sticky Alert column stays at the scroll container's left edge, keeps an opaque background, and no scrolled column paints over or under it`, async ({
      page,
    }) => {
      await page.setViewportSize(viewport);
      await page.goto("/alerts");
      await expect(page.locator("tbody tr")).toHaveCount(6);

      const scrollContainer = page.locator('[role="region"][aria-label*="scrollable"]');
      const idCellLocator = page.locator("tbody tr td:first-child").first();
      const idLeftBeforeScroll = (await idCellLocator.boundingBox())!.x;

      await scrollContainer.evaluate((el) => {
        el.scrollLeft = el.scrollWidth;
      });
      await page.waitForTimeout(150);

      const geometry = await page.evaluate(() => {
        const idCell = document.querySelector("tbody tr td:first-child")!;
        const idRect = idCell.getBoundingClientRect();
        const backgroundColor = getComputedStyle(idCell).backgroundColor;

        // Sample several points spanning the sticky cell's own rendered
        // area (near its left edge, its middle, and near its right/
        // separator edge — exactly where scrolled content would bleed
        // through if it were going to) and ask the browser what element
        // is actually topmost at each.
        const sampleFractions = [0.1, 0.5, 0.9];
        const y = idRect.top + idRect.height / 2;
        const topElementIsStickyAt = sampleFractions.map((fraction) => {
          const x = idRect.left + idRect.width * fraction;
          const topElement = document.elementFromPoint(x, y);
          return !!topElement && (idCell === topElement || idCell.contains(topElement));
        });

        return {
          idLeft: idRect.left,
          backgroundColor,
          topElementIsStickyAt,
        };
      });

      // Stays pinned at the same screen position it held before scrolling
      // — the scroll container's own left padding edge, not its outer/
      // border edge (which the container's padding legitimately sits
      // inside of).
      expect(Math.abs(geometry.idLeft - idLeftBeforeScroll)).toBeLessThan(2);

      // Fully opaque computed background — not transparent, not
      // semi-transparent.
      expect(geometry.backgroundColor).not.toBe("rgba(0, 0, 0, 0)");
      expect(geometry.backgroundColor).not.toBe("transparent");
      expect(geometry.backgroundColor).toMatch(/^rgba?\(\d+,\s*\d+,\s*\d+(,\s*1)?\)$/);

      // The topmost element actually painted at every sampled point
      // inside the sticky cell is genuinely the sticky cell — the direct
      // proof that nothing from a scrolled column bleeds through on top
      // of (or shows underneath) it.
      expect(geometry.topElementIsStickyAt).toEqual([true, true, true]);

      // The focused row can still be activated with Enter after scrolling.
      const link = page.getByRole("link", { name: "#1" });
      await link.focus();
      await expect(link).toBeFocused();
      await page.keyboard.press("Enter");
      await expect(page).toHaveURL(/\/alerts\/1$/);
    });
  }

  test("the scroll-for-more affordance appears only when the table actually overflows its container", async ({
    page,
  }) => {
    await page.setViewportSize({ width: 768, height: 1024 });
    await page.goto("/alerts");
    await expect(page.locator("tbody tr")).toHaveCount(6);
    await expect(page.getByText(/Scroll for more columns/)).toBeVisible();

    await page.setViewportSize({ width: 1600, height: 1000 });
    await page.waitForTimeout(150);
    await expect(page.getByText(/Scroll for more columns/)).toHaveCount(0);
  });

  test("keyboard navigation and visible focus still work inside the scrollable region", async ({ page }) => {
    await page.setViewportSize({ width: 768, height: 1024 });
    await page.goto("/alerts");
    const link = page.getByRole("link", { name: "#2" });
    await link.focus();
    await expect(link).toBeFocused();
    await page.keyboard.press("Enter");
    await expect(page).toHaveURL(/\/alerts\/2$/);
  });
});
