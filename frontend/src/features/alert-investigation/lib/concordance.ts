import type { AlertInvestigationResponse } from "@/types/contract";
import { characteristicGroup } from "./characteristicGroups";
import { buildDeclaredConditions, type ConditionGroupKind } from "./detectionConditions";
import { deriveProvenanceState, type ProvenanceState } from "./provenance";

/**
 * The Evidence Concordance row model (UX spec §3.3, §8): the CLAIM's
 * DOCUMENTED CONDITION → OBSERVED FACT record. Every declared
 * `requires_any`/`requires_all` characteristic produces a row — satisfied
 * or not — since UX spec §8 requires a declared-but-unsatisfied
 * `requires_any` option to render in its own recessed, still-selectable
 * state, never silently discarded (FR-022). A row's own `satisfied` field
 * is what distinguishes matched evidence from a declared-only fact; only
 * matched evidence may be traced to source (UX spec §12), so `provenance`
 * is present only when `satisfied` is `true` — never fabricated for an
 * unsatisfied option.
 *
 * Rows are a flat array; each row self-identifies its origin via `kind`,
 * so a consumer that wants requires_any and requires_all rendered as two
 * separately labeled groups can do so by filtering on `kind` — no nested
 * grouping structure is imposed here.
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

/** Fields every declared characteristic row carries regardless of whether
 *  it was satisfied — `group` (`lib/characteristicGroups.ts`) is a property
 *  of the characteristic's own identity, not of whether it matched, so it
 *  belongs here rather than on only one union member. */
interface CharacteristicConcordanceRowBase {
  kind: ConditionGroupKind;
  id: string;
  description: string;
  group: string;
}

/** A declared characteristic the detection result actually matched.
 *  `provenance` is mandatory here — matched evidence always has a real,
 *  derived provenance record (UX spec §12, §13). */
export interface SatisfiedCharacteristicRow extends CharacteristicConcordanceRowBase {
  satisfied: true;
  provenance: ProvenanceState;
}

/** A declared characteristic (only possible within a `requires_any` group,
 *  FR-022) the detection result did not match. `provenance` is not a
 *  representable field on this member at all — there is no matched
 *  evidence to derive a raw origin from, so the type itself forbids
 *  fabricating one (UX spec §8, §12's "only matched evidence may be traced
 *  to source"). */
export interface UnsatisfiedCharacteristicRow extends CharacteristicConcordanceRowBase {
  satisfied: false;
}

/** Every declared `requires_any`/`requires_all` characteristic produces a
 *  row — satisfied or not (UX spec §8; FR-022) — and the two are a proper
 *  discriminated union on `satisfied`: a consumer that narrows on
 *  `row.satisfied` gets `row.provenance` typed correctly (mandatory when
 *  `true`, absent entirely when `false`) with no cast or non-null
 *  assertion required anywhere. */
export type CharacteristicConcordanceRow = SatisfiedCharacteristicRow | UnsatisfiedCharacteristicRow;

export type ConcordanceRow =
  | OperationConcordanceRow
  | OutcomeConcordanceRow
  | CharacteristicConcordanceRow;

export function isCharacteristicRow(row: ConcordanceRow): row is CharacteristicConcordanceRow {
  return row.kind === "requires_any" || row.kind === "requires_all";
}

/** Narrows a `CharacteristicConcordanceRow` to its satisfied member — the
 *  one place a consumer should test satisfaction, rather than
 *  independently checking whether `provenance` happens to be present. */
export function isSatisfiedCharacteristicRow(
  row: CharacteristicConcordanceRow,
): row is SatisfiedCharacteristicRow {
  return row.satisfied;
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
      const shared = {
        kind: group.kind,
        id: c.id,
        description: c.description,
        group: characteristicGroup(c.id),
      };
      rows.push(
        c.satisfied
          ? { ...shared, satisfied: true, provenance: deriveProvenanceState(c, data) }
          : { ...shared, satisfied: false },
      );
    }
  }

  const declaredCharacteristicCount =
    (declared.requiresAny?.characteristics.length ?? 0) +
    (declared.requiresAll?.characteristics.length ?? 0);
  const satisfiedCharacteristicCount = rows.filter(isCharacteristicRow).filter((r) => r.satisfied).length;

  return {
    available: true,
    rows,
    declaredCharacteristicCount,
    satisfiedCharacteristicCount,
    showCharacteristicCount: declaredCharacteristicCount > 0,
  };
}
