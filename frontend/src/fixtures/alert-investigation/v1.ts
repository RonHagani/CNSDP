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
 * The `partial availability` (id: 2), `broken traceability` (id: 3), and
 * `broken traceability, source_key` (id: 6) variants reuse the same
 * underlying event and definition content, varying only the fields the
 * real backend contract allows to vary independently:
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
 *   - id 6's traceability failure mode (`failedLink: "source_key"`, all six
 *     artifacts still available) mirrors, at the unit level,
 *     `TestVerifyAlert_AuditIdentityChangedWithoutSourceKeyUpdate` in
 *     internal/traceability/traceability_integration_test.go — see its own
 *     definition below for why the third contract-permitted value,
 *     `"alert"`, deliberately has no corresponding fixture.
 *
 * Provenance for the scenario-2 (id: 4) and scenario-3 (id: 5) fixtures,
 * added to exercise the second and third approved detection scenarios
 * (Alert Investigation UX Specification v0.1 §5.2, §5.3):
 *
 *   - Raw source events: byte-identical to the single item in
 *     internal/validation/testdata/scenario2-valid.json and
 *     scenario3-valid.json respectively -- real, committed backend fixture
 *     data already used by the Go backend's own validation tests.
 *   - Normalized events: hand-derived by applying the documented
 *     normalization rules (internal/normalization/normalization.go
 *     `Normalize`, `podCreationCharacteristics`, `clusterRoleBindingCharacteristics`)
 *     to those raw events. Scenario 2's five podCreation booleans are read
 *     directly off the real requestObject's Pod spec (privileged from
 *     containers[].securityContext.privileged; hostNetwork/hostPID/hostIPC
 *     from the top-level spec booleans; hostPathVolume from the presence of
 *     a volumes[].hostPath entry) -- this fixture's raw event happens to
 *     satisfy all five. Scenario 3's clusterRoleBinding fields are read
 *     directly off the real requestObject's roleRef/subjects/metadata.name
 *     (objectRef.name takes precedence per the documented fallback rule).
 *   - Detection definitions: byte-identical in content to
 *     definitions/scenario-2.yaml and definitions/scenario-3.yaml.
 *   - Detection-definition revisions: sha256 hex digests computed the same
 *     way as id 1's illustrative revision (ADR-0004's documented scheme --
 *     internal/detection `Definition.Canonical` / `RevisionID`): a
 *     best-effort reconstruction of the canonical definition JSON, not
 *     verified against a live backend instance, and not authoritative.
 *   - A confirmed, deliberate ordering asymmetry, reproduced faithfully
 *     rather than smoothed over: `detectionDefinition.definition.conditions
 *     .requires_any` / `.requires_all` reflect `Canonical()`'s
 *     characteristic-ID sort (internal/detection/detection.go
 *     `sortedByID`), because `internal/evidence.Compose` reads the
 *     detection definition back via `detection.GetDefinition`, which
 *     unmarshals the *persisted* canonical content
 *     (internal/detection/detection.go `loadDefinitionsFromFS` /
 *     `insertDefinition`) -- while `detectionResult.matchReason
 *     .satisfiedCharacteristics` preserves the YAML file's original
 *     declared order, because match evaluation
 *     (internal/detection/evaluate.go `Advance`) reads the active
 *     definition via `ActiveDefinitions`, which re-parses the YAML fresh
 *     and never sorts it. Scenario 2's five-characteristic list is the
 *     first fixture where these two orders actually visibly differ.
 *   - Match reasons and alert summaries: derived directly from the above,
 *     matching internal/detection/evaluate.go's `evaluate` /
 *     `characteristicSet` / `satisfiedFrom` and
 *     internal/alerting/alerting.go's `buildSummary` construction rules.
 *   - alertId: 4 and 5, the next sequential ids after the three existing
 *     scenario-1 fixtures; the scenario is distinguished by content and by
 *     fixture naming, not by a reserved id range (id 2 is likewise a
 *     scenario-1 variant, not a distinct scenario).
 *   - Field-level raw-to-normalized provenance for these two fixtures is
 *     deliberately NOT modeled here or in lib/lineage.ts: podCreation and
 *     clusterRoleBinding characteristics derive from requestObject, which
 *     the frontend contract types as `unknown` (contract.ts), so no
 *     structurally verified raw path exists for them yet (UX spec §15 item
 *     6). lib/lineage.ts is intentionally left unchanged by these fixtures
 *     -- it correctly produces no link for these fields, which is what
 *     leaves them in the UX specification's Partial provenance state
 *     (§3.5) rather than manufacturing an unverified one.
 *
 * Provenance for the AWS CloudTrail scenario-5 (id: 7) fixture, added to
 * exercise the platform's second telemetry source family (ADR-0006) end to
 * end in this UI:
 *
 *   - Raw source event: byte-identical in content to the real-shaped
 *     `cloudTrailScenario5JSON` fixture already used by the Go backend's own
 *     tests (internal/normalization/normalization_test.go,
 *     cmd/platform/walkingskeleton_integration_test.go
 *     `cloudTrailScenario5EventJSON`) -- a real CreateAccessKey management
 *     event, not synthetic data invented for this fixture.
 *   - Normalized event: hand-derived by applying the documented CloudTrail
 *     normalization rules (internal/normalization/normalization.go
 *     `normalizeCloudTrail`) to that raw event -- subject.username from the
 *     requesting identity's ARN, operation.verb from eventName,
 *     target.resource from eventSource, target.name and
 *     cloudTrailIAMAction.affectedUser from requestParameters.userName,
 *     outcome.successful from the absence of a recorded errorCode.
 *   - Detection definition: byte-identical in content to
 *     definitions/scenario-5.yaml.
 *   - Detection-definition revision: computed the same illustrative way as
 *     every Kubernetes fixture's revision above (ADR-0004's documented
 *     scheme) -- not verified against a live backend instance, and not
 *     authoritative.
 *   - Match reason and alert summary: derived directly from the above,
 *     matching internal/detection/evaluate.go's `evaluate` /
 *     `characteristicSet` and internal/alerting/alerting.go's
 *     `buildSummary` construction rules.
 *   - alertId: 7, the next sequential id after the six existing Kubernetes
 *     fixtures.
 *   - Field-level raw-to-normalized provenance is deliberately NOT modeled
 *     here or in lib/lineage.ts, for the same reason as the scenario-2/3
 *     fixtures above: lib/lineage.ts only ever describes Kubernetes' own
 *     raw-to-normalized rules (lib/provenance.ts's own `rawEvent` guard
 *     never passes it a CloudTrail-shaped raw event), so
 *     `cloudTrailIAMAction`'s fields correctly resolve to the Partial
 *     provenance state rather than an invented raw path.
 */

