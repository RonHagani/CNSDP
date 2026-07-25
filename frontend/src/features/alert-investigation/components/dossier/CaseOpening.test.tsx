import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { CaseOpening } from "./CaseOpening";
import { buildInvestigationViewModel } from "@/features/alert-investigation/lib/investigationViewModel";
import {
  fixtureIntact,
  fixtureScenario2Intact,
  fixtureScenario3Intact,
} from "@/fixtures/alert-investigation/v1";

describe("CaseOpening — real identity fields", () => {
  it("renders the real alert id, scenario, definition name, and revision from scenario 1", () => {
    const { docketHeader, finding } = buildInvestigationViewModel(fixtureIntact);
    render(<CaseOpening docketHeader={docketHeader} finding={finding} />);

    expect(screen.getByText(/Alert #1/)).toBeInTheDocument();
    expect(screen.getByText(/scenario-1/)).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: docketHeader.matchedDefinitionName! })).toBeInTheDocument();
    expect(screen.getByText(new RegExp(docketHeader.pinnedRevision!))).toBeInTheDocument();
  });

  it("renders the real alert id and scenario for scenario 2, not scenario 1's identity", () => {
    const { docketHeader, finding } = buildInvestigationViewModel(fixtureScenario2Intact);
    render(<CaseOpening docketHeader={docketHeader} finding={finding} />);

    expect(screen.getByText(/Alert #4/)).toBeInTheDocument();
    expect(screen.getByText(/scenario-2/)).toBeInTheDocument();
    expect(screen.queryByText(/scenario-1/)).not.toBeInTheDocument();
  });

  it("renders scenario 3's real cluster-admin grant identity, not scenario 1's", () => {
    const { docketHeader, finding } = buildInvestigationViewModel(fixtureScenario3Intact);
    render(<CaseOpening docketHeader={docketHeader} finding={finding} />);

    expect(screen.getByText(/scenario-3/)).toBeInTheDocument();
    expect(screen.queryByText(/scenario-1/)).not.toBeInTheDocument();
  });
});

describe("CaseOpening — observed act", () => {
  it("renders scenario 2's real subject, operation, target, and outcome as one factual sentence", () => {
    const { docketHeader, finding } = buildInvestigationViewModel(fixtureScenario2Intact);
    render(<CaseOpening docketHeader={docketHeader} finding={finding} />);

    expect(screen.getByText(finding.subject!.username)).toBeInTheDocument();
    expect(screen.getByText(finding.operation!.verb)).toBeInTheDocument();
    expect(screen.getByText(String(finding.outcome!.code))).toBeInTheDocument();
  });

  it("never introduces severity, confidence, intent, or maliciousness language", () => {
    const { docketHeader, finding } = buildInvestigationViewModel(fixtureIntact);
    const { container } = render(<CaseOpening docketHeader={docketHeader} finding={finding} />);
    // Not "high-risk" — the fixture's real target resource is legitimately
    // named "high-risk-pod" (Kubernetes resource identity, not an invented
    // severity claim), so it correctly appears in the observed act.
    const text = container.textContent!.toLowerCase();
    for (const forbidden of ["severity", "confidence", "critical", "malicious", "intent"]) {
      expect(text).not.toContain(forbidden);
    }
  });

  it("renders an explicit unavailable statement, not a blank region, when the alert summary is unavailable", () => {
    const { docketHeader } = buildInvestigationViewModel(fixtureIntact);
    render(<CaseOpening docketHeader={docketHeader} finding={{ available: false }} />);
    expect(screen.getByText(/not available/i)).toBeInTheDocument();
    expect(screen.queryByText(/matched detection/i)).not.toBeInTheDocument();
  });
});

describe("CaseOpening — no invented content", () => {
  it("states detection identity as unavailable, honestly, when the model carries no definition name", () => {
    render(
      <CaseOpening
        docketHeader={{
          alertId: 7,
          matchedScenario: undefined,
          matchedDefinitionName: undefined,
          pinnedRevision: undefined,
          traceabilityIntact: true,
        }}
        finding={{ available: false }}
      />,
    );
    expect(screen.getByRole("heading", { name: "Detection identity unavailable" })).toBeInTheDocument();
  });
});

describe("CaseOpening — traceability status is readable", () => {
  it("reads Intact when traceabilityIntact is true", () => {
    render(
      <CaseOpening
        docketHeader={{ alertId: 1, traceabilityIntact: true }}
        finding={{ available: false }}
      />,
    );
    expect(screen.getByText("Intact")).toBeInTheDocument();
  });

  it("reads Broken when traceabilityIntact is false", () => {
    render(
      <CaseOpening
        docketHeader={{ alertId: 1, traceabilityIntact: false }}
        finding={{ available: false }}
      />,
    );
    expect(screen.getByText("Broken")).toBeInTheDocument();
  });
});

describe("CaseOpening — long values remain fully accessible", () => {
  it("renders the complete, untruncated revision string as visible text", () => {
    const { docketHeader, finding } = buildInvestigationViewModel(fixtureIntact);
    render(<CaseOpening docketHeader={docketHeader} finding={finding} />);
    expect(screen.getByText(new RegExp(docketHeader.pinnedRevision!))).toBeInTheDocument();
    expect(docketHeader.pinnedRevision!.length).toBeGreaterThan(40);
  });
});

describe("CaseOpening — optional command-palette trigger", () => {
  it("does not render a trigger when no callback is supplied", () => {
    const { docketHeader, finding } = buildInvestigationViewModel(fixtureIntact);
    render(<CaseOpening docketHeader={docketHeader} finding={finding} />);
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });

  it("renders a real, keyboard-operable button that calls the supplied callback", async () => {
    const user = userEvent.setup();
    const onOpenCommandPalette = vi.fn();
    const { docketHeader, finding } = buildInvestigationViewModel(fixtureIntact);
    render(
      <CaseOpening
        docketHeader={docketHeader}
        finding={finding}
        onOpenCommandPalette={onOpenCommandPalette}
      />,
    );

    const trigger = screen.getByRole("button", { name: /command palette/i });
    trigger.focus();
    await user.keyboard("{Enter}");
    expect(onOpenCommandPalette).toHaveBeenCalledTimes(1);
  });
});
