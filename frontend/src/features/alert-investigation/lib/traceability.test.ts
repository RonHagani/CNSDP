import { describe, expect, it } from "vitest";
import { buildTraceabilityProof, TRACEABILITY_CHAIN_ORDER } from "./traceability";
import { fixtureBrokenTraceability, fixtureIntact } from "@/fixtures/alert-investigation/v1";
import { allArtifactsAvailable } from "./evidenceRegister";
import type { AlertInvestigationResponse } from "@/types/contract";

function withFailedLink(failedLink: "alert" | "source_key" | "raw_event_sha256"): AlertInvestigationResponse {
  return { ...fixtureIntact, traceability: { intact: false, failedLink } };
}

describe("buildTraceabilityProof — intact", () => {
  it("reports intact true with no failedLink and a positive explanation", () => {
    const proof = buildTraceabilityProof(fixtureIntact, allArtifactsAvailable(fixtureIntact));
    expect(proof.intact).toBe(true);
    expect(proof.failedLink).toBeUndefined();
    expect(proof.explanation.length).toBeGreaterThan(0);
    expect(proof.explanation.toLowerCase()).not.toContain("broken");
  });

  it("carries the real four-link chain in resolution order", () => {
    const proof = buildTraceabilityProof(fixtureIntact, true);
    expect(proof.chain).toEqual(TRACEABILITY_CHAIN_ORDER);
    expect(proof.chain).toEqual(["alert", "detection-result", "normalized-event", "submission"]);
  });
});

describe("buildTraceabilityProof — each failed-link value", () => {
  it("alert: specific, distinct explanation", () => {
    const data = withFailedLink("alert");
    const proof = buildTraceabilityProof(data, allArtifactsAvailable(data));
    expect(proof.intact).toBe(false);
    expect(proof.failedLink).toBe("alert");
    expect(proof.explanation).toMatch(/alert itself does not resolve/i);
  });

  it("source_key: specific, distinct explanation", () => {
    const data = withFailedLink("source_key");
    const proof = buildTraceabilityProof(data, allArtifactsAvailable(data));
    expect(proof.failedLink).toBe("source_key");
    expect(proof.explanation).toMatch(/source identity/i);
  });

  it("raw_event_sha256: specific, distinct explanation (fixtureBrokenTraceability)", () => {
    const proof = buildTraceabilityProof(
      fixtureBrokenTraceability,
      allArtifactsAvailable(fixtureBrokenTraceability),
    );
    expect(proof.failedLink).toBe("raw_event_sha256");
    expect(proof.explanation).toMatch(/integrity digest/i);
  });

  it("the three failed-link explanations are all different from one another", () => {
    const explanations = new Set(
      (["alert", "source_key", "raw_event_sha256"] as const).map(
        (link) => buildTraceabilityProof(withFailedLink(link), true).explanation,
      ),
    );
    expect(explanations.size).toBe(3);
  });
});

describe("buildTraceabilityProof — evidence-set completeness implication", () => {
  it("is true only when traceability is intact AND all six artifacts are available", () => {
    expect(buildTraceabilityProof(fixtureIntact, true).evidenceSetComplete).toBe(true);
    expect(buildTraceabilityProof(fixtureIntact, false).evidenceSetComplete).toBe(false);
    expect(buildTraceabilityProof(fixtureBrokenTraceability, true).evidenceSetComplete).toBe(false);
  });

  it("fixtureBrokenTraceability has all six artifacts available yet is not evidence-complete — the exact 'must not conflate' case", () => {
    expect(allArtifactsAvailable(fixtureBrokenTraceability)).toBe(true);
    const proof = buildTraceabilityProof(
      fixtureBrokenTraceability,
      allArtifactsAvailable(fixtureBrokenTraceability),
    );
    expect(proof.intact).toBe(false);
    expect(proof.evidenceSetComplete).toBe(false);
  });

  it("does not replace the independently-readable intact flag", () => {
    const proof = buildTraceabilityProof(fixtureIntact, false);
    expect(proof.intact).toBe(true);
    expect(proof.evidenceSetComplete).toBe(false);
  });
});

describe("buildTraceabilityProof — partial provenance does not imply broken traceability", () => {
  it("an alert with an unavailable source event (Unavailable provenance elsewhere) can still be traceability-intact", () => {
    const data: AlertInvestigationResponse = { ...fixtureIntact, sourceEvent: { available: false } };
    const proof = buildTraceabilityProof(data, allArtifactsAvailable(data));
    expect(data.traceability.intact).toBe(true);
    expect(proof.intact).toBe(true);
    // evidenceSetComplete correctly reflects the missing artifact without
    // altering the independent `intact` verification result.
    expect(proof.evidenceSetComplete).toBe(false);
  });
});

describe("buildTraceabilityProof — contract discipline", () => {
  it("never mutates its input response", () => {
    const snapshot = JSON.parse(JSON.stringify(fixtureBrokenTraceability));
    buildTraceabilityProof(fixtureBrokenTraceability, true);
    expect(fixtureBrokenTraceability).toEqual(snapshot);
  });

  it("never invents an id absent from the response", () => {
    const proof = buildTraceabilityProof(fixtureIntact, true);
    expect(proof).not.toHaveProperty("id");
    expect(proof).not.toHaveProperty("alertId");
  });

  it("does not represent traceability as a seventh evidence artifact — no artifact-shaped fields", () => {
    const proof = buildTraceabilityProof(fixtureIntact, true);
    expect(proof).not.toHaveProperty("index");
    expect(proof).not.toHaveProperty("available");
  });
});
