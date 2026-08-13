import { test, expect } from "@playwright/test";
import { mockDataSourcesBody, mockDataSourcesStatus } from "./support/mockApi";
import type { DataSourcesResponse } from "../src/types/contract";

/**
 * Real-browser coverage for `/data-sources` — a read-only, single-channel
 * retrospective summary (FR-036). The production frontend calls the real
 * `GET /v1/data-sources` endpoint (src/lib/api/client.ts, proxied
 * same-origin by Vite — see vite.config.ts); `mockDataSourcesBody`/
 * `mockDataSourcesStatus` (e2e/support/mockApi.ts) stand in for that backend
 * at the network layer, mirroring alert-inventory.spec.ts's own pattern.
 *
 * Deliberately excludes an "empty result" case: unlike Detections or
 * Submissions, the real backend (internal/datasources/datasources.go) always
 * returns exactly one data source row, whether or not it has admitted any
 * events yet — there is no product-defined empty-list state to test here.
 * Detailed rendering logic (zero-count formatting, timestamp formatting) is
 * already covered at the unit level (DataSourcesPage.test.tsx); this suite
 * only proves what jsdom cannot: real routing, real layout, and real
 * browser-rendered states.
 */

const SUCCESS_BODY: DataSourcesResponse = {
  dataSources: [
    {
      id: "audit-events",
      displayName: "Audit Events API",
      endpoint: "POST /v1/audit-events",
      eventCount: 42,
      lastEventAt: "2026-08-02T12:00:00Z",
    },
  ],
  total: 1,
};

test.describe("Data Sources — primary entry", () => {
  test("renders the real route with backend-sourced content and a visible count", async ({ page }) => {
    await mockDataSourcesBody(page, SUCCESS_BODY);
    await page.goto("/data-sources");

    await expect(page.getByRole("heading", { name: "Data Sources" })).toBeVisible();
    await expect(page.getByText("1 source")).toBeVisible();
    await expect(page.getByRole("heading", { name: "Audit Events API" })).toBeVisible();
    await expect(page.getByText("POST /v1/audit-events")).toBeVisible();
    await expect(page.getByText("Events observed")).toBeVisible();
    await expect(page.getByText("42")).toBeVisible();
  });

  test("is reachable from the primary navigation and is marked the active destination", async ({ page }) => {
    await mockDataSourcesBody(page, SUCCESS_BODY);
    await page.goto("/alerts");

    await page.getByRole("link", { name: "Data Sources" }).click();

    await expect(page).toHaveURL(/\/data-sources$/);
    await expect(page.getByRole("heading", { name: "Data Sources" })).toBeVisible();
    await expect(page.getByRole("link", { name: "Data Sources" })).toHaveAttribute("aria-current", "page");
  });

  test("the loading state is reachable before the response resolves", async ({ page }) => {
    await page.route("**/api/v1/data-sources", () => new Promise(() => {})); // never resolves
    await page.goto("/data-sources");
    await expect(page.getByText(/Loading data sources/)).toBeVisible();
  });

  test("renders an unauthorized state for a 401", async ({ page }) => {
    await mockDataSourcesStatus(page, 401);
    await page.goto("/data-sources");
    await expect(page.getByText("Authentication required")).toBeVisible();
  });

  test("the error state renders with a working retry action for a 5xx", async ({ page }) => {
    await mockDataSourcesStatus(page, 500);
    await page.goto("/data-sources");
    await expect(page.getByText(/The data sources backend could not be reached/)).toBeVisible();

    const retry = page.getByRole("button", { name: "Retry" });
    await expect(retry).toBeVisible();
    await retry.click();
    // Still failing (same stub), so the same state persists — proves the
    // click genuinely re-issued the request rather than being inert.
    await expect(page.getByText(/The data sources backend could not be reached/)).toBeVisible();
  });

  test("renders without horizontal page overflow at a standard viewport", async ({ page }) => {
    await mockDataSourcesBody(page, SUCCESS_BODY);
    await page.setViewportSize({ width: 1024, height: 900 });
    await page.goto("/data-sources");

    await expect(page.getByRole("heading", { name: "Data Sources" })).toBeVisible();
    const hasOverflow = await page.evaluate(
      () => document.documentElement.scrollWidth > document.documentElement.clientWidth,
    );
    expect(hasOverflow).toBe(false);
  });
});
