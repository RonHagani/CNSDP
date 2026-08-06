/**
 * Wire types for `GET /v1/alerts/{id}`.
 *
 * Mirrors, field for field, the real backend response defined by:
 *   - internal/retrieval/retrieval.go        (response, sourceEventField,
 *                                              validationOutcomeField,
 *                                              normalizedEventField,
 *                                              definitionField,
 *                                              detectionResultField,
 *                                              alertField, traceabilityField)
 *   - internal/normalization/normalization.go (Event, Subject, Operation,
 *                                              Target, Outcome,
 *                                              ExecCharacteristics,
 *                                              PodCreationCharacteristics,
 *                                              ClusterRoleBindingCharacteristics)
 *   - internal/detection/detection.go         (Definition, Conditions,
 *                                              Operation, Characteristic)
 *   - internal/detection/evaluate.go          (MatchReason)
 *   - internal/alerting/alerting.go           (Summary)
 *   - internal/traceability/traceability.go   (Result)
 *
 * This file introduces no field the real contract does not already define.
 * When the real backend contract changes, this file must change with it —
 * see docs/frontend/product-experience-brief.md §10.1 (fixture policy).
 */

export interface RawAuditEvent {
  kind: "Event";
  apiVersion: "audit.k8s.io/v1";
  level?: string;
  auditID: string;
  stage: string;
  requestURI: string;
  verb: string;
  user: {
    username: string;
    groups?: string[];
    extra?: Record<string, string[]>;
  };
  sourceIPs?: string[];
  userAgent?: string;
  objectRef?: {
    resource: string;
    namespace?: string;
    name?: string;
    apiVersion?: string;
    subresource?: string;
    apiGroup?: string;
  };
  responseStatus?: {
    code?: number;
  };
  requestObject?: unknown;
  requestReceivedTimestamp: string;
  stageTimestamp?: string;
  annotations?: Record<string, string>;
}

export interface Subject {
  username: string;
}

export interface Operation {
  verb: string;
  requestURI: string;
}

export interface Target {
  resource: string;
  name: string;
  namespace?: string;
  subresource?: string;
}

export interface Outcome {
  code?: number;
}

export interface ExecCharacteristics {
  stdin: boolean;
  tty: boolean;
}

export interface PodCreationCharacteristics {
  privileged: boolean;
  hostNetwork: boolean;
  hostPID: boolean;
  hostIPC: boolean;
  hostPathVolume: boolean;
}

export interface RoleRef {
  apiGroup: string;
  kind: string;
  name: string;
}

export interface RbacSubject {
  kind: string;
  apiGroup?: string;
  name: string;
  namespace?: string;
}

export interface ClusterRoleBindingCharacteristics {
  bindingName: string;
  roleRef: RoleRef;
  subjects?: RbacSubject[];
}

/** internal/normalization.Event */
export interface NormalizedEvent {
  subject: Subject;
  operation: Operation;
  target: Target;
  outcome: Outcome;
  requestTime: string;
  exec?: ExecCharacteristics;
  podCreation?: PodCreationCharacteristics;
  clusterRoleBinding?: ClusterRoleBindingCharacteristics;
}

export interface Characteristic {
  id: string;
  description: string;
}

export interface DetectionOperation {
  resource: string;
  subresource?: string;
  verb?: string;
}

export interface DetectionConditions {
  operation: DetectionOperation;
  requires_outcome?: string;
  requires_any?: Characteristic[];
  requires_all?: Characteristic[];
}

/** internal/detection.Definition */
export interface DetectionDefinition {
  scenario: string;
  name: string;
  description: string;
  conditions: DetectionConditions;
}

/** internal/detection.MatchReason */
export interface MatchReason {
  scenario: string;
  definitionName: string;
  definitionRevision: string;
  satisfiedCharacteristics: Characteristic[];
}

/** internal/alerting.Summary */
export interface AlertSummary {
  matchReason: MatchReason;
  subject: Subject;
  operation: Operation;
  target: Target;
  outcome: Outcome;
  requestTime: string;
}