import type { AlertInvestigationResponse, CloudTrailRawRecord, RawAuditEvent } from "@/types/contract";

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

// --- Scenario 2: successful high-risk Pod creation --------------------

const scenario2RawEvent: RawAuditEvent = {
  kind: "Event",
  apiVersion: "audit.k8s.io/v1",
  level: "RequestResponse",
  auditID: "96fe2eaf-085e-48e6-943a-b33e1e0d6c4f",
  stage: "ResponseComplete",
  requestURI:
    "/api/v1/namespaces/default/pods?fieldManager=kubectl-client-side-apply&fieldValidation=Strict",
  verb: "create",
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
  },
  responseStatus: { code: 201 },
  requestObject: {
    kind: "Pod",
    apiVersion: "v1",
    metadata: { name: "high-risk-pod", namespace: "default", labels: { spike: "audit-intake" } },
    spec: {
      volumes: [{ name: "host-root", hostPath: { path: "/", type: "Directory" } }],
      containers: [
        {
          name: "high-risk-container",
          image: "busybox:1.36",
          command: ["sleep", "3600"],
          securityContext: { privileged: true },
        },
      ],
      hostNetwork: true,
      hostPID: true,
      hostIPC: true,
    },
    status: {},
  },
  requestReceivedTimestamp: "2026-07-21T15:23:33.525838Z",
  stageTimestamp: "2026-07-21T15:23:33.531528Z",
};

