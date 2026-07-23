import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { InvestigationFinding } from "./InvestigationFinding";
import { buildFinding } from "@/features/alert-investigation/lib/finding";
import {
  fixtureIntact,
  fixtureScenario2Intact,
  fixtureScenario3Intact,
} from "@/fixtures/alert-investigation/v1";

describe("InvestigationFinding — backend-provided match reason", () => {
  it("renders scenario 1's real matched definition name and scenario id", () => {
    const finding = buildFinding(fixtureIntact);
    render(<InvestigationFinding finding={finding} />);
    expect(screen.getByText(new RegExp(finding.matchedDefinitionName!))).toBeInTheDocument();
    expect(screen.getByText(new RegExp(finding.matchedScenario!))).toBeInTheDocument();
  });

  it("renders scenario 2's real subject, operation, target, and outcome", () => {
    const finding = buildFinding(fixtureScenario2Intact);
    render(<InvestigationFinding finding={finding} />);
    expect(screen.getByText(finding.subject!.username)).toBeInTheDocument();
    expect(screen.getByText(finding.operation!.verb)).toBeInTheDocument();
    expect(screen.getByText(String(finding.outcome!.code))).toBeInTheDocument();
  });

  it("renders scenario 3's real cluster-admin grant identity, not scenario 1's", () => {
    const finding = buildFinding(fixtureScenario3Intact);
    render(<InvestigationFinding finding={finding} />);
    expect(screen.getByText(/scenario-3/)).toBeInTheDocument();
    expect(screen.queryByText(/scenario-1/)).not.toBeInTheDocument();
  });

  it("never introduces severity, confidence, intent, or maliciousness language", () => {
    const finding = buildFinding(fixtureIntact);
    const { container } = render(<InvestigationFinding finding={finding} />);
    const text = container.textContent!.toLowerCase();
    for (const forbidden of ["severity", "confidence", "malicious", "intent"]) {
      expect(text).not.toContain(forbidden);
    }
  });
});

describe("InvestigationFinding — unavailable state is honest", () => {
  it("renders an explicit unavailable statement, not a blank region, when the alert summary is unavailable", () => {
    render(<InvestigationFinding finding={{ available: false }} />);
    expect(screen.getByText(/not available/i)).toBeInTheDocument();
    expect(screen.queryByText(/matched detection/i)).not.toBeInTheDocument();
  });
});