export type FailedLink = "alert" | "source_key" | "raw_event_sha256";

/** internal/traceability.Result */
export interface TraceabilityResult {
  intact: boolean;
  failedLink?: FailedLink;
}

/**
 * The exact `GET /v1/alerts/{id}` response shape (internal/retrieval
 * response). Every top-level artifact field carries its own `available`
 * flag (FR-035) — an unavailable artifact is a distinct, visible state,
 * never a blank or a fabricated placeholder. Consumers must render each
 * `available: false` case explicitly.
 */
export interface AlertInvestigationResponse {
  alertId: number;
  sourceEvent: {
    available: boolean;
    rawEvent?: RawAuditEvent;
  };
  validationOutcome: {
    available: boolean;
    outcome?: "valid" | "invalid" | "incomplete" | "unsupported";
    reason?: string;
  };
  normalizedEvent: {
    available: boolean;
    event?: NormalizedEvent;
  };
  detectionDefinition: {
    available: boolean;
    revision?: string;
    definition?: DetectionDefinition;
  };
  detectionResult: {
    available: boolean;
    matchReason?: MatchReason;
  };
  alert: {
    available: boolean;
    summary?: AlertSummary;
  };
  traceability: TraceabilityResult;
}

/**
 * Wire types for `GET /v1/alerts` (internal/retrieval/list.go). A
 * dedicated summary contract, distinct from `AlertInvestigationResponse`
 * -- each row carries only the fields an inventory row needs, never any
 * row's raw event, normalized event, or detection definition content.
 */
export interface AlertSummaryListItem {
  alertId: number;
  detectionName: string;
  subject: Subject;
  operation: Operation;
  target: Target;
  outcome: Outcome;
  requestTime: string;
  traceability: TraceabilityResult;
}

/** internal/retrieval `listResponse` (GET /v1/alerts). */
export interface AlertInventoryResponse {
  alerts: AlertSummaryListItem[];
  total: number;
}

/**
 * The six real evidence-artifact identities (PD-04 scope decision 8;
 * FR-031). Traceability verification is explicitly not a seventh artifact
 * — see docs/frontend/product-experience-brief.md §3.3.
 */
export type EvidenceArtifactId =
  | "source-event"
  | "validation-outcome"
  | "normalized-event"
  | "detection-definition"
  | "detection-result"
  | "alert";

/**
 * The eight Signal Path visual stages (product-experience-brief.md §3.4).
 * Distinct from, and mapped onto, the six real workflow states
 * (admitted…evidenced) and the six evidence artifacts above — never
 * conflated with either.
 */
export type SignalPathStageId =
  | "intake"
  | "validation"
  | "normalization"
  | "detection-evaluation"
  | "alerting"
  | "evidence"
  | "traceability"
  | "investigation";

/**
 * Wire types for `GET /v1/data-sources` (internal/datasources/datasources.go).
 * A retrospective summary of telemetry already admitted through one
 * ingestion channel (FR-036) — never a health, staleness, or
 * missing-delivery judgment (docs/scope.md scope decision 6).
 */
export interface DataSourceItem {
  id: string;
  displayName: string;
  endpoint: string;
  eventCount: number;
  lastEventAt: string | null;
}

/** internal/datasources `listResponse` (GET /v1/data-sources). */
export interface DataSourcesResponse {
  dataSources: DataSourceItem[];
  total: number;
}

/**
 * Wire types for `GET /v1/detections` (internal/retrieval/detections.go).
 * The currently active detection definitions only (FR-020, FR-021,
 * FR-022) — never a historical, superseded, or inactive revision
 * (ADR-0004: no code path exists to edit or manage a definition through
 * the product itself).
 */
export interface DetectionItem extends DetectionDefinition {
  revision: string;
}

/** internal/retrieval `detectionsListResponse` (GET /v1/detections). */
export interface DetectionsResponse {
  detections: DetectionItem[];
  total: number;
}
