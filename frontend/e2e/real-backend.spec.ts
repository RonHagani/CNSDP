import { test, expect } from "@playwright/test";

/**
 * The one spec in this suite that installs no network interception at
 * all. Every other e2e file mocks `/api/v1/alerts*` (e2e/support/mockApi.ts)
 * so it can run without infrastructure; this file instead runs against a
 * real, already-running backend (docs/reference-environment.md's
 * docker-compose stack) through Vite preview's real `/api` proxy
 * (vite.config.ts) — proving the frontend genuinely fetches from a live
 * backend, not merely that Playwright can fake a response shaped like
 * one.
 *
 * Prerequisites (see the corrective-pass report for the exact commands
 * used to set this up): `docker compose up -d --build` from the
 * repository root with a `.env` whose `API_TOKEN` matches
 * `frontend/.env`'s `API_PROXY_TOKEN` (server-side dev-proxy
 * configuration — never `VITE_`-prefixed; see `.env.example`), then
 * three real events submitted
 * through `POST /v1/audit-events` and processed by the worker loop to
 * `evidenced` — the committed scenario-1 eventlist fixture
 * (internal/intake/testdata/scenario-1-eventlist.json) plus scenario-2
 * and scenario-3's single-event testdata
 * (internal/validation/testdata/scenario{2,3}-valid.json, each wrapped
 * in an EventList envelope) — landing on a fresh database as alerts 1,
 * 2, and 3 respectively. If the backend isn't reachable, this test fails
 * with a clear timeout on real content rather than silently passing.
 */

test.describe("Real backend — primary successful journey", () => {
  test("open /alerts, open a non-default alert, confirm real content, return, open a different alert, confirm isolation", async ({
    page,
  }) => {
    await page.goto("/alerts");

    const rows = page.locator("tbody tr");
    await expect(rows.first()).toBeVisible({ timeout: 15_000 });
    const count = await rows.count();
    expect(count).toBeGreaterThanOrEqual(3);
    await expect(page.getByText(`${count} alert${count === 1 ? "" : "s"}`)).toBeVisible();

    // Real backend content — the seeded scenario-2 and scenario-3
    // detections, never a bundled fixture string.
    await expect(page.getByText("High-risk Pod creation")).toBeVisible();
    await expect(page.getByText("Cluster-admin ClusterRoleBinding grant")).toBeVisible();

    // Open a non-default alert (#2 — not the route's own alertId="1"
    // fallback) and confirm its real, backend-composed investigation
    // content renders.
    await page.getByRole("link", { name: "#2" }).click();
    await expect(page).toHaveURL(/\/alerts\/2$/);
    await expect(page.getByText("Alert #2", { exact: false }).first()).toBeVisible();
    await expect(page.getByText("High-risk Pod creation", { exact: false }).first()).toBeVisible();
    await expect(page.getByText("Traceability intact").first()).toBeVisible();
    await expect(page.getByRole("button", { name: /privileged_container/i })).toBeVisible();

    // Establish selection state that must not survive a navigation to a
    // different alert.
    await page.getByRole("button", { name: /privileged_container/i }).click();
    await expect(page.getByRole("group", { name: /Provenance record/i })).toBeVisible();

    await page.getByRole("link", { name: /Back to alerts/ }).click();
    await expect(page).toHaveURL(/\/alerts$/);

    await page.getByRole("link", { name: "#3" }).click();
    await expect(page).toHaveURL(/\/alerts\/3$/);
    await expect(page.getByText("Alert #3", { exact: false }).first()).toBeVisible();
    await expect(
      page.getByText("Cluster-admin ClusterRoleBinding grant", { exact: false }).first(),
    ).toBeVisible();
    await expect(page.getByText("Traceability intact").first()).toBeVisible();

    // No pin preselected, and no leftover provenance record carried over
    // from alert 2's selection.
    await expect(page.getByRole("group", { name: /Provenance record/i })).toHaveCount(0);
  });
});

test.describe("Real backend — credential boundary", () => {
  test("the browser's own request to /api/v1/alerts carries no client-constructed Authorization header", async ({
    page,
  }) => {
    const capturedRequests: Record<string, string>[] = [];
    page.on("request", (request) => {
      if (request.url().includes("/api/v1/alerts")) {
        capturedRequests.push(request.headers());
      }
    });

    await page.goto("/alerts");
    await expect(page.locator("tbody tr").first()).toBeVisible({ timeout: 15_000 });

    // At least one request was actually captured — this assertion is not
    // vacuously true just because nothing was observed.
    expect(capturedRequests.length).toBeGreaterThan(0);
    for (const headers of capturedRequests) {
      expect(headers).not.toHaveProperty("authorization");
    }
  });

  test("a forged browser-supplied Authorization header is overwritten by the dev proxy, never forwarded to the real backend", async ({
    page,
  }) => {
    // Injects a forged Authorization header onto the outgoing browser
    // request itself, before it leaves the page — proving the defense
    // holds even against a page that actively tries to send its own
    // credential, not just one that passively sends none.
    await page.route("**/api/v1/alerts", async (route) => {
      await route.continue({
        headers: {
          ...route.request().headers(),
          authorization: "Bearer forged-browser-value-should-never-reach-backend",
        },
      });
    });

    await page.goto("/alerts");

    // If the forged header had actually reached the real backend
    // unchanged, the backend would reject it with 401 and this route
    // would render "Authentication required" instead of real rows — the
    // real content rendering successfully is the direct proof that the
    // proxy's own server-side credential is what actually reached the
    // backend, not the forged one.
    await expect(page.locator("tbody tr").first()).toBeVisible({ timeout: 15_000 });
    await expect(page.getByText("Authentication required")).toHaveCount(0);
  });
});