/** Illustrative revision hash — see file-level provenance note above. */
const scenario2Revision = "a25131f3e9210ea15e3b24a4a990a180fc75440efb2f3850632f356edc16c47d";

/**
 * requires_any below is in characteristic-ID sorted order
 * (host_ipc, host_network, host_path_volume, host_pid, privileged_container)
 * — the persisted-content order — deliberately not the YAML file's
 * declared order. See the file-level provenance note above.
 */
const scenario2Definition: AlertInvestigationResponse["detectionDefinition"] = {
  available: true,
  revision: scenario2Revision,
  definition: {
    scenario: "scenario-2",
    name: "High-risk Pod creation",
    description:
      "Detection of the creation of a Pod whose specification includes at least one documented high-risk privilege or host-access characteristic.\n",
    conditions: {
      operation: { resource: "pods", verb: "create" },
      requires_outcome: "success",
      requires_any: [
        {
          id: "host_ipc",
          description: "The Pod uses the host inter-process-communication (IPC) namespace.",
        },
        { id: "host_network", description: "The Pod uses the host network." },
        {
          id: "host_path_volume",
          description: "The Pod mounts a volume backed by a host filesystem path.",
        },
        {
          id: "host_pid",
          description: "The Pod uses the host process-identifier (PID) namespace.",
        },
        {
          id: "privileged_container",
          description: "A container in the Pod requests privileged mode.",
        },
      ],
    },
  },
};

/**
 * satisfiedCharacteristics below is in the definition's YAML-declared
 * order (privileged_container, host_network, host_pid, host_ipc,
 * host_path_volume) — the order match evaluation actually produces — not
 * the sorted order above. See the file-level provenance note above. All
 * five are present because this fixture's raw event (real backend
 * testdata) genuinely sets every one of the five documented conditions.
 */
const scenario2MatchReason: AlertInvestigationResponse["detectionResult"] = {
  available: true,
  matchReason: {
    scenario: "scenario-2",
    definitionName: "High-risk Pod creation",
    definitionRevision: scenario2Revision,
    satisfiedCharacteristics: [
      { id: "privileged_container", description: "A container in the Pod requests privileged mode." },
      { id: "host_network", description: "The Pod uses the host network." },
      {
        id: "host_pid",
        description: "The Pod uses the host process-identifier (PID) namespace.",
      },
      {
        id: "host_ipc",
        description: "The Pod uses the host inter-process-communication (IPC) namespace.",
      },
      {
        id: "host_path_volume",
        description: "The Pod mounts a volume backed by a host filesystem path.",
      },
    ],
  },
};

const scenario2NormalizedEvent: AlertInvestigationResponse["normalizedEvent"] = {
  available: true,
  event: {
    subject: { username: "kubernetes-admin" },
    operation: { verb: "create", requestURI: scenario2RawEvent.requestURI },
    target: {
      resource: "pods",
      name: "high-risk-pod",
      namespace: "default",
    },
    outcome: { code: 201 },
    requestTime: "2026-07-21T15:23:33.525838Z",
    podCreation: {
      privileged: true,
      hostNetwork: true,
      hostPID: true,
      hostIPC: true,
      hostPathVolume: true,
    },
  },
};

const scenario2AlertSummary: AlertInvestigationResponse["alert"] = {
  available: true,
  summary: {
    matchReason: scenario2MatchReason.matchReason!,
    subject: { username: "kubernetes-admin" },
    operation: { verb: "create", requestURI: scenario2RawEvent.requestURI },
    target: {
      resource: "pods",
      name: "high-risk-pod",
      namespace: "default",
    },
    outcome: { code: 201 },
    requestTime: "2026-07-21T15:23:33.525838Z",
  },
};

