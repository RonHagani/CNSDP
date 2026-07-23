import type { AlertInvestigationResponse, SignalPathStageId } from "@/types/contract";
import type { StatusTone } from "@/components/StatusBadge/StatusBadge";

export interface StageStatus {
  tone: StatusTone;
  available: boolean;
}

/**
 * Derives each Signal Path stage's real status directly from the response
 * — never a simulated or invented state. "Evidence" reflects whether the
 * complete six-artifact set is present (mirrors evidence.Inventory.Complete
 * gating the alerted→evidenced transition server-side). "Traceability" and
 * "Investigation" are not workflow states (stageMapping.ts) and are derived
 * from the live verification result and the fact that this screen itself is
 * the Investigation stage, respectively.
 */
export function deriveStageStatus(
  data: AlertInvestigationResponse,
): Record<SignalPathStageId, StageStatus> {
  const allSixAvailable =
    data.sourceEvent.available &&
    data.validationOutcome.available &&
    data.normalizedEvent.available &&
    data.detectionDefinition.available &&
    data.detectionResult.available &&
    data.alert.available;

  const validationTone: StatusTone = !data.validationOutcome.available
    ? "severed"
    : data.validationOutcome.outcome === "valid"
      ? "signal"
      : "branch";

  return {
    intake: {
      tone: data.sourceEvent.available ? "signal" : "severed",
      available: data.sourceEvent.available,
    },
    validation: { tone: validationTone, available: data.validationOutcome.available },
    normalization: {
      tone: data.normalizedEvent.available ? "signal" : "severed",
      available: data.normalizedEvent.available,
    },
    "detection-evaluation": {
      tone:
        data.detectionDefinition.available && data.detectionResult.available ? "signal" : "severed",
      available: data.detectionDefinition.available && data.detectionResult.available,
    },
    alerting: {
      tone: data.alert.available ? "signal" : "severed",
      available: data.alert.available,
    },
    evidence: { tone: allSixAvailable ? "signal" : "branch", available: allSixAvailable },
    traceability: {
      tone: data.traceability.intact ? "signal" : "severed",
      available: true,
    },
    investigation: { tone: "signal", available: true },
  };
}
