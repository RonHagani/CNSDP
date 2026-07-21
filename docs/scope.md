# Cloud-Native Security Telemetry and Detection Platform — v0.1 Scope and Non-Goals

| Field | Value |
| --- | --- |
| Document ID | PD-04 |
| Version | 0.1 |
| Status | Approved — Phase 0 baseline |
| Phase | Phase 0 — Product Definition and Requirements |

## Purpose and scope

This document defines the v0.1 scope and non-goals of the platform described
in the Product Charter (PD-01, `docs/product.md`). It is the Scope and
Non-Goals document anticipated by PC-011. It selects the concrete v0.1
telemetry source family and detection scenarios required by PC-C-002, and it
resolves or classifies each deferred decision recorded in the Use Cases
document (PD-03, `docs/use-cases.md`).

The scope defined here must allow UC-001, UC-002, and UC-003 to be exercised
end to end by the personas defined in PD-02 (`docs/personas.md`).

This document decides product scope only. It does not define functional
requirements, non-functional requirements, acceptance criteria, architecture,
threat models, or implementation plans. It selects no technologies, protocols,
storage systems, programming languages, frameworks, infrastructure, deployment
topology, implementation components, or user-interface mechanisms (PC-C-004).
Matters delegated to later artifacts are listed in "Undecided — delegated to
requirements or architecture".

## v0.1 scope statement

v0.1 is the smallest credible release that demonstrates the complete primary
product outcome (PC-005):

> Telemetry collection → validation and normalization → detection →
> alert generation → evidence-based investigation

for one deliberately limited telemetry source family and three detection
scenarios, with explainable alerts, a defined minimum evidence set, and
end-to-end traceability (PC-P-001, PC-P-003, PC-P-005).

Breadth of integrations, detection volume, automation, and enterprise
workflow features are not v0.1 success measures (PC-010). The non-goals in
PC-011 remain binding.

## Selected telemetry source family

The single v0.1 telemetry source family is **Kubernetes API server audit
events**: the audit records produced by a Kubernetes API server that describe
requests to the cluster control plane, including the requesting subject, the
operation performed, the target resource, and the outcome of the request.

Selecting this source family is a product data-scope decision: it defines
which telemetry the product accepts. It is not a decision about the
technologies used to build, operate, or deploy the platform itself and places
no constraint on those later decisions (PC-C-004).

Justification:

1. A single coherent source family satisfies the deliberate limitation
   required by PC-C-002 and the end-to-end-before-breadth principle
   (PC-P-001).
2. Kubernetes API server audit events are security-relevant, cloud-native
   control-plane telemetry, directly aligned with the problem statement
   (PC-003).
3. Each audit event describes the actor, the action, the target, and the
   outcome in one record, supporting explainable detection, evidence-based
   investigation, and end-to-end traceability (PC-G-005, PC-G-006, PC-G-007).
4. The family supports all three use cases: intake and validation
   classification (UC-001), detection verification against documented
   conditions (UC-002), and investigation from an alert back to the source
   event (UC-003).

## Selected detection scenarios

v0.1 includes exactly three detection scenarios. Each scenario is evaluated
against individual audit events; stateful, aggregation-based, and
baseline-driven detection semantics are excluded from v0.1. Each scenario's
detection conditions must be documented and reviewable (UC-002), and each
resulting alert must be explainable and backed by the minimum evidence set
defined in scope decision 8. The exact initial detection conditions for every
scenario are defined during requirements work.

Each scenario supports UC-002 and UC-003 and traces to PC-G-004 and PC-G-005.
UC-001 is supported by the selected source family independently of any
individual scenario.

### Scenario 1 — Container exec request with interactive characteristics

Detection of a Kubernetes API request to the pods/exec subresource that
satisfies explicitly documented interactive-execution characteristics.

The exact interactive-execution characteristics are defined during
requirements work.

Security relevance: an exec request that exhibits documented interactive
characteristics indicates attempted interactive access to a running workload
and warrants review, even when it represents legitimate operational activity.

Explainability and evidence expectation: the supported audit event form must
identify the requesting subject, the target workload, the exec subresource,
the relevant request characteristics, and the recorded outcome information
required by the detection, so the alert can state which documented conditions
matched and which event supports the match.

### Scenario 2 — Creation of a workload with documented high-risk privilege or host-access characteristics

Detection of the creation of a workload whose specification includes
explicitly documented high-risk privilege or host-access characteristics.

This scenario is deliberately bounded: v0.1 covers only a limited, explicitly
documented set of such characteristics, defined during requirements work. It
is not a single unlimited detection over every possible privileged or
host-access property.

Security relevance: workloads with elevated privileges or host access weaken
the isolation between workload and host and are a recognized privilege
escalation and persistence vector.

