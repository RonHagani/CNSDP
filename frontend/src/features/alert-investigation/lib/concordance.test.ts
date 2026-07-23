import { describe, expect, it } from "vitest";
import { buildEvidenceConcordance, isCharacteristicRow } from "./concordance";
import {
  fixtureIntact,
  fixturePartial,
  fixtureScenario2Intact,
  fixtureScenario3Intact,
} from "@/fixtures/alert-investigation/v1";
import type { AlertInvestigationResponse } from "@/types/contract";

describe("buildEvidenceConcordance — scenario 1 (interactive pods/exec)", () => {
  it("includes an operation row, no outcome row (none declared), and both satisfied exec characteristics", () => {
    const concordance = buildEvidenceConcordance(fixtureIntact);
    expect(concordance.available).toBe(true);
    expect(concordance.rows.find((r) => r.kind === "operation")).toBeDefined();
    expect(concordance.rows.find((r) => r.kind === "outcome")).toBeUndefined();

    const characteristicRows = concordance.rows.filter(isCharacteristicRow);
    expect(characteristicRows.map((r) => r.id).sort()).toEqual(["stdin_streaming", "tty_allocation"]);
    expect(concordance.declaredCharacteristicCount).toBe(2);
    expect(concordance.satisfiedCharacteristicCount).toBe(2);
  });

  it("supports a fixture where only one of the two exec characteristics is satisfied", () => {
    const oneSatisfied: AlertInvestigationResponse = {
      ...fixtureIntact,
      detectionResult: {
        available: true,
        matchReason: {
          ...fixtureIntact.detectionResult.matchReason!,
          satisfiedCharacteristics: fixtureIntact.detectionResult.matchReason!.satisfiedCharacteristics.filter(
            (c) => c.id === "tty_allocation",
          ),
        },
      },
    };
    const concordance = buildEvidenceConcordance(oneSatisfied);
    const characteristicRows = concordance.rows.filter(isCharacteristicRow);
    expect(characteristicRows).toHaveLength(1);
    expect(characteristicRows[0].id).toBe("tty_allocation");
    expect(concordance.declaredCharacteristicCount).toBe(2);
    expect(concordance.satisfiedCharacteristicCount).toBe(1);
  });

  it("every characteristic row carries a provenance state", () => {
    const concordance = buildEvidenceConcordance(fixtureIntact);
    for (const row of concordance.rows.filter(isCharacteristicRow)) {
      expect(row.provenance.kind).toBe("verified");
    }
  });
});

describe("buildEvidenceConcordance — scenario 2 (high-risk Pod creation)", () => {
  it("includes an operation row, a satisfied outcome row, and all five real satisfied characteristics", () => {
    const concordance = buildEvidenceConcordance(fixtureScenario2Intact);
    expect(concordance.rows.find((r) => r.kind === "outcome")).toBeDefined();

    const characteristicRows = concordance.rows.filter(isCharacteristicRow);
    expect(characteristicRows).toHaveLength(5);
    expect(characteristicRows.every((r) => r.kind === "requires_any")).toBe(true);
    expect(concordance.declaredCharacteristicCount).toBe(5);
    expect(concordance.satisfiedCharacteristicCount).toBe(5);
  });

  it("every satisfied characteristic row reports Partial provenance", () => {
    const concordance = buildEvidenceConcordance(fixtureScenario2Intact);
    for (const row of concordance.rows.filter(isCharacteristicRow)) {
      expect(row.provenance.kind).toBe("partial");
    }
  });

  it("does not require or reference any exec/stdin/tty field", () => {
    const concordance = buildEvidenceConcordance(fixtureScenario2Intact);
    const serialized = JSON.stringify(concordance);
    expect(serialized).not.toMatch(/stdin|tty|exec\./);
  });
});

describe("buildEvidenceConcordance — scenario 3 (cluster-admin ClusterRoleBinding)", () => {
  it("includes a satisfied outcome row and exactly the one real requires_all characteristic", () => {
    const concordance = buildEvidenceConcordance(fixtureScenario3Intact);
    expect(concordance.rows.find((r) => r.kind === "outcome")).toBeDefined();

    const characteristicRows = concordance.rows.filter(isCharacteristicRow);
    expect(characteristicRows).toHaveLength(1);
    expect(characteristicRows[0]).toMatchObject({ kind: "requires_all", id: "role_ref_cluster_admin" });
    expect(concordance.declaredCharacteristicCount).toBe(1);
  });

  it("the operation row reflects creation, not modification or deletion", () => {
    const concordance = buildEvidenceConcordance(fixtureScenario3Intact);
    const operationRow = concordance.rows.find((r) => r.kind === "operation");
    expect(operationRow).toMatchObject({ verb: "create" });
  });
});

describe("buildEvidenceConcordance — general rules", () => {
  it("never renders an unsatisfied declared characteristic as a row", () => {
    const withUnsatisfiedOption: AlertInvestigationResponse = {
      ...fixtureScenario2Intact,
      detectionResult: {
        available: true,
        matchReason: {
          ...fixtureScenario2Intact.detectionResult.matchReason!,
          satisfiedCharacteristics: fixtureScenario2Intact.detectionResult.matchReason!.satisfiedCharacteristics.slice(
            0,
            2,
          ),
        },
      },
    };
    const concordance = buildEvidenceConcordance(withUnsatisfiedOption);
    const characteristicRows = concordance.rows.filter(isCharacteristicRow);
    expect(characteristicRows).toHaveLength(2);
    expect(concordance.declaredCharacteristicCount).toBe(5);
    expect(concordance.satisfiedCharacteristicCount).toBe(2);
  });

  it("suppresses the summary count when no characteristic list is declared at all", () => {
    const noCharacteristics: AlertInvestigationResponse = {
      ...fixtureScenario3Intact,
      detectionDefinition: {
        available: true,
        revision: fixtureScenario3Intact.detectionDefinition.revision,
        definition: {
          ...fixtureScenario3Intact.detectionDefinition.definition!,
          conditions: {
            operation: fixtureScenario3Intact.detectionDefinition.definition!.conditions.operation,
            requires_outcome: "success",
          },
        },
      },
      detectionResult: {
        available: true,
        matchReason: {
          ...fixtureScenario3Intact.detectionResult.matchReason!,
          satisfiedCharacteristics: [],
        },
      },
    };
    const concordance = buildEvidenceConcordance(noCharacteristics);
    expect(concordance.declaredCharacteristicCount).toBe(0);
    expect(concordance.showCharacteristicCount).toBe(false);
    expect(concordance.rows.filter(isCharacteristicRow)).toHaveLength(0);
  });

  it("is unavailable when the detection definition artifact is unavailable (fixturePartial)", () => {
    const concordance = buildEvidenceConcordance(fixturePartial);
    expect(concordance.available).toBe(false);
    expect(concordance.rows).toHaveLength(0);
  });

  it("never mutates its input response", () => {
    const snapshot = JSON.parse(JSON.stringify(fixtureScenario2Intact));
    buildEvidenceConcordance(fixtureScenario2Intact);
    expect(fixtureScenario2Intact).toEqual(snapshot);
  });

  it("is deterministic across repeated calls", () => {
    const first = buildEvidenceConcordance(fixtureIntact);
    const second = buildEvidenceConcordance(fixtureIntact);
    expect(first).toEqual(second);
  });
});
