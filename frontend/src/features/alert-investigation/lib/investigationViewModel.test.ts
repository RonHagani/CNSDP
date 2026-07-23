import { describe, expect, it } from "vitest";
import { buildInvestigationViewModel } from "./investigationViewModel";
import {
  fixtureBrokenTraceability,
  fixtureIntact,
  fixturePartial,
  fixtureScenario2Intact,
  fixtureScenario3Intact,
} from "@/fixtures/alert-investigation/v1";

const ALL_FIXTURES = [
  ["scenario 1 (intact)", fixtureIntact],
  ["partial availability", fixturePartial],
  ["broken traceability", fixtureBrokenTraceability],
  ["scenario 2 (intact)", fixtureScenario2Intact],
  ["scenario 3 (intact)", fixtureScenario3Intact],
] as const;

describe.each(ALL_FIXTURES)("buildInvestigationViewModel — %s", (_label, fixture) => {
  it("carries the correct alertId in both the root and the docket header", () => {
    const model = buildInvestigationViewModel(fixture);
    expect(model.alertId).toBe(fixture.alertId);
    expect(model.docketHeader.alertId).toBe(fixture.alertId);
  });

  it("always produces exactly six register entries", () => {
    expect(buildInvestigationViewModel(fixture).register).toHaveLength(6);
  });

  it("the docket header's traceability flag matches the response", () => {
    const model = buildInvestigationViewModel(fixture);
    expect(model.docketHeader.traceabilityIntact).toBe(fixture.traceability.intact);
    expect(model.traceability.intact).toBe(fixture.traceability.intact);
  });

  it("is deterministic across repeated calls", () => {
    expect(buildInvestigationViewModel(fixture)).toEqual(buildInvestigationViewModel(fixture));
  });

  it("never mutates its input response", () => {
    const snapshot = JSON.parse(JSON.stringify(fixture));
    buildInvestigationViewModel(fixture);
    expect(fixture).toEqual(snapshot);
  });
});

describe("buildInvestigationViewModel — scenario grounding", () => {
  it("scenario 1's default selection is detection-result, with Verified provenance rows", () => {
    const model = buildInvestigationViewModel(fixtureIntact);
    expect(model.defaultSelectedArtifact).toBe("detection-result");
    expect(
      model.concordance.rows
        .filter((r) => r.kind === "requires_any" || r.kind === "requires_all")
        .every((r) => "provenance" in r && r.provenance.kind === "verified"),
    ).toBe(true);
  });

  it("scenario 2's concordance carries all five real satisfied characteristics with Partial provenance", () => {
    const model = buildInvestigationViewModel(fixtureScenario2Intact);
    const characteristicRows = model.concordance.rows.filter(
      (r) => r.kind === "requires_any" || r.kind === "requires_all",
    );
    expect(characteristicRows).toHaveLength(5);
    expect(characteristicRows.every((r) => "provenance" in r && r.provenance.kind === "partial")).toBe(
      true,
    );
  });

  it("scenario 3's finding and concordance both reflect the real cluster-admin grant", () => {
    const model = buildInvestigationViewModel(fixtureScenario3Intact);
    expect(model.finding.matchedScenario).toBe("scenario-3");
    expect(model.artifacts.normalizedEvent.available).toBe(true);
    if (model.artifacts.normalizedEvent.available) {
      expect(model.artifacts.normalizedEvent.event.clusterRoleBinding?.roleRef.name).toBe(
        "cluster-admin",
      );
    }
  });
});

describe("buildInvestigationViewModel — availability composition", () => {
  it("partial availability: the register shows the gap, but the docket header and finding still render from what remains", () => {
    const model = buildInvestigationViewModel(fixturePartial);
    const definitionEntry = model.register.find((e) => e.key === "detection-definition")!;
    expect(definitionEntry.available).toBe(false);
    expect(model.finding.available).toBe(true);
    expect(model.docketHeader.pinnedRevision).toBeUndefined();
  });

  it("broken traceability: all six artifacts remain available while traceability itself is broken, and neither fact masks the other", () => {
    const model = buildInvestigationViewModel(fixtureBrokenTraceability);
    expect(model.register.every((e) => e.available)).toBe(true);
    expect(model.traceability.intact).toBe(false);
    expect(model.traceability.evidenceSetComplete).toBe(false);
  });
});

describe("buildInvestigationViewModel — contract discipline", () => {
  it("contains no severity, confidence, case-status, assignment, or disposition field anywhere in the tree", () => {
    const serialized = JSON.stringify(buildInvestigationViewModel(fixtureIntact)).toLowerCase();
    for (const forbidden of ["severity", "confidence", "assignee", "disposition", "case_status"]) {
      expect(serialized).not.toContain(forbidden);
    }
  });

  it("does not use rejected presentation vocabulary anywhere in the tree", () => {
    const serialized = JSON.stringify(buildInvestigationViewModel(fixtureIntact)).toLowerCase();
    for (const forbidden of [
      "signal path",
      "stage rail",
      "conduit",
      "exhibit procession",
      "decoder strip",
    ]) {
      expect(serialized).not.toContain(forbidden);
    }
  });
});
