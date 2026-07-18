# Cloud-Native Security Telemetry and Detection Platform — Use Cases

| Field | Value |
| --- | --- |
| Document ID | PD-03 |
| Version | 0.1 |
| Status | Approved — Phase 0 baseline |
| Phase | Phase 0 — Product Definition and Requirements |

## Purpose and scope

This document defines the v0.1 product use cases for the platform described in
the Product Charter (PD-01, `docs/product.md`) and operated by the personas
defined in the Personas document (PD-02, `docs/personas.md`).

Per PC-015, each use case traces to at least one persona (PER-###) and at
least one product goal (PC-G-###). Future functional requirements (FR-###)
must trace to at least one use case defined here.

The use cases describe user goals and observable product behavior only. This
document does not decide telemetry sources, detection scenarios, delivery
mechanisms, detection-content maintenance mechanisms, architecture, or
technology. Undecided matters are recorded in the Deferred decisions section.

Together, the three use cases cover the primary product outcome (PC-005):

> Telemetry collection → validation and normalization → detection →
> alert generation → evidence-based investigation

## Use-case conventions

- Identifiers use the stable UC-### namespace and are never renumbered or
  reused.
- Identifiers follow the end-to-end workflow order of PC-005. This ordering
  does not express persona priority: the Security Analyst (PER-001) remains
  the primary product persona, and UC-003 is the use case the product is
  built around.
- Actors are the personas defined in PD-02. The platform itself is not an
  actor.
- Each use case records: primary actor, supporting actors, user goal, trigger,
  preconditions, main flow, alternative and failure outcomes, successful
  outcome, traceability, and its necessity for v0.1.
- Alternative and failure outcomes are part of the required product behavior,
  in line with PC-P-004: invalid, incomplete, and unsupported telemetry must
  be handled visibly and intentionally.

## Use-case overview

| Use case | Name | Primary persona | Product goals |
| --- | --- | --- | --- |
| UC-001 | Deliver telemetry and verify its intake and validation outcome | PER-003 | PC-G-001, PC-G-002 |
| UC-002 | Verify a detection match and evaluate alert explainability | PER-002 | PC-G-003, PC-G-004, PC-G-005 |
| UC-003 | Investigate an alert using its explanation and supporting evidence | PER-001 (primary) | PC-G-005, PC-G-006, PC-G-007 |

## UC-001 — Deliver telemetry and verify its intake and validation outcome

**Primary actor:** PER-003 — Platform / Cloud Engineer.

**Supporting actors:** None.

**User goal:** Confirm that telemetry submitted to or received by the
platform's defined intake is explicitly classified as valid, invalid,
incomplete, or unsupported, and determine how the received telemetry was
handled.

**Trigger:** Telemetry from a source is delivered to the platform's defined
intake.

**Preconditions:** At least one telemetry source is designated as supported
and its expected form is documented. The concrete supported sources are not
selected in this document (see Deferred decisions).

**Main flow:** Telemetry is delivered through the defined intake. The platform
validates it. The engineer reviews the intake and validation outcome
associated with the received telemetry. Valid telemetry is observably
accepted for normalization.

**Alternative and failure outcomes:**

- Invalid telemetry is visibly rejected, with a stated reason.
- Incomplete telemetry is visibly flagged as incomplete, with the deficiency
  identified.
- Unsupported telemetry — from an unsupported source or in an unsupported
  form — is explicitly classified as unsupported. It is never silently
  dropped and never silently accepted.

This use case covers only telemetry that reaches the defined intake. It does
not include detecting or inferring that expected telemetry never arrived (see
Deferred decisions).

**Successful outcome:** The engineer can determine how telemetry that reached
the defined intake was classified and handled, consistent with the PER-003
success criteria.

**Persona traceability:** PER-003.

**Product-goal traceability:** PC-G-001, PC-G-002. Aligned with PC-P-004.

**Why necessary for v0.1:** This use case is the entry point of the primary
product outcome (PC-005) and the only use case that justifies explicit
data-quality behavior (PC-009 items 1 and 2). Without it, intake and
validation visibility would have no documented user justification (PC-C-003).

## UC-002 — Verify a detection match and evaluate alert explainability

**Primary actor:** PER-002 — Detection Engineer.

**Supporting actors:** None.

**User goal:** Verify that a detection matched for its documented conditions
against the normalized telemetry it evaluated, and assess whether the
resulting alert is explainable and supported by evidence.

**Trigger:** Defined detection logic has evaluated normalized telemetry and
produced a detection result and alert that the engineer wants to verify.

**Preconditions:** Detection conditions are documented and reviewable.
Normalized telemetry exists. How detection content is defined and maintained
is not decided in this document (see Deferred decisions).

**Main flow:** The engineer reviews the detection definition and its
documented conditions; reviews the normalized event representation that was
evaluated; reviews the detection result and the recorded reason for the
match; compares the recorded reason against the documented conditions; and
reviews the generated alert with its supporting telemetry to assess whether
the alert is explainable.

**Alternative and failure outcomes:**

- The recorded match reason does not correspond to the documented detection
  conditions. The discrepancy is identifiable from the product's recorded
  information rather than hidden.
- The generated alert lacks sufficient supporting telemetry. The gap is
  visible as an explainability deficiency rather than an unverifiable claim.

**Successful outcome:** The engineer verifies that the detection matched for
its documented reason and can judge whether the alert is explainable and
supported by evidence, consistent with the PER-002 success criteria.

**Persona traceability:** PER-002.

**Product-goal traceability:** PC-G-003, PC-G-004, PC-G-005. Aligned with
PC-P-003.

**Why necessary for v0.1:** This use case is the only one that justifies why
detection conditions must be documented, match reasons recorded, and the
normalized representation inspectable (PC-009 items 3 through 6). UC-003
consumes this behavior but does not justify it at the definition level.

## UC-003 — Investigate an alert using its explanation and supporting evidence

**Primary actor:** PER-001 — Security Analyst. PER-001 is the primary product
persona; this use case is the terminal step of the primary product outcome.

**Supporting actors:** None.

**User goal:** Reach an evidence-backed assessment of the detected activity
and determine whether the alert is supported by its explanation and
telemetry. Response workflows, case management, and automated action are
outside this use case and outside v0.1 (PC-011).

**Trigger:** A generated alert is available for the analyst's review.

**Preconditions:** Supported telemetry was accepted and normalized. Defined
detection logic evaluated the normalized telemetry, matched, and produced an
alert. The required platform behavior is described by UC-001 and UC-002, but
those use cases do not need to have been performed manually before this use
case begins.

**Main flow:** The analyst reviews the alert; reads its explanation of what
was detected and which documented detection conditions matched; inspects the
normalized event or events behind the alert; inspects the original source
telemetry as evidence; follows the traceability chain from the alert through
detection and normalization back to the source telemetry; and reaches an
evidence-backed assessment of the detected activity.

**Alternative and failure outcomes:**

- The alert explanation, supporting evidence, or traceability information is
  insufficient or unavailable. The limitation is visible, and the analyst
  cannot complete an evidence-backed assessment confidently.
- The analyst assesses the activity as not genuinely suspicious. This is a
  valid outcome; the explanation and evidence must still support that
  assessment.

**Successful outcome:** The analyst determines what triggered the alert, why
it matched, and which telemetry and evidence support it, without needing to
reconstruct the platform's detection reasoning outside the product,
consistent with the PER-001 success criteria.

**Persona traceability:** PER-001.

**Product-goal traceability:** PC-G-005, PC-G-006, PC-G-007. Aligned with
PC-P-002 and PC-P-005.

**Why necessary for v0.1:** This use case realizes the core value proposition
(PC-008) and the final stages of PC-005. Without it, PC-009 items 6 through 8
cannot be demonstrated.

## Excluded or deferred use cases

The following candidate use cases were considered and are excluded from or
deferred beyond this document:

1. **Detection-content authoring and maintenance (PER-002)** — deferred. The
   mechanism by which detection content is defined and maintained is
   undecided and belongs to later scope and architecture work. PER-002's
   review of detection definitions, documented conditions, recorded match
   reasons, normalized telemetry, and resulting alerts is retained in UC-002.
2. **End-to-end demonstration for the Technical Reviewer** — excluded. The
   Technical Reviewer is a non-persona stakeholder (PD-02) and cannot anchor
   a use case under PC-015. PC-G-009 is served by UC-001 through UC-003
   together with project documentation.
3. **Tracked alert disposition and state management** — excluded. Initial
   alert review and an evidence-backed assessment are included in UC-003, but
   persisted disposition states, assignments, workflow transitions, and
   case-management behavior are outside v0.1 (PC-011).
4. **Platform operation and monitoring** — excluded. Operability is an
   engineering requirement (PC-G-008); PD-02 defines no operator persona.
5. **A separate use case for invalid, incomplete, or unsupported telemetry**
   — excluded. These are explicit alternative outcomes of UC-001; a separate
   use case would duplicate the same intake behavior.
6. **A separate traceability-navigation use case** — excluded. Tracing an
   alert to its source telemetry is a step within UC-003 (PC-G-007), not an
   independent user goal.
7. **Telemetry search or exploration independent of an alert** — excluded. It
   has no charter basis and drifts toward SIEM capability (PC-011).

## Deferred decisions

The following decisions are required by later project artifacts and are
deliberately not made in this document:

1. Selection of the concrete v0.1 telemetry sources (PC-C-002).
2. Selection of the concrete v0.1 detection scenarios (PC-C-002).
3. The mechanism for defining and maintaining detection content.
4. The telemetry delivery mechanism or protocol for the defined intake
   (PC-C-004).
5. Whether incomplete telemetry may proceed to normalization or detection.
6. Whether v0.1 includes source-health or missing-delivery visibility, such
   as determining that expected telemetry never reached the defined intake.
7. Whether detection evaluation results for non-matching telemetry are
   visible to PER-002.
8. What constitutes "contextual evidence" (PC-G-006) beyond the source
   telemetry and normalized events supporting an alert.
9. Whether validation outcomes are reviewable per submission, in aggregate,
   or both.

## Unvalidated assumptions

1. A single alert-investigation use case covers both initial review and
   deeper investigation for v0.1; no analyst tier split is needed (PD-02
   assumption 1; PC-A-004).
2. PER-003's product interaction is fully captured by visibility into the
   classification of telemetry that reaches the defined intake, without
   delivery-pipeline management and, pending the related deferred decision,
   without source-health visibility (PD-02 assumption 3).
3. PER-002 can assess alert quality from the same recorded match reasons and
   evidence the product presents, without dedicated quality-evaluation
   capability in v0.1.
4. The three use cases together are sufficient to demonstrate the complete
   PC-005 workflow credibly (PC-A-002), including for portfolio review
   (PC-G-009), without a dedicated demonstration use case.

These assumptions must be validated, refined, or rejected during the
remaining Phase 0 definition work.