/** id: 4 — Scenario 2, successful high-risk Pod creation. All six artifacts available; traceability intact. */
export const fixtureScenario2Intact: AlertInvestigationResponse = {
  alertId: 4,
  sourceEvent: { available: true, rawEvent: scenario2RawEvent },
  validationOutcome: { available: true, outcome: "valid" },
  normalizedEvent: scenario2NormalizedEvent,
  detectionDefinition: scenario2Definition,
  detectionResult: scenario2MatchReason,
  alert: scenario2AlertSummary,
  traceability: { intact: true },
};

// --- Scenario 3: successful cluster-admin ClusterRoleBinding creation --

const scenario3RawEvent: RawAuditEvent = {
  kind: "Event",
  apiVersion: "audit.k8s.io/v1",
  level: "RequestResponse",
  auditID: "5eceafdc-699c-4ae8-83b9-c3feb1e28cd5",
  stage: "ResponseComplete",
  requestURI:
    "/apis/rbac.authorization.k8s.io/v1/clusterrolebindings?fieldManager=kubectl-client-side-apply&fieldValidation=Strict",
  verb: "create",
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
    resource: "clusterrolebindings",
    name: "spike-crb-jsonpatch",
    apiGroup: "rbac.authorization.k8s.io",
    apiVersion: "v1",
  },
  responseStatus: { code: 201 },
  requestObject: {
    kind: "ClusterRoleBinding",
    apiVersion: "rbac.authorization.k8s.io/v1",
    metadata: { name: "spike-crb-jsonpatch" },
    subjects: [{ kind: "User", apiGroup: "rbac.authorization.k8s.io", name: "alice@example.com" }],
    roleRef: { apiGroup: "rbac.authorization.k8s.io", kind: "ClusterRole", name: "cluster-admin" },
  },
  requestReceivedTimestamp: "2026-07-21T15:26:24.599870Z",
  stageTimestamp: "2026-07-21T15:26:24.601990Z",
};

/** Illustrative revision hash — see file-level provenance note above. */
const scenario3Revision = "9961d2356b15d5a2936520071f59d8ae66e0da591f528dfa355f7c71c39e71e5";

const scenario3Definition: AlertInvestigationResponse["detectionDefinition"] = {
  available: true,
  revision: scenario3Revision,
  definition: {
    scenario: "scenario-3",
    name: "Cluster-admin ClusterRoleBinding grant",
    description:
      "Detection of the creation of a ClusterRoleBinding that references the cluster-admin ClusterRole. Modification of an existing binding, including a subject addition, is not evaluated by this scenario in v0.1.\n",
    conditions: {
      operation: { resource: "clusterrolebindings", verb: "create" },
      requires_outcome: "success",
      requires_all: [
        {
          id: "role_ref_cluster_admin",
          description: "The ClusterRoleBinding's role reference is the cluster-admin ClusterRole.",
        },
      ],
    },
  },
};

const scenario3MatchReason: AlertInvestigationResponse["detectionResult"] = {
  available: true,
  matchReason: {
    scenario: "scenario-3",
    definitionName: "Cluster-admin ClusterRoleBinding grant",
    definitionRevision: scenario3Revision,
    satisfiedCharacteristics: [
      {
        id: "role_ref_cluster_admin",
        description: "The ClusterRoleBinding's role reference is the cluster-admin ClusterRole.",
      },
    ],
  },
};

const scenario3NormalizedEvent: AlertInvestigationResponse["normalizedEvent"] = {
  available: true,
  event: {
    subject: { username: "kubernetes-admin" },
    operation: { verb: "create", requestURI: scenario3RawEvent.requestURI },
    target: {
      resource: "clusterrolebindings",
      name: "spike-crb-jsonpatch",
    },
    outcome: { code: 201 },
    requestTime: "2026-07-21T15:26:24.599870Z",
    clusterRoleBinding: {
      bindingName: "spike-crb-jsonpatch",
      roleRef: { apiGroup: "rbac.authorization.k8s.io", kind: "ClusterRole", name: "cluster-admin" },
      subjects: [{ kind: "User", apiGroup: "rbac.authorization.k8s.io", name: "alice@example.com" }],
    },
  },
};

