# Cloud-Native Security Telemetry and Detection Platform — Personas

| Field | Value |
| --- | --- |
| Document ID | PD-02 |
| Version | 0.1 |
| Status | Approved — Phase 0 baseline |
| Phase | Phase 0 — Product Definition and Requirements |

## Purpose and scope

This document defines the product personas deferred by PC-007 of the Product
Charter (`docs/product.md`). It records who the product serves, what each role
needs from the product, and which future product interactions each persona is
expected to influence.

Per PC-015, each future use case (UC-###) must trace to at least one persona
defined here and at least one product goal (PC-G-###).

Primary personas are the roles the product's security workflow is built
around. Supporting personas influence product behavior and requirements but
the product is not built around their workflows. Detailed use cases,
requirements, and access design are defined in later project artifacts, not
here.

## PER-001 — Security Analyst

**Classification:** Primary persona.

**Role summary:** Investigates security alerts generated from cloud-native
telemetry and decides whether the detected activity warrants further action.

**Responsibilities:** Review generated alerts; determine what activity
occurred; assess whether the activity is genuinely suspicious; inspect the
telemetry and evidence supporting each alert.

**Goals:** Understand what was detected and why; reach an evidence-backed
judgment on each alert; trust that every alert is traceable to real telemetry.

**Product-relevant workflows:** Alert review; reading alert explanations;
inspecting supporting evidence; tracing an alert back through detection and
normalization to the source telemetry.

**Pain points addressed:** Alerts without sufficient context (PC-003) — not
knowing what happened, which telemetry caused the alert, why it was considered
suspicious, which detection triggered, or what evidence to review.

**Information and access needs:** Needs to view generated alerts and their
explanations; the detection conditions that matched; the normalized events
behind an alert; the original source telemetry as evidence; and the full
traceability chain between them.

**Success criteria:** Can determine what triggered an alert, why it matched,
and which supporting telemetry and evidence should be reviewed, without
needing to reconstruct the platform's basic detection reasoning outside the
product.

**Expected future product interactions:** Alert review and investigation;
evidence inspection; traceability navigation from alert to source telemetry.

**Charter traceability:** PC-G-005, PC-G-006, PC-G-007.

## PER-002 — Detection Engineer

**Classification:** Supporting persona. Consolidates the "security engineers
responsible for detection content" and "detection engineers evaluating alert
quality" stakeholders named in PC-007 into a single role.

**Role summary:** Owns and maintains detection content through the project's
supported workflow and evaluates whether the resulting alerts are explainable
and justified.

**Responsibilities:** Define documented detection conditions; maintain
detection content against the normalized telemetry representation through the
supported project workflow; evaluate the quality and explainability of
generated alerts.

**Goals:** Detections evaluate normalized telemetry predictably; every match
is explainable against its documented conditions; alert quality can be
assessed from the product itself.

**Product-relevant workflows:** Reviewing and maintaining detection
definitions; reviewing detection results and match reasons; assessing
generated alerts against their supporting telemetry.

**Pain points addressed:** Opaque detections whose match reasons cannot be
verified; detection logic disconnected from the telemetry it evaluates
(PC-003).

**Information and access needs:** Needs to view the normalized event
representation; detection definitions and their documented conditions;
detection results with the reason each match occurred; and the generated
alerts.

**Success criteria:** Can verify that a detection matched for its documented
reason and evaluate whether the resulting alert is explainable and supported
by evidence.

**Expected future product interactions:** Detection-definition review and
maintenance through the supported workflow; detection-result evaluation;
alert-quality review.

**Charter traceability:** PC-G-003, PC-G-004, PC-G-005.

## PER-003 — Platform / Cloud Engineer

**Classification:** Supporting persona.

**Role summary:** Responsible for the availability of telemetry from the
supported cloud-native sources into the platform.

**Responsibilities:** Ensure supported telemetry sources deliver data to the
platform's intake; understand how submitted telemetry was accepted, rejected,
or flagged.

**Goals:** Confirm that telemetry from supported sources reaches the platform;
see explicit, visible outcomes for invalid, incomplete, and unsupported data
rather than silent loss.

**Product-relevant workflows:** Telemetry intake; reviewing validation
outcomes for submitted telemetry.

**Pain points addressed:** Fragmented, inconsistent telemetry that is
difficult to validate (PC-003); telemetry rejected or dropped without visible
explanation, contrary to PC-P-004.

**Information and access needs:** Needs to view intake status for supported
sources and validation outcomes that distinguish valid, invalid, incomplete,
and unsupported telemetry.

**Success criteria:** Can determine whether telemetry from a supported source
arrived and how the platform classified it.

**Expected future product interactions:** Supported-source intake
verification; telemetry-delivery status review; validation-outcome review.

**Charter traceability:** PC-G-001, PC-G-002.

## Non-persona stakeholders

**Technical Reviewer** (PC-007, PC-G-009) — evaluates the project as a
production-oriented portfolio system. This stakeholder influences
documentation quality, demonstrability of the end-to-end workflow, and
portfolio evaluation, but does not operate the product's security workflow and
therefore is not defined as a persona.

## Excluded stakeholders

- **Separate Security Engineer** — merged into PER-002; both roles need the
  same things from the product.
- **Platform Operator** — operability of the platform is an engineering
  requirement (PC-G-008, PC-009 item 9), not a persona-driven product workflow
  in v0.1.
- **Incident Responder / SOC Case Manager** — incident response and case
  management are non-goals (PC-011).
- **SOC Manager / CISO / Compliance Officer** — no charter basis; would expand
  scope beyond PC-C-001.
- **SaaS Tenant / End Customer** — multi-tenant SaaS is excluded from v0.1
  (PC-011, PC-C-005).
- **Threat Intelligence Analyst / Red Team** — no charter basis; no v0.1
  workflow involves them.

## Unvalidated assumptions

1. One analyst persona covers both alert triage and deeper investigation in
   v0.1; no tier split is needed at this scale.
2. Detection content ownership and alert-quality evaluation can be represented
   as one role without losing product-relevant distinctions.
3. The platform/cloud engineer's product interaction is limited to telemetry
   intake and validation visibility, not detection or investigation.
4. The technical reviewer needs only documentation and an observable
   demonstration of the workflow (PC-G-009), not runtime product capability.
5. Operational procedures are executed by whoever deploys the platform; no
   operator persona is required for v0.1.
6. Role-level information and access descriptions are sufficient for the
   personas artifact; detailed access requirements and security design are
   deferred to later project artifacts.
