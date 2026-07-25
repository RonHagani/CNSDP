import { describe, expect, it } from "vitest";
import { buildEvidenceConcordance, isCharacteristicRow, isSatisfiedCharacteristicRow } from "./concordance";
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

  it("retains the declared-but-unsatisfied exec characteristic as its own row, not discarded", () => {
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
    expect(characteristicRows).toHaveLength(2);
    expect(concordance.declaredCharacteristicCount).toBe(2);
    expect(concordance.satisfiedCharacteristicCount).toBe(1);

    const satisfiedRow = characteristicRows.find((r) => r.id === "tty_allocation")!;
    expect(satisfiedRow.satisfied).toBe(true);
    if (satisfiedRow.satisfied) {
      expect(satisfiedRow.provenance).toBeDefined();
    }

    const unsatisfiedRow = characteristicRows.find((r) => r.id === "stdin_streaming")!;
    expect(unsatisfiedRow.satisfied).toBe(false);
    expect(unsatisfiedRow).not.toHaveProperty("provenance");
  });

  it("every satisfied characteristic row carries a provenance state", () => {
    const concordance = buildEvidenceConcordance(fixtureIntact);
    const characteristicRows = concordance.rows.filter(isCharacteristicRow);
    expect(characteristicRows.every((r) => r.satisfied)).toBe(true);
    for (const row of characteristicRows.filter(isSatisfiedCharacteristicRow)) {
      expect(row.provenance.kind).toBe("verified");
    }
  });

  it("scenario 1's two characteristics resolve to a single shared group label, never scenario 2's real labels", () => {
    const concordance = buildEvidenceConcordance(fixtureIntact);
    const groups = new Set(concordance.rows.filter(isCharacteristicRow).map((r) => r.group));
    expect(groups.size).toBe(1);
    expect(groups.has("Host access")).toBe(false);
    expect(groups.has("Privilege")).toBe(false);
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
    const characteristicRows = concordance.rows.filter(isCharacteristicRow);
    expect(characteristicRows.every((r) => r.satisfied)).toBe(true);
    for (const row of characteristicRows.filter(isSatisfiedCharacteristicRow)) {
      expect(row.provenance.kind).toBe("partial");
    }
  });

  it("does not require or reference any exec/stdin/tty field", () => {
    const concordance = buildEvidenceConcordance(fixtureScenario2Intact);
    const serialized = JSON.stringify(concordance);
    expect(serialized).not.toMatch(/stdin|tty|exec\./);
  });

  it("splits its five real characteristics into exactly the host-access/privilege groups, data-driven from characteristic id alone", () => {
    const concordance = buildEvidenceConcordance(fixtureScenario2Intact);
    const rows = concordance.rows.filter(isCharacteristicRow);
    const byId = Object.fromEntries(rows.map((r) => [r.id, r.group]));
    expect(byId).toEqual({
      privileged_container: "Privilege",
      host_network: "Host access",
      host_pid: "Host access",
      host_ipc: "Host access",
      host_path_volume: "Host access",
    });
    const groups = new Set(rows.map((r) => r.group));
    expect(groups).toEqual(new Set(["Host access", "Privilege"]));
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

  it("its single characteristic resolves to the shared ungrouped label, not scenario 2's real group labels", () => {
    const concordance = buildEvidenceConcordance(fixtureScenario3Intact);
    const characteristicRows = concordance.rows.filter(isCharacteristicRow);
    expect(characteristicRows).toHaveLength(1);
    expect(characteristicRows[0].group).not.toBe("Host access");
    expect(characteristicRows[0].group).not.toBe("Privilege");
  });

  it("does not require or reference any podCreation or exec field (no cross-scenario data leak)", () => {
    const concordance = buildEvidenceConcordance(fixtureScenario3Intact);
    const serialized = JSON.stringify(concordance);
    expect(serialized).not.toMatch(/podCreation|stdin_streaming|tty_allocation|host_/);
  });
});

describe("buildEvidenceConcordance — general rules", () => {
  /**
   * A `requires_any` group's real semantics (UX spec §2) require only "at
   * least one" declared option to be satisfied for a match — a detection
   * result whose `satisfiedCharacteristics` names fewer than every declared
   * option is therefore a legitimate, contract-representable shape, not a
   * fabricated one. No real captured backend testdata exercises this for
   * scenario 2 today (implementation plan §8), so — per the plan's own
   * documented fallback for exactly this gap — this is a scoped, hand-built
   * response object for this test file alone, built by varying only the
   * one field (`satisfiedCharacteristics`) the real contract already
   * allows to vary independently of the rest of the response.
   */
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

  it("retains every declared requires_any characteristic as a row, satisfied or not", () => {
    const concordance = buildEvidenceConcordance(withUnsatisfiedOption);
    const characteristicRows = concordance.rows.filter(isCharacteristicRow);
    expect(characteristicRows).toHaveLength(5);
    expect(concordance.declaredCharacteristicCount).toBe(5);
    expect(concordance.satisfiedCharacteristicCount).toBe(2);
  });

  it("distinguishes satisfied from declared-but-unsatisfied rows, and never fabricates provenance for the latter", () => {
    const concordance = buildEvidenceConcordance(withUnsatisfiedOption);
    const characteristicRows = concordance.rows.filter(isCharacteristicRow);
    const satisfiedRows = characteristicRows.filter(isSatisfiedCharacteristicRow);
    const unsatisfiedRows = characteristicRows.filter((r) => !r.satisfied);
    expect(satisfiedRows).toHaveLength(2);
    expect(unsatisfiedRows).toHaveLength(3);
    for (const r of satisfiedRows) {
      expect(r.provenance).toBeDefined();
    }
    for (const r of unsatisfiedRows) {
      // Not merely undefined — the discriminated union makes `provenance`
      // an unrepresentable property on an unsatisfied row at all.
      expect(r).not.toHaveProperty("provenance");
    }
  });

  it("still assigns a real group label to declared-but-unsatisfied rows (grouping is independent of satisfaction)", () => {
    const concordance = buildEvidenceConcordance(withUnsatisfiedOption);
    const unsatisfiedRows = concordance.rows.filter(isCharacteristicRow).filter((r) => !r.satisfied);
    expect(unsatisfiedRows.length).toBeGreaterThan(0);
    for (const r of unsatisfiedRows) {
      expect(["Host access", "Privilege"]).toContain(r.group);
    }
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
