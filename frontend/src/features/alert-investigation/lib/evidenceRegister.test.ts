import { describe, expect, it } from "vitest";
import {
  allArtifactsAvailable,
  buildEvidenceRegister,
  EVIDENCE_ARTIFACT_ORDER,
  isArtifactAvailable,
  resolveDefaultSelection,
} from "./evidenceRegister";
import {
  fixtureBrokenTraceability,
  fixtureIntact,
  fixturePartial,
  fixtureScenario2Intact,
  fixtureScenario3Intact,
} from "@/fixtures/alert-investigation/v1";
import type { AlertInvestigationResponse } from "@/types/contract";

describe("buildEvidenceRegister — canonical shape", () => {
  it("returns exactly six entries in the canonical fixed order for every fixture", () => {
    for (const fixture of [
      fixtureIntact,
      fixturePartial,
      fixtureBrokenTraceability,
      fixtureScenario2Intact,
      fixtureScenario3Intact,
    ]) {
      const register = buildEvidenceRegister(fixture, resolveDefaultSelection(fixture));
      expect(register).toHaveLength(6);
      expect(register.map((e) => e.key)).toEqual([
        "source-event",
        "validation-outcome",
        "normalized-event",
        "detection-definition",
        "detection-result",
        "alert",
      ]);
      register.forEach((entry, i) => {
        expect(entry.index).toBe(i + 1);
        expect(entry.total).toBe(6);
      });
    }
  });

  it("matches the exact EVIDENCE_ARTIFACT_ORDER constant", () => {
    expect(EVIDENCE_ARTIFACT_ORDER).toEqual([
      "source-event",
      "validation-outcome",
      "normalized-event",
      "detection-definition",
      "detection-result",
      "alert",
    ]);
  });
});

describe("buildEvidenceRegister — Detection Result defaults to selected", () => {
  it("marks detection-result as the default selection when it is available (all intact fixtures)", () => {
    for (const fixture of [fixtureIntact, fixtureScenario2Intact, fixtureScenario3Intact]) {
      const register = buildEvidenceRegister(fixture, resolveDefaultSelection(fixture));
      const defaultEntries = register.filter((e) => e.isDefaultSelection);
      expect(defaultEntries).toHaveLength(1);
      expect(defaultEntries[0].key).toBe("detection-result");
    }
  });
});

describe("buildEvidenceRegister — availability", () => {
  it("keeps an unavailable artifact present, in place, with an explicit summary (fixturePartial)", () => {
    const register = buildEvidenceRegister(fixturePartial, resolveDefaultSelection(fixturePartial));
    const definitionEntry = register.find((e) => e.key === "detection-definition")!;
    expect(definitionEntry.available).toBe(false);
    expect(definitionEntry.summary).toMatch(/not available/i);

    // The other five remain independently available and present.
    const others = register.filter((e) => e.key !== "detection-definition");
    expect(others.every((e) => e.available)).toBe(true);
    expect(others).toHaveLength(5);
  });

  it("isArtifactAvailable agrees with the register entry for every key", () => {
    for (const key of EVIDENCE_ARTIFACT_ORDER) {
      const register = buildEvidenceRegister(fixturePartial, resolveDefaultSelection(fixturePartial));
      const entry = register.find((e) => e.key === key)!;
      expect(entry.available).toBe(isArtifactAvailable(key, fixturePartial));
    }
  });

  it("allArtifactsAvailable is true only when every one of the six is available", () => {
    expect(allArtifactsAvailable(fixtureIntact)).toBe(true);
    expect(allArtifactsAvailable(fixturePartial)).toBe(false);
    // Broken traceability with all six artifacts available is exactly the
    // case that must not be conflated with artifact availability.
    expect(allArtifactsAvailable(fixtureBrokenTraceability)).toBe(true);
  });
});

describe("resolveDefaultSelection — deterministic fallback", () => {
  it("falls back to the first available artifact in canonical order when Detection Result is unavailable", () => {
    const data: AlertInvestigationResponse = {
      ...fixtureIntact,
      detectionResult: { available: false },
    };
    expect(resolveDefaultSelection(data)).toBe("source-event");
  });

  it("skips further into the canonical order when earlier artifacts are also unavailable", () => {
    const data: AlertInvestigationResponse = {
      ...fixtureIntact,
      sourceEvent: { available: false },
      validationOutcome: { available: false },
      detectionResult: { available: false },
    };
    expect(resolveDefaultSelection(data)).toBe("normalized-event");
  });

  it("is total: returns detection-result even when every artifact is unavailable", () => {
    const data: AlertInvestigationResponse = {
      alertId: 999,
      sourceEvent: { available: false },
      validationOutcome: { available: false },
      normalizedEvent: { available: false },
      detectionDefinition: { available: false },
      detectionResult: { available: false },
      alert: { available: false },
      traceability: { intact: false, failedLink: "alert" },
    };
    expect(resolveDefaultSelection(data)).toBe("detection-result");
  });

  it("is deterministic across repeated calls with the same input", () => {
    const data: AlertInvestigationResponse = {
      ...fixtureIntact,
      detectionResult: { available: false },
    };
    expect(resolveDefaultSelection(data)).toBe(resolveDefaultSelection(data));
  });
});

describe("buildEvidenceRegister — related-artifact references", () => {
  it("relates the source event and normalized event to one another", () => {
    const register = buildEvidenceRegister(fixtureIntact, resolveDefaultSelection(fixtureIntact));
    const sourceEvent = register.find((e) => e.key === "source-event")!;
    const normalizedEvent = register.find((e) => e.key === "normalized-event")!;
    expect(sourceEvent.relatedArtifacts).toContain("normalized-event");
    expect(normalizedEvent.relatedArtifacts).toContain("source-event");
  });

  it("relates the detection definition and detection result to one another", () => {
    const register = buildEvidenceRegister(fixtureIntact, resolveDefaultSelection(fixtureIntact));
    const definition = register.find((e) => e.key === "detection-definition")!;
    const result = register.find((e) => e.key === "detection-result")!;
    expect(definition.relatedArtifacts).toContain("detection-result");
    expect(result.relatedArtifacts).toContain("detection-definition");
  });
});

describe("buildEvidenceRegister — contract discipline", () => {
  it("never mutates its input response", () => {
    const snapshot = JSON.parse(JSON.stringify(fixtureScenario2Intact));
    buildEvidenceRegister(fixtureScenario2Intact, resolveDefaultSelection(fixtureScenario2Intact));
    expect(fixtureScenario2Intact).toEqual(snapshot);
  });

  it("is deterministic across repeated calls", () => {
    const defaultKey = resolveDefaultSelection(fixtureIntact);
    expect(buildEvidenceRegister(fixtureIntact, defaultKey)).toEqual(
      buildEvidenceRegister(fixtureIntact, defaultKey),
    );
  });

  it("does not use legacy stage vocabulary in any label or summary", () => {
    const register = buildEvidenceRegister(fixtureIntact, resolveDefaultSelection(fixtureIntact));
    const serialized = JSON.stringify(register).toLowerCase();
    for (const forbidden of ["signal path", "stage rail", "conduit", "exhibit", "decoder strip"]) {
      expect(serialized).not.toContain(forbidden);
    }
  });
});
