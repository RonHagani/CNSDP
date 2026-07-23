import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { EvidenceConcordance } from "./EvidenceConcordance";
import { buildEvidenceConcordance } from "@/features/alert-investigation/lib/concordance";
import {
  fixtureIntact,
  fixturePartial,
  fixtureScenario2Intact,
  fixtureScenario3Intact,
} from "@/fixtures/alert-investigation/v1";
import type { AlertInvestigationResponse } from "@/types/contract";

function renderConcordance(fixture: AlertInvestigationResponse, selectedConditionKey?: string | null) {
  const concordance = buildEvidenceConcordance(fixture);
  const onSelectCondition = vi.fn();
  const utils = render(
    <EvidenceConcordance
      concordance={concordance}
      selectedConditionKey={selectedConditionKey}
      onSelectCondition={onSelectCondition}
    />,
  );
  return { ...utils, concordance, onSelectCondition };
}

describe("EvidenceConcordance — scenario 1 (variable row count)", () => {
  it("renders both real satisfied exec characteristics by id", () => {
    renderConcordance(fixtureIntact);
    expect(screen.getByText("stdin_streaming")).toBeInTheDocument();
    expect(screen.getByText("tty_allocation")).toBeInTheDocument();
  });

  it("supports a fixture with only one satisfied characteristic without assuming exactly two", () => {
    const oneSatisfied: AlertInvestigationResponse = {
      ...fixtureIntact,
      detectionResult: {
        available: true,
        matchReason: {
          ...fixtureIntact.detectionResult.matchReason!,
          satisfiedCharacteristics: fixtureIntact.detectionResult.matchReason!.satisfiedCharacteristics.filter(
            (c) => c.id === "tty_allocation",
          ),
        },
      },
    };
    renderConcordance(oneSatisfied);
    expect(screen.getByText("tty_allocation")).toBeInTheDocument();
    expect(screen.queryByText("stdin_streaming")).not.toBeInTheDocument();
  });
});

describe("EvidenceConcordance — scenario 2 (five high-risk characteristics + outcome)", () => {
  it("renders all five real satisfied characteristics and the satisfied outcome row", () => {
    renderConcordance(fixtureScenario2Intact);
    for (const id of ["privileged_container", "host_network", "host_pid", "host_ipc", "host_path_volume"]) {
      expect(screen.getByText(id)).toBeInTheDocument();
    }
    expect(screen.getByText(/Required outcome: success/)).toBeInTheDocument();
  });

  it("reports Partial provenance visibly for a requestObject-derived characteristic", () => {
    renderConcordance(fixtureScenario2Intact);
    expect(screen.getAllByText(/^Partial —/).length).toBeGreaterThan(0);
  });
});

describe("EvidenceConcordance — scenario 3 (real condition structure)", () => {
  it("renders the one real requires_all characteristic and the satisfied outcome row", () => {
    renderConcordance(fixtureScenario3Intact);
    expect(screen.getByText("role_ref_cluster_admin")).toBeInTheDocument();
    expect(screen.getByText(/Required outcome: success/)).toBeInTheDocument();
  });

  it("does not require or reference scenario 1's exec fields", () => {
    const { container } = renderConcordance(fixtureScenario3Intact);
    expect(container.textContent).not.toMatch(/stdin|tty_allocation/);
  });
});

describe("EvidenceConcordance — provenance visibility and matched-evidence discipline", () => {
  it("verified provenance rows (scenario 1) can be selected", async () => {
    const user = userEvent.setup();
    const { onSelectCondition } = renderConcordance(fixtureIntact);
    const row = screen.getByRole("button", { name: /stdin_streaming/i });
    await user.click(row);
    expect(onSelectCondition).toHaveBeenCalledWith("stdin_streaming");
  });

  it("never renders an unsatisfied declared characteristic as a matched row", () => {
    const partiallyMatched: AlertInvestigationResponse = {
      ...fixtureScenario2Intact,
      detectionResult: {
        available: true,
        matchReason: {
          ...fixtureScenario2Intact.detectionResult.matchReason!,
          satisfiedCharacteristics: fixtureScenario2Intact.detectionResult.matchReason!.satisfiedCharacteristics.slice(
            0,
            2,
          ),
        },
      },
    };
    renderConcordance(partiallyMatched);
    // Only the first two remain rendered as rows; the other three declared
    // characteristics must not appear as matched evidence here.
    expect(screen.getByText("privileged_container")).toBeInTheDocument();
    expect(screen.getByText("host_network")).toBeInTheDocument();
    expect(screen.queryByText("host_pid")).not.toBeInTheDocument();
  });
});

describe("EvidenceConcordance — selection", () => {
  it("the selection callback emits exactly the selected row's condition id", async () => {
    const user = userEvent.setup();
    const { onSelectCondition } = renderConcordance(fixtureScenario3Intact);
    await user.click(screen.getByRole("button", { name: /role_ref_cluster_admin/i }));
    expect(onSelectCondition).toHaveBeenCalledTimes(1);
    expect(onSelectCondition).toHaveBeenCalledWith("role_ref_cluster_admin");
  });

  it("keyboard activation (Enter on a focused row) emits the same callback as a click", async () => {
    const user = userEvent.setup();
    const { onSelectCondition } = renderConcordance(fixtureIntact);
    const row = screen.getByRole("button", { name: /tty_allocation/i });
    row.focus();
    await user.keyboard("{Enter}");
    expect(onSelectCondition).toHaveBeenCalledWith("tty_allocation");
  });

  it("marks the selected row's pressed state accessibly", () => {
    renderConcordance(fixtureIntact, "stdin_streaming");
    expect(screen.getByRole("button", { name: /stdin_streaming/i })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(screen.getByRole("button", { name: /tty_allocation/i })).toHaveAttribute(
      "aria-pressed",
      "false",
    );
  });

  it("operation and outcome rows are not interactive buttons", () => {
    renderConcordance(fixtureScenario2Intact);
    expect(screen.queryByRole("button", { name: /operation match/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /required outcome/i })).not.toBeInTheDocument();
  });
});

describe("EvidenceConcordance — unavailable definition", () => {
  it("renders an honest unavailable statement when the detection definition artifact is unavailable", () => {
    renderConcordance(fixturePartial);
    expect(screen.getByText(/cannot be shown/i)).toBeInTheDocument();
  });
});
