import type { AlertInvestigationResponse, Operation, Outcome, Subject, Target } from "@/types/contract";

/**
 * The Finding model (UX spec §3.2): the CLAIM — what the platform asserts
 * happened and what it detected — built entirely from `alert.summary` and
 * `detectionResult.matchReason`, both preserved verbatim as the backend
 * recorded them. Nothing here is inferred, scored, or invented: no
 * severity, confidence, or intent is derived, and scenario identity comes
 * from the real `scenario`/`definitionName` strings the backend already
 * produced, never from a hardcoded assumption about any one scenario's
 * fields.
 */
export interface FindingViewModel {
  available: boolean;
  subject?: Subject;
  operation?: Operation;
  target?: Target;
  outcome?: Outcome;
  requestTime?: string;
  matchedScenario?: string;
  matchedDefinitionName?: string;
}

const UNAVAILABLE_FINDING: FindingViewModel = { available: false };

/**
 * Builds the Finding. When `alert.summary` is unavailable, returns the
 * explicit unavailable variant (FR-035) rather than a blank or
 * partially-populated result.
 */
export function buildFinding(data: AlertInvestigationResponse): FindingViewModel {
  if (!data.alert.available || !data.alert.summary) {
    return UNAVAILABLE_FINDING;
  }
  const summary = data.alert.summary;
  const matchReason = data.detectionResult.available ? data.detectionResult.matchReason : undefined;

  return {
    available: true,
    subject: summary.subject,
    operation: summary.operation,
    target: summary.target,
    outcome: summary.outcome,
    requestTime: summary.requestTime,
    matchedScenario: matchReason?.scenario,
    matchedDefinitionName: matchReason?.definitionName,
  };
}
