/**
 * Content tests for the scenario-2 and scenario-3 fixtures (id 4, id 5).
 * These assert the fixtures are shaped correctly against
 * `AlertInvestigationResponse` and are internally consistent with the
 * real backend definitions and domain behavior they are derived from
 * (see the file-level provenance note in v1.ts). Coverage for the
 * existing scenario-1, partial-availability, and broken-traceability
 * fixtures (id 1-3) is intentionally not duplicated here — their content
 * is untouched by this file, and their existing behavior is exercised
 * unchanged by alertSource.test.ts, deriveStageStatus.test.ts,
 * lineage.test.ts, and AlertInvestigationPage.test.tsx.
 */
import { describe, expect, it } from "vitest";
import { fixtureScenario2Intact, fixtureScenario3Intact, fixturesById } from "./v1";
import type { AlertInvestigationResponse } from "@/types/contract";

const ARTIFACT_FIELDS = [
  "sourceEvent",
  "validationOutcome",
  "normalizedEvent",
  "detectionDefinition",
  "detectionResult",
  "alert",
] as const;

describe.each([
  ["scenario 2 (id 4)", fixtureScenario2Intact],
  ["scenario 3 (id 5)", fixtureScenario3Intact],
])("%s fixture", (_label, fixture: AlertInvestigationResponse) => {
  it("satisfies the AlertInvestigationResponse contract shape", () => {
    expect(typeof fixture.alertId).toBe("number");
    for (const field of ARTIFACT_FIELDS) {
      expect(typeof fixture[field].available).toBe("boolean");
    }
    expect(typeof fixture.traceability.intact).toBe("boolean");
  });

  it("contains exactly the six evidence artifacts — traceability is not a seventh", () => {
    const keys = Object.keys(fixture);
    expect(keys).toHaveLength(8); // alertId + 6 artifacts + traceability
    for (const field of ARTIFACT_FIELDS) {
      expect(keys).toContain(field);
    }
    expect(keys).toContain("traceability");
  });

  it("is registered in fixturesById and resolves by its own alertId", () => {
    expect(fixturesById[String(fixture.alertId)]).toBe(fixture);
  });

  it("marks all six artifacts independently available (intact fixture)", () => {
    for (const field of ARTIFACT_FIELDS) {
      expect(fixture[field].available).toBe(true);
    }
  });

  it("reports traceability intact with no failedLink", () => {
    expect(fixture.traceability.intact).toBe(true);
    expect(fixture.traceability.failedLink).toBeUndefined();
  });

  it("pins the detection result to the same definition identity and revision as the detection definition", () => {
    const def = fixture.detectionDefinition.definition;
    const matchReason = fixture.detectionResult.matchReason;
    expect(matchReason?.scenario).toBe(def?.scenario);
    expect(matchReason?.definitionName).toBe(def?.name);
    expect(matchReason?.definitionRevision).toBe(fixture.detectionDefinition.revision);
  });

  it("does not require any scenario-1-only field to load — no exec/stdin/tty present", () => {
    expect(fixture.normalizedEvent.event?.exec).toBeUndefined();
  });
});

describe("scenario 2 fixture — high-risk Pod creation", () => {
  const fixture = fixtureScenario2Intact;

  it("declares the required successful outcome and reports a successful recorded outcome", () => {
    expect(fixture.detectionDefinition.definition?.conditions.requires_outcome).toBe("success");
    const code = fixture.alert.summary?.outcome.code ?? 0;
    expect(code).toBeGreaterThanOrEqual(200);
    expect(code).toBeLessThan(300);
  });

  it("carries the podCreation normalized block, not clusterRoleBinding", () => {
    expect(fixture.normalizedEvent.event?.podCreation).toBeDefined();
    expect(fixture.normalizedEvent.event?.clusterRoleBinding).toBeUndefined();
  });

  it("every satisfied characteristic is supported by a true normalized podCreation field", () => {
    const podCreation = fixture.normalizedEvent.event?.podCreation;
    const satisfiedIds = new Set(
      fixture.detectionResult.matchReason?.satisfiedCharacteristics.map((c) => c.id),
    );
    const FIELD_BY_ID: Record<string, boolean | undefined> = {
      privileged_container: podCreation?.privileged,
      host_network: podCreation?.hostNetwork,
      host_pid: podCreation?.hostPID,
      host_ipc: podCreation?.hostIPC,
      host_path_volume: podCreation?.hostPathVolume,
    };
    for (const id of satisfiedIds) {
      expect(FIELD_BY_ID[id]).toBe(true);
    }
  });

  it("does not claim a characteristic absent from the raw source event's Pod specification", () => {
    const rawSpec = (
      fixture.sourceEvent.rawEvent?.requestObject as {
        spec?: { hostNetwork?: boolean; hostPID?: boolean; hostIPC?: boolean; containers?: { securityContext?: { privileged?: boolean } }[]; volumes?: { hostPath?: unknown }[] };
      }
    )?.spec;
    const satisfiedIds = new Set(
      fixture.detectionResult.matchReason?.satisfiedCharacteristics.map((c) => c.id),
    );
    if (satisfiedIds.has("host_network")) expect(rawSpec?.hostNetwork).toBe(true);
    if (satisfiedIds.has("host_pid")) expect(rawSpec?.hostPID).toBe(true);
    if (satisfiedIds.has("host_ipc")) expect(rawSpec?.hostIPC).toBe(true);
    if (satisfiedIds.has("privileged_container")) {
      expect(rawSpec?.containers?.some((c) => c.securityContext?.privileged === true)).toBe(true);
    }
    if (satisfiedIds.has("host_path_volume")) {
      expect(rawSpec?.volumes?.some((v) => v.hostPath !== undefined)).toBe(true);
    }
  });

  it("declares more than two requires_any characteristics — never assumes exactly two", () => {
    expect(fixture.detectionDefinition.definition?.conditions.requires_any?.length).toBe(5);
    expect(fixture.detectionResult.matchReason?.satisfiedCharacteristics.length).toBe(5);
  });
});

describe("scenario 3 fixture — cluster-admin ClusterRoleBinding creation", () => {
  const fixture = fixtureScenario3Intact;

  it("represents creation, not modification or deletion", () => {
    expect(fixture.normalizedEvent.event?.operation.verb).toBe("create");
    expect(fixture.detectionDefinition.definition?.conditions.operation.verb).toBe("create");
  });

  it("references the cluster-admin ClusterRole", () => {
    expect(fixture.normalizedEvent.event?.clusterRoleBinding?.roleRef).toEqual({
      apiGroup: "rbac.authorization.k8s.io",
      kind: "ClusterRole",
      name: "cluster-admin",
    });
    expect(
      fixture.detectionResult.matchReason?.satisfiedCharacteristics.some(
        (c) => c.id === "role_ref_cluster_admin",
      ),
    ).toBe(true);
  });

  it("carries the clusterRoleBinding normalized block, not podCreation", () => {
    expect(fixture.normalizedEvent.event?.clusterRoleBinding).toBeDefined();
    expect(fixture.normalizedEvent.event?.podCreation).toBeUndefined();
  });

  it("declares the required successful outcome and reports a successful recorded outcome", () => {
    expect(fixture.detectionDefinition.definition?.conditions.requires_outcome).toBe("success");
    const code = fixture.alert.summary?.outcome.code ?? 0;
    expect(code).toBeGreaterThanOrEqual(200);
    expect(code).toBeLessThan(300);
  });
});