Explainability and evidence expectation: the matching audit event contains
the workload specification, so the alert can state which documented high-risk
characteristic was present and which documented conditions matched.

### Scenario 3 — Grant of cluster-admin privileges through a ClusterRoleBinding

Detection of the creation of a ClusterRoleBinding that references the
cluster-admin ClusterRole.

This scenario is deliberately bounded to creation. Detection of a subject
being added to an already-existing ClusterRoleBinding is deferred to a
future release: empirical review of representative Kubernetes audit events
found that common modification techniques do not reliably provide
single-event, stateless evidence distinguishing a newly added subject from
one already present (see "Deferred to later releases").

Security relevance: granting cluster-wide administrative privileges through a
ClusterRoleBinding is a high-impact privilege escalation and persistence
action.

Explainability and evidence expectation: the matching audit event identifies
the requesting subject, the ClusterRoleBinding, and the referenced
cluster-admin ClusterRole, so the alert can state which documented conditions
matched and which event supports the match.

## Scope decisions

The following resolutions correspond, by number, to the deferred decisions
recorded in PD-03:

1. **Concrete v0.1 telemetry sources** — resolved. Kubernetes API server
   audit events are the single supported source family (see "Selected
   telemetry source family").
2. **Concrete v0.1 detection scenarios** — resolved. The three scenarios
   defined in "Selected detection scenarios".
3. **Mechanism for defining and maintaining detection content** — boundary
   resolved. Detection definitions and their documented conditions are
   reviewable in v0.1. In-product detection authoring, maintenance workflows,
   and content lifecycle management are excluded from v0.1. The mechanism by
   which detection definitions are stored and maintained is deliberately not
   decided here and is delegated to later design work.
4. **Telemetry delivery mechanism or protocol** — remains deferred to
   architecture and design work (PC-C-004).
5. **Whether incomplete telemetry may proceed** — resolved. Telemetry
   classified as incomplete is visibly flagged with the deficiency identified
   and does not proceed to normalization or detection in v0.1. Only telemetry
   classified as valid proceeds.
6. **Source-health or missing-delivery visibility** — resolved. Excluded from
   v0.1 and deferred to a later release. v0.1 covers only telemetry that
   reaches the defined intake, preserving the UC-001 boundary.
7. **Detection-result visibility for non-matching telemetry** — resolved.
   Excluded from v0.1. Verification under UC-002 relies on the recorded match
   reasons of matching detection results that produce alerts.
8. **Definition of contextual evidence** — boundary resolved. The minimum
   evidence available for the investigation of an alert consists of:
   1. the source Kubernetes audit event
   2. the normalized event
   3. the detection definition and its documented conditions
   4. the recorded match reason
   5. the generated alert
   6. traceability links between these artifacts

   Requirements work defines the required content of each artifact in the
   minimum evidence set. Additional contextual evidence beyond this minimum
   is deferred to later releases.
9. **Validation-outcome review granularity** — boundary resolved.
   Per-submission validation-outcome visibility for telemetry that reaches
   the defined intake is in scope. Aggregate validation reporting is
   deferred, and no presentation mechanism is chosen.

## In scope for v0.1

1. A defined intake for Kubernetes API server audit events. The intake
   mechanism is not decided here (UC-001, PC-G-001).
2. Validation that classifies each received submission as valid, invalid,
   incomplete, or unsupported, with a per-submission visible outcome and a
   stated reason (UC-001, PC-G-002, PC-P-004).
3. A documented normalized representation of supported audit events
   (PC-G-003).
4. The three selected detection scenarios, each with documented, reviewable
   detection conditions (UC-002, PC-G-004).
5. A recorded match reason for every matching detection result that produces
   an alert (UC-002, PC-G-004, PC-G-005).
6. Explainable alert generation: what was detected, which documented
   conditions matched, why they matched, and which telemetry supports the
   alert (UC-003, PC-G-005).
7. Evidence-based investigation over the minimum evidence set defined in
   scope decision 8, with traceability from the alert back to the source
   audit event (UC-003, PC-G-006, PC-G-007).
8. Detection-definition review: reading and assessing detection definitions
   and their documented conditions through the supported workflow (UC-002).
   Authoring is excluded (see "Explicitly excluded from v0.1").

## Explicitly excluded from v0.1

Exclusion from v0.1 does not by itself reject a capability permanently;
capabilities anticipated for later releases are listed in "Deferred to later
releases". Capabilities excluded by the PC-011 non-goals remain excluded
unless the approved product scope changes.

1. SIEM-style search or telemetry exploration independent of an alert
   (PC-011).
2. Incident response, case management, and persisted alert disposition or
   state management (PC-011).
3. Automated response and orchestration (PC-011, PC-P-002).
4. Multi-tenant and SaaS capability (PC-011, PC-C-005).
5. Compliance reporting (no charter basis; PC-C-001).
6. In-product detection authoring, maintenance workflows, and content
   lifecycle management (scope decision 3).
7. Source-health and missing-delivery visibility (scope decision 6).
8. Detection-result visibility for non-matching telemetry (scope decision 7).
9. Aggregate validation reporting (scope decision 9).
10. Stateful, aggregation-based, or baseline/anomaly detection semantics.
11. Contextual evidence beyond the minimum evidence set (scope decision 8).
12. Any telemetry source family other than Kubernetes API server audit
    events.
13. Unlimited coverage of privileged or host-access workload characteristics
    in scenario 2.

## Deferred to later releases

1. Additional telemetry source families.
2. Additional detection scenarios, including multi-event correlation and
   stateful detection semantics.
3. Source-health and missing-delivery visibility.
4. Aggregate validation reporting.
5. Detection-content lifecycle tooling.
6. Contextual evidence beyond the approved v0.1 minimum evidence set.
7. Detection of a subject added to an already-existing ClusterRoleBinding
   referencing the cluster-admin ClusterRole. Empirical review of
   representative Kubernetes audit events for common ClusterRoleBinding
   modification techniques found that a single audit event does not
   reliably provide evidence distinguishing a newly added subject from one
   already present, without comparison to the binding's prior state (see
   Scenario 3).

