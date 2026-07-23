import type {
  AlertInvestigationResponse,
  AlertSummary,
  DetectionDefinition,
  EvidenceArtifactId,
  MatchReason,
  NormalizedEvent,
  RawAuditEvent,
} from "@/types/contract";
import { buildDeclaredConditions, type DeclaredConditions } from "./detectionConditions";

/**
 * Typed, presentation-neutral view models for each of the six evidence
 * artifacts' full Inspection Leaf content (UX spec §3.5a, §6). Each
 * artifact's shape is genuinely different, so each gets its own typed
 * discriminated union rather than being forced into a generic key/value
 * array — a raw event, an outcome-plus-reason pair, and a literal
 * detection-conditions object are not interchangeable shapes.
 */

export type SourceEventInspection = { available: true; rawEvent: RawAuditEvent } | { available: false };

export type ValidationOutcomeInspection =
  | { available: true; outcome: "valid" | "invalid" | "incomplete" | "unsupported"; reason?: string }
  | { available: false };

export type NormalizedEventInspection = { available: true; event: NormalizedEvent } | { available: false };

export type DetectionDefinitionInspection =
  | {
      available: true;
      revision: string;
      definition: DetectionDefinition;
      /** Full declared-vs-satisfied breakdown, including any
       *  declared-but-unsatisfied requires_any option — the one place an
       *  unsatisfied characteristic is inspectable (UX spec §6 item 4). */
      declaredConditions: DeclaredConditions | null;
    }
  | { available: false };

export type DetectionResultInspection = { available: true; matchReason: MatchReason } | { available: false };

export type AlertInspection = { available: true; summary: AlertSummary } | { available: false };

export interface ArtifactInspectionModel {
  sourceEvent: SourceEventInspection;
  validationOutcome: ValidationOutcomeInspection;
  normalizedEvent: NormalizedEventInspection;
  detectionDefinition: DetectionDefinitionInspection;
  detectionResult: DetectionResultInspection;
  alert: AlertInspection;
}

/**
 * Builds the full-detail inspection model for every artifact keyed by
 * `EvidenceArtifactId` for lookup convenience, alongside a keyed record.
 * Deterministic and side-effect-free; never mutates `data`.
 */
export function buildArtifactInspection(data: AlertInvestigationResponse): ArtifactInspectionModel {
  return {
    sourceEvent:
      data.sourceEvent.available && data.sourceEvent.rawEvent
        ? { available: true, rawEvent: data.sourceEvent.rawEvent }
        : { available: false },
    validationOutcome:
      data.validationOutcome.available && data.validationOutcome.outcome
        ? {
            available: true,
            outcome: data.validationOutcome.outcome,
            reason: data.validationOutcome.reason,
          }
        : { available: false },
    normalizedEvent:
      data.normalizedEvent.available && data.normalizedEvent.event
        ? { available: true, event: data.normalizedEvent.event }
        : { available: false },
    detectionDefinition:
      data.detectionDefinition.available &&
      data.detectionDefinition.definition &&
      data.detectionDefinition.revision
        ? {
            available: true,
            revision: data.detectionDefinition.revision,
            definition: data.detectionDefinition.definition,
            declaredConditions: buildDeclaredConditions(
              data.detectionDefinition,
              data.detectionResult,
              data.normalizedEvent,
            ),
          }
        : { available: false },
    detectionResult:
      data.detectionResult.available && data.detectionResult.matchReason
        ? { available: true, matchReason: data.detectionResult.matchReason }
        : { available: false },
    alert:
      data.alert.available && data.alert.summary
        ? { available: true, summary: data.alert.summary }
        : { available: false },
  };
}

/** Looks up one artifact's inspection model by key — a convenience for
 *  consumers that already have an `EvidenceArtifactId` (e.g. the current
 *  Evidence Register selection) and want its detail without a switch. */
export function artifactInspectionFor(
  key: EvidenceArtifactId,
  model: ArtifactInspectionModel,
):
  | SourceEventInspection
  | ValidationOutcomeInspection
  | NormalizedEventInspection
  | DetectionDefinitionInspection
  | DetectionResultInspection
  | AlertInspection {
  switch (key) {
    case "source-event":
      return model.sourceEvent;
    case "validation-outcome":
      return model.validationOutcome;
    case "normalized-event":
      return model.normalizedEvent;
    case "detection-definition":
      return model.detectionDefinition;
    case "detection-result":
      return model.detectionResult;
    case "alert":
      return model.alert;
  }
}
