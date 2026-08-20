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

/**
 * `code` is Kubernetes' own outcome signal (an HTTP response code);
 * `errorCode`/`successful` are AWS CloudTrail's (an absent `errorCode`
 * means success) -- `code` is always absent for a CloudTrail event and
 * `errorCode` is always absent for a Kubernetes one. The real backend
 * always serializes `successful` (no `omitempty` on
 * internal/normalization.Outcome.Successful), including for a Kubernetes
 * event whose scenario never consults it (e.g. scenario 1, where it is
 * always `false` without meaning "failed"); it is typed optional here
 * only so fixtures/tests written before this field existed keep
 * type-checking unchanged. See `formatOutcome` (src/lib/outcome.ts), the
 * one place this ambiguity is resolved for display.
 */
export interface Outcome {
  code?: number;
  errorCode?: string;
  successful?: boolean;
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

/** internal/normalization.CloudTrailIAMAction -- AWS CloudTrail scenarios
 *  4-6's (ADR-0006) affected-principal content: the affected user and MFA
 *  device (scenario 4), the affected user (scenario 5), or the affected
 *  principal and referenced policy ARN (scenario 6, held verbatim in
 *  `policyName` despite the field's name -- see the backend's own doc
 *  comment). */
export interface CloudTrailIAMAction {
  affectedUser?: string;
  affectedDevice?: string;
  policyName?: string;
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
  cloudTrailIAMAction?: CloudTrailIAMAction;
}

/**
 * The AWS CloudTrail management-event record shape (ADR-0006, FR-002):
 * the fields internal/validation and internal/normalization's own
 * `cloudTrailRecord` decode targets read, restated here for the frontend.
 * The real stored raw event may carry additional standard CloudTrail
 * fields this platform does not currently read (FR-032 preserves the
 * submission byte-for-byte) -- this type models only what the product
 * itself relies on, the same "not an exhaustive per-API field mapping"
 * scope internal/normalization/normalization.go's own doc comments
 * describe for `cloudTrailTargetOf`/`cloudTrailIAMActionOf`.
 */
export interface CloudTrailRawRecord {
  eventID: string;
  eventTime: string;
  eventSource: string;
  eventName: string;
  eventCategory: string;
  userIdentity: {
    type?: string;
    arn?: string;
    userName?: string;
  };
  requestParameters?: Record<string, unknown> | null;
  responseElements?: Record<string, unknown> | null;
  errorCode?: string;
  /** A real posted record carries many standard CloudTrail fields this
   *  product does not read (eventVersion, awsRegion, eventType, ...) --
   *  FR-032 preserves them byte-for-byte, so this index signature accepts
   *  them rather than narrowing the type to only the fields above. */
  [key: string]: unknown;
}

/** Distinguishes a CloudTrail-shaped raw event from a Kubernetes one by
 *  the one field only CloudTrail records ever carry -- `RawAuditEvent` has
 *  no `eventID` field at all. The wire response carries no explicit
 *  source-family discriminator (internal/retrieval/retrieval.go), so
 *  content shape is the only signal available to the frontend, mirroring
 *  how the backend itself attributes source family by which intake
 *  endpoint received the submission rather than by sniffing content. */
export function isCloudTrailRawEvent(
  raw: RawAuditEvent | CloudTrailRawRecord,
): raw is CloudTrailRawRecord {
  return "eventID" in raw;
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
    rawEvent?: RawAuditEvent | CloudTrailRawRecord;
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

/**
 * Wire types for `GET /v1/submissions` (internal/retrieval/submissions.go).
 * A keyset-paginated, optionally outcome-filtered review of every
 * submission received at the defined intake (FR-011, FR-012, FR-013) --
 * including submissions that never produce an alert (invalid, incomplete,
 * unsupported, and valid-but-non-matching), none of which is reachable
 * through `GET /v1/alerts`.
 */

/** internal/submission.Status -- the six workflow stages a submission
 *  passes through (ARCH-01 §3). */
export type SubmissionStatus = "admitted" | "validated" | "normalized" | "evaluated" | "alerted" | "evidenced";

/** internal/validation.Outcome -- the four mutually exclusive validation
 *  outcomes (FR-005). */
export type ValidationOutcomeName = "valid" | "invalid" | "incomplete" | "unsupported";

/**
 * The `outcome` query filter GET /v1/submissions accepts: one of the four
 * real validation outcomes, or "pending" -- a submission still at
 * `status: "admitted"` with no validation outcome recorded yet. "pending"
 * is never a fifth ValidationOutcomeName: it is a filter/presentation
 * concept only, derived from submission status, never persisted as an
 * outcome (see internal/retrieval/submissions.go's package doc).
 */
export type SubmissionOutcomeFilter = "pending" | ValidationOutcomeName;

/** internal/retrieval `submissionItem` (one GET /v1/submissions row). The
 *  `available` flag carries the same "never fabricate" contract as every
 *  other artifact in this contract file: a submission not yet validated
 *  has `validationOutcome.available === false` and no `outcome`/`reason`
 *  field, never a fabricated or zero-valued outcome. */
export interface SubmissionListItem {
  submissionId: number;
  status: SubmissionStatus;
  auditId: string;
  auditStage: string;
  createdAt: string;
  validationOutcome: {
    available: boolean;
    outcome?: ValidationOutcomeName;
    reason?: string;
  };
}

/** internal/retrieval `submissionsListResponse` (GET /v1/submissions).
 *  `nextCursor` is the last-seen `submissionId` to pass back as the
 *  `cursor` query parameter for the next keyset page, or `null` when this
 *  is the last page. `total` reflects the full count matching the active
 *  `outcome` filter, independent of this page's own size. */
export interface SubmissionsResponse {
  submissions: SubmissionListItem[];
  nextCursor: number | null;
  total: number;
}
