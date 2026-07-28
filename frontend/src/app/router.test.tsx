import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { RouterProvider, createMemoryRouter } from "react-router-dom";
import { fixtureIntact, fixtureScenario2Intact } from "@/fixtures/alert-investigation/v1";
import type { AlertInvestigationResponse, AlertSummaryListItem } from "@/types/contract";
import { mockJsonResponse, stubFetchRoutes } from "@/test/mockFetch";
import { routes } from "./router";

/** Projects a full investigation fixture into the lightweight summary row
 *  shape GET /v1/alerts actually returns — test-only use of fixture data
 *  (see alertInventorySource.ts's own doc comment), never production
 *  wiring. */
function summaryRow(fixture: AlertInvestigationResponse): AlertSummaryListItem {
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

function stubBackend() {
  return stubFetchRoutes({
    "/v1/alerts/1": () => mockJsonResponse(200, fixtureIntact),
    "/v1/alerts/4": () => mockJsonResponse(200, fixtureScenario2Intact),
    "/v1/alerts": () =>
      mockJsonResponse(200, {
        alerts: [summaryRow(fixtureIntact), summaryRow(fixtureScenario2Intact)],
        total: 2,
      }),
  });
}

function renderRouterAt(initialEntry: string) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(routes, { initialEntries: [initialEntry] });
  render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return router;
}

describe("app router — navigation flow", () => {
  it("redirects `/` to `/alerts` and renders the alert inventory from the backend", async () => {
    stubBackend();
    const router = renderRouterAt("/");
    await screen.findByRole("heading", { name: "Alerts" });
    expect(router.state.location.pathname).toBe("/alerts");
    expect(await screen.findByText("2 alerts")).toBeInTheDocument();
  });

  it("renders the alert investigation screen for a direct /alerts/:alertId visit", async () => {
    stubBackend();
    renderRouterAt("/alerts/1");
    await screen.findAllByText(/Alert #1\b/);
  });

  it("renders a not-found page for a path that matches no route", async () => {
    stubBackend();
    renderRouterAt("/this-path-does-not-exist");
    await screen.findByText("This page does not exist.");
  });

  it("selecting a row in the inventory navigates to its investigation screen", async () => {
    stubBackend();
    const user = userEvent.setup();
    const router = renderRouterAt("/alerts");
    const link = await screen.findByRole("link", { name: "#1" });
    await user.click(link);

    await screen.findAllByText(/Alert #1\b/);
    expect(router.state.location.pathname).toBe("/alerts/1");
  });

  it("the investigation screen's 'Back to alerts' action returns to the inventory", async () => {
    stubBackend();
    const user = userEvent.setup();
    const router = renderRouterAt("/alerts/1");
    await screen.findAllByText(/Alert #1\b/);

    await user.click(screen.getByRole("link", { name: /Back to alerts/ }));

    await screen.findByRole("heading", { name: "Alerts" });
    expect(router.state.location.pathname).toBe("/alerts");
  });

  it("switching from one alert to a different alert via the inventory leaves no prior provenance selection behind", async () => {
    stubBackend();
    const user = userEvent.setup();
    renderRouterAt("/alerts");

    await user.click(await screen.findByRole("link", { name: "#1" }));
    await screen.findAllByText(/Alert #1\b/);
    await user.click(screen.getByRole("button", { name: /tty_allocation/ }));
    expect(screen.getByRole("group", { name: /Provenance record/i })).toBeInTheDocument();

    await user.click(screen.getByRole("link", { name: /Back to alerts/ }));
    await screen.findByRole("heading", { name: "Alerts" });

    await user.click(await screen.findByRole("link", { name: "#4" }));
    await screen.findAllByText(/Alert #4\b/);

    expect(screen.queryByRole("group", { name: /Provenance record/i })).not.toBeInTheDocument();
  });
});
