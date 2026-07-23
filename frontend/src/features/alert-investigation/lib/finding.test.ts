import { describe, expect, it } from "vitest";
import { buildFinding } from "./finding";
import {
  fixtureIntact,
  fixtureScenario2Intact,
  fixtureScenario3Intact,
} from "@/fixtures/alert-investigation/v1";
import type { AlertInvestigationResponse } from "@/types/contract";

describe("buildFinding — scenario 1", () => {
  it("states subject, operation, target, outcome, request time, and matched identity verbatim from the response", () => {
    const finding = buildFinding(fixtureIntact);
    const summary = fixtureIntact.alert.summary!;
    expect(finding.available).toBe(true);
    expect(finding.subject).toEqual(summary.subject);
    expect(finding.operation).toEqual(summary.operation);
    expect(finding.target).toEqual(summary.target);
    expect(finding.outcome).toEqual(summary.outcome);
    expect(finding.requestTime).toBe(summary.requestTime);
    expect(finding.matchedScenario).toBe(fixtureIntact.detectionResult.matchReason!.scenario);
    expect(finding.matchedDefinitionName).toBe(
      fixtureIntact.detectionResult.matchReason!.definitionName,
    );
  });
});

describe("buildFinding — scenario 2 and 3", () => {
  it("scenario 2's Finding matches its own real summary, not scenario 1's", () => {
    const finding = buildFinding(fixtureScenario2Intact);
    expect(finding.matchedScenario).toBe("scenario-2");
    expect(finding.target?.resource).toBe("pods");
    expect(finding.outcome?.code).toBe(201);
  });

  it("scenario 3's Finding matches its own real summary", () => {
    const finding = buildFinding(fixtureScenario3Intact);
    expect(finding.matchedScenario).toBe("scenario-3");
    expect(finding.target?.resource).toBe("clusterrolebindings");
    expect(finding.operation?.verb).toBe("create");
  });
});

describe("buildFinding — honesty and availability", () => {
  it("returns the explicit unavailable variant when the alert summary is unavailable", () => {
    const data: AlertInvestigationResponse = { ...fixtureIntact, alert: { available: false } };
    const finding = buildFinding(data);
    expect(finding).toEqual({ available: false });
  });

  it("still states outcome and time even when the matched scenario cannot be identified (FR-029)", () => {
    const data: AlertInvestigationResponse = { ...fixtureIntact, detectionResult: { available: false } };
    const finding = buildFinding(data);
    expect(finding.available).toBe(true);
    expect(finding.outcome).toBeDefined();
    expect(finding.requestTime).toBeDefined();
    expect(finding.matchedScenario).toBeUndefined();
    expect(finding.matchedDefinitionName).toBeUndefined();
  });

  it("never introduces a severity, confidence, or intent field", () => {
    const finding = buildFinding(fixtureIntact);
    const keys = Object.keys(finding);
    for (const forbidden of ["severity", "confidence", "intent", "malicious", "risk"]) {
      expect(keys).not.toContain(forbidden);
    }
  });

  it("never mutates its input response", () => {
    const snapshot = JSON.parse(JSON.stringify(fixtureIntact));
    buildFinding(fixtureIntact);
    expect(fixtureIntact).toEqual(snapshot);
  });
});
