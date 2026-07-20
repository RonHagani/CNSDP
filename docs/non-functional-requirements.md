# Cloud-Native Security Telemetry and Detection Platform — v0.1 Non-Functional Requirements

| Field | Value |
| --- | --- |
| Document ID | PD-06 |
| Version | 0.1 |
| Status | Approved — Phase 0 baseline |
| Phase | Phase 0 — Product Definition and Requirements |

## Purpose and scope

This document defines the v0.1 non-functional requirements of the platform
described in the Product Charter (PD-01, `docs/product.md`), for the
personas defined in the Personas document (PD-02, `docs/personas.md`), the
use cases defined in the Use Cases document (PD-03, `docs/use-cases.md`),
the scope approved in the v0.1 Scope and Non-Goals document (PD-04,
`docs/scope.md`), and the functional requirements defined in the Functional
Requirements document (PD-05, `docs/functional-requirements.md`).

The requirements in this document describe the required quality
characteristics of the platform's behavior — performance, capacity,
reliability, availability, security, operability, maintainability,
testability, documentation, retention, portability, and resource
efficiency — rather than the product behavior itself, which is defined by
PD-05.

This document does not define:

- functional behavior — what the platform accepts, classifies, transforms,
  detects, or presents (PD-05);
- acceptance criteria — concrete pass/fail demonstrations per requirement
  (future AC-### artifact);
- architecture or design — trust boundaries, authentication and
  authorization mechanisms, admission-control mechanisms, storage and
  resource sizing, transport-protection mechanisms, revision-identification
  mechanisms, or the identity of the reference environment.

No user interface, product API, CLI, internal data format, database,
queue, service, framework, deployment mechanism, cloud provider, or
observability product is assumed by any requirement.

## Conventions

- Identifiers use the stable NFR-### namespace defined in PC-015 and are
  never renumbered or reused.
- Each requirement describes one quality characteristic using normative
  "shall" wording, together with its traceability, its rationale, and a
  mechanism-neutral verification approach. No requirement selects a tool,
  protocol, or implementation mechanism.
- Per PC-015, every requirement traces to at least one product goal
  (PC-G-###) or principle (PC-P-###). Traceability to use cases (UC-###)
  and personas (PER-###) is recorded where the requirement is anchored to a
  specific use case or persona; engineering-facing requirements with no
  single anchoring use case trace to product goals and principles alone.
- Requirements are grouped into twelve category sections. Grouping is
  editorial; only the NFR-### identifiers are normative.
- Where a numeric target is stated, it is an approved v0.1 target. Where a
  measurement detail — sample size, percentile method, reference data
  volume, or test duration — is not fixed by this document, it is
  delegated to acceptance criteria, and the requirement says so explicitly.

## Definitions

- **Submission attempt** — an attempted delivery presented to the
  platform's admission boundary.
- **Admitted submission** — a submission attempt accepted through the
  applicable authorization and capacity admission controls into the
  defined functional intake.
- **Telemetry data-quality outcome** — one of the four outcomes defined by
  PD-05 for admitted submissions: valid, invalid, incomplete, or
  unsupported.
- **Admission and security outcome** — an outcome of the admission
  boundary itself, assessed before and independently of telemetry
  data-quality classification: unauthorized-submission rejection,
  over-capacity rejection, or capacity deferral.
- **Platform fault** — an internal processing, availability, integrity, or
  recovery failure of the platform, distinct from both telemetry
  data-quality outcomes and admission and security outcomes.
- **Recorded persistent state** — the stored representation of recorded
  artifacts and traceability links that the platform maintains across
  restarts of its execution environment.

The following clarifies how the admitted-submission concept relates to the
approved functional requirements, without modifying them:

- FR-004 through FR-014 apply to admitted submissions.
- Capacity and authorization rejection or deferral occurs before
  functional telemetry validation.
- Capacity outcomes and admission and security outcomes must never be
  represented as valid, invalid, incomplete, or unsupported telemetry.
- This is an interpretive clarification of "received at the defined
  intake" as used in the functional requirements document, not a
  modification of any approved functional requirement.
- The NFR-001 latency measurement begins when a submission is admitted
  into the defined intake, not when the submission attempt first reaches
  an external boundary.

## Section 1 — Performance

### NFR-001 — Intake-to-alert processing latency

For an admitted submission whose normalized event produces a matching
detection result, the platform shall make the resulting alert available
for review no more than 60 seconds after the submission is admitted into
the defined intake, while operating within the approved capacity envelope
(NFR-003).

**Traceability:** UC-002, UC-003; FR-001, FR-027, FR-030; PC-G-005,
PC-G-008; PC-P-006.

**Rationale:** A bounded, demonstrable event-to-alert time makes the
end-to-end workflow credible without a real-time claim. Measuring from
admission, rather than from the initial submission attempt, keeps the
target independent of whatever authentication or capacity handling
precedes admission.

**Verification:** Measured elapsed time from admission to alert
availability for representative scenario events, under representative
loads up to and including the approved sustained capacity envelope
(NFR-003). Sample size, percentile method, exact load profile, and
measurement procedure are defined by acceptance criteria.

### NFR-002 — Review and retrieval responsiveness

The platform shall make one requested recorded artifact — a validation
outcome, an alert, or an available artifact of the minimum evidence set —
available to the requesting user within 5 seconds per retrieval, at a
documented reference data volume representative of operation within the
approved capacity envelope.

**Traceability:** UC-001–UC-003; PER-001–PER-003; FR-013, FR-030, FR-031;
PC-G-006.

**Rationale:** Verification and investigation are interactive; the target
bounds retrieval time without assuming a presentation mechanism or a
specific accumulated data size.

**Verification:** Measured retrieval time per artifact class at the
documented reference data volume, while the platform operates under
representative loads up to and including the approved sustained capacity
envelope (NFR-003). Sample size, percentile method, exact load profile,
the reference data volume, accumulation duration, artifact mix, warm or
cold retrieval conditions, and measurement procedure are defined by
acceptance criteria.

## Section 2 — Capacity and scalability

### NFR-003 — Documented v0.1 capacity envelope

The platform shall document a supported v0.1 capacity envelope of 10
submissions per second sustained at the defined intake, and shall satisfy
NFR-001, NFR-002, NFR-005 through NFR-008, and NFR-035 while operating
within that envelope.

**Traceability:** UC-001; FR-001, FR-004; PC-C-001; PC-P-006; PC-G-008.

**Rationale:** A stated, modest envelope is what makes every dependent
quality claim verifiable.

**Verification:** Sustained processing at the documented rate with the
dependent requirements' observations holding. Sustained-test duration and
burst profile are defined by acceptance criteria.

### NFR-004 — Defined and visible behavior beyond supported capacity

When a submission attempt cannot be admitted because the offered load
exceeds the approved capacity envelope, the platform shall visibly
identify the attempt as rejected or deferred because of capacity. This is
an admission and security outcome: it shall never be represented as a
valid, invalid, incomplete, or unsupported telemetry outcome, and no
admitted submission shall be silently lost.

**Traceability:** UC-001; FR-003, FR-005, FR-011; PC-P-004; PC-G-002.

**Rationale:** Overload handling must be explicit and must not enter the
telemetry data-quality classification, which describes the content of
admitted telemetry, not offered load.

**Verification:** Offered load above the envelope; observation of
capacity-identified outcomes distinct from data-quality outcomes;
reconciliation showing no silent loss of admitted submissions.

## Section 3 — Reliability and data integrity

### NFR-005 — Deterministic processing outcomes

Given the same submission content, the same documented classification
criteria, the same documented normalized representation, and the same
detection definitions, the platform shall produce the same validation
outcome, the same normalized event content, and the same detection
results.

**Traceability:** UC-001, UC-002; FR-005, FR-015, FR-023; PC-G-004;
PC-P-003.

**Rationale:** Explainability requires repeatability; achievable because
v0.1 detection is single-event and stateless.

**Verification:** Repeated processing of fixture submissions with outcome
comparison across runs.

### NFR-006 — No silent loss or corruption of recorded artifacts

The platform shall not silently lose or silently corrupt any recorded
product artifact, including submission records, source-event evidence,
validation outcomes, normalized events, identifiable detection-definition
revisions, match reasons, alerts, evidence inventories, and traceability
links. Any loss, corruption, or unavailability of a recorded artifact
shall be observable, and the platform shall never represent a missing or
corrupted artifact as present and intact. This requirement applies in
full to ordinary interruption and restart of the platform's execution
environment; it does not require continued artifact accessibility during
an execution-environment outage itself.

**Traceability:** UC-001, UC-003; FR-003, FR-011, FR-031, FR-032, FR-035;
PC-P-004; PC-G-006, PC-G-007; NFR-025.

**Rationale:** Loss must be visible, not impossible, while restart and
recovery behavior remain governed separately by NFR-009 through NFR-011.

**Verification:** Artifact reconciliation across processing and restart;
demonstration that induced unavailability or corruption is surfaced, never
masked.

### NFR-007 — Traceability-link integrity

Traceability links shall remain mutually consistent with the artifacts
they associate: the platform shall not present a link to a nonexistent
artifact as intact, and shall not present an artifact chain containing a
broken link as complete.

**Traceability:** UC-003; FR-033–FR-035; PC-G-007; PC-P-005.

**Rationale:** Silent link rot turns the evidence chain into an
unverifiable claim.

**Verification:** Chain-walk of alert links against stored artifacts,
including after restart.

### NFR-008 — Per-submission fault isolation

A platform fault encountered while processing one submission shall not
prevent the processing of other submissions and shall not corrupt
artifacts recorded for other submissions.

**Traceability:** UC-001; FR-004, FR-023; PC-G-008; PC-P-006.

**Rationale:** One pathological event must not halt or corrupt the
pipeline.

**Verification:** Induced fault on one fixture submission; confirmation
that other submissions process normally.

## Section 4 — Availability and recovery

### NFR-009 — Recovery-time objective

Where the platform's recorded persistent state remains available, the
platform shall be restorable to full documented operation within 15
minutes after an interruption or failure of its execution environment, by
following the documented recovery procedure alone. Restoration after
complete loss or destruction of recorded persistent state is outside
v0.1; backup, off-site recovery, and restoration after total
persistent-state loss are deferred to later releases.

**Traceability:** PC-009 item 9; PC-G-008; PC-P-006; NFR-030.

**Rationale:** A recovery-time objective is measurable and honest for a
single-environment portfolio system. No high availability, multi-region
operation, or disaster-recovery infrastructure is required or implied.

**Verification:** Timed execution of the documented recovery procedure
from a stopped or failed state, with recorded persistent state intact.

### NFR-010 — Durability of recorded artifacts across restart

Every recorded product artifact within the scope of NFR-006 shall survive
an orderly shutdown and restart of the platform without loss or
corruption.

**Traceability:** UC-003; FR-031, FR-033; PC-G-006, PC-G-007; NFR-006.

**Rationale:** Investigation may occur long after processing; evidence
must outlive restarts.

**Verification:** Artifact and link comparison before and after restart.

### NFR-011 — Determinable state of interrupted submissions

After recovery from an interruption, the platform shall make it
determinable, for every submission admitted at the defined intake before
the interruption, whether its processing completed; any submission whose
processing did not complete shall be visibly identifiable rather than
silently absent from results.

**Traceability:** UC-001; FR-003, FR-011; PC-P-004; PC-G-002.

**Rationale:** This fixes visibility of incomplete processing, not
reprocessing semantics, which belong to architecture.

**Verification:** Interruption induced mid-processing; post-recovery
reconciliation of submission records against completion states.

## Section 5 — Security and platform self-protection

### NFR-012 — Authenticated and authorized product access

The platform shall require authentication and authorization for access to
product data and product functions — telemetry, normalized events,
detection definitions, match reasons, alerts, evidence, and validation
outcomes — exposed through any product-exposed access path or interface,
and shall deny access to unauthenticated or unauthorized parties. v0.1
defines a single authenticated product trust level; persona-differentiated
authorization is not required.

**Traceability:** PC-P-008; PC-G-008; FR-013, FR-022, FR-030, FR-031.

**Rationale:** The platform holds security-sensitive telemetry; the
requirement binds the product boundary only. Trust-boundary definition,
identity mechanism, authorization mechanism, and deployment controls are
delegated to threat modeling and architecture.

**Verification:** Access attempts without and with invalid credentials are
denied at every product-exposed access path.

### NFR-013 — Authorized submission at the defined intake

The platform shall admit submission attempts at the defined intake only
from an authorized submission path or submitter. Rejection of an
unauthorized submission attempt occurs at admission, before telemetry
validation; it is an admission and security outcome and shall never be
represented as a valid, invalid, incomplete, or unsupported telemetry
outcome.

**Traceability:** UC-001; FR-001, FR-032; PC-P-008; PC-G-006.

**Rationale:** Evidence provenance depends on the intake not accepting
forged telemetry, while the outcome taxonomy stays clean: telemetry
data-quality outcomes describe admitted telemetry only.

**Verification:** Unauthorized submission attempts are rejected at
admission, observable as admission and security outcomes, and absent from
data-quality outcome records.

### NFR-014 — Least-privilege operation

The platform shall be operable with a documented set of privileges
limited to those its documented function requires, and its documentation
shall state the privileges required and why.

**Traceability:** PC-P-008; PC-G-008.

**Verification:** Review of the documented privilege set; demonstration
that operation succeeds with only that set.

### NFR-015 — Platform-managed secret handling

Platform-managed secrets — credentials, keys, and tokens used to operate
or protect the platform — shall not be stored in the source repository
and shall not be exposed in diagnostic output, alerts, evidence, or other
recorded product artifacts. Diagnostic information shall not copy
sensitive source-telemetry content beyond what its diagnostic purpose
requires. This requirement governs platform-managed secrets only; source
telemetry and source-event evidence may themselves contain sensitive
source data, and their evidentiary fidelity remains governed by FR-032.

**Traceability:** PC-P-008; PC-G-008; FR-032; NFR-021.

**Rationale:** This separates platform secret hygiene, which binds now,
from source-data sensitivity, which is a threat-model and security-design
matter.

**Verification:** Repository and output inspection for platform-managed
secret material; review of diagnostics for unnecessary copying of
sensitive source content; review of the documented secret-supply
procedure.

### NFR-016 — Protection of product data crossing external boundaries

Where product data in the platform's custody traverses a communication
path outside its execution environment, the platform shall protect that
data against unauthorized disclosure and undetected modification in
transit.

**Traceability:** PC-P-008; PC-G-006; FR-032.

**Rationale:** Conditional wording assumes no boundary or network topology
must exist; where one does, unprotected transit would contradict
PC-P-008.

**Verification:** Engineering evidence that every documented external
communication path carries the stated protection.

### NFR-017 — Evidence integrity protection

The platform shall protect recorded artifacts of the minimum evidence set
against undetected modification: modification outside the platform's
documented behavior shall be either prevented or detectable.

**Traceability:** UC-003; FR-031, FR-032; PC-G-006; PC-P-008.

**Verification:** Demonstration that out-of-band modification of a stored
artifact is prevented or surfaced as an integrity failure.

### NFR-018 — Dependency and vulnerability management

The project shall maintain a documented, repeatable practice for
identifying its third-party dependencies and for discovering and
responding to known vulnerabilities in them.

**Traceability:** PC-G-008; PC-P-008.

**Rationale:** No numerical response service level, tool, cadence, or CI
system is prescribed.

**Verification:** Review of the documented practice and evidence of at
least one executed cycle.

### NFR-019 — Visibility of security-relevant platform events

The platform shall make security-relevant events at its own boundaries —
failed authentication, denied access, and rejected unauthorized submission
attempts — observable in its diagnostic information.

**Traceability:** PC-P-008; PC-G-008; NFR-013, NFR-021.

**Verification:** Induced authentication and authorization failures
appear in structured diagnostics.

## Section 6 — Operability and observability

### NFR-020 — Platform health visibility

The platform shall make it determinable, through a documented means,
whether it is operational and able to receive and process submissions.

**Traceability:** PC-009 item 9; PC-G-008.

**Rationale:** This covers the platform only; source-health and
delivery-gap visibility remain excluded from v0.1.

**Verification:** Health determination demonstrated in operational and
deliberately non-operational states.

### NFR-021 — Structured and correlatable diagnostics

The platform shall emit diagnostic information for processing activity
and faults in a structured form that, where applicable, references the
identifier of the affected submission or artifact.

**Traceability:** PC-G-008; FR-003; PC-P-005; NFR-015.

**Verification:** Inspection of diagnostics for structure and identifier
correlation during normal and fault processing.

### NFR-022 — Distinct reporting of outcome families

The platform shall report outcomes belonging to three distinct families —
telemetry data-quality outcomes, admission and security outcomes, and
platform faults — distinguishably from one another. No outcome from one
family shall be represented as an outcome from another family.

**Traceability:** UC-001; FR-005, FR-012; PC-P-004; NFR-004, NFR-013.

**Rationale:** Each family has a different cause and a different meaning
to the reviewing persona; conflating them would misinform the platform
engineer and the analyst about what actually happened.

**Verification:** Induced platform fault, fixture data-quality
submissions, and rejected or deferred admission attempts produce
distinguishable reported outcomes.

## Section 7 — Maintainability and change safety

### NFR-023 — Change isolation across workflow stages

A change confined to the documented behavior of one workflow stage —
intake, validation, normalization, detection, alert generation, or
evidence and traceability provision — shall not require changes to
unrelated workflow stages, except where a documented inter-stage contract
is intentionally changed.

**Traceability:** PC-G-008, PC-G-010; PC-P-007; PC-005.

**Rationale:** This states a change-isolation quality; how the separation
is achieved is an architecture decision.

**Verification:** Engineering evidence: a representative confined change
demonstrating no ripple beyond an intentionally changed contract.

### NFR-024 — Automated regression validation

Changes to the platform shall be validated by an automated, repeatable
test suite that exercises the documented v0.1 workflow behavior —
including the four telemetry data-quality outcomes and the three
detection scenarios — before the change is adopted.

**Traceability:** PC-009 item 10; PC-G-008; FR-005, FR-024–FR-026.

**Verification:** Existence and execution evidence of the suite against a
candidate change.

### NFR-025 — Identifiable detection-definition revisions

The platform shall associate every alert and recorded match reason with
an identifiable revision of the detection definition that produced it,
such that a later change to a detection definition does not alter what a
previously generated alert is traceable to.

**Traceability:** UC-002, UC-003; FR-020, FR-028, FR-033; PC-G-007.

**Verification:** Definition change applied; pre-change alert still
resolves to the revision that matched.

### NFR-026 — Identifiable normalized-representation revision

The platform shall make it determinable which revision of the documented
normalized representation each normalized event conforms to, and changes
to the documented representation shall be recorded in its documentation.

**Traceability:** UC-002; FR-015, FR-016; PC-G-003, PC-G-007.

**Verification:** Representation revision determinable for stored
normalized events; documented change history.

## Section 8 — Testability and reproducibility

### NFR-027 — Deterministic test fixtures for documented outcomes

The project shall provide deterministic fixture submissions that
reproducibly exercise each of the four telemetry data-quality outcomes and
each of the three detection scenarios, producing the documented product
behavior on every run.

**Traceability:** UC-001, UC-002; FR-005–FR-010, FR-024–FR-026; PC-G-008,
PC-G-009; NFR-005.

**Verification:** Repeated fixture runs with identical outcomes.

### NFR-028 — Reproducible end-to-end scenario demonstration

The platform shall support reproducing, from documented inputs and
documented steps alone, the complete workflow for each approved detection
scenario: from submission at the defined intake through validation,
normalization, detection, alert generation, and inspection of the full
minimum evidence set.

**Traceability:** UC-001–UC-003; PC-005; PC-009 items 1–8; PC-G-009.

**Verification:** Independent execution of the documented demonstration
procedure for all three scenarios.

### NFR-029 — Verifiable end-to-end traceability

The platform shall support automated verification, for any generated
alert, of whether every required traceability link exists and resolves to
the correct recorded artifact through to the source audit event. When a
required link is missing, broken, or resolves incorrectly, the
verification shall fail visibly and identify the affected link or
relationship.

**Traceability:** UC-003; FR-033, FR-034, FR-035; PC-G-007; PC-P-005.

**Rationale:** A verification mechanism that could only confirm success
would leave a broken evidence chain undetectable. Visible, specific
failure reporting is consistent with FR-035, under which a broken or
incomplete evidence chain must never be represented as complete.

**Verification:** Automated chain-walk over generated alerts in test
execution, exercising both intact and deliberately broken or incomplete
traceability chains to confirm visible, specific failure reporting.

## Section 9 — Documentation and usability

### NFR-030 — Complete v0.1 documentation set

The project shall provide documentation sufficient for a competent third
party, without access to the authors, to: set up the platform; operate
it, including start, stop, restart, health determination, and recovery;
review detection definitions and their documented conditions; and perform
the investigation workflow.

**Traceability:** PC-009 item 9; PC-G-008, PC-G-009; UC-002, UC-003;
NFR-009, NFR-020.

**Verification:** Documentation walkthrough by a party not involved in
authoring.

### NFR-031 — Self-contained understandability of product information

Validation reasons, match reasons, alert explanations, and
evidence-inventory information shall be expressed in terms of the
product's documented concepts — the supported source-event form, the
documented normalized representation, and the documented detection
conditions — such that each persona's success criteria can be met without
inspecting the platform's internal implementation.

**Traceability:** UC-001–UC-003; PER-001–PER-003; FR-012, FR-028, FR-029,
FR-031; PC-G-005; PC-008.

**Verification:** Structured persona walkthrough mapping each reason or
explanation element to documented product concepts.

## Section 10 — Retention and lifecycle

### NFR-032 — Deployment-lifetime retention of recorded artifacts

The platform shall retain every recorded submission, validation outcome,
and artifact of the minimum evidence set, retrievable per FR-013 and
FR-031, for the lifetime of the v0.1 deployment. Automatic deletion,
archival, and retention-policy management are not required in v0.1. This
retention model operates subject to documented storage limits and the
visible resource-exhaustion behavior required by NFR-036.

**Traceability:** UC-003; FR-013, FR-031; PC-G-006; PC-P-006; NFR-035,
NFR-036.

**Rationale:** Evidence cannot lawfully vanish before investigation, and
no lifecycle machinery beyond this is invented.

**Verification:** Retrieval of early-recorded artifacts late in a
sustained operation period.

## Section 11 — Portability and deployment

### NFR-033 — Reproducible deployment into one reference environment

The platform shall be deployable into a clean instance of exactly one
documented reference environment by following the documented setup
procedure alone, yielding an operational platform capable of the
demonstration required by NFR-028.

**Traceability:** PC-009 item 9; PC-G-008, PC-G-009; NFR-030.

**Rationale:** The reference environment's identity is documented later;
no platform or topology is chosen here.

**Verification:** Clean-environment deployment executed from
documentation.

### NFR-034 — Avoidance of unnecessary provider lock-in

The platform's documented v0.1 workflow shall not require a capability
obtainable only from a single commercial provider's proprietary service,
unless a documented product constraint justifies the dependency.

**Traceability:** PC-G-010; PC-P-007; PC-C-005.

**Verification:** Review of the dependency inventory against the
criterion.

## Section 12 — Resource efficiency

### NFR-035 — Bounded non-retention resource consumption

While operating within the approved capacity envelope, the platform's
non-retention resource consumption — memory, execution resources,
handles, and temporary processing state — shall remain bounded and shall
not grow indefinitely during sustained operation. Persistent storage may
grow in proportion to retained recorded artifacts under the approved
deployment-lifetime retention model.

**Traceability:** PC-P-006; PC-C-001; PC-G-008; NFR-003, NFR-032.

**Rationale:** This separates legitimate retention-driven storage growth
from leaks and unbounded transient state.

**Verification:** Sustained operation at the envelope with observation of
non-retention resource stability over a defined duration.

### NFR-036 — Visible behavior at resource exhaustion

When a documented resource limit is reached, the platform shall exhibit
its documented behavior, the condition shall be observable through its
diagnostics, no recorded artifact shall be silently lost, and the
platform shall not silently continue operating in a corrupted state.

**Traceability:** PC-P-004; PC-G-008; NFR-004, NFR-006, NFR-021, NFR-032.

**Verification:** Induced resource-limit condition; observation of
documented behavior, diagnostics, and artifact reconciliation.

## Approved-target table

| Item | Approved value or boundary | Scope and conditions | Deferred to acceptance criteria |
| --- | --- | --- | --- |
| Intake-to-alert latency (NFR-001) | 60 seconds or less | Measured from admission into the defined intake, for an admitted submission with a matching detection result, within the capacity envelope | Sample size, percentile method, measurement protocol |
| Retrieval responsiveness (NFR-002) | 5 seconds or less per retrieval | One validation outcome, alert, or evidence artifact, at a documented reference data volume within the capacity envelope | Reference data volume, accumulation duration, artifact mix, sampling and percentile method, warm or cold conditions, measurement procedure |
| Capacity envelope (NFR-003) | 10 submissions per second, sustained | Dependent requirements NFR-001, NFR-002, NFR-005 through NFR-008, and NFR-035 must hold within it | Sustained-test duration, burst profile |
| Recovery-time objective (NFR-009) | 15 minutes or less | Documented recovery procedure alone; recorded persistent state remains available; no high availability, multi-region operation, or disaster recovery | Recovery demonstration script |
| Retention (NFR-032) | Deployment-lifetime; no automatic deletion, archival, or policy management | Subject to documented storage limits and NFR-036 behavior | Storage sizing |
| Reference environments (NFR-033) | Exactly one documented reference environment | Clean-instance deployment from documentation alone | Environment identity |
| Dependency and vulnerability practice (NFR-018) | Qualitative, documented, repeatable; no numeric response service level | — | Tooling and cadence |
| Provider lock-in (NFR-034) | Avoidance included | Exception requires a documented product constraint | Dependency review |
| Authorization depth (NFR-012) | Single authenticated product trust level | Deny unauthenticated or unauthorized access; no persona-specific authorization or role-based access control in Phase 0 | Trust boundaries, identity and authorization mechanisms |
| Intake authorization (NFR-013) | Authorized submission path or submitter only | Rejection is an admission and security outcome, assessed before validation | Authentication mechanism, delivery protocol |

## Traceability summary

| NFR | Use cases / personas | Product goals / principles | Related requirements |
| --- | --- | --- | --- |
| NFR-001 | UC-002, UC-003 | PC-G-005, PC-G-008; PC-P-006 | FR-001, FR-027, FR-030 |
| NFR-002 | UC-001–UC-003; PER-001–PER-003 | PC-G-006 | FR-013, FR-030, FR-031 |
| NFR-003 | UC-001 | PC-C-001; PC-P-006; PC-G-008 | FR-001, FR-004 |
| NFR-004 | UC-001 | PC-P-004; PC-G-002 | FR-003, FR-005, FR-011 |
| NFR-005 | UC-001, UC-002 | PC-G-004; PC-P-003 | FR-005, FR-015, FR-023 |
| NFR-006 | UC-001, UC-003 | PC-P-004; PC-G-006, PC-G-007 | FR-003, FR-011, FR-031, FR-032, FR-035; NFR-025 |
| NFR-007 | UC-003 | PC-G-007; PC-P-005 | FR-033–FR-035 |
| NFR-008 | UC-001 | PC-G-008; PC-P-006 | FR-004, FR-023 |
| NFR-009 | — | PC-G-008; PC-P-006 | NFR-030 |
| NFR-010 | UC-003 | PC-G-006, PC-G-007 | FR-031, FR-033; NFR-006 |
| NFR-011 | UC-001 | PC-P-004; PC-G-002 | FR-003, FR-011 |
| NFR-012 | — | PC-P-008; PC-G-008 | FR-013, FR-022, FR-030, FR-031 |
| NFR-013 | UC-001 | PC-P-008; PC-G-006 | FR-001, FR-032 |
| NFR-014 | — | PC-P-008; PC-G-008 | — |
| NFR-015 | — | PC-P-008; PC-G-008 | FR-032 |
| NFR-016 | — | PC-P-008; PC-G-006 | FR-032 |
| NFR-017 | UC-003 | PC-G-006; PC-P-008 | FR-031, FR-032 |
| NFR-018 | — | PC-G-008; PC-P-008 | — |
| NFR-019 | — | PC-P-008; PC-G-008 | — |
| NFR-020 | — | PC-G-008 | — |
| NFR-021 | — | PC-G-008; PC-P-005 | FR-003 |
| NFR-022 | UC-001 | PC-P-004 | FR-005, FR-012 |
| NFR-023 | — | PC-G-008, PC-G-010; PC-P-007 | — |
| NFR-024 | — | PC-G-008 | FR-005, FR-024–FR-026 |
| NFR-025 | UC-002, UC-003 | PC-G-007 | FR-020, FR-028, FR-033 |
| NFR-026 | UC-002 | PC-G-003, PC-G-007 | FR-015, FR-016 |
| NFR-027 | UC-001, UC-002 | PC-G-008, PC-G-009 | FR-005–FR-010, FR-024–FR-026 |
| NFR-028 | UC-001–UC-003 | PC-G-009 | — |
| NFR-029 | UC-003 | PC-G-007; PC-P-005 | FR-033, FR-034, FR-035 |
| NFR-030 | UC-002, UC-003 | PC-G-008, PC-G-009 | — |
| NFR-031 | UC-001–UC-003; PER-001–PER-003 | PC-G-005 | FR-012, FR-028, FR-029, FR-031 |
| NFR-032 | UC-003 | PC-G-006; PC-P-006 | FR-013, FR-031 |
| NFR-033 | — | PC-G-008, PC-G-009 | — |
| NFR-034 | — | PC-G-010; PC-P-007 | — |
| NFR-035 | — | PC-P-006; PC-C-001; PC-G-008 | — |
| NFR-036 | — | PC-P-004; PC-G-008 | — |

## Excluded and deferred candidates

The following candidate requirements were considered and are excluded
from v0.1, with their basis:

1. **Percentage-uptime availability objective** — superseded by the
   recovery-time objective (NFR-009).
2. **High availability, multi-region operation, and disaster-recovery
   infrastructure** — not required or implied by NFR-009.
3. **Backup and off-site recovery, and restoration after complete loss or
   destruction of recorded persistent state** — deferred to later
   releases; no backup or disaster-recovery requirement is added in
   v0.1.
4. **Automatic deletion, archival, and retention-policy management** —
   not required by NFR-032.
5. **Persona-differentiated authorization and any role-based access
   control model** — excluded from Phase 0 by NFR-012.
6. **A second reference environment** — excluded by NFR-033.
7. **A numeric vulnerability-response service level** — excluded by
   NFR-018.
8. **Automatic or horizontal scaling** — not required by NFR-003 or
   NFR-004.
9. **Compliance frameworks and certifications** — no charter basis.
10. **Specific encryption, hashing, or transport mechanisms named at the
    requirement level** — mechanism selection is delegated to
    architecture (NFR-016, NFR-017).
11. **A dedicated penetration-testing requirement** — no charter basis
    for v0.1.
12. **A platform self-monitoring or meta-alerting subsystem** — beyond
    the health and diagnostic visibility required by NFR-020 and
    NFR-021.
13. **Source-health or missing-delivery visibility** — excluded by PD-04
    scope decision 6; also excluded from NFR-020.
14. **Alert-notification latency**, as distinct from alert availability
    — no charter basis; NFR-001 governs availability only.
15. **Internationalization and accessibility requirements** — no charter
    basis for v0.1.
16. **A numeric clock-accuracy or timestamp-synchronization requirement**
    — no charter basis; ordering and timing behavior beyond NFR-001 and
    NFR-002 is not defined.

## Matters delegated to acceptance criteria, threat modeling, architecture, or later engineering work

- **Acceptance criteria:** sample sizes, percentile methods, and
  measurement protocols for NFR-001 and NFR-002; the reference data
  volume, accumulation duration, artifact mix, and warm or cold retrieval
  conditions for NFR-002; sustained-test duration and burst profile for
  NFR-003; the recovery demonstration script for NFR-009; storage sizing
  for NFR-032; the documented resource limits exercised for NFR-035 and
  NFR-036.
- **Threat modeling:** trust boundaries, identity and authorization
  mechanisms, and the admission-control mechanism for NFR-012 and
  NFR-013; privacy, masking, and sensitive-data handling for audit
  subjects and request content carried in source telemetry.
- **Architecture:** transport-protection mechanisms for NFR-016; the
  mechanism achieving change isolation for NFR-023; revision-
  identification mechanisms for NFR-025 and NFR-026; the identity of the
  reference environment for NFR-033; the storage and resource-limit
  values underlying NFR-035 and NFR-036.
- **Later engineering work:** the dependency-management and
  vulnerability-discovery tooling and cadence for NFR-018; the automated
  test-suite framework and continuous-integration tooling for NFR-024.

## Unvalidated assumptions

1. A sustained rate of 10 submissions per second is representative of the
   selected source family in a realistically sized environment and
   credible for portfolio review.
2. The 60-second and 5-second targets are achievable without forcing
   premature architecture commitments.
3. Deterministic processing is achievable because detection semantics are
   single-event and stateless.
4. Deployment-lifetime retention stays within practical storage bounds
   over realistic v0.1 operation periods, guarded by NFR-035 and
   NFR-036.
5. An authorized submission path can be defined for whichever delivery
   mechanism architecture later selects.
6. A competent third party can reproduce deployment and the
   three-scenario demonstration in the single reference environment from
   documentation alone.
7. A recovery-time objective, conditioned on surviving persistent state
   and without an uptime percentage, sufficiently demonstrates
   production-oriented operation for portfolio review.

These assumptions must be validated, refined, or rejected during the
remaining Phase 0 definition work.
