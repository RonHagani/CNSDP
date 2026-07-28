import { describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { mockJsonResponse, stubFetchRoutes } from "@/test/mockFetch";
import { AlertInventoryPage } from "./AlertInventoryPage";

function row(alertId: number, overrides: Record<string, unknown> = {}) {
  return {
    alertId,
    detectionName: "Interactive container exec request",
    subject: { username: "kubernetes-admin" },
    operation: { verb: "get", requestURI: "/api/v1/namespaces/default/pods/high-risk-pod/exec" },
    target: { resource: "pods", name: "high-risk-pod", subresource: "exec" },
    outcome: { code: 101 },
    requestTime: "2026-07-21T15:25:11.901891Z",
    traceability: { intact: true },
    ...overrides,
  };
}

const THREE_ROWS = [row(1), row(2), row(3, { traceability: { intact: false, failedLink: "raw_event_sha256" } })];

function renderAt(path: string) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[path]}>
        <Routes>
          <Route path="/alerts" element={<AlertInventoryPage />} />
          <Route path="/alerts/:alertId" element={<div>investigation screen</div>} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("AlertInventoryPage", () => {
  it("requests GET /v1/alerts through the same-origin /api proxy prefix", async () => {
    const fetchMock = stubFetchRoutes({
      "/v1/alerts": () => mockJsonResponse(200, { alerts: THREE_ROWS, total: 3 }),
    });
    renderAt("/alerts");
    await screen.findByText("3 alerts");
    expect(fetchMock).toHaveBeenCalledWith("/api/v1/alerts", expect.anything());
  });

  it("renders a loading state before the inventory resolves", () => {
    stubFetchRoutes({ "/v1/alerts": () => new Promise<Response>(() => {}) });
    renderAt("/alerts");
    expect(screen.getByText(/Loading alert inventory/)).toBeInTheDocument();
  });

  it("renders every row from a successful API response, in the server's own order, with a visible total count", async () => {
    stubFetchRoutes({ "/v1/alerts": () => mockJsonResponse(200, { alerts: THREE_ROWS, total: 3 }) });
    renderAt("/alerts");

    const allRows = await screen.findAllByRole("row");
    expect(allRows.slice(1)).toHaveLength(3); // drop the header row
    expect(screen.getByText("3 alerts")).toBeInTheDocument();

    const ids = screen
      .getAllByRole("link")
      .map((link) => link.textContent)
      .filter((text): text is string => !!text && text.startsWith("#"))
      .map((text) => Number(text.slice(1)));
    expect(ids).toEqual([1, 2, 3]);
  });

  it("renders the exact content the backend returned, never a placeholder", async () => {
    stubFetchRoutes({
      "/v1/alerts": () =>
        mockJsonResponse(200, {
          alerts: [row(7, { detectionName: "High-risk Pod creation" })],
          total: 1,
        }),
    });
    renderAt("/alerts");
    await screen.findByText("High-risk Pod creation");
    expect(screen.getByRole("link", { name: "#7" })).toBeInTheDocument();
  });

  it("renders the empty state from a real empty API response, with no rows and no fabricated content", async () => {
    stubFetchRoutes({ "/v1/alerts": () => mockJsonResponse(200, { alerts: [], total: 0 }) });
    renderAt("/alerts");
    await screen.findByText(/No alerts have been generated yet/);
    expect(screen.queryByRole("row")).not.toBeInTheDocument();
  });

  it("renders an unauthorized state for a backend 401", async () => {
    stubFetchRoutes({ "/v1/alerts": () => mockJsonResponse(401, {}) });
    renderAt("/alerts");
    await screen.findByText("Authentication required");
  });

  it("renders an error state with a retry action for a 5xx", async () => {
    stubFetchRoutes({ "/v1/alerts": () => mockJsonResponse(500, {}) });
    renderAt("/alerts");
    await screen.findByText(/The alert inventory backend could not be reached/);
    expect(screen.getByRole("button", { name: "Retry" })).toBeInTheDocument();
  });

  it("renders an error state on a genuine network failure", async () => {
    // stubFetchRoutes always resolves; a network failure needs its own stub.
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new TypeError("Failed to fetch")));
    renderAt("/alerts");
    await screen.findByText(/The alert inventory backend could not be reached/);
  });

  it("retry performs a real new HTTP request", async () => {
    const user = userEvent.setup();
    const fetchMock = stubFetchRoutes({ "/v1/alerts": () => mockJsonResponse(500, {}) });
    renderAt("/alerts");
    await screen.findByText(/The alert inventory backend could not be reached/);
    expect(fetchMock).toHaveBeenCalledTimes(1);

    await user.click(screen.getByRole("button", { name: "Retry" }));

    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2));
    // Still failing (same stub), so the same state persists — proving the
    // click issued a real second request rather than being inert.
    expect(screen.getByText(/The alert inventory backend could not be reached/)).toBeInTheDocument();
  });

  it("a row is keyboard-focusable and Enter navigates via the row's own link", async () => {
    const user = userEvent.setup();
    stubFetchRoutes({ "/v1/alerts": () => mockJsonResponse(200, { alerts: THREE_ROWS, total: 3 }) });
    renderAt("/alerts");
    const link = await screen.findByRole("link", { name: "#1" });

    link.focus();
    expect(link).toHaveFocus();
    await user.keyboard("{Enter}");

    expect(await screen.findByText("investigation screen")).toBeInTheDocument();
  });

  it("clicking a row navigates to its investigation screen", async () => {
    const user = userEvent.setup();
    stubFetchRoutes({ "/v1/alerts": () => mockJsonResponse(200, { alerts: THREE_ROWS, total: 3 }) });
    renderAt("/alerts");
    await user.click(await screen.findByRole("link", { name: "#2" }));
    expect(await screen.findByText("investigation screen")).toBeInTheDocument();
  });
});

describe("AlertInventoryPage — shell", () => {
  it("renders the product identity and an active Alerts destination alongside disabled future destinations", async () => {
    stubFetchRoutes({ "/v1/alerts": () => mockJsonResponse(200, { alerts: THREE_ROWS, total: 3 }) });
    renderAt("/alerts");
    await screen.findByRole("heading", { name: "Alerts" });

    expect(screen.getByText("CNSDP")).toBeInTheDocument();
    const alertsLink = screen.getByRole("link", { name: "Alerts" });
    expect(alertsLink).toHaveAttribute("aria-current", "page");

    for (const label of ["Detections", "Data Sources", "System Health"]) {
      const disabled = screen.getByText(label).closest("span");
      expect(disabled).toHaveAttribute("aria-disabled", "true");
    }
  });
});
