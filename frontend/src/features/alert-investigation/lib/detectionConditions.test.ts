import { describe, expect, it } from "vitest";
import { buildDeclaredConditions, isOutcomeSatisfied } from "./detectionConditions";
import {
  fixtureIntact,
  fixtureScenario2Intact,
  fixtureScenario3Intact,
} from "@/fixtures/alert-investigation/v1";
import type { AlertInvestigationResponse, DetectionDefinition, MatchReason } from "@/types/contract";

function definitionField(
  conditions: DetectionDefinition["conditions"],
): AlertInvestigationResponse["detectionDefinition"] {
  return {
    available: true,
    revision: "test-revision",
    definition: {
      scenario: "scenario-test",
      name: "Test definition",
      description: "A definition constructed for combination testing.",
      conditions,
    },
  };
}

function resultField(satisfiedIds: string[]): AlertInvestigationResponse["detectionResult"] {
  const matchReason: MatchReason = {
    scenario: "scenario-test",
    definitionName: "Test definition",
    definitionRevision: "test-revision",
    satisfiedCharacteristics: satisfiedIds.map((id) => ({ id, description: `Characteristic ${id}.` })),
  };
  return { available: true, matchReason };
}

const AVAILABLE_NORMALIZED_SUCCESS: AlertInvestigationResponse["normalizedEvent"] =
  fixtureScenario2Intact.normalizedEvent;

describe("isOutcomeSatisfied", () => {
  it("treats 2xx codes as satisfying a 'success' requirement", () => {
    expect(isOutcomeSatisfied("success", 200)).toBe(true);
    expect(isOutcomeSatisfied("success", 201)).toBe(true);
    expect(isOutcomeSatisfied("success", 299)).toBe(true);
  });

  it("treats non-2xx codes as not satisfying a 'success' requirement", () => {
    expect(isOutcomeSatisfied("success", 199)).toBe(false);
    expect(isOutcomeSatisfied("success", 300)).toBe(false);
    expect(isOutcomeSatisfied("success", 404)).toBe(false);
  });

  it("fails closed for an unrecognized requires_outcome value, mirroring the backend", () => {
    expect(isOutcomeSatisfied("failure", 500)).toBe(false);
    expect(isOutcomeSatisfied("unknown-value", 200)).toBe(false);
  });
});

describe("buildDeclaredConditions — the seven declared-condition combinations", () => {
  it("requires_any only", () => {
    const def = definitionField({
      operation: { resource: "pods", subresource: "exec" },
      requires_any: [
        { id: "a", description: "A." },
        { id: "b", description: "B." },
      ],
    });
    const result = resultField(["a"]);
    const declared = buildDeclaredConditions(def, result, { available: false });
    expect(declared?.outcome).toBeNull();
    expect(declared?.requiresAll).toBeNull();
    expect(declared?.requiresAny?.characteristics).toHaveLength(2);
    expect(declared?.requiresAny?.characteristics.find((c) => c.id === "a")?.satisfied).toBe(true);
    expect(declared?.requiresAny?.characteristics.find((c) => c.id === "b")?.satisfied).toBe(false);
    expect(declared?.requiresAny?.satisfied).toBe(true);
  });

  it("requires_all only", () => {
    const def = definitionField({
      operation: { resource: "clusterrolebindings", verb: "create" },
      requires_all: [{ id: "x", description: "X." }],
    });
    const result = resultField(["x"]);
    const declared = buildDeclaredConditions(def, result, { available: false });
    expect(declared?.outcome).toBeNull();
    expect(declared?.requiresAny).toBeNull();
    expect(declared?.requiresAll?.characteristics).toHaveLength(1);
    expect(declared?.requiresAll?.satisfied).toBe(true);
  });

  it("requires_outcome only", () => {
    const def = definitionField({
      operation: { resource: "pods", verb: "create" },
      requires_outcome: "success",
    });
    const result = resultField([]);
    const declared = buildDeclaredConditions(def, result, AVAILABLE_NORMALIZED_SUCCESS);
    expect(declared?.requiresAny).toBeNull();
    expect(declared?.requiresAll).toBeNull();
    expect(declared?.outcome?.requiredOutcome).toBe("success");
    expect(declared?.outcome?.satisfied).toBe(true);
  });

  it("requires_any + requires_outcome", () => {
    const def = definitionField({
      operation: { resource: "pods", verb: "create" },
      requires_outcome: "success",
      requires_any: [{ id: "a", description: "A." }],
    });
    const result = resultField(["a"]);
    const declared = buildDeclaredConditions(def, result, AVAILABLE_NORMALIZED_SUCCESS);
    expect(declared?.outcome?.satisfied).toBe(true);
    expect(declared?.requiresAny?.satisfied).toBe(true);
    expect(declared?.requiresAll).toBeNull();
  });

  it("requires_all + requires_outcome", () => {
    const def = definitionField({
      operation: { resource: "clusterrolebindings", verb: "create" },
      requires_outcome: "success",
      requires_all: [{ id: "x", description: "X." }],
    });
    const result = resultField(["x"]);
    const declared = buildDeclaredConditions(def, result, AVAILABLE_NORMALIZED_SUCCESS);
    expect(declared?.outcome?.satisfied).toBe(true);
    expect(declared?.requiresAll?.satisfied).toBe(true);
    expect(declared?.requiresAny).toBeNull();
  });

  it("requires_any + requires_all together", () => {
    const def = definitionField({
      operation: { resource: "pods", verb: "create" },
      requires_any: [{ id: "a", description: "A." }],
      requires_all: [{ id: "x", description: "X." }],
    });
    const result = resultField(["a", "x"]);
    const declared = buildDeclaredConditions(def, result, { available: false });
    expect(declared?.requiresAny?.satisfied).toBe(true);
    expect(declared?.requiresAll?.satisfied).toBe(true);
  });

  it("all three (requires_any + requires_all + requires_outcome) together", () => {
    const def = definitionField({
      operation: { resource: "pods", verb: "create" },
      requires_outcome: "success",
      requires_any: [{ id: "a", description: "A." }],
      requires_all: [{ id: "x", description: "X." }],
    });
    const result = resultField(["a", "x"]);
    const declared = buildDeclaredConditions(def, result, AVAILABLE_NORMALIZED_SUCCESS);
    expect(declared?.outcome?.satisfied).toBe(true);
    expect(declared?.requiresAny?.satisfied).toBe(true);
    expect(declared?.requiresAll?.satisfied).toBe(true);
  });

  it("no declared characteristic list — operation only", () => {
    const def = definitionField({ operation: { resource: "pods", verb: "create" } });
    const result = resultField([]);
    const declared = buildDeclaredConditions(def, result, { available: false });
    expect(declared?.outcome).toBeNull();
    expect(declared?.requiresAny).toBeNull();
    expect(declared?.requiresAll).toBeNull();
    expect(declared?.operation).toEqual({ resource: "pods", verb: "create" });
  });
});

