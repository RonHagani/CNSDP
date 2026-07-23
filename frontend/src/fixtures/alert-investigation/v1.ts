/**
 * Alert Investigation fixture set — version 1.
 *
 * FIXTURE DATA. Not a live backend response. Governed by
 * docs/frontend/product-experience-brief.md §10.1 (fixture policy):
 *
 *   - derived from real, committed repository contracts (cited per field
 *     group below), never hand-invented;
 *   - versioned by this file's name (`v1.ts`) and the notes below;
 *   - replaceable by a live `GET /v1/alerts/{id}` call with no UI redesign
 *     required — see src/features/alert-investigation/lib/alertSource.ts,
 *     the single seam a real fetch will be substituted behind.
 *
 * Provenance for the primary (id: 1) fixture:
 *
 *   - Raw source event: byte-identical to the single item in
 *     internal/intake/testdata/scenario-1-eventlist.json — a real,
 *     Spike-1-captured `pods/exec` request with TTY allocation, the same
 *     fixture the Go backend's own walking-skeleton and retrieval
 *     integration tests use (internal/retrieval/retrieval_integration_test.go
 *     `validEventJSON`).
 *   - Normalized event: hand-derived by applying the documented
 *     normalization rules (internal/normalization/normalization.go
 *     `Normalize`) to that raw event — subject.username from user.username,
 *     operation from verb/requestURI, target from objectRef, outcome from
 *     responseStatus.code, exec characteristics parsed from the
 *     requestURI's `stdin`/`tty` query parameters.
 *   - Detection definition: byte-identical in content to
 *     definitions/scenario-1.yaml.
 *   - Detection-definition revision: a sha256 hex digest computed from this
 *     fixture's own best-effort reconstruction of the canonical definition
 *     JSON (ADR-0004's documented revision scheme — internal/detection
 *     `Definition.Canonical` / `RevisionID`). It is illustrative of the real
 *     revision-ID format (a 64-character lowercase hex digest) and was
 *     computed by, not copied from, a live backend instance — it is not
 *     guaranteed to equal what a running deployment computes for this exact
 *     definition file and must not be treated as authoritative.
 *   - Match reason and alert summary: derived directly from the above,
 *     matching internal/detection/evaluate.go's `MatchReason` and
 *     internal/alerting/alerting.go's `Summary` construction rules.
 *   - alertId: 1, matching docs/reference-environment.md's documented
 *     Scenario 1 demonstration ("on a fresh database, the first alert is
 *     1").
 *
 * The `partial availability` (id: 2) and `broken traceability` (id: 3)
 * variants reuse the same underlying event and definition content, varying
 * only the fields the real backend contract allows to vary independently:
 *   - id 3's traceability failure mode (`failedLink: "raw_event_sha256"`,
 *     all six artifacts still available) mirrors, field for field, the real
 *     backend behavior proven by
 *     `TestServeHTTP_TamperedChain_Returns200WithGapVisible` in
 *     internal/retrieval/retrieval_integration_test.go.
 *   - id 2's partial-availability state (one artifact unavailable while the
 *     rest remain inspectable) exercises the best-effort composition
 *     contract documented in internal/evidence/evidence.go's `Compose`,
 *     which independently marks each of the six artifacts unavailable
 *     rather than aborting the whole response (FR-035).
 */

import type { AlertInvestigationResponse, RawAuditEvent } from "@/types/contract";

const scenario1RawEvent: RawAuditEvent = {
  kind: "Event",
  apiVersion: "audit.k8s.io/v1",
  level: "RequestResponse",
  auditID: "34b75a57-e1c0-4659-a21f-2d39256f018c",
  stage: "ResponseComplete",
  requestURI:
    "/api/v1/namespaces/default/pods/high-risk-pod/exec?command=%2Fbin%2Fsh&command=-c&command=echo+interactive-exec-marker&container=high-risk-container&stdin=true&stdout=true&tty=true",
  verb: "get",
  user: {
    username: "kubernetes-admin",
    groups: ["kubeadm:cluster-admins", "system:authenticated"],
    extra: {
      "authentication.kubernetes.io/credential-id": [
        "X509SHA256=9a4a60dc1b9f9da18482a661944357f22913875d97fd8808e3c19936537749b2",
      ],
    },
  },
  sourceIPs: ["172.19.0.1"],
  userAgent: "kubectl.exe/v1.34.1 (windows/amd64) kubernetes/93248f9",
  objectRef: {
    resource: "pods",
    namespace: "default",
    name: "high-risk-pod",
    apiVersion: "v1",
    subresource: "exec",
  },
  responseStatus: { code: 101 },
  requestReceivedTimestamp: "2026-07-21T15:25:11.901891Z",
  stageTimestamp: "2026-07-21T15:25:11.959819Z",
  annotations: {
    "authorization.k8s.io/decision": "allow",
    "authorization.k8s.io/reason":
      'RBAC: allowed by ClusterRoleBinding "kubeadm:cluster-admins" of ClusterRole "cluster-admin" to Group "kubeadm:cluster-admins"',
  },
};

