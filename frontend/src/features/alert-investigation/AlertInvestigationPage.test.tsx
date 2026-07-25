import { describe, expect, it } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { AlertInvestigationPage } from "./AlertInvestigationPage";

function renderAt(path: string) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[path]}>
        <Routes>
          <Route path="/alerts/:alertId" element={<AlertInvestigationPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("AlertInvestigationPage — required route-level states", () => {
  it("renders the loading state before data resolves", () => {
    renderAt("/alerts/1?demo=slow");
    expect(screen.getByText(/Retrieving alert investigation record/)).toBeInTheDocument();
  });

  it("renders the full Causal Evidence Dossier for the intact fixture (valid traceability)", async () => {
    renderAt("/alerts/1");
    await screen.findAllByText(/Alert #1\b/);
    await screen.findByText("Traceability verified.");
    // All six register entries render, none marked unavailable.
    const registerSection = screen.getByRole("heading", { name: "Evidence register" }).closest("section")!;
    for (const label of [
      "Source submission",
      "Validation outcome",
      "Normalized event",
      "Detection definition",
      "Detection result",
      "Generated alert",
    ]) {
      expect(
        within(registerSection).getByRole("button", { name: new RegExp(label, "i") }),
      ).toBeInTheDocument();
    }
  });

  it("renders partial artifact availability distinctly, with the gap named explicitly", async () => {
    renderAt("/alerts/2");
    await screen.findAllByText(/Alert #2\b/);
    await waitFor(() =>
      expect(screen.getByText(/Detection definition is not available/i)).toBeInTheDocument(),
    );
    // The other five artifacts remain unaffected.
    expect(screen.getByRole("button", { name: /Source submission/i })).toBeInTheDocument();
  });

  it("renders broken traceability with the specific failed link named", async () => {
    renderAt("/alerts/3");
    await screen.findAllByText(/Alert #3\b/);
    await screen.findByText("Traceability broken.");
    await waitFor(() => expect(screen.getByText("Failed link: raw_event_sha256")).toBeInTheDocument());
  });

  it("renders broken traceability (source_key) with the specific failed link named", async () => {
    renderAt("/alerts/6");
    await screen.findAllByText(/Alert #6\b/);
    await screen.findByText("Traceability broken.");
    await waitFor(() => expect(screen.getByText("Failed link: source_key")).toBeInTheDocument());
  });

  it("renders a not-found state for an unknown alert id", async () => {
    renderAt("/alerts/999");
    await waitFor(() =>
      expect(screen.getByText(/No alert exists with id 999/)).toBeInTheDocument(),
    );
  });

  it("renders an unauthorized state under the unauthorized demo scenario", async () => {
    renderAt("/alerts/1?demo=unauthorized");
    await waitFor(() => expect(screen.getByText("Authentication required")).toBeInTheDocument());
  });

  it("renders an unavailable-backend state with a retry action", async () => {
    renderAt("/alerts/1?demo=unavailable");
    await waitFor(() =>
      expect(
        screen.getByText("The investigation backend could not be reached"),
      ).toBeInTheDocument(),
    );
    expect(screen.getByRole("button", { name: "Retry" })).toBeInTheDocument();
  });

  it("never renders fabricated alert content in a route-level failure state", async () => {
    renderAt("/alerts/999");
    await waitFor(() =>
      expect(screen.getByText(/No alert exists with id 999/)).toBeInTheDocument(),
    );
    expect(screen.queryByRole("heading", { name: "Finding" })).not.toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "Evidence register" })).not.toBeInTheDocument();
  });
});

describe("AlertInvestigationPage — no legacy presentation in the live DOM", () => {
  it("renders none of the rejected legacy vocabulary or attributes", async () => {
    const { container } = renderAt("/alerts/1");
    await screen.findAllByText(/Alert #1\b/);
    const text = container.textContent!.toLowerCase();
    for (const forbidden of ["signal path", "stage rail", "conduit", "exhibit procession", "decoder strip", "evidence theater"]) {
      expect(text).not.toContain(forbidden);
    }
    expect(container.querySelector('[data-role="conduit-field"]')).toBeNull();
    expect(container.querySelector('[data-role="evidence-ribbon"]')).toBeNull();
    expect(container.querySelector('[data-role="proof-chain"]')).toBeNull();
  });
});
