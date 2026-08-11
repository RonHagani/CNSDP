import { describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { mockJsonResponse, stubFetchRoutes } from "@/test/mockFetch";
import { SubmissionsPage } from "./SubmissionsPage";

function row(submissionId: number, overrides: Record<string, unknown> = {}) {
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

function renderAt(path: string) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[path]}>
        <Routes>
          <Route path="/submissions" element={<SubmissionsPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("SubmissionsPage", () => {
  it("requests GET /v1/submissions through the same-origin /api proxy prefix, with no filter by default", async () => {
    const fetchMock = stubFetchRoutes({
      "/v1/submissions": () => mockJsonResponse(200, { submissions: [pendingRow], nextCursor: null, total: 1 }),
    });
    renderAt("/submissions");
    await screen.findByText("1 of 1 submission");
    expect(fetchMock).toHaveBeenCalledWith("/api/v1/submissions", expect.anything());
  });

  it("renders a loading state before the submissions resolve", () => {
    stubFetchRoutes({ "/v1/submissions": () => new Promise<Response>(() => {}) });
    renderAt("/submissions");
    expect(screen.getByText(/Loading submissions/)).toBeInTheDocument();
  });

  it("renders every row from a successful response, in the server's own order, with a visible count", async () => {
    // Deliberately returned in descending-id order (not ascending): if the
    // page ever re-sorted rows instead of preserving the server's own
    // order, this would be the case that catches it.
    stubFetchRoutes({
      "/v1/submissions": () => mockJsonResponse(200, { submissions: [invalidRow, pendingRow], nextCursor: null, total: 2 }),
    });
    renderAt("/submissions");

    const allRows = await screen.findAllByRole("row");
    const dataRows = allRows.slice(1); // drop the header row
    expect(dataRows).toHaveLength(2);
    expect(screen.getByText("2 of 2 submissions")).toBeInTheDocument();
    expect(within(dataRows[0]).getByText("#2")).toBeInTheDocument();
    expect(within(dataRows[1]).getByText("#1")).toBeInTheDocument();
  });

  it("renders a pending submission with a Pending badge and no fabricated outcome or reason", async () => {
    stubFetchRoutes({
      "/v1/submissions": () => mockJsonResponse(200, { submissions: [pendingRow], nextCursor: null, total: 1 }),
    });
    renderAt("/submissions");

    const dataRow = (await screen.findAllByRole("row"))[1];
    expect(within(dataRow).getByText("Pending")).toBeInTheDocument();
    expect(within(dataRow).getByText("—")).toBeInTheDocument(); // reason column
  });

  it("renders a non-pending submission's outcome badge and reason text exactly as returned", async () => {
    stubFetchRoutes({
      "/v1/submissions": () => mockJsonResponse(200, { submissions: [invalidRow], nextCursor: null, total: 1 }),
    });
    renderAt("/submissions");

    const dataRow = (await screen.findAllByRole("row"))[1];
    expect(within(dataRow).getByText("Invalid")).toBeInTheDocument();
    expect(within(dataRow).getByText("malformed request")).toBeInTheDocument();
  });

  it("renders the empty state from a real empty response, with no rows and no fabricated content", async () => {
    stubFetchRoutes({ "/v1/submissions": () => mockJsonResponse(200, { submissions: [], nextCursor: null, total: 0 }) });
    renderAt("/submissions");
    await screen.findByText(/No submissions have been received yet/);
    expect(screen.queryByRole("row")).not.toBeInTheDocument();
  });

  it("renders an unauthorized state for a backend 401", async () => {
    stubFetchRoutes({ "/v1/submissions": () => mockJsonResponse(401, {}) });
    renderAt("/submissions");
    await screen.findByText("Authentication required");
  });

  it("renders an error state with a retry action for a 5xx, and retry issues a real new request", async () => {
    const user = userEvent.setup();
    const fetchMock = stubFetchRoutes({ "/v1/submissions": () => mockJsonResponse(500, {}) });
    renderAt("/submissions");
    await screen.findByText(/The submissions backend could not be reached/);
    expect(fetchMock).toHaveBeenCalledTimes(1);

    await user.click(screen.getByRole("button", { name: "Retry" }));

    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2));
    expect(screen.getByText(/The submissions backend could not be reached/)).toBeInTheDocument();
  });

  it("renders an error state on a genuine network failure", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new TypeError("Failed to fetch")));
    renderAt("/submissions");
    await screen.findByText(/The submissions backend could not be reached/);
  });

  it("a 400 from an unexpected filter value renders the generic backend-unavailable state, not the auth state", async () => {
    stubFetchRoutes({ "/v1/submissions": () => mockJsonResponse(400, {}) });
    renderAt("/submissions");
    await screen.findByText(/The submissions backend could not be reached/);
  });

  describe("truncated-value tooltips", () => {
    it("exposes the full reason via a title attribute when a reason is present", async () => {
      const longReason =
        "missing operation performed (verb and/or requestURI) required by FR-007(d), a reason long enough to be visually truncated in the Reason column";
      stubFetchRoutes({
        "/v1/submissions": () =>
          mockJsonResponse(200, {
            submissions: [row(3, { validationOutcome: { available: true, outcome: "incomplete", reason: longReason } })],
            nextCursor: null,
            total: 1,
          }),
      });
      renderAt("/submissions");

      const dataRow = (await screen.findAllByRole("row"))[1];
      expect(within(dataRow).getByTitle(longReason)).toBeInTheDocument();
    });

    it("exposes the full audit ID via a title attribute", async () => {
      const longAuditId = "34b75a57-e1c0-4659-a21f-2d39256f018c-a-genuinely-long-audit-identifier";
      stubFetchRoutes({
        "/v1/submissions": () =>
          mockJsonResponse(200, { submissions: [row(4, { auditId: longAuditId })], nextCursor: null, total: 1 }),
      });
      renderAt("/submissions");

      const dataRow = (await screen.findAllByRole("row"))[1];
      expect(within(dataRow).getByTitle(longAuditId)).toBeInTheDocument();
    });

    it("never sets a title attribute on a pending row's empty reason cell", async () => {
      stubFetchRoutes({
        "/v1/submissions": () => mockJsonResponse(200, { submissions: [pendingRow], nextCursor: null, total: 1 }),
      });
      renderAt("/submissions");

      const dataRow = (await screen.findAllByRole("row"))[1];
      expect(within(dataRow).getByText("—")).not.toHaveAttribute("title");
    });

    it("never sets a title attribute when the audit ID is empty", async () => {
      stubFetchRoutes({
        "/v1/submissions": () =>
          mockJsonResponse(200, { submissions: [row(5, { auditId: "" })], nextCursor: null, total: 1 }),
      });
      renderAt("/submissions");

      const dataRow = (await screen.findAllByRole("row"))[1];
      const auditIdCell = within(dataRow).getAllByRole("cell")[2];
      expect(auditIdCell).not.toHaveAttribute("title");
      expect(auditIdCell).toHaveTextContent("");
    });
  });

  describe("filtering", () => {
    it("clicking a filter issues a new request with the matching outcome query param and replaces the rows", async () => {
      const user = userEvent.setup();
      stubFetchRoutes({
        "/v1/submissions": () => mockJsonResponse(200, { submissions: [pendingRow, invalidRow], nextCursor: null, total: 2 }),
        "/v1/submissions?outcome=invalid": () => mockJsonResponse(200, { submissions: [invalidRow], nextCursor: null, total: 1 }),
      });
      renderAt("/submissions");
      await screen.findByText("2 of 2 submissions");

      const invalidFilter = screen.getByRole("button", { name: "Invalid" });
      expect(invalidFilter).toHaveAttribute("aria-pressed", "false");

      await user.click(invalidFilter);

      await screen.findByText("1 of 1 submission");
      expect(invalidFilter).toHaveAttribute("aria-pressed", "true");
      expect(screen.queryByText("#1")).not.toBeInTheDocument(); // pending row is gone
      expect(screen.getByText("#2")).toBeInTheDocument();
    });

    it("the pending filter renders only pending submissions", async () => {
      const user = userEvent.setup();
      stubFetchRoutes({
        "/v1/submissions": () => mockJsonResponse(200, { submissions: [pendingRow, invalidRow], nextCursor: null, total: 2 }),
        "/v1/submissions?outcome=pending": () => mockJsonResponse(200, { submissions: [pendingRow], nextCursor: null, total: 1 }),
      });
      renderAt("/submissions");
      await screen.findByText("2 of 2 submissions");

      await user.click(screen.getByRole("button", { name: "Pending" }));

      await screen.findByText("1 of 1 submission");
      expect(screen.getByText("#1")).toBeInTheDocument();
      expect(screen.queryByText("#2")).not.toBeInTheDocument();
    });
  });

  describe("pagination", () => {
    it("shows Load more only when the backend reports a next cursor, and hides it once exhausted", async () => {
      stubFetchRoutes({
        "/v1/submissions": () => mockJsonResponse(200, { submissions: [pendingRow], nextCursor: 1, total: 2 }),
      });
      renderAt("/submissions");
      await screen.findByText("1 of 2 submissions");
      expect(screen.getByRole("button", { name: "Load more" })).toBeInTheDocument();
    });

    it("does not show Load more when nextCursor is null", async () => {
      stubFetchRoutes({
        "/v1/submissions": () => mockJsonResponse(200, { submissions: [pendingRow], nextCursor: null, total: 1 }),
      });
      renderAt("/submissions");
      await screen.findByText("1 of 1 submission");
      expect(screen.queryByRole("button", { name: "Load more" })).not.toBeInTheDocument();
    });

    it("clicking Load more requests the next keyset page using the previous page's cursor and appends its rows", async () => {
      const user = userEvent.setup();
      const fetchMock = stubFetchRoutes({
        "/v1/submissions": () => mockJsonResponse(200, { submissions: [pendingRow], nextCursor: 1, total: 2 }),
        "/v1/submissions?cursor=1": () => mockJsonResponse(200, { submissions: [invalidRow], nextCursor: null, total: 2 }),
      });
      renderAt("/submissions");
      await screen.findByText("1 of 2 submissions");

      await user.click(screen.getByRole("button", { name: "Load more" }));

      await screen.findByText("2 of 2 submissions");
      expect(fetchMock).toHaveBeenCalledWith("/api/v1/submissions?cursor=1", expect.anything());
      // Both pages' rows are present -- accumulated, not replaced.
      expect(screen.getByText("#1")).toBeInTheDocument();
      expect(screen.getByText("#2")).toBeInTheDocument();
      expect(screen.queryByRole("button", { name: "Load more" })).not.toBeInTheDocument();
    });

    // Each backend page computes its own total independently (no shared
    // transaction across pages), so a later page can legitimately report a
    // different total than an earlier one -- e.g. a new submission arrived
    // between the two fetches. The displayed count must track the newest
    // page's total, not freeze to whatever the first page reported.
    it("displays the newest page's total once it differs from the first page's, after Load more", async () => {
      const user = userEvent.setup();
      stubFetchRoutes({
        "/v1/submissions": () => mockJsonResponse(200, { submissions: [pendingRow], nextCursor: 1, total: 2 }),
        "/v1/submissions?cursor=1": () => mockJsonResponse(200, { submissions: [invalidRow], nextCursor: null, total: 3 }),
      });
      renderAt("/submissions");
      await screen.findByText("1 of 2 submissions");

      await user.click(screen.getByRole("button", { name: "Load more" }));

      await screen.findByText("2 of 3 submissions");
      expect(screen.queryByText("2 of 2 submissions")).not.toBeInTheDocument();
    });
  });
});