/** Illustrative revision hash — see file-level provenance note above. */
const scenario1Revision = "a393a8978cbdd646f2d3bdb72dfd0021726c3c45be7563fa2cf1d060c6aa84a2";

const scenario1Definition: AlertInvestigationResponse["detectionDefinition"] = {
  available: true,
  revision: scenario1Revision,
  definition: {
    scenario: "scenario-1",
    name: "Interactive container exec request",
    description:
      "Detection of a Kubernetes API request to the pods/exec subresource that exhibits documented interactive-execution characteristics.",
    conditions: {
      operation: { resource: "pods", subresource: "exec" },
      requires_any: [
        {
          id: "stdin_streaming",
          description: "The exec request enables standard-input streaming.",
        },
        {
          id: "tty_allocation",
          description: "The exec request requests interactive terminal (TTY) allocation.",
        },
      ],
    },
  },
};

const scenario1MatchReason: AlertInvestigationResponse["detectionResult"] = {
  available: true,
  matchReason: {
    scenario: "scenario-1",
    definitionName: "Interactive container exec request",
    definitionRevision: scenario1Revision,
    satisfiedCharacteristics: [
      { id: "stdin_streaming", description: "The exec request enables standard-input streaming." },
      {
        id: "tty_allocation",
        description: "The exec request requests interactive terminal (TTY) allocation.",
      },
    ],
  },
};

const scenario1NormalizedEvent: AlertInvestigationResponse["normalizedEvent"] = {
  available: true,
  event: {
    subject: { username: "kubernetes-admin" },
    operation: { verb: "get", requestURI: scenario1RawEvent.requestURI },
    target: {
      resource: "pods",
      name: "high-risk-pod",
      namespace: "default",
      subresource: "exec",
    },
    outcome: { code: 101 },
    requestTime: "2026-07-21T15:25:11.901891Z",
    exec: { stdin: true, tty: true },
  },
};

const scenario1AlertSummary: AlertInvestigationResponse["alert"] = {
  available: true,
  summary: {
    matchReason: scenario1MatchReason.matchReason!,
    subject: { username: "kubernetes-admin" },
    operation: { verb: "get", requestURI: scenario1RawEvent.requestURI },
    target: {
      resource: "pods",
      name: "high-risk-pod",
      namespace: "default",
      subresource: "exec",
    },
    outcome: { code: 101 },
    requestTime: "2026-07-21T15:25:11.901891Z",
  },
};

/** id: 1 — happy path. All six artifacts available; traceability intact. */
export const fixtureIntact: AlertInvestigationResponse = {
  alertId: 1,
  sourceEvent: { available: true, rawEvent: scenario1RawEvent },
  validationOutcome: { available: true, outcome: "valid" },
  normalizedEvent: scenario1NormalizedEvent,
  detectionDefinition: scenario1Definition,
  detectionResult: scenario1MatchReason,
  alert: scenario1AlertSummary,
  traceability: { intact: true },
};

/**
 * id: 2 — partial artifact availability. The detection-definition artifact
 * is unavailable while the other five remain inspectable, exercising
 * evidence.Compose's independent per-artifact availability (FR-035).
 */
export const fixturePartial: AlertInvestigationResponse = {
  ...fixtureIntact,
  alertId: 2,
  detectionDefinition: { available: false },
  traceability: { intact: true },
};

/**
 * id: 3 — broken traceability. Mirrors
 * TestServeHTTP_TamperedChain_Returns200WithGapVisible: all six artifacts
 * remain available, but the raw-event digest no longer matches its stored
 * integrity record.
 */
export const fixtureBrokenTraceability: AlertInvestigationResponse = {
  ...fixtureIntact,
  alertId: 3,
  traceability: { intact: false, failedLink: "raw_event_sha256" },
};

export const fixturesById: Record<string, AlertInvestigationResponse> = {
  "1": fixtureIntact,
  "2": fixturePartial,
  "3": fixtureBrokenTraceability,
};
