import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { AlertIdentityHeader } from "./AlertIdentityHeader";
import { buildInvestigationViewModel } from "@/features/alert-investigation/lib/investigationViewModel";
import { fixtureIntact, fixtureScenario2Intact } from "@/fixtures/alert-investigation/v1";

describe("AlertIdentityHeader — real identity fields", () => {
  it("renders the real alert id, definition name, and revision from scenario 1", () => {
    const { docketHeader, finding } = buildInvestigationViewModel(fixtureIntact);
    render(<AlertIdentityHeader docketHeader={docketHeader} finding={finding} />);
    expect(screen.getByText(/Alert #1/)).toBeInTheDocument();
    expect(screen.getByText(new RegExp(docketHeader.matchedDefinitionName!))).toBeInTheDocument();
  });

  it("renders scenario 2's real actor/operation/target, not scenario 1's", () => {
    const { docketHeader, finding } = buildInvestigationViewModel(fixtureScenario2Intact);
    render(<AlertIdentityHeader docketHeader={docketHeader} finding={finding} />);
    expect(screen.getByText(/Alert #4/)).toBeInTheDocument();
    expect(screen.getByText(finding.subject!.username, { exact: false })).toBeInTheDocument();
  });
});

describe("AlertIdentityHeader — traceability indicator", () => {
  it("reads intact when traceabilityIntact is true", () => {
    const { docketHeader, finding } = buildInvestigationViewModel(fixtureIntact);
    render(<AlertIdentityHeader docketHeader={docketHeader} finding={finding} />);
    expect(screen.getByText("Traceability intact")).toBeInTheDocument();
  });

  it("reads broken when traceabilityIntact is false", () => {
    render(
      <AlertIdentityHeader
        docketHeader={{ alertId: 3, traceabilityIntact: false }}
        finding={{ available: false }}
      />,
    );
    expect(screen.getByText("Traceability broken")).toBeInTheDocument();
  });
});

describe("AlertIdentityHeader — no invented content", () => {
  it("states detection identity and alert summary as unavailable, honestly", () => {
    render(
      <AlertIdentityHeader
        docketHeader={{ alertId: 7, traceabilityIntact: true }}
        finding={{ available: false }}
      />,
    );
    expect(screen.getByText(/detection identity unavailable/)).toBeInTheDocument();
    expect(screen.getByText("Alert summary unavailable")).toBeInTheDocument();
  });

  it("never introduces severity, confidence, or intent language", () => {
    const { docketHeader, finding } = buildInvestigationViewModel(fixtureIntact);
    const { container } = render(<AlertIdentityHeader docketHeader={docketHeader} finding={finding} />);
    const text = container.textContent!.toLowerCase();
    for (const forbidden of ["severity", "confidence", "critical", "malicious", "intent"]) {
      expect(text).not.toContain(forbidden);
    }
  });
});