describe("buildDeclaredConditions — availability and edge behavior", () => {
  it("returns null when the detection definition is unavailable", () => {
    const declared = buildDeclaredConditions(
      { available: false },
      resultField([]),
      { available: false },
    );
    expect(declared).toBeNull();
  });

  it("treats a requires_outcome as satisfied when the detection result exists but the normalized outcome code cannot currently be read", () => {
    const def = definitionField({
      operation: { resource: "pods", verb: "create" },
      requires_outcome: "success",
    });
    const declared = buildDeclaredConditions(def, resultField([]), { available: false });
    expect(declared?.outcome?.satisfied).toBe(true);
    expect(declared?.outcome?.recordedOutcomeCode).toBeUndefined();
  });

  it("never mutates its input arguments", () => {
    const def = definitionField({
      operation: { resource: "pods", subresource: "exec" },
      requires_any: [{ id: "a", description: "A." }],
    });
    const defSnapshot = JSON.parse(JSON.stringify(def));
    const result = resultField(["a"]);
    const resultSnapshot = JSON.parse(JSON.stringify(result));

    buildDeclaredConditions(def, result, { available: false });

    expect(def).toEqual(defSnapshot);
    expect(result).toEqual(resultSnapshot);
  });
});

describe("buildDeclaredConditions — real scenario fixtures", () => {
  it("scenario 1: requires_any only, no outcome requirement (FR-024)", () => {
    const declared = buildDeclaredConditions(
      fixtureIntact.detectionDefinition,
      fixtureIntact.detectionResult,
      fixtureIntact.normalizedEvent,
    );
    expect(declared?.outcome).toBeNull();
    expect(declared?.requiresAny?.characteristics.map((c) => c.id).sort()).toEqual([
      "stdin_streaming",
      "tty_allocation",
    ]);
  });

  it("scenario 2: requires_any (5 characteristics) + requires_outcome", () => {
    const declared = buildDeclaredConditions(
      fixtureScenario2Intact.detectionDefinition,
      fixtureScenario2Intact.detectionResult,
      fixtureScenario2Intact.normalizedEvent,
    );
    expect(declared?.requiresAny?.characteristics).toHaveLength(5);
    expect(declared?.requiresAny?.characteristics.every((c) => c.satisfied)).toBe(true);
    expect(declared?.outcome?.requiredOutcome).toBe("success");
    expect(declared?.outcome?.satisfied).toBe(true);
  });

  it("scenario 3: requires_all (1 characteristic) + requires_outcome, creation only", () => {
    const declared = buildDeclaredConditions(
      fixtureScenario3Intact.detectionDefinition,
      fixtureScenario3Intact.detectionResult,
      fixtureScenario3Intact.normalizedEvent,
    );
    expect(declared?.operation.verb).toBe("create");
    expect(declared?.requiresAll?.characteristics).toEqual([
      expect.objectContaining({ id: "role_ref_cluster_admin", satisfied: true }),
    ]);
    expect(declared?.outcome?.satisfied).toBe(true);
  });
});
