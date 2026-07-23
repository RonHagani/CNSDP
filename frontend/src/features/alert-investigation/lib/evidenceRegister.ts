import type { AlertInvestigationResponse, EvidenceArtifactId } from "@/types/contract";

/**
 * The Evidence Register model (UX spec §3.4): the fixed six-artifact
 * ledger. Traceability is deliberately absent — it is not a seventh
 * artifact (PD-04 scope decision 8; FR-031). The per-artifact summary
 * field choices below extract the same fields StageContent.tsx's
 * per-artifact renders already treat as each artifact's essential content
 * (validation outcome + reason; detection definition name/scenario;
 * matched definition/subject/operation/target/outcome for the alert) —
 * StageContent.tsx itself is not imported or modified; only its field
 * selection is reused, condensed into a single factual line per artifact.
 */

export const EVIDENCE_ARTIFACT_ORDER: readonly EvidenceArtifactId[] = [
  "source-event",
  "validation-outcome",
  "normalized-event",
  "detection-definition",
  "detection-result",
  "alert",
];

export const EVIDENCE_ARTIFACT_LABELS: Record<EvidenceArtifactId, string> = {
  "source-event": "Source submission",
  "validation-outcome": "Validation outcome",
  "normalized-event": "Normalized event",
  "detection-definition": "Detection definition",
  "detection-result": "Detection result",
  alert: "Generated alert",
};

export interface EvidenceRegisterEntry {
  key: EvidenceArtifactId;
  index: number; // 1-6, fixed, never reordered
  total: number; // always 6
  label: string;
  available: boolean;
  /** A concise, factual, single-line account of this artifact's real
   *  content — never a severity, confidence, or invented claim. */
  summary: string;
  isDefaultSelection: boolean;
  /** Other artifact keys this one has a real, documented relationship to
   *  (e.g. the source event a normalized event was produced from), for a
   *  future "related artifact" navigation action — not itself a
   *  navigation or DOM concept. */
  relatedArtifacts: EvidenceArtifactId[];
}

export function isArtifactAvailable(key: EvidenceArtifactId, data: AlertInvestigationResponse): boolean {
  switch (key) {
    case "source-event":
      return data.sourceEvent.available;
    case "validation-outcome":
      return data.validationOutcome.available;
    case "normalized-event":
      return data.normalizedEvent.available;
    case "detection-definition":
      return data.detectionDefinition.available;
    case "detection-result":
      return data.detectionResult.available;
    case "alert":
      return data.alert.available;
  }
}

export function allArtifactsAvailable(data: AlertInvestigationResponse): boolean {
  return EVIDENCE_ARTIFACT_ORDER.every((key) => isArtifactAvailable(key, data));
}

function unavailableSummary(key: EvidenceArtifactId): string {
  return `${EVIDENCE_ARTIFACT_LABELS[key]} is not available for this alert.`;
}

function summarizeArtifact(key: EvidenceArtifactId, data: AlertInvestigationResponse): string {
  if (!isArtifactAvailable(key, data)) return unavailableSummary(key);

  switch (key) {
    case "source-event": {
      const raw = data.sourceEvent.rawEvent;
      if (!raw) return unavailableSummary(key);
      return `${raw.verb} request, audit ID ${raw.auditID}`;
    }
    case "validation-outcome": {
      const outcome = data.validationOutcome.outcome;
      if (!outcome) return unavailableSummary(key);
      return data.validationOutcome.reason
        ? `Classified ${outcome} — ${data.validationOutcome.reason}`
        : `Classified ${outcome}`;
    }
    case "normalized-event": {
      const event = data.normalizedEvent.event;
      if (!event) return unavailableSummary(key);
      const subresource = event.target.subresource ? `/${event.target.subresource}` : "";
      return `${event.operation.verb} ${event.target.resource}${subresource}`;
    }
    case "detection-definition": {
      const definition = data.detectionDefinition.definition;
      const revision = data.detectionDefinition.revision;
      if (!definition || !revision) return unavailableSummary(key);
      return `${definition.name} (${definition.scenario}), revision ${revision}`;
    }
    case "detection-result": {
      const matchReason = data.detectionResult.matchReason;
      if (!matchReason) return unavailableSummary(key);
      const count = matchReason.satisfiedCharacteristics.length;
      return `Matched ${matchReason.scenario} — ${count} characteristic${count === 1 ? "" : "s"} satisfied`;
    }
    case "alert": {
      const summary = data.alert.summary;
      if (!summary) return unavailableSummary(key);
      return `Alert #${data.alertId} — ${summary.matchReason.definitionName}`;
    }
  }
}

function relatedArtifactsFor(key: EvidenceArtifactId): EvidenceArtifactId[] {
  switch (key) {
    case "source-event":
      return ["normalized-event"];
    case "validation-outcome":
      return [];
    case "normalized-event":
      return ["source-event", "detection-result"];
    case "detection-definition":
      return ["detection-result"];
    case "detection-result":
      return ["detection-definition", "normalized-event", "alert"];
    case "alert":
      return ["detection-result"];
  }
}

/**
 * Default-selection rule (UX spec §3.4): Detection Result when available;
 * otherwise a deterministic fallback so a selection always exists. The
 * fallback walks the fixed canonical order and returns the first
 * available artifact, so the choice never depends on object iteration
 * order or which artifact happened to be requested last. If, in the
 * unreachable case that every artifact is unavailable, no fallback is
 * found, Detection Result is still returned — the function is total and
 * never returns undefined.
 */
export function resolveDefaultSelection(data: AlertInvestigationResponse): EvidenceArtifactId {
  if (data.detectionResult.available) return "detection-result";
  const fallback = EVIDENCE_ARTIFACT_ORDER.find((key) => isArtifactAvailable(key, data));
  return fallback ?? "detection-result";
}

/**
 * Builds the fixed six-entry Evidence Register. Always exactly six
 * entries, in canonical order, regardless of availability — an
 * unavailable artifact remains present with its own explicit summary
 * (FR-035), never omitted.
 *
 * `defaultSelectedArtifact` is supplied by the caller (see
 * `resolveDefaultSelection`) rather than recomputed here, so the whole
 * investigation view model resolves the default selection exactly once
 * and every consumer of it — this register's `isDefaultSelection` flags
 * and the root view model's own `defaultSelectedArtifact` field — agrees
 * with a single source of truth by construction, not by two independent
 * calls that merely happen to return the same value.
 */
export function buildEvidenceRegister(
  data: AlertInvestigationResponse,
  defaultSelectedArtifact: EvidenceArtifactId,
): EvidenceRegisterEntry[] {
  return EVIDENCE_ARTIFACT_ORDER.map((key, i) => ({
    key,
    index: i + 1,
    total: EVIDENCE_ARTIFACT_ORDER.length,
    label: EVIDENCE_ARTIFACT_LABELS[key],
    available: isArtifactAvailable(key, data),
    summary: summarizeArtifact(key, data),
    isDefaultSelection: key === defaultSelectedArtifact,
    relatedArtifacts: relatedArtifactsFor(key),
  }));
}
