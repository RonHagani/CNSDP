import { test, expect } from "@playwright/test";
import { mockSubmissionsBody, mockSubmissionsStatus } from "./support/mockApi";
import type { SubmissionListItem, SubmissionsResponse } from "../src/types/contract";

/**
 * Real-browser coverage for `/submissions` — a keyset-paginated, optionally
 * outcome-filtered, read-only review of every submission received at the
 * defined intake (FR-011, FR-012, FR-013). The production frontend calls
 * the real `GET /v1/submissions` endpoint (src/lib/api/client.ts, proxied
 * same-origin by Vite — see vite.config.ts); `mockSubmissionsBody`/
 * `mockSubmissionsStatus` (e2e/support/mockApi.ts) stand in for that backend
 * at the network layer, mirroring alert-inventory.spec.ts's own pattern.
 *
 * Filtering and pagination are exercised here as real click -> real HTTP
 * request round trips (jsdom can only prove the request *shape*, not that a
 * real browser click genuinely triggers it) — but the underlying
 * state/rendering logic for each outcome and page transition is already
 * exhaustively covered at the unit level (SubmissionsPage.test.tsx), so this
 * suite stays to one representative case each rather than re-deriving that
 * matrix. The horizontal scroll hint is real-browser-only by construction:
 * useHorizontalOverflow's own doc comment notes jsdom performs no real
 * layout, so `hasOverflow` always resolves false under vitest.
 */

function row(submissionId: number, overrides: Partial<SubmissionListItem> = {}): SubmissionListItem {
  return {
    submissionId,
    status: "validated",
    auditId: `audit-${submissionId}`,
    auditStage: "ResponseComplete",
    createdAt: "2026-08-02T12:00:00Z",
    validationOutcome: { available: true, outcome: "valid" },
    ...overrides,
  };
}

const pendingRow = row(1, { status: "admitted", validationOutcome: { available: false } });
const invalidRow = row(2, { validationOutcome: { available: true, outcome: "invalid", reason: "malformed request" } });

const TWO_ROWS: SubmissionsResponse = { submissions: [pendingRow, invalidRow], nextCursor: null, total: 2 };

test.describe("Submissions — review list", () => {
  test("renders real backend content with a visible count", async ({ page }) => {
    await mockSubmissionsBody(page, TWO_ROWS);
    await page.goto("/submissions");

    await expect(page.getByRole("heading", { name: "Submissions" })).toBeVisible();
    await expect(page.getByText("2 of 2 submissions")).toBeVisible();
    await expect(page.getByText("#1")).toBeVisible();
    await expect(page.getByText("#2")).toBeVisible();
    // Scoped to table cells, not plain text: "Pending"/"Invalid" are also
    // filter-bar button labels (FilterBar renders one button per
    // SubmissionOutcomeFilter value), so an unscoped getByText match is
    // ambiguous between the two.
    await expect(page.getByRole("cell", { name: "Pending" })).toBeVisible();
    await expect(page.getByRole("cell", { name: "Invalid" })).toBeVisible();
    await expect(page.getByText("malformed request")).toBeVisible();
  });

  test("renders the empty state from a real empty response", async ({ page }) => {
    await mockSubmissionsBody(page, { submissions: [], nextCursor: null, total: 0 });
    await page.goto("/submissions");
    await expect(page.getByText(/No submissions have been received yet/)).toBeVisible();
  });

  test("is reachable from the primary navigation and is marked the active destination", async ({ page }) => {
    await mockSubmissionsBody(page, TWO_ROWS);
    await page.goto("/alerts");

    await page.getByRole("link", { name: "Submissions" }).click();

    await expect(page).toHaveURL(/\/submissions$/);
    await expect(page.getByRole("heading", { name: "Submissions" })).toBeVisible();
    await expect(page.getByRole("link", { name: "Submissions" })).toHaveAttribute("aria-current", "page");
  });

  test("the loading state is reachable before the response resolves", async ({ page }) => {
    await page.route("**/api/v1/submissions", () => new Promise(() => {})); // never resolves
    await page.goto("/submissions");
    await expect(page.getByText(/Loading submissions/)).toBeVisible();
  });

  test("renders an unauthorized state for a 401", async ({ page }) => {
    await mockSubmissionsStatus(page, 401);
    await page.goto("/submissions");
    await expect(page.getByText("Authentication required")).toBeVisible();
  });

  test("the error state renders with a working retry action for a 5xx", async ({ page }) => {
    await mockSubmissionsStatus(page, 500);
    await page.goto("/submissions");
    await expect(page.getByText(/The submissions backend could not be reached/)).toBeVisible();

    const retry = page.getByRole("button", { name: "Retry" });
    await expect(retry).toBeVisible();
    await retry.click();
    await expect(page.getByText(/The submissions backend could not be reached/)).toBeVisible();
  });

  test("clicking a filter issues a real request for that outcome and replaces the rows", async ({ page }) => {
    await mockSubmissionsBody(page, TWO_ROWS);
    await mockSubmissionsBody(page, { submissions: [invalidRow], nextCursor: null, total: 1 }, "outcome=invalid");
    await page.goto("/submissions");
    await expect(page.getByText("2 of 2 submissions")).toBeVisible();

    await page.getByRole("button", { name: "Invalid" }).click();

    await expect(page.getByText("1 of 1 submission")).toBeVisible();
    await expect(page.getByText("#1")).toHaveCount(0);
    await expect(page.getByText("#2")).toBeVisible();
  });

  test("Load more requests the next keyset page and appends its rows", async ({ page }) => {
    await mockSubmissionsBody(page, { submissions: [pendingRow], nextCursor: 1, total: 2 });
    await mockSubmissionsBody(page, { submissions: [invalidRow], nextCursor: null, total: 2 }, "cursor=1");
    await page.goto("/submissions");
    await expect(page.getByText("1 of 2 submissions")).toBeVisible();

    await page.getByRole("button", { name: "Load more" }).click();

    await expect(page.getByText("2 of 2 submissions")).toBeVisible();
    await expect(page.getByText("#1")).toBeVisible();
    await expect(page.getByText("#2")).toBeVisible();
    await expect(page.getByRole("button", { name: "Load more" })).toHaveCount(0);
  });

  test("the scroll-for-more affordance appears only when the table actually overflows its container", async ({
    page,
  }) => {
    await mockSubmissionsBody(page, TWO_ROWS);

    await page.setViewportSize({ width: 768, height: 1024 });
    await page.goto("/submissions");
    await expect(page.getByText("2 of 2 submissions")).toBeVisible();
    await expect(page.getByText(/Scroll for more columns/)).toBeVisible();

    await page.setViewportSize({ width: 1600, height: 1000 });
    await page.waitForTimeout(150);
    await expect(page.getByText(/Scroll for more columns/)).toHaveCount(0);
  });

  test("renders without horizontal page overflow at a standard viewport", async ({ page }) => {
    await mockSubmissionsBody(page, TWO_ROWS);
    await page.setViewportSize({ width: 1024, height: 900 });
    await page.goto("/submissions");

    await expect(page.getByRole("heading", { name: "Submissions" })).toBeVisible();
    const hasOverflow = await page.evaluate(
      () => document.documentElement.scrollWidth > document.documentElement.clientWidth,
    );
    expect(hasOverflow).toBe(false);
  });
});