const scenario3AlertSummary: AlertInvestigationResponse["alert"] = {
  available: true,
  summary: {
    matchReason: scenario3MatchReason.matchReason!,
    subject: { username: "kubernetes-admin" },
    operation: { verb: "create", requestURI: scenario3RawEvent.requestURI },
    target: {
      resource: "clusterrolebindings",
      name: "spike-crb-jsonpatch",
    },
    outcome: { code: 201 },
    requestTime: "2026-07-21T15:26:24.599870Z",
  },
};

/** id: 5 — Scenario 3, successful cluster-admin ClusterRoleBinding creation. All six artifacts available; traceability intact. */
export const fixtureScenario3Intact: AlertInvestigationResponse = {
  alertId: 5,
  sourceEvent: { available: true, rawEvent: scenario3RawEvent },
  validationOutcome: { available: true, outcome: "valid" },
  normalizedEvent: scenario3NormalizedEvent,
  detectionDefinition: scenario3Definition,
  detectionResult: scenario3MatchReason,
  alert: scenario3AlertSummary,
  traceability: { intact: true },
};

/**
 * id: 6 — broken traceability, source_key. Mirrors, at the unit level,
 * `TestVerifyAlert_AuditIdentityChangedWithoutSourceKeyUpdate`
 * (internal/traceability/traceability_integration_test.go): the
 * submission's audit identity (audit_id/audit_stage) was changed directly
 * in storage without updating source_key to match, so `VerifyAlert`'s
 * re-derived source_key no longer matches the stored one. This check runs
 * strictly after the chain's join already resolved, and independently of
 * every other read `evidence.Compose` performs (submission, validation,
 * normalized event, detection result/definition, alert) — so, exactly as
 * `TestServeHTTP_TamperedChain_Returns200WithGapVisible` proves for
 * `raw_event_sha256`, all six artifacts remain available; only the
 * traceability chain reports the mismatch.
 *
 * A fixture for the third contract-permitted `failedLink` value, "alert",
 * is deliberately NOT added. Direct inspection of
 * internal/traceability/traceability.go's `VerifyAlert` shows
 * `FailedLink: "alert"` is returned only when the alert's own join query
 * finds no rows (`sql.ErrNoRows`) — and `internal/evidence.Compose` calls
 * the identical join (`traceability.Locate`) *before* reading any artifact,
 * returning `evidence.ErrNotFound` (mapped to a 404 by
 * `internal/retrieval`, `TestServeHTTP_NonexistentAlert_Returns404`) in
 * that exact same case. A response with `failedLink: "alert"` and all six
 * artifacts marked available is therefore not a state the real backend can
 * ever produce — the frontend's existing not-found route state already
 * correctly represents what `failedLink: "alert"` means in practice, so a
 * separate fixture for it would misrepresent the contract.
 */
export const fixtureBrokenTraceabilitySourceKey: AlertInvestigationResponse = {
  ...fixtureIntact,
  alertId: 6,
  traceability: { intact: false, failedLink: "source_key" },
};

// --- Scenario 5: successful AWS CloudTrail CreateAccessKey (ADR-0006) --

const cloudTrailScenario5RawEvent: CloudTrailRawRecord = {
  eventVersion: "1.08",
  eventTime: "2026-08-18T12:34:56Z",
  eventSource: "iam.amazonaws.com",
  eventName: "CreateAccessKey",
  eventCategory: "Management",
  awsRegion: "us-east-1",
  userIdentity: {
    type: "IAMUser",
    arn: "arn:aws:iam::123456789012:user/alice",
    userName: "alice",
  },
  requestParameters: { userName: "bob" },
  responseElements: { accessKey: { userName: "bob", accessKeyId: "AKIAEXAMPLE", status: "Active" } },
  eventID: "5f6a2b1c-0000-0000-0000-000000000001",
  eventType: "AwsApiCall",
};

