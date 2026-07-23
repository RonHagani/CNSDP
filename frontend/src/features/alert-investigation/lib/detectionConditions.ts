import type { AlertInvestigationResponse, Characteristic, DetectionOperation } from "@/types/contract";

/**
 * The declared-conditions layer (UX spec §3.3, §6 item 4): a normalized,
 * presentation-neutral view of one detection definition's conditions that
 * keeps operation, outcome, and each characteristic group distinct — never
 * flattened into one list — so a consumer can tell whether a condition
 * came from `requires_any`, `requires_all`, or the outcome requirement
 * without re-deriving it from raw contract shape. This is the layer the
 * Detection Definition Inspection Leaf needs, since it must show the full
 * literal definition, including any declared-but-unsatisfied
 * `requires_any` option, without assuming every declared condition
 * matched (only `requires_any` can have such an option; `requires_all` and
 * the outcome requirement are, for an existing alert, always satisfied by
 * construction — see buildDeclaredConditions below).
 */

export interface DeclaredCharacteristic extends Characteristic {
  satisfied: boolean;
}

export type ConditionGroupKind = "requires_any" | "requires_all";

export interface DeclaredConditionGroup {
  kind: ConditionGroupKind;
  characteristics: DeclaredCharacteristic[];
  /**
   * True when this group's own logical requirement holds: at least one
   * characteristic satisfied for requires_any; every declared
   * characteristic satisfied for requires_all.
   */
  satisfied: boolean;
}

export interface DeclaredOutcomeRequirement {
  requiredOutcome: string;
  recordedOutcomeCode?: number;
  satisfied: boolean;
}

export interface DeclaredConditions {
  operation: DetectionOperation;
  outcome: DeclaredOutcomeRequirement | null;
  requiresAny: DeclaredConditionGroup | null;
  requiresAll: DeclaredConditionGroup | null;
}

/**
 * Recognized `requires_outcome` values and their evaluation rule, mirroring
 * the backend's own definition exactly (internal/detection/evaluate.go
 * `outcomeSatisfied`): "success" means a recorded HTTP response code in
 * the inclusive-exclusive range [200, 300). Any other declared value is a
 * definition-authoring detail this frontend does not validate and, like
 * the backend, treats as never satisfied rather than throwing — an
 * unrecognized constraint fails closed, not open.
 */
export function isOutcomeSatisfied(requiredOutcome: string, code: number): boolean {
  switch (requiredOutcome) {
    case "success":
      return code >= 200 && code < 300;
    default:
      return false;
  }
}

function buildGroup(
  kind: ConditionGroupKind,
  list: Characteristic[] | undefined,
  satisfiedIds: Set<string>,
): DeclaredConditionGroup | null {
  if (!list || list.length === 0) return null;
  const characteristics: DeclaredCharacteristic[] = list.map((c) => ({
    ...c,
    satisfied: satisfiedIds.has(c.id),
  }));
  const satisfied =
    kind === "requires_any"
      ? characteristics.some((c) => c.satisfied)
      : characteristics.every((c) => c.satisfied);
  return { kind, characteristics, satisfied };
}

/**
 * Builds the full declared-conditions model for the matched detection
 * definition. Returns null only when there is no definition to describe —
 * `detectionDefinition.available` is false or its content is missing
 * (FR-035: absence is represented, never fabricated).
 */
export function buildDeclaredConditions(
  detectionDefinition: AlertInvestigationResponse["detectionDefinition"],
  detectionResult: AlertInvestigationResponse["detectionResult"],
  normalizedEvent: AlertInvestigationResponse["normalizedEvent"],
): DeclaredConditions | null {
  const definition = detectionDefinition.available ? detectionDefinition.definition : undefined;
  if (!definition) return null;

  const satisfiedIds = new Set(
    detectionResult.available
      ? (detectionResult.matchReason?.satisfiedCharacteristics.map((c) => c.id) ?? [])
      : [],
  );

  const recordedOutcomeCode = normalizedEvent.available
    ? normalizedEvent.event?.outcome.code
    : undefined;

  const requiresOutcome = definition.conditions.requires_outcome;
  const outcome: DeclaredOutcomeRequirement | null = requiresOutcome
    ? {
        requiredOutcome: requiresOutcome,
        recordedOutcomeCode,
        satisfied:
          recordedOutcomeCode !== undefined
            ? isOutcomeSatisfied(requiresOutcome, recordedOutcomeCode)
            : // The recorded outcome code is not currently readable (the
              // normalized event is unavailable), but a persisted matching
              // detection result can only exist if every declared
              // condition, including this one, held at evaluation time
              // (FR-024-FR-026's AND-combined matching) — its existence is
              // itself sufficient evidence, even though the specific code
              // cannot currently be re-displayed.
              detectionResult.available,
      }
    : null;

  return {
    operation: definition.conditions.operation,
    outcome,
    requiresAny: buildGroup("requires_any", definition.conditions.requires_any, satisfiedIds),
    requiresAll: buildGroup("requires_all", definition.conditions.requires_all, satisfiedIds),
  };
}