## Undecided — delegated to requirements or architecture

1. The telemetry delivery mechanism or protocol for the defined intake —
   architecture and design (PC-C-004).
2. The mechanism for storing and maintaining detection definitions — later
   design work.
3. The content and format of the normalized representation — requirements
   define what it must convey; architecture defines its form.
4. The exact validation rules that distinguish valid, invalid, incomplete,
   and unsupported submissions — requirements.
5. The exact initial detection conditions for each scenario, including the
   bounded set of high-risk privilege and host-access characteristics for
   scenario 2 — requirements.
6. Alert content fields and the representation of traceability links —
   requirements and later design.
7. The presentation surfaces for validation outcomes, detection results,
   alerts, and evidence. No user interface, API, or other presentation
   mechanism is assumed — requirements and later design.
8. The required content, fields, and presentation behavior of each artifact
   in the approved minimum evidence set — requirements and later design.
   Requirements must not expand the approved v0.1 evidence boundary.
9. The exact Kubernetes audit-event fields and supported source-event form
   required for validation, detection, explainability, and investigation,
   including the request, target, subject, outcome, workload-specification,
   and RBAC details needed by the selected scenarios — requirements. No audit
   configuration, collection mechanism, delivery protocol, or implementation
   technology is selected.

## Unvalidated assumptions

1. Kubernetes API audit events provide sufficient source evidence for an
   evidence-backed assessment of the selected scenarios when combined with
   the normalized event, the detection definition and documented conditions,
   the recorded match reason, the generated alert, and the traceability links
   between them (refines PC-A-004).
2. Three detection scenarios evaluated against individual audit events are
   sufficient to credibly demonstrate the complete PC-005 workflow, including
   for portfolio review (PC-A-002, PC-G-009).
3. A bounded, explicitly documented set of high-risk privilege and
   host-access characteristics can be defined during requirements work
   without expanding scenario 2 into unlimited coverage.
4. Selecting the Kubernetes audit-event source family constrains product data
   scope only and does not constrain the platform's implementation technology
   (PC-C-004).
5. Supported telemetry is available at the defined intake in a documented
   expected form; how that telemetry is produced and delivered is outside
   this document.

These assumptions must be validated, refined, or rejected during the
remaining Phase 0 definition work.

## Traceability summary

| Scope element | Use cases | Personas | Product goals |
| --- | --- | --- | --- |
| Intake for the selected source family | UC-001 | PER-003 | PC-G-001 |
| Validation and per-submission outcome visibility | UC-001 | PER-003 | PC-G-002 |
| Normalized representation | UC-002 | PER-002 | PC-G-003 |
| Detection scenarios and reviewable conditions | UC-002 | PER-002 | PC-G-004 |
| Recorded match reasons and explainable alerts | UC-002, UC-003 | PER-002, PER-001 | PC-G-005 |
| Minimum evidence set and investigation | UC-003 | PER-001 | PC-G-006 |
| End-to-end traceability links | UC-003 | PER-001 | PC-G-007 |

PC-G-008 (production-oriented engineering) and PC-G-009 (portfolio
demonstration) apply to v0.1 as cross-cutting goals rather than as individual
scoped capabilities. PC-G-010 (responsible extensibility) is honored by the
exclusions and deferrals above.

The deliberate limitation of this scope traces to PC-C-001, PC-C-002, and
PC-C-003 and follows PC-P-001, PC-P-006, and PC-P-007. Explicit data-quality
behavior follows PC-P-004. Evidence before automation follows PC-P-002.
