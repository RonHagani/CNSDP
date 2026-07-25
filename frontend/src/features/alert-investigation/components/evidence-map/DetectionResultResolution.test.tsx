import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { DetectionResultResolution } from "./DetectionResultResolution";
import { buildInvestigationViewModel } from "@/features/alert-investigation/lib/investigationViewModel";
import { fixtureIntact, fixturePartial, fixtureScenario2Intact } from "@/fixtures/alert-investigation/v1";

describe("DetectionResultResolution", () => {
  it("renders the real declared/satisfied tally for scenario 1 (2 of 2)", () => {
    const vm = buildInvestigationViewModel(fixtureIntact);
    render(
      <DetectionResultResolution
        detectionResult={vm.artifacts.detectionResult}
        concordance={vm.concordance}
      />,
    );
    expect(screen.getByText("2/2")).toBeInTheDocument();
  });

  it("renders the real declared/satisfied tally for scenario 2 (5 of 5)", () => {
    const vm = buildInvestigationViewModel(fixtureScenario2Intact);
    render(
      <DetectionResultResolution
        detectionResult={vm.artifacts.detectionResult}
        concordance={vm.concordance}
      />,
    );
    expect(screen.getByText("5/5")).toBeInTheDocument();
  });

  it("never renders a percentage, score, or risk-level word", () => {
    const vm = buildInvestigationViewModel(fixtureIntact);
    const { container } = render(
      <DetectionResultResolution
        detectionResult={vm.artifacts.detectionResult}
        concordance={vm.concordance}
      />,
    );
    const text = container.textContent!.toLowerCase();
    for (const forbidden of ["%", "score", "risk", "high", "medium", "low"]) {
      expect(text).not.toContain(forbidden);
    }
  });

  it("renders the resolved-match fact with no fabricated '0 of 0' when no characteristics are declared", () => {
    const vm = buildInvestigationViewModel(fixtureIntact);
    render(
      <DetectionResultResolution
        detectionResult={vm.artifacts.detectionResult}
        concordance={{
          available: true,
          rows: [],
          declaredCharacteristicCount: 0,
          satisfiedCharacteristicCount: 0,
          showCharacteristicCount: false,
        }}
      />,
    );
    expect(screen.getByText("Resolved match")).toBeInTheDocument();
    expect(screen.queryByText("0/0")).not.toBeInTheDocument();
    expect(screen.queryByText(/satisfied/)).not.toBeInTheDocument();
  });

  it("renders an explicit unavailable statement when detection result is unavailable", () => {
    const vm = buildInvestigationViewModel(fixturePartial);
    // fixturePartial marks detectionDefinition unavailable, not
    // detectionResult — exercise the genuinely unavailable shape directly.
    render(
      <DetectionResultResolution detectionResult={{ available: false }} concordance={vm.concordance} />,
    );
    expect(screen.getByText("Detection result unavailable")).toBeInTheDocument();
  });
});
