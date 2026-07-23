import type { AlertInvestigationResponse } from "@/types/contract";
import { buildDeclaredConditions, type ConditionGroupKind } from "./detectionConditions";
import { deriveProvenanceState, type ProvenanceState } from "./provenance";

/**
 * The Evidence Concordance row model (UX spec §3.3): the CLAIM's
 * DOCUMENTED CONDITION → OBSERVED FACT record. Contains only conditions
 * actually satisfied by the current detection result — it is a record of
 * matched evidence, not a rendering of the full definition (that full
 * text, satisfied or not, is `detectionConditions.ts`'s job, consumed
 * separately by the Detection Definition Inspection Leaf).
 *
 * Rows are a flat array; each row self-identifies its origin via `kind`,
 * so a consumer that wants requires_any and requires_all rendered as two
 * separately labeled groups can do so by filtering on `kind` — no nested
 * grouping structure is imposed here.
 *
 * Only characteristic rows (`requires_any` / `requires_all`) carry a
 * ProvenanceState. Operation and outcome rows are structural facts about
 * which operation and outcome this definition requires — real content,
 * but not a characteristic with its own `{id, description}` identity in
 * the contract, so they are represented distinctly rather than being
 * forced into the same shape.
 */

export interface OperationConcordanceRow {
  kind: "operation";
  resource: string;
  subresource?: string;
  verb?: string;
}

export interface OutcomeConcordanceRow {
  kind: "outcome";
  requiredOutcome: string;
  recordedOutcomeCode?: number;
}

export interface CharacteristicConcordanceRow {
  kind: ConditionGroupKind;
  id: string;
  description: string;
  provenance: ProvenanceState;
}

export type ConcordanceRow =
  | OperationConcordanceRow
  | OutcomeConcordanceRow
  | CharacteristicConcordanceRow;

export function isCharacteristicRow(row: ConcordanceRow): row is CharacteristicConcordanceRow {
  return row.kind === "requires_any" || row.kind === "requires_all";
}

export interface EvidenceConcordance {
  available: boolean;
  rows: ConcordanceRow[];
  declaredCharacteristicCount: number;
  satisfiedCharacteristicCount: number;
  /** Whether a "N of M declared characteristics satisfied" caption applies
   *  at all — false when no requires_any/requires_all list is declared,
   *  so a consumer never fabricates a "0 of 0" count. */
  showCharacteristicCount: boolean;
}

const EMPTY_CONCORDANCE: EvidenceConcordance = {
  available: false,
  rows: [],
  declaredCharacteristicCount: 0,
  satisfiedCharacteristicCount: 0,
  showCharacteristicCount: false,
};

/**
 * Builds the Evidence Concordance for the current detection result.
 * Deterministic and side-effect-free; never mutates `data`.
 */
export function buildEvidenceConcordance(data: AlertInvestigationResponse): EvidenceConcordance {
  const declared = buildDeclaredConditions(
    data.detectionDefinition,
    data.detectionResult,
    data.normalizedEvent,
  );
  if (!declared) return EMPTY_CONCORDANCE;

  const rows: ConcordanceRow[] = [
    {
      kind: "operation",
      resource: declared.operation.resource,
      subresource: declared.operation.subresource,
      verb: declared.operation.verb,
    },
  ];

  if (declared.outcome && declared.outcome.satisfied) {
    rows.push({
      kind: "outcome",
      requiredOutcome: declared.outcome.requiredOutcome,
      recordedOutcomeCode: declared.outcome.recordedOutcomeCode,
    });
  }

  for (const group of [declared.requiresAny, declared.requiresAll]) {
    if (!group) continue;
    for (const c of group.characteristics) {
      if (!c.satisfied) continue; // never render an unsatisfied declared characteristic as matched evidence
      rows.push({
        kind: group.kind,
        id: c.id,
        description: c.description,
        provenance: deriveProvenanceState(c, data),
      });
    }
  }

  const declaredCharacteristicCount =
    (declared.requiresAny?.characteristics.length ?? 0) +
    (declared.requiresAll?.characteristics.length ?? 0);
  const satisfiedCharacteristicCount = rows.filter(isCharacteristicRow).length;

  return {
    available: true,
    rows,
    declaredCharacteristicCount,
    satisfiedCharacteristicCount,
    showCharacteristicCount: declaredCharacteristicCount > 0,
  };
}
