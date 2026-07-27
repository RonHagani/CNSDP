import { describe, expect, it } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
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

  it("renders the full Dark Evidence Map for the intact fixture (valid traceability)", async () => {
    renderAt("/alerts/1");
    await screen.findAllByText(/Alert #1\b/);
    expect(screen.getByText("Traceability intact")).toBeInTheDocument();
    // All six evidence artifacts render, none marked unavailable.
    for (const label of [
      "Source submission",
      "Normalized event",
      "Detection definition",
      "Detection result",
      "Generated alert",
    ]) {
      expect(screen.getByRole("heading", { name: label })).toBeInTheDocument();
    }
    expect(screen.getByRole("group", { name: "Validation outcome" })).toBeInTheDocument();
    expect(screen.queryByText(/unavailable/i)).not.toBeInTheDocument();
  });

  it("selecting a satisfied characteristic pin reveals its Verified provenance record", async () => {
    const user = userEvent.setup();
    renderAt("/alerts/1");
    await screen.findAllByText(/Alert #1\b/);
    await user.click(screen.getByRole("button", { name: /tty_allocation/ }));
    const record = screen.getByRole("group", { name: /Provenance record/i });
    expect(record).toBeInTheDocument();
    expect(within(record).getByText(/Verified/)).toBeInTheDocument();
  });

  it("selecting a satisfied characteristic pin reveals its Partial provenance record (scenario 2)", async () => {
    const user = userEvent.setup();
    renderAt("/alerts/4");
    await screen.findAllByText(/Alert #4\b/);
    await user.click(screen.getByRole("button", { name: /privileged_container/ }));
    const record = screen.getByRole("group", { name: /Provenance record/i });
    expect(record).toBeInTheDocument();
    expect(within(record).getByText(/Partial/)).toBeInTheDocument();
  });

  it("supports keyboard-only selection: Tab-focusing a pin and activating with Enter", async () => {
    const user = userEvent.setup();
    renderAt("/alerts/1");
    await screen.findAllByText(/Alert #1\b/);
    const pin = screen.getByRole("button", { name: /stdin_streaming/ });
    pin.focus();
    await user.keyboard("{Enter}");
    expect(screen.getByRole("group", { name: /Provenance record/i })).toBeInTheDocument();
  });

  it("renders scenario 3's real cluster-admin grant identity", async () => {
    renderAt("/alerts/5");
    await screen.findAllByText(/Alert #5\b/);
    expect(screen.getByRole("button", { name: /role_ref_cluster_admin/ })).toBeInTheDocument();
  });

  it("renders partial artifact availability distinctly, with the gap named explicitly", async () => {
    renderAt("/alerts/2");
    await screen.findAllByText(/Alert #2\b/);
    await waitFor(() => expect(screen.getByText("Detection definition unavailable")).toBeInTheDocument());
    // The other artifacts remain unaffected.
    expect(screen.getByRole("heading", { name: "Source submission" })).toBeInTheDocument();
  });

  it("renders broken traceability with the specific failed link named", async () => {
    renderAt("/alerts/3");
    await screen.findAllByText(/Alert #3\b/);
    await screen.findByText("Traceability broken");
    // Named on both the header chip / canvas callout and the Traceability
    // Rail explanation (UX requirement: the canvas itself localizes the
    // break, not only the rail below it) — so more than one match is
    // correct here, not a duplication bug.
    await waitFor(() => expect(screen.getAllByText(/recorded integrity digest/).length).toBeGreaterThan(0));
    expect(screen.getAllByText("raw_event_sha256").length).toBeGreaterThan(0);
  });

  it("renders broken traceability (source_key) with the specific failed link named", async () => {
    renderAt("/alerts/6");
    await screen.findAllByText(/Alert #6\b/);
    await screen.findByText("Traceability broken");
    await waitFor(() => expect(screen.getAllByText(/source identity/).length).toBeGreaterThan(0));
    expect(screen.getAllByText("source_key").length).toBeGreaterThan(0);
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
    expect(screen.queryByRole("heading", { name: "Source submission" })).not.toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "Detection result" })).not.toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "Generated alert" })).not.toBeInTheDocument();
  });
});

describe("AlertInvestigationPage — no legacy presentation in the live DOM", () => {
  it("renders none of the rejected Signal Path or Forensic Case Folio vocabulary or attributes", async () => {
    const { container } = renderAt("/alerts/1");
    await screen.findAllByText(/Alert #1\b/);
    const text = container.textContent!.toLowerCase();
    for (const forbidden of [
      "signal path",
      "stage rail",
      "conduit",
      "exhibit procession",
      "decoder strip",
      "evidence theater",
      "folio",
      "case opening",
      "custody",
      "evidentiary clause",
      "admission threshold",
      "accession",
      "rust mark",
    ]) {
      expect(text).not.toContain(forbidden);
    }
    expect(container.querySelector('[data-role="conduit-field"]')).toBeNull();
    expect(container.querySelector('[data-role="evidence-ribbon"]')).toBeNull();
    expect(container.querySelector('[data-role="proof-chain"]')).toBeNull();
  });
});
