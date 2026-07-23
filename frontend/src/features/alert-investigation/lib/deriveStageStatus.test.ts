import { describe, expect, it } from "vitest";
import { deriveStageStatus } from "./deriveStageStatus";
import {
  fixtureBrokenTraceability,
  fixtureIntact,
  fixturePartial,
} from "@/fixtures/alert-investigation/v1";

describe("deriveStageStatus", () => {
  it("marks every stage signal when the inventory is complete and traceability is intact", () => {
    const statuses = deriveStageStatus(fixtureIntact);
    expect(statuses.intake.tone).toBe("signal");
    expect(statuses.validation.tone).toBe("signal");
    expect(statuses.evidence.tone).toBe("signal");
    expect(statuses.evidence.available).toBe(true);
    expect(statuses.traceability.tone).toBe("signal");
  });

  it("marks detection-evaluation severed and evidence non-complete when an artifact is unavailable", () => {
    const statuses = deriveStageStatus(fixturePartial);
    expect(statuses["detection-evaluation"].tone).toBe("severed");
    expect(statuses["detection-evaluation"].available).toBe(false);
    expect(statuses.evidence.available).toBe(false);
  });

  it("marks traceability severed on a broken chain while other stages remain signal", () => {
    const statuses = deriveStageStatus(fixtureBrokenTraceability);
    expect(statuses.traceability.tone).toBe("severed");
    expect(statuses.intake.tone).toBe("signal");
  });

  it("investigation is always available — it represents the screen itself", () => {
    expect(deriveStageStatus(fixtureIntact).investigation).toEqual({
      tone: "signal",
      available: true,
    });
  });
});
