import { describe, expect, it } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { mockJsonResponse, stubFetchRoutes } from "@/test/mockFetch";
import { DataSourcesPage } from "./DataSourcesPage";

function source(overrides: Record<string, unknown> = {}) {
  return {
    id: "audit-events",
    displayName: "Audit Events API",
    endpoint: "POST /v1/audit-events",
    eventCount: 0,
    lastEventAt: null,
    ...overrides,
  };
}

function renderPage() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={["/data-sources"]}>
        <DataSourcesPage />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("DataSourcesPage", () => {
  it("requests GET /v1/data-sources through the same-origin /api proxy prefix", async () => {
    const fetchMock = stubFetchRoutes({
      "/v1/data-sources": () => mockJsonResponse(200, { dataSources: [source()], total: 1 }),
    });
    renderPage();
    await screen.findByText("1 source");
    expect(fetchMock).toHaveBeenCalledWith("/api/v1/data-sources", expect.anything());
  });

  it("renders a loading state before the data sources resolve", () => {
    stubFetchRoutes({ "/v1/data-sources": () => new Promise<Response>(() => {}) });
    renderPage();
    expect(screen.getByText(/Loading data sources/)).toBeInTheDocument();
  });

  it("renders an unauthorized state for a backend 401", async () => {
    stubFetchRoutes({ "/v1/data-sources": () => mockJsonResponse(401, {}) });
    renderPage();
    await screen.findByText("Authentication required");
  });

  it("renders an error state with a retry action for a 5xx", async () => {
    stubFetchRoutes({ "/v1/data-sources": () => mockJsonResponse(500, {}) });
    renderPage();
    await screen.findByText(/The data sources backend could not be reached/);
    expect(screen.getByRole("button", { name: "Retry" })).toBeInTheDocument();
  });

  it("retry performs a real new HTTP request", async () => {
    const user = userEvent.setup();
    const fetchMock = stubFetchRoutes({ "/v1/data-sources": () => mockJsonResponse(500, {}) });
    renderPage();
    await screen.findByText(/The data sources backend could not be reached/);
    expect(fetchMock).toHaveBeenCalledTimes(1);

    await user.click(screen.getByRole("button", { name: "Retry" }));

    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2));
  });

  it("renders the no-data case honestly, with a zero count and no fabricated last-event time", async () => {
    stubFetchRoutes({
      "/v1/data-sources": () => mockJsonResponse(200, { dataSources: [source()], total: 1 }),
    });
    renderPage();
    await screen.findByText("Audit Events API");
    expect(screen.getByText("No events observed")).toBeInTheDocument();
    expect(screen.getByText("0")).toBeInTheDocument();
    expect(screen.getByText("—")).toBeInTheDocument();
    expect(screen.getByText("POST /v1/audit-events")).toBeInTheDocument();
  });

  it("renders the observed case with the exact count and formatted latest timestamp the backend returned", async () => {
    stubFetchRoutes({
      "/v1/data-sources": () =>
        mockJsonResponse(200, {
          dataSources: [source({ eventCount: 42, lastEventAt: "2026-08-02T12:00:00Z" })],
          total: 1,
        }),
    });
    renderPage();
    await screen.findByText("Audit Events API");
    expect(screen.getByText("Events observed")).toBeInTheDocument();
    expect(screen.getByText("42")).toBeInTheDocument();
    expect(screen.getByText("2026-08-02 12:00:00.000 UTC")).toBeInTheDocument();
  });
});

describe("DataSourcesPage — shell", () => {
  it("marks Data Sources as the active navigation destination, alongside the real Detections link and the still-disabled System Health", async () => {
    stubFetchRoutes({
      "/v1/data-sources": () => mockJsonResponse(200, { dataSources: [source()], total: 1 }),
    });
    renderPage();
    await screen.findByRole("heading", { name: "Data Sources" });

    const dataSourcesLink = screen.getByRole("link", { name: "Data Sources" });
    expect(dataSourcesLink).toHaveAttribute("aria-current", "page");

    const detectionsLink = screen.getByRole("link", { name: "Detections" });
    expect(detectionsLink).not.toHaveAttribute("aria-current", "page");

    const systemHealth = screen.getByText("System Health").closest("span");
    expect(systemHealth).toHaveAttribute("aria-disabled", "true");
  });
});