/** Illustrative revision hash — see file-level provenance note above. */
const cloudTrailScenario5Revision = "ca5128f067ef7a698ae5585670db30e3ba218832e8d6cd11077c7f1a246adebb";

const cloudTrailScenario5Definition: AlertInvestigationResponse["detectionDefinition"] = {
  available: true,
  revision: cloudTrailScenario5Revision,
  definition: {
    scenario: "scenario-5",
    name: "New IAM access key created",
    description:
      "Detection of a successful CreateAccessKey AWS IAM API call, indicating a new access key was issued for an IAM user.\n",
    conditions: {
      operation: { resource: "iam.amazonaws.com", verb: "CreateAccessKey" },
      requires_outcome: "success",
      requires_all: [
        {
          id: "access_key_created",
          description: "The API call created a new IAM access key (CreateAccessKey).",
        },
      ],
    },
  },
};

const cloudTrailScenario5MatchReason: AlertInvestigationResponse["detectionResult"] = {
  available: true,
  matchReason: {
    scenario: "scenario-5",
    definitionName: "New IAM access key created",
    definitionRevision: cloudTrailScenario5Revision,
    satisfiedCharacteristics: [
      {
        id: "access_key_created",
        description: "The API call created a new IAM access key (CreateAccessKey).",
      },
    ],
  },
};

const cloudTrailScenario5NormalizedEvent: AlertInvestigationResponse["normalizedEvent"] = {
  available: true,
  event: {
    subject: { username: "arn:aws:iam::123456789012:user/alice" },
    operation: { verb: "CreateAccessKey", requestURI: "" },
    target: { resource: "iam.amazonaws.com", name: "bob" },
    outcome: { successful: true },
    requestTime: "2026-08-18T12:34:56Z",
    cloudTrailIAMAction: { affectedUser: "bob" },
  },
};

const cloudTrailScenario5AlertSummary: AlertInvestigationResponse["alert"] = {
  available: true,
  summary: {
    matchReason: cloudTrailScenario5MatchReason.matchReason!,
    subject: { username: "arn:aws:iam::123456789012:user/alice" },
    operation: { verb: "CreateAccessKey", requestURI: "" },
    target: { resource: "iam.amazonaws.com", name: "bob" },
    outcome: { successful: true },
    requestTime: "2026-08-18T12:34:56Z",
  },
};

/**
 * id: 7 — Scenario 5, successful AWS CloudTrail CreateAccessKey. All six
 * artifacts available; traceability intact. The first AWS CloudTrail-family
 * fixture (ADR-0006): its raw event, normalized event, and detection
 * definition are all a different shape from every Kubernetes fixture above
 * -- `sourceEvent.rawEvent` is a `CloudTrailRawRecord`, not a
 * `RawAuditEvent`, and `normalizedEvent.event` carries `cloudTrailIAMAction`
 * rather than `exec`/`podCreation`/`clusterRoleBinding` -- proving the
 * investigation UI renders a second source family without redesign.
 */
export const fixtureCloudTrailScenario5Intact: AlertInvestigationResponse = {
  alertId: 7,
  sourceEvent: { available: true, rawEvent: cloudTrailScenario5RawEvent },
  validationOutcome: { available: true, outcome: "valid" },
  normalizedEvent: cloudTrailScenario5NormalizedEvent,
  detectionDefinition: cloudTrailScenario5Definition,
  detectionResult: cloudTrailScenario5MatchReason,
  alert: cloudTrailScenario5AlertSummary,
  traceability: { intact: true },
};

export const fixturesById: Record<string, AlertInvestigationResponse> = {
  "1": fixtureIntact,
  "2": fixturePartial,
  "3": fixtureBrokenTraceability,
  "4": fixtureScenario2Intact,
  "5": fixtureScenario3Intact,
  "6": fixtureBrokenTraceabilitySourceKey,
  "7": fixtureCloudTrailScenario5Intact,
};
