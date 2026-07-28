import type { Page } from "@playwright/test";
import { fixturesById } from "../../src/fixtures/alert-investigation/v1";
import type { AlertInvestigationResponse, AlertSummaryListItem } from "../../src/types/contract";

/**
 * Test-only network interception for the production frontend's real API
 * layer (src/lib/api/client.ts calls same-origin `/api/v1/...`, proxied
 * by Vite to a real backend). This is exactly the "isolated test adapter
 * / explicit test-only dependency injection" boundary the corrective
 * pass calls out as an acceptable place for fixture data to keep living
 * — the production modules themselves import no fixture (see
 * src/test/noFixtureImports.test.ts).
 */

function projectSummary(fixture: AlertInvestigationResponse): AlertSummaryListItem {
  const summary = fixture.alert.summary!;
  return {
    alertId: fixture.alertId,
    detectionName: summary.matchReason.definitionName,
    subject: summary.subject,
    operation: summary.operation,
    target: summary.target,
    outcome: summary.outcome,
    requestTime: summary.requestTime,
    traceability: fixture.traceability,
  };
}

/**
 * Installs a default mock backend covering every committed fixture id
 * (1-6) plus the inventory list projected from them — a faithful stand-in
 * for a real backend serving exactly that seeded data, registered once so
 * individual tests don't need to repeat it. Any alert id outside the
 * fixture set resolves 404, matching the real backend's own behavior for
 * a nonexistent alert. Individual tests override a specific route
 * afterwards (Playwright resolves the most-recently-registered matching
 * route first) to exercise a failure state for that one request.
 */
export async function mockFixtureBackend(page: Page) {
  await page.route("**/api/v1/alerts/*", (route) => route.fulfill({ status: 404, body: "" }));
  await page.route("**/api/v1/alerts", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        alerts: Object.values(fixturesById)
          .filter((f) => f.alert.available)
          .map(projectSummary)
          .sort((a, b) => a.alertId - b.alertId),
        total: Object.values(fixturesById).filter((f) => f.alert.available).length,
      }),
    }),
  );
  for (const fixture of Object.values(fixturesById)) {
    await page.route(`**/api/v1/alerts/${fixture.alertId}`, (route) =>
      route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(fixture) }),
    );
  }
}

/** Overrides a single alert id's response for the current test only —
 *  registered after `mockFixtureBackend`, so it wins for that one id. */
export async function mockAlertStatus(page: Page, alertId: number | string, status: number) {
  await page.route(`**/api/v1/alerts/${alertId}`, (route) => route.fulfill({ status, body: "" }));
}

/** Overrides the inventory list endpoint's response for the current test. */
export async function mockAlertListStatus(page: Page, status: number) {
  await page.route("**/api/v1/alerts", (route) => route.fulfill({ status, body: "" }));
}

export async function mockAlertListBody(page: Page, body: { alerts: AlertSummaryListItem[]; total: number }) {
  await page.route("**/api/v1/alerts", (route) =>
    route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(body) }),
  );
}
