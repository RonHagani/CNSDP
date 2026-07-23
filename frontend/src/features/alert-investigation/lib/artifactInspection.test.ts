import { describe, expect, it } from "vitest";
import { artifactInspectionFor, buildArtifactInspection } from "./artifactInspection";
import {
  fixtureIntact,
  fixturePartial,
  fixtureScenario2Intact,
  fixtureScenario3Intact,
} from "@/fixtures/alert-investigation/v1";

describe("buildArtifactInspection — available content", () => {
  it("exposes the complete raw source record for source-event", () => {
    const model = buildArtifactInspection(fixtureIntact);
    expect(model.sourceEvent.available).toBe(true);
    if (model.sourceEvent.available) {
      expect(model.sourceEvent.rawEvent).toEqual(fixtureIntact.sourceEvent.rawEvent);
    }
  });

  it("exposes the complete normalized event record for normalized-event", () => {
    const model = buildArtifactInspection(fixtureScenario2Intact);
    expect(model.normalizedEvent.available).toBe(true);
    if (model.normalizedEvent.available) {
      expect(model.normalizedEvent.event).toEqual(fixtureScenario2Intact.normalizedEvent.event);
    }
  });

  it("exposes declared-vs-satisfied detection review for detection-definition", () => {
    const model = buildArtifactInspection(fixtureScenario2Intact);
    expect(model.detectionDefinition.available).toBe(true);
    if (model.detectionDefinition.available) {
      expect(model.detectionDefinition.declaredConditions?.requiresAny?.characteristics).toHaveLength(5);
      expect(model.detectionDefinition.revision).toBe(fixtureScenario2Intact.detectionDefinition.revision);
    }
  });

  it("exposes the literal match reason for detection-result", () => {
    const model = buildArtifactInspection(fixtureScenario3Intact);
    expect(model.detectionResult.available).toBe(true);
    if (model.detectionResult.available) {
      expect(model.detectionResult.matchReason).toEqual(
        fixtureScenario3Intact.detectionResult.matchReason,
      );
    }
  });

  it("exposes the literal outcome and reason for validation-outcome", () => {
    const model = buildArtifactInspection(fixtureIntact);
    expect(model.validationOutcome).toEqual({ available: true, outcome: "valid", reason: undefined });
  });

  it("exposes the literal alert summary for alert", () => {
    const model = buildArtifactInspection(fixtureIntact);
    expect(model.alert.available).toBe(true);
    if (model.alert.available) {
      expect(model.alert.summary).toEqual(fixtureIntact.alert.summary);
    }
  });
});

describe("buildArtifactInspection — unavailable content", () => {
  it("represents an unavailable detection definition explicitly, not as a blank", () => {
    const model = buildArtifactInspection(fixturePartial);
    expect(model.detectionDefinition).toEqual({ available: false });
  });

  it("does not carry declaredConditions data when the definition itself is unavailable", () => {
    const model = buildArtifactInspection(fixturePartial);
    expect(model.detectionDefinition).not.toHaveProperty("declaredConditions");
  });
});

describe("artifactInspectionFor — lookup by key", () => {
  it("resolves each of the six keys to the matching model field", () => {
    const model = buildArtifactInspection(fixtureIntact);
    expect(artifactInspectionFor("source-event", model)).toBe(model.sourceEvent);
    expect(artifactInspectionFor("validation-outcome", model)).toBe(model.validationOutcome);
    expect(artifactInspectionFor("normalized-event", model)).toBe(model.normalizedEvent);
    expect(artifactInspectionFor("detection-definition", model)).toBe(model.detectionDefinition);
    expect(artifactInspectionFor("detection-result", model)).toBe(model.detectionResult);
    expect(artifactInspectionFor("alert", model)).toBe(model.alert);
  });
});

describe("buildArtifactInspection — contract discipline", () => {
  it("never mutates its input response", () => {
    const snapshot = JSON.parse(JSON.stringify(fixtureIntact));
    buildArtifactInspection(fixtureIntact);
    expect(fixtureIntact).toEqual(snapshot);
  });

  it("is deterministic across repeated calls", () => {
    expect(buildArtifactInspection(fixtureIntact)).toEqual(buildArtifactInspection(fixtureIntact));
  });
});
