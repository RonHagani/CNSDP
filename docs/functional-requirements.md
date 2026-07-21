# Cloud-Native Security Telemetry and Detection Platform — v0.1 Functional Requirements

| Field | Value |
| --- | --- |
| Document ID | PD-05 |
| Version | 0.1 |
| Status | Approved — Phase 0 baseline |
| Phase | Phase 0 — Product Definition and Requirements |

## Purpose and scope

This document defines the v0.1 functional requirements of the platform
described in the Product Charter (PD-01, `docs/product.md`), for the personas
defined in the Personas document (PD-02, `docs/personas.md`), the use cases
defined in the Use Cases document (PD-03, `docs/use-cases.md`), and the scope
approved in the v0.1 Scope and Non-Goals document (PD-04, `docs/scope.md`).

The requirements translate the approved product decisions into precise,
testable statements of observable product behavior and required product
information. Together they define the minimum behavior necessary for the
narrow but complete v0.1 workflow (PC-005) and fully support UC-001, UC-002,
and UC-003.

This document defines functional requirements only. It does not define:

- non-functional requirements — performance, latency, throughput,
  availability, retention, reliability, platform self-security, operability,
  or maintainability (future NFR-### artifact);
- acceptance criteria — concrete pass/fail demonstrations per requirement
  (future AC-### artifact);
- architecture or design — the telemetry delivery mechanism or protocol, the
  storage and maintenance mechanism for detection definitions, the format of
  the normalized representation, the representation of traceability links,
  and the presentation surfaces for validation outcomes, detection
  definitions, alerts, and evidence (PC-C-004; PD-04 "Undecided — delegated
  to requirements or architecture").

No user interface, product API, CLI, internal data format, database,
queue, service, framework, deployment mechanism, or presentation
mechanism is assumed by any requirement. The only concrete external data
schema selected by this document is the approved Kubernetes
audit.k8s.io/v1 source-event contract defined by FR-002, which defines
the product input contract without selecting any implementation or
delivery mechanism.

## Conventions

- Identifiers use the stable FR-### namespace defined in PC-015 and are
  never renumbered or reused.
- Each requirement describes one atomic, independently testable product
  behavior or one required set of product information, using normative
  "The platform shall…" wording.
- Per PC-015, every requirement traces to at least one use case (UC-###).
  Traceability to personas (PER-###), product goals (PC-G-###), and the
  PD-04 scope basis is recorded where relevant.
- Behavior shared by multiple use cases or personas is stated once with
  shared traceability rather than restated per persona.
- Alternative and failure behavior required by UC-001, UC-002, and UC-003 is
  part of the requirement set (PC-P-004).
- Requirements are grouped into sections A–G following the PC-005 workflow
  order. Grouping is editorial; only the FR-### identifiers are normative.

## Definitions

- **Submission** — the unit of telemetry received at the defined intake. In
  v0.1 each submission carries a single Kubernetes API server audit event.
  How any batched delivery maps onto per-event submissions is delegated to
  architecture together with the delivery mechanism (PD-04 delegated
  decision 1).
- **Source event** — the Kubernetes API server audit event carried by a
  submission, in the content and form received at the defined intake.
- **Supported source-event form** — the documented form of a Kubernetes API
  server audit event that the platform supports (FR-002): a Kubernetes
  audit.k8s.io/v1 Event, including its documented structural constraints
  and the required event information defined in FR-002 and FR-007. The
  supported form is the product input contract only; it does not select
  the mechanism that produces, collects, or delivers events.
- **Validation outcome** — exactly one of: valid, invalid, incomplete, or
  unsupported (PC-G-002), defined mutually exclusively:
  - **Unsupported** — the submission is not attributable to the supported
    Kubernetes API server audit-event source family, or belongs to a
    documented source-event variant, version, or audit-event form that v0.1
    explicitly does not support.
  - **Invalid** — the submission is attributable to the supported source
    family and a supported event form, but cannot be parsed or violates the
    documented structural constraints of that form.
  - **Incomplete** — the submission is parseable and structurally valid,
    but lacks information required for the normalization or detection
    behavior applicable to that event.
  - **Valid** — the submission is supported, parseable, structurally
    valid, and contains all information required for the product behavior
    applicable to that event.
  A submission is assessed in the order unsupported, invalid, incomplete,
  valid, and receives exactly one outcome.
- **Scenario-relevant operation** — an operation covered by one of the
  three approved detection scenarios: a request to the pods/exec
  subresource of a pod (scenario 1); a Pod-creation request (scenario 2);
  or the creation of a ClusterRoleBinding (scenario 3).
- **Normalized event** — the representation of a valid source event after
  transformation into the documented normalized representation (PC-G-003).
- **Detection definition** — the identifiable definition of one detection
  scenario, including its documented detection conditions.
- **Documented detection conditions** — the documented statement of the
  event characteristics required for a detection definition to match.
- **Matching detection result** — the outcome of evaluating a normalized
  event against a detection definition where the documented detection
  conditions are satisfied.
- **Match reason** — the recorded explanation of why a matching detection
  result matched (FR-028).
- **Alert** — the structured product artifact produced for a matching
  detection result (PC-G-005).
- **Minimum evidence set** — the six investigation artifacts approved in
  PD-04 scope decision 8 (FR-031).
- **Evidence inventory** — the per-alert account of the six artifacts of
  the minimum evidence set, identifying each artifact and whether it is
  available (FR-031, FR-035). The inventory is a visibility mechanism over
  the approved evidence boundary, not additional contextual evidence.
- **Traceability link** — a recorded association between two artifacts of
  the minimum evidence set (PC-G-007).

## Section A — Telemetry intake

### FR-001 — Defined intake for Kubernetes API server audit events

The platform shall receive telemetry submissions of the Kubernetes API
server audit-event source family through a defined intake.

**Traceability:** UC-001; PER-003; PC-G-001; PD-04 in-scope 1, scope
decision 1.

**Rationale:** The entry point of the primary product outcome (PC-005).
UC-001 is triggered by telemetry reaching the defined intake. The delivery
mechanism is deliberately unstated (PD-04 delegated decision 1).

### FR-002 — Documented supported source-event form

The platform shall document the supported source-event form as a
Kubernetes audit.k8s.io/v1 Event that: (a) preserves the source request
identity, including the auditID and the audit stage; (b) contains the
requesting subject, the request time, the verb, the request URI, the
target resource reference, and the recorded outcome information;
(c) contains the request details required by an applicable detection
scenario — for Pod-creation and ClusterRoleBinding operations, the
recorded request content required by FR-025 or FR-026, and for pods/exec
operations, the recorded request URI or equivalent request
characteristics required by FR-024; and (d) provides outcome information
from an event stage sufficient to determine whether an operation
completed successfully where a documented detection condition requires
successful completion.

**Traceability:** UC-001; PER-003; PC-G-001, PC-G-002; PD-04 delegated
decision 9.

**Rationale:** UC-001's precondition requires the expected form of the
supported source to be documented. The invalid and unsupported
classifications are meaningless without a documented form to conform to.
This requirement defines the product input contract only: no audit
backend, audit-policy configuration, delivery protocol, collector,
storage mechanism, or serialization or presentation mechanism is
selected (PD-04 delegated decisions 1 and 9).

### FR-003 — Unambiguous submission identification

The platform shall record each submission received at the defined intake
and associate it with an identification that unambiguously distinguishes it
from every other received submission.

**Traceability:** UC-001, UC-003; PER-003, PER-001; PC-G-002, PC-G-007;
PD-04 scope decision 9.

**Rationale:** Per-submission validation-outcome visibility and all
downstream traceability require an unambiguous handle on each received unit.

## Section B — Validation and classification

### FR-004 — Validation of every submission

The platform shall validate every submission received at the defined intake.

**Traceability:** UC-001; PER-003; PC-G-002; PD-04 in-scope 2.

**Rationale:** Precludes silent acceptance (PC-P-004) and realizes the
validation step of the UC-001 main flow.

### FR-005 — Exclusive four-way classification

The platform shall classify each received submission into exactly one of
the outcomes: valid, invalid, incomplete, or unsupported.

**Traceability:** UC-001; PER-003; PC-G-002; PD-04 in-scope 2.

**Rationale:** PC-G-002 names the four classes; exclusivity makes every
outcome unambiguous and testable.

### FR-006 — Documented classification criteria

The platform shall document the criteria, including their order of
precedence, by which each submission is assigned exactly one of the four
validation outcomes.

**Traceability:** UC-001; PER-003; PC-G-002; PD-04 delegated decision 4.

**Rationale:** PER-003 must be able to understand how telemetry was
classified, not only what the outcome was. The mutually exclusive outcome
definitions (see Definitions) and their documented assessment order
(unsupported, invalid, incomplete, valid) make the exclusivity required by
FR-005 achievable and reviewable.

### FR-007 — Criteria for the valid outcome

The platform shall classify a submission as valid only if it is supported,
parseable, and structurally conformant to the documented supported
source-event form (FR-002), and contains: (a) the source event identity,
including the auditID and the audit stage; (b) the time of the request;
(c) the requesting subject; (d) the operation performed, including the
verb and the request URI; (e) the target resource of the operation;
(f) the recorded outcome information of the request, from an event stage
sufficient to determine whether the operation completed successfully
where an applicable documented detection condition requires successful
completion; and (g) where the recorded operation is a scenario-relevant
operation, the information required to evaluate the documented detection
conditions of the applicable scenario.

**Traceability:** UC-001; PER-003; PC-G-002; PD-04 delegated decision 9,
scope decision 5.

**Rationale:** Defines the gate for what may proceed and guarantees that
normalization (FR-017) and detection have the information they need — a
valid submission is normalizable by construction and, where it records a
scenario-relevant operation, evaluable by construction. Item (g) prevents
missing scenario-required information from surfacing downstream as a
silent non-match (Section E).

### FR-008 — Criteria for the invalid outcome

The platform shall classify as invalid a submission that is attributable
to the supported source family and a supported event form, but cannot be
parsed or violates the documented structural constraints of that form.

**Traceability:** UC-001 (failure outcome); PER-003; PC-G-002; PD-04
in-scope 2.

**Rationale:** UC-001 requires malformed telemetry to be visibly rejected,
distinctly from unsupported telemetry.

### FR-009 — Criteria for the incomplete outcome

The platform shall classify as incomplete a submission that is parseable
and structurally valid against the documented supported source-event form
but lacks one or more of the information items required by FR-007 —
whether core event information, items (a) through (f), or the
scenario-required information of item (g).

**Traceability:** UC-001 (failure outcome); PER-003; PC-G-002; PD-04 scope
decision 5.

**Rationale:** UC-001 requires incomplete telemetry to be flagged with its
deficiency identified, distinctly from malformed (invalid) data. Missing
scenario-required information yields a visible incomplete outcome that
does not proceed (FR-014), never a silent non-match.

### FR-010 — Criteria for the unsupported outcome

The platform shall classify as unsupported a submission that is not
attributable to the supported Kubernetes API server audit-event source
family, or belongs to a documented source-event variant, version, or
audit-event form that v0.1 explicitly does not support.

**Traceability:** UC-001 (failure outcome); PER-003; PC-G-001, PC-G-002;
PD-04 exclusion 12.

**Rationale:** UC-001 requires unsupported telemetry to be explicitly
classified — never silently dropped and never silently accepted.

## Section C — Validation-outcome visibility and progression

### FR-011 — Recorded validation outcome per submission

The platform shall record, for every received submission, the validation
outcome assigned to it.

**Traceability:** UC-001; PER-003; PC-G-002; PD-04 in-scope 2, scope
decision 9.

**Rationale:** Recording all four outcomes — including valid — is what
makes acceptance and rejection equally observable; no silent path exists.
Together with FR-014 this also makes the acceptance of valid telemetry for
normalization observable (UC-001 main flow).

### FR-012 — Stated reason for non-valid outcomes

For every submission classified invalid, incomplete, or unsupported, the
platform shall record a stated reason identifying, respectively: the
nonconformance; the missing required information; or the basis on which the
submission is unsupported.

**Traceability:** UC-001 (failure outcomes); PER-003; PC-G-002; PD-04
in-scope 2.

**Rationale:** Each UC-001 alternative outcome demands a stated reason or
identified deficiency, not only a classification label.

### FR-013 — Per-submission outcome reviewability

The platform shall make the recorded validation outcome and any stated
reason of each received submission available for review.

**Traceability:** UC-001; PER-003; PC-G-002; PD-04 scope decision 9.

**Rationale:** PER-003's success criterion — determining how each piece of
telemetry that reached the intake was classified and handled. The
presentation mechanism is deliberately unstated.

### FR-014 — Progression restricted to valid telemetry

The platform shall exclude submissions classified invalid, incomplete, or
unsupported from normalization and detection; only submissions classified
valid shall proceed.

**Traceability:** UC-001; PER-003; PC-G-002, PC-G-003; PD-04 scope
decision 5.

**Rationale:** The approved resolution of PD-03 deferred decision 5; keeps
data quality explicit through the whole workflow.

## Section D — Normalization and source traceability

### FR-015 — Normalization of every valid event

The platform shall transform every submission classified valid into
exactly one normalized event conforming to the documented normalized
representation.

**Traceability:** UC-001, UC-002; PER-003, PER-002; PC-G-003; PD-04
in-scope 3.

**Rationale:** The normalization step of PC-005; detection evaluates only
the normalized representation. The one-to-one mapping is an approved v0.1
product behavior (Resolved product details); one-to-many normalization is
not a v0.1 behavior.

### FR-016 — Documented normalized representation

The platform shall document the normalized representation, including the
meaning of each information element it conveys.

**Traceability:** UC-002; PER-002; PC-G-003; PD-04 delegated decision 3.

**Rationale:** PER-002 reviews the normalized event representation.
Requirements define what the representation must convey; its form is
delegated to architecture (PD-04 delegated decision 3).

### FR-017 — Core normalized event content

Each normalized event shall convey at least: the requesting subject; the
operation performed, including the verb; the target resource including
its kind and name and, where applicable, its namespace and subresource;
the recorded outcome information of the request, sufficient to determine
whether the operation completed successfully where a documented detection
condition requires successful completion; and the time of the request.

**Traceability:** UC-002, UC-003; PER-002, PER-001; PC-G-003; PD-04
delegated decisions 3 and 9.

**Rationale:** The minimum any of the three scenarios and any alert
explanation needs in order to state actor, action, target, outcome, and
time.

### FR-018 — Scenario-required normalized content

For each normalized event whose source event records a scenario-relevant
operation, the normalized event shall convey the request details required
by the documented detection conditions of the applicable scenario: for
pods/exec requests, the recorded exec request characteristics conveyed by
the recorded request URI or equivalent recorded request information; for
Pod-creation requests, the parts of the Pod specification covered by the
documented high-risk characteristics; and for ClusterRoleBinding creation
requests, the identity of the target binding, the referenced role, and
the bound subjects.

**Traceability:** UC-002; PER-002; PC-G-003, PC-G-004; PD-04 delegated
decision 9.

**Rationale:** Without this content the three detections cannot evaluate
their documented conditions against normalized events alone. Validation
guarantees that a valid scenario-relevant source event carries this
information (FR-007); normalization must not lose it.

### FR-019 — Normalized-to-source reference

Each normalized event shall unambiguously reference the source submission
from which it was produced and shall preserve or unambiguously reference
the source audit-event identity, including the auditID and the audit
stage.

**Traceability:** UC-002, UC-003; PER-002, PER-001; PC-G-007; PD-04
in-scope 7.

**Rationale:** The first traceability link; alert-to-source traceability
(FR-033, FR-034) is inherited from it. Preserving the source audit
identity keeps the chain anchored to the original event. The storage
representation of the reference is not defined by this document (PD-04
delegated decision 6).

## Section E — Detection definitions and evaluation

Validation guarantees that a valid source event recording a
scenario-relevant operation carries the information required to evaluate
the applicable scenario (FR-007); a submission lacking that information is
classified incomplete and does not proceed to normalization or detection
(FR-009, FR-014). A non-match therefore occurs only when all information
required to evaluate the applicable scenario was available and the
documented detection conditions were not satisfied. Detection-result
visibility for non-matching telemetry remains excluded from v0.1 (PD-04
scope decision 7).

### FR-020 — Detection definitions for the approved scenarios

The platform shall provide an identifiable detection definition for each of
the three approved v0.1 detection scenarios.

**Traceability:** UC-002; PER-002; PC-G-004; PD-04 in-scope 4, scope
decision 2.

**Rationale:** Match reasons and alerts must reference which definition
matched; identifiability enables that reference.

### FR-021 — Documented detection conditions

Each detection definition shall include documented detection conditions
stating the event characteristics required for a match.

**Traceability:** UC-002; PER-002; PC-G-004; PD-04 in-scope 4.

**Rationale:** UC-002's core verification act is comparing recorded match
reasons against documented conditions; the conditions must exist as
documented product content.

### FR-022 — Reviewability of definitions and conditions

The platform shall make each detection definition and its documented
conditions available for review.

**Traceability:** UC-002; PER-002; PC-G-004; PD-04 in-scope 8, scope
decision 3.

**Rationale:** The approved PD-04 boundary — detection-definition review is
in scope; in-product authoring, maintenance workflows, and content
lifecycle management are excluded.

### FR-023 — Evaluation of every normalized event

The platform shall evaluate every normalized event individually against the
documented conditions of each of the three detection definitions.

**Traceability:** UC-002; PER-002; PC-G-004; PD-04 in-scope 4,
exclusion 10.

**Rationale:** The detection step of PC-005. "Individually" encodes the
approved single-event semantics: stateful, aggregation-based, and
baseline-driven evaluation are excluded.

### FR-024 — Scenario 1 match — interactive exec request

The platform shall identify a detection match for scenario 1 when a
normalized event records a request to the pods/exec subresource of a pod,
regardless of the recorded outcome of the request, and the recorded request
characteristics satisfy the documented interactive-execution
characteristics, which are: the exec request enables standard-input
streaming, or the exec request requests interactive terminal (TTY)
allocation. Every documented characteristic present in the request shall be
recorded in the match reason.

**Traceability:** UC-002, UC-003; PER-002, PER-001; PC-G-004, PC-G-005;
PD-04 scenario 1, delegated decision 5.

**Rationale:** PD-04 delegates the exact interactive-execution
characteristics to requirements work; this requirement fixes them as
reviewable product content. Matching is independent of the recorded outcome
because attempted interactive access warrants review (PD-04 scenario 1
security relevance); the recorded outcome remains visible in the alert
(FR-029).

### FR-025 — Scenario 2 match — high-risk Pod creation

The platform shall identify a detection match for scenario 2 when a
normalized event records a Pod-creation request whose recorded outcome
indicates the Pod creation completed successfully and whose Pod
specification contains at least one of the documented high-risk privilege
or host-access characteristics, which are: (1) a container requesting
privileged mode; (2) use of the host network; (3) use of the host
process-identifier (PID) namespace; (4) use of the host
inter-process-communication (IPC) namespace; (5) a volume that mounts a
host filesystem path. In v0.1 this scenario evaluates Pod-creation
requests only; the creation of higher-level workload resources — such as
Deployment, StatefulSet, DaemonSet, Job, or CronJob creation — is not
directly evaluated by this requirement. A Pod-creation source event must
contain the Pod specification information required to evaluate this
characteristic set (FR-007, FR-018).

**Traceability:** UC-002, UC-003; PER-002, PER-001; PC-G-004, PC-G-005;
PD-04 scenario 2, delegated decision 5, exclusion 13.

**Rationale:** PD-04 requires the bounded characteristic set to be defined
during requirements work and forbids unlimited coverage; enumerating the
five characteristics keeps the boundary explicit. Limiting the scenario to
Pod-creation requests keeps it evaluable from a single audit event.
Observing controller-created Pods does not provide equivalent attribution
to, or complete coverage of, higher-level workload creation; direct
evaluation of higher-level workload-resource creation belongs to later
releases (PD-04 "Deferred to later releases" item 2; see Unvalidated
assumptions).

### FR-026 — Scenario 3 match — cluster-admin ClusterRoleBinding

The platform shall identify a detection match for scenario 3 when a
normalized event records the creation of a ClusterRoleBinding whose role
reference is the cluster-admin ClusterRole, where the recorded outcome
indicates the creation completed successfully. Modification of an
existing ClusterRoleBinding — including a subject addition — is not
evaluated by this scenario in v0.1 (PD-04 "Deferred to later releases").
Deletion of a ClusterRoleBinding is not within this scenario.

**Traceability:** UC-002, UC-003; PER-002, PER-001; PC-G-004, PC-G-005;
PD-04 scenario 3, delegated decision 5, exclusion 10.

**Rationale:** Completes the three approved scenarios with reviewable
conditions. Empirical review of representative Kubernetes audit events
showed that the common ways an existing ClusterRoleBinding is modified do
not reliably provide single-event, stateless evidence that a subject was
newly added rather than already present; detecting modification is
therefore deferred rather than approximated with an unreliable signal
(PD-04 "Deferred to later releases"). Scenarios 2 and 3 match only
successful operations because their approved definitions describe
completed creation (PD-04), in contrast to scenario 1's request-level
definition.

## Section F — Match reasons and alert generation

### FR-027 — Alert production for every matching result

The platform shall produce exactly one alert for every matching detection
result.

**Traceability:** UC-002, UC-003; PER-002, PER-001; PC-G-005; PD-04
in-scope 6.

**Rationale:** The alert-generation step of PC-005. With deduplication,
suppression, and aggregation excluded (PD-04 exclusion 10), one match
yields exactly one alert — an approved v0.1 product behavior (Resolved
product details).

### FR-028 — Recorded match reason

For every matching detection result that produces an alert, the platform
shall record a match reason identifying the detection definition, the
documented conditions that were satisfied, and the event information that
satisfied them.

**Traceability:** UC-002; PER-002; PC-G-004, PC-G-005; PD-04 in-scope 5.

**Rationale:** UC-002's verification hinges on comparing the recorded
reason against the documented conditions; the reason must cite both sides
of that comparison.

### FR-029 — Alert explainability content

Each alert shall convey at least: the identity of the detection definition
that matched; a statement of what was detected, including the requesting
subject, the operation, the target resource, the recorded outcome, and the
time; the documented conditions that matched; and the recorded match
reason.

**Traceability:** UC-002, UC-003; PER-001, PER-002; PC-G-005; PD-04
in-scope 6, delegated decision 6.

**Rationale:** PC-G-005's "what was detected, why it matched, and which
telemetry supports it" expressed as testable required content. PD-04
delegates alert content fields to requirements.

### FR-030 — Alert availability for review

The platform shall make every generated alert available for review.

**Traceability:** UC-002, UC-003; PER-001, PER-002; PC-G-005, PC-G-006;
PD-04 in-scope 6.

**Rationale:** UC-003 is triggered by a generated alert being available for
the analyst's review; without this behavior the use case cannot begin. The
presentation mechanism is deliberately unstated.

## Section G — Investigation evidence and traceability

### FR-031 — Minimum evidence set per alert

For every alert, the platform shall provide an evidence inventory that
covers all six artifacts of the approved minimum evidence set — (1) the
source Kubernetes audit event; (2) the normalized event; (3) the detection
definition and its documented conditions; (4) the recorded match reason;
(5) the generated alert; and (6) the traceability links between these
artifacts — and shall make every available artifact available for
inspection.

**Traceability:** UC-002, UC-003; PER-001, PER-002; PC-G-006; PD-04 scope
decision 8, in-scope 7.

**Rationale:** Adopts the approved minimum evidence set — neither expanded
nor reduced; the evidence inventory is a visibility mechanism over that
set, not additional contextual evidence (see Definitions). One requirement
serves both UC-003 investigation and UC-002 verification through shared
traceability.

### FR-032 — Evidentiary fidelity of the source event

The source-event evidence made available for an alert shall faithfully
represent the submission content as received at the defined intake. Any
later access-control, masking, or presentation behavior shall not alter
the underlying evidentiary meaning and shall not cause the product to
represent modified content as the original received event.

**Traceability:** UC-003; PER-001; PC-G-006; PD-04 scope decision 8.

**Rationale:** Evidence-based assessment requires the original event, not a
reconstruction from the normalized form (UC-003 main flow: "inspects the
original source telemetry as evidence"). Access control, masking rules,
storage, and presentation mechanisms are not designed in this document;
this requirement constrains only their evidentiary effect (PC-P-008).

### FR-033 — Traceability links among artifacts

The platform shall maintain traceability links associating each alert with
its recorded match reason, the detection definition that matched, the
normalized event that was evaluated, and the source audit event.

**Traceability:** UC-002, UC-003; PER-001, PER-002; PC-G-007; PD-04
in-scope 7.

**Rationale:** Traceability by design (PC-P-005). The links are themselves
artifact (6) of the minimum evidence set.

### FR-034 — Traceability navigation from alert to source

The platform shall enable following the traceability links from any alert
through the detection and normalization artifacts to the source audit
event.

**Traceability:** UC-003; PER-001; PC-G-007; PD-04 in-scope 7.

**Rationale:** Link existence (FR-033) and link followability are
independently testable behaviors; UC-003's main flow explicitly requires
following the chain from alert back to source telemetry.

### FR-035 — Visible absence of required explanation, evidence, or traceability

Where any artifact of the minimum evidence set or any traceability link
required for an alert is missing, unavailable, or incomplete, the platform
shall identify the affected item in the alert's evidence inventory, shall
make the limitation visible, and shall not represent the evidence set as
complete. Where any explanation element required by FR-029 is unavailable
or incomplete, the platform shall likewise visibly indicate the absence
rather than omitting it silently.

**Traceability:** UC-002 (failure outcome), UC-003 (failure outcome);
PER-001, PER-002; PC-G-005, PC-G-006, PC-G-007; PD-04 in-scope 6 and 7;
aligned with PC-P-004.

**Rationale:** Both use cases require gaps to be visible limitations rather
than unverifiable claims: UC-002's explainability deficiency and UC-003's
insufficient-evidence outcome.

## Traceability summary

| Requirement | Use cases | Personas | Product goals | PD-04 basis |
| --- | --- | --- | --- | --- |
| FR-001 | UC-001 | PER-003 | PC-G-001 | In-scope 1; decision 1 |
| FR-002 | UC-001 | PER-003 | PC-G-001, PC-G-002 | Delegated 9 |
| FR-003 | UC-001, UC-003 | PER-003, PER-001 | PC-G-002, PC-G-007 | Decision 9 |
| FR-004 | UC-001 | PER-003 | PC-G-002 | In-scope 2 |
| FR-005 | UC-001 | PER-003 | PC-G-002 | In-scope 2 |
| FR-006 | UC-001 | PER-003 | PC-G-002 | Delegated 4 |
| FR-007 | UC-001 | PER-003 | PC-G-002 | Delegated 9; decision 5 |
| FR-008 | UC-001 | PER-003 | PC-G-002 | In-scope 2 |
| FR-009 | UC-001 | PER-003 | PC-G-002 | Decision 5 |
| FR-010 | UC-001 | PER-003 | PC-G-001, PC-G-002 | Exclusion 12 |
| FR-011 | UC-001 | PER-003 | PC-G-002 | In-scope 2; decision 9 |
| FR-012 | UC-001 | PER-003 | PC-G-002 | In-scope 2 |
| FR-013 | UC-001 | PER-003 | PC-G-002 | Decision 9 |
| FR-014 | UC-001 | PER-003 | PC-G-002, PC-G-003 | Decision 5 |
| FR-015 | UC-001, UC-002 | PER-003, PER-002 | PC-G-003 | In-scope 3 |
| FR-016 | UC-002 | PER-002 | PC-G-003 | Delegated 3 |
| FR-017 | UC-002, UC-003 | PER-002, PER-001 | PC-G-003 | Delegated 3, 9 |
| FR-018 | UC-002 | PER-002 | PC-G-003, PC-G-004 | Delegated 9 |
| FR-019 | UC-002, UC-003 | PER-002, PER-001 | PC-G-007 | In-scope 7 |
| FR-020 | UC-002 | PER-002 | PC-G-004 | In-scope 4; decision 2 |
| FR-021 | UC-002 | PER-002 | PC-G-004 | In-scope 4 |
| FR-022 | UC-002 | PER-002 | PC-G-004 | In-scope 8; decision 3 |
| FR-023 | UC-002 | PER-002 | PC-G-004 | In-scope 4; exclusion 10 |
| FR-024 | UC-002, UC-003 | PER-002, PER-001 | PC-G-004, PC-G-005 | Scenario 1; delegated 5 |
| FR-025 | UC-002, UC-003 | PER-002, PER-001 | PC-G-004, PC-G-005 | Scenario 2; delegated 5; exclusion 13 |
| FR-026 | UC-002, UC-003 | PER-002, PER-001 | PC-G-004, PC-G-005 | Scenario 3; delegated 5; exclusion 10 |
| FR-027 | UC-002, UC-003 | PER-002, PER-001 | PC-G-005 | In-scope 6 |
| FR-028 | UC-002 | PER-002 | PC-G-004, PC-G-005 | In-scope 5 |
| FR-029 | UC-002, UC-003 | PER-001, PER-002 | PC-G-005 | In-scope 6; delegated 6 |
| FR-030 | UC-002, UC-003 | PER-001, PER-002 | PC-G-005, PC-G-006 | In-scope 6 |
| FR-031 | UC-002, UC-003 | PER-001, PER-002 | PC-G-006 | Decision 8; in-scope 7 |
| FR-032 | UC-003 | PER-001 | PC-G-006 | Decision 8 |
| FR-033 | UC-002, UC-003 | PER-001, PER-002 | PC-G-007 | In-scope 7 |
| FR-034 | UC-003 | PER-001 | PC-G-007 | In-scope 7 |
| FR-035 | UC-002, UC-003 | PER-001, PER-002 | PC-G-005, PC-G-006, PC-G-007 | In-scope 6, 7 |

Behavior required by more than one use case or persona is stated once with
shared traceability. In particular: "never silently dropped and never
silently accepted" (UC-001) is not a separate requirement — it emerges
jointly from FR-004, FR-005, FR-011, and FR-013; and "valid telemetry is
observably accepted for normalization" (UC-001 main flow) is covered by
FR-011 together with FR-014.

## Excluded requirement candidates

The following candidate requirements were considered and are excluded, with
their basis:

1. **Aggregate validation reporting** — deferred by PD-04 scope decision 9.
2. **Source-health and missing-delivery visibility** — excluded by PD-04
   scope decision 6.
3. **Detection-result visibility for non-matching telemetry** — excluded by
   PD-04 scope decision 7.
4. **Platform self-detection of discrepancies between a recorded match
   reason and the documented conditions** — UC-002 requires only that a
   discrepancy be identifiable from recorded information; FR-021, FR-028,
   and FR-031 provide both sides for PER-002's comparison. An automated
   consistency checker would be a new capability with no scope basis.
5. **In-product detection authoring, maintenance, and lifecycle
   management** — excluded by PD-04 scope decision 3.
6. **Alert disposition, state, or assignment management** — excluded by
   PD-04 exclusion 2 (PC-011).
7. **General normalized-event or telemetry browsing and search** — SIEM
   drift, excluded by PD-04 exclusion 1; normalized events are reachable
   through alerts (FR-031), which is sufficient for UC-002 and UC-003.
8. **A defined product outcome for normalization failure of valid
   telemetry** — avoided by defining the valid outcome (FR-007) to
   guarantee the information normalization needs; such a failure is an
   engineering fault, not a product data-quality outcome (see Unvalidated
   assumptions).
9. **Alert notification or delivery** — a presentation and delivery
   concern, deferred by PD-04 delegated decision 7.
10. **Per-persona access control** — security hardening and access design
    belong to non-functional requirements and later design (PD-02
    assumption 6, PC-P-008).
11. **Alert deduplication or suppression** — stateful semantics are
    excluded by PD-04 exclusion 10.
12. **Stateful comparison of ClusterRoleBinding modifications with
    previous object states** — excluded by PD-04 exclusion 10; scenario 3
    evaluates only ClusterRoleBinding creation in v0.1, and detection of a
    modification (which would require either stateful comparison or an
    unreliable single-event signal) is deferred to a future release
    (PD-04 "Deferred to later releases").

## Resolved product details and remaining delegations

The following product details, delegated to requirements work by PD-04,
are resolved by this document:

1. **Unit of validation** — each submission carries a single audit event;
   validation, classification, and outcome visibility apply per submission
   (see Definitions). The mapping of any batched delivery onto per-event
   submissions is delegated to architecture with the delivery mechanism.
2. **Request outcome and matching** — scenario 1 matches the request
   regardless of its recorded outcome; scenario 2 matches only successful
   Pod creation; scenario 3 matches only successful creation (FR-024,
   FR-025, FR-026). The recorded outcome is always conveyed in the alert
   (FR-029).
3. **Scenario 1 interactive-execution characteristics** — standard-input
   streaming enabled, or interactive terminal (TTY) allocation requested
   (FR-024).
4. **Scenario 2 bounds** — the scenario evaluates Pod-creation requests
   only; higher-level workload-resource creation is not directly
   evaluated; the bounded high-risk characteristic set is the five
   characteristics enumerated in FR-025.
5. **Scenario 3 scope** — v0.1 evaluates only the creation of a
   cluster-admin ClusterRoleBinding; modification (including a subject
   addition) and deletion are excluded from this scenario in v0.1
   (FR-026; PD-04 "Deferred to later releases").
6. **Missing scenario-required information** — a submission recording a
   scenario-relevant operation that lacks the information required to
   evaluate the applicable scenario is classified incomplete and does not
   proceed (FR-007, FR-009, FR-014); a non-match occurs only when all
   required information was available and the documented conditions were
   not satisfied (Section E).
7. **Supported source-event contract** — the supported source-event form
   is a Kubernetes audit.k8s.io/v1 Event that preserves the source
   request identity (auditID and audit stage), carries the core and
   scenario-required request information, and provides outcome
   information from an event stage sufficient to determine successful
   completion where a detection condition requires it (FR-002, FR-007).
   The mechanisms that produce, collect, and deliver events remain
   undecided.
8. **One-to-one normalization and alerting** — each valid submission
   produces exactly one normalized event (FR-015), and each matching
   detection result produces exactly one alert (FR-027). Deduplication,
   suppression, aggregation, and one-to-many normalization are not v0.1
   behaviors (PD-04 exclusion 10).

The following remain delegated to later artifacts:

1. The telemetry delivery mechanism or protocol, and the audit collection
   mechanism and audit-policy configuration that produce and deliver
   supported events to the intake (architecture; PC-C-004).
2. The storage and maintenance mechanism for detection definitions (later
   design work).
3. The format of the normalized representation (architecture); this
   document defines only what it must convey (FR-016 through FR-018).
4. Alert content format and the representation of traceability links
   (later design); this document defines only required content (FR-029)
   and required associations (FR-033).
5. The presentation surfaces for validation outcomes, detection
   definitions, alerts, and evidence (later design).
6. Concrete pass/fail acceptance criteria per requirement (future AC-###
   artifact).
7. Non-functional requirements (future NFR-### artifact).

## Unvalidated assumptions

1. Kubernetes API server audit events, in the documented supported form,
   carry the request-level detail the three scenarios need — the exec
   request characteristics, the Pod specification, and the recorded
   ClusterRoleBinding request content (refines PD-04 assumptions 1 and 5).
2. Defining the valid outcome to include all information required for
   normalization (FR-007) makes a normalization failure of valid telemetry
   an engineering fault rather than a product data-quality outcome.
3. Deliberately limiting scenario 2 to Pod-creation audit events, without
   direct evaluation of higher-level workload-resource creation, is
   sufficient for the v0.1 demonstration goals (FR-025).

These assumptions must be validated, refined, or rejected during the
remaining Phase 0 definition work.
