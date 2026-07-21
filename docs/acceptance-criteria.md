# Cloud-Native Security Telemetry and Detection Platform — v0.1 Acceptance Criteria

| Field | Value |
| --- | --- |
| Document ID | PD-07 |
| Version | 0.1 |
| Status | Approved - Phase 0 baseline |
| Phase | Phase 0 — Product Definition and Requirements |

## Purpose and scope

This document defines the v0.1 acceptance criteria for the platform described
in the Product Charter (PD-01, `docs/product.md`), for the personas defined in
the Personas document (PD-02, `docs/personas.md`), the use cases defined in
the Use Cases document (PD-03, `docs/use-cases.md`), the scope approved in the
v0.1 Scope and Non-Goals document (PD-04, `docs/scope.md`), the functional
requirements defined in the Functional Requirements document (PD-05,
`docs/functional-requirements.md`), and the non-functional requirements
defined in the Non-Functional Requirements document (PD-06,
`docs/non-functional-requirements.md`).

Acceptance criteria define how v0.1 will be verified as satisfying its
approved functional and non-functional behavior. Each criterion is
independently passable or fail-able and traces to at least one functional or
non-functional requirement, per PC-015.

This document does not define architecture, threat models, implementation
plans, test automation design, or executable test code. No technology,
protocol, storage mechanism, presentation surface, or implementation
mechanism is selected or assumed by any criterion.

Criteria are grouped into ten categories following the PC-005 workflow order,
consistent with the grouping used by PD-05 and PD-06. Grouping is editorial;
only the AC-### identifiers are normative.

## Conventions

- Identifiers use the stable AC-### namespace defined in PC-015 and are never
  renumbered or reused.
- Each criterion states a criterion type, preconditions or test context, an
  observable action or condition, pass conditions, required evidence, and
  traceability to UC-###, FR-###, and NFR-### as applicable.
- A criterion may trace to several functional and non-functional requirements
  through shared traceability rather than restating each requirement's
  normative text.
- Criterion types used in this document: Data-Quality, Detection,
  Documentation, Evidence, Explainability, Functional, Maintainability,
  Operability, Performance, Recovery, Reliability, Security.

### Required evidence

Required evidence describes what must be observable or reviewable to judge a
criterion passed or failed. It does not, by itself, require the platform to
create or persist a new product artifact, log, record, endpoint, or interface
beyond what an already-approved functional or non-functional requirement
requires. Acceptable evidence sources include: product artifacts already
required by an approved requirement; structured diagnostics already required
by NFR-021; test-harness observations and measurements taken during
acceptance testing; documentation review; and engineering inspection. Where
required evidence for a criterion refers to a recorded or reported product
artifact, that artifact is required only because an approved functional or
non-functional requirement already mandates it.

## Definitions

- **Boundary-fixture set** — the shared set of four fixtures used by AC-003
  through AC-006 to demonstrate the documented validation precedence order
  and mutual exclusivity of the four data-quality outcomes:
  1. an event from an explicitly unsupported source-event variant that also
     contains malformed content;
  2. an event attributable to the supported source family and form but
     structurally malformed;
  3. a supported, parseable, structurally valid event missing required
     information;
  4. a supported, parseable, structurally valid, complete event.

  Each fixture in the set receives exactly one data-quality outcome, assigned
  according to the documented precedence order (unsupported, invalid,
  incomplete, valid) even where a fixture superficially qualifies for more
  than one outcome.
- **Reference dataset** — the documented accumulated dataset used by AC-021:
  at least 10,000 accumulated admitted submissions and at least 300 generated
  alerts, representing all four data-quality outcomes and containing
  artifacts produced by all three approved detection scenarios.
- **Acceptance observation period** — the documented window, at least 30
  minutes, over which AC-023 samples resource usage at regular documented
  intervals. It may overlap the AC-022 sustained-capacity run only where the
  resulting evidence for each acceptance concern remains independently
  interpretable; this is a qualitative acceptance-planning constraint, not a
  fixed overlap rule.

## Section A — Admission and Security Outcomes

### AC-001 — Authorized submission admitted and recorded

**Type:** Security / Functional.

**Context:** An authorized submission path presents one Kubernetes audit-event
submission attempt at the defined intake, within the approved capacity
envelope.

**Action or condition:** The attempt is presented to the defined intake.

**Pass conditions:** The submission is admitted — not rejected as unauthorized
or over-capacity; it is associated with an identification that unambiguously
distinguishes it from every other admitted submission; the admission itself
is never represented as a telemetry data-quality outcome.

**Required evidence:** Test execution evidence that the admitted submission
carries a unique, stable identification as required by FR-003; confirmation
that no two admitted submissions in the sample share an identification.

**Traceability:** UC-001; FR-001, FR-003; NFR-013 (positive case); PER-003.

### AC-002 — Admission and security-outcome visibility: unauthorized and over-capacity attempts

**Type:** Security.

**Context:** Branch (a) — a submission attempt is presented from a path or
submitter not recognized as authorized. Branch (b) — a submission attempt is
presented while offered load exceeds the documented 10 submissions per second
capacity envelope (NFR-003).

**Action or condition:** Present each attempt and observe how the platform
identifies the resulting outcome. The required visibility may be demonstrated
by a product outcome, structured diagnostic information, or another
documented observable mechanism selected later by architecture. This
criterion does not require a persisted rejection or deferral product record.

**Pass conditions:** For branch (a), the unauthorized attempt is visibly
identified as an admission and security outcome and is never represented as a
valid, invalid, incomplete, or unsupported telemetry outcome. For branch (b),
the over-capacity attempt or attempts are visibly identified as an admission
and security outcome — capacity rejection or capacity deferral — likewise
never represented as a data-quality outcome; no admitted submission is
silently lost during the over-capacity condition.

**Required evidence:** Captured observation — via a product outcome,
structured diagnostic output, or another documented observable mechanism — of
each attempt's identified outcome, for both branches; a test-harness
reconciliation of offered, admitted, and rejected or deferred attempts for
branch (b) confirming no admitted submission was silently lost.

**Traceability:** UC-001; NFR-004, NFR-013 (negative case), NFR-022;
PER-003.

## Section B — Telemetry Validation and Classification

The four criteria in this section share the boundary-fixture set defined
above, which proves the documented precedence order and mutual exclusivity of
the four data-quality outcomes.

### AC-003 — Valid submission classified and advanced to normalization

**Type:** Data-Quality.

**Context:** An admitted submission is supported, parseable, structurally
conformant to the documented source-event form (FR-002), and contains all
information required by FR-007. The test set includes the boundary-fixture
set.

**Action or condition:** The submission is validated at the defined intake.

**Pass conditions:** The submission receives exactly the valid outcome; no
stated deficiency is recorded; the outcome is available for review; the
submission proceeds to normalization. Boundary fixture 4 is classified valid
and only valid; boundary fixtures 1 through 3 are not classified valid.

**Required evidence:** The outcome recorded for the submission, as required
by FR-011; test execution evidence of downstream normalized-event
progression; boundary-fixture classification results confirming exactly one
outcome per fixture.

**Traceability:** UC-001; FR-002, FR-004, FR-005, FR-006, FR-007, FR-011,
FR-013, FR-014; PER-003.

### AC-004 — Invalid submission rejected with a stated reason

**Type:** Data-Quality.

**Context:** An admitted submission is attributable to the supported source
family and event form but cannot be parsed or violates a documented
structural constraint. The test set includes the boundary-fixture set;
boundary fixture 2 is specific to this criterion.

**Action or condition:** The submission is validated at the defined intake.

**Pass conditions:** The submission receives exactly the invalid outcome; a
stated reason identifies the specific nonconformance; the outcome and reason
are available for review; the submission does not proceed to normalization or
detection. Boundary fixture 2 is classified invalid and only invalid.

**Required evidence:** The outcome and stated reason recorded for the
submission, as required by FR-011 and FR-012; confirmation of no downstream
normalized event; the boundary-fixture 2 classification result.

**Traceability:** UC-001 (failure outcome); FR-002, FR-005, FR-006, FR-008,
FR-011, FR-012, FR-013, FR-014; PER-003.

### AC-005 — Incomplete submission flagged with the deficiency identified

**Type:** Data-Quality.

**Context:** An admitted submission is parseable and structurally valid but
lacks one or more items required by FR-007. The test set includes the
boundary-fixture set; boundary fixture 3 is specific to this criterion.

**Action or condition:** The submission is validated at the defined intake.

**Pass conditions:** The submission receives exactly the incomplete outcome;
the missing information item or items are stated; the outcome and reason are
available for review; the submission does not proceed to normalization or
detection. Boundary fixture 3 is classified incomplete and only incomplete —
not invalid, since it is structurally valid, and not unsupported.

**Required evidence:** The outcome and stated reason recorded for the
submission, as required by FR-011 and FR-012; confirmation of no downstream
normalized event or detection result; the boundary-fixture 3 classification
result.

**Traceability:** UC-001 (failure outcome); FR-005, FR-006, FR-009, FR-011,
FR-012, FR-013, FR-014; PER-003.

### AC-006 — Unsupported submission explicitly classified

**Type:** Data-Quality.

**Context:** An admitted submission is not attributable to the supported
Kubernetes audit-event source family, or belongs to a documented unsupported
variant, version, or form. The test set includes the boundary-fixture set;
boundary fixture 1 is specific to this criterion.

**Action or condition:** The submission is validated at the defined intake.

**Pass conditions:** The submission receives exactly the unsupported outcome;
a stated basis is recorded; the outcome and basis are available for review;
the submission does not proceed to normalization or detection; it is never
silently dropped nor silently accepted. Boundary fixture 1 is classified
unsupported and only unsupported, even though it also contains malformed
content, demonstrating that precedence assessment governs the outcome.

**Required evidence:** The outcome and stated basis recorded for the
submission, as required by FR-011 and FR-012; confirmation of no downstream
artifacts; the boundary-fixture 1 classification result.

**Traceability:** UC-001 (failure outcome); FR-002, FR-005, FR-006, FR-010,
FR-011, FR-012, FR-013, FR-014; PER-003.

## Section C — Normalization and Source Traceability

### AC-007 — One normalized event per valid submission, with documented core content

**Type:** Functional.

**Context:** A submission has been classified valid.

**Action or condition:** The submission is normalized.

**Pass conditions:** Exactly one normalized event is produced, conforming to
the documented normalized representation; it conveys at least the requesting
subject, the operation performed including the verb, the target resource
(kind, name, and namespace or subresource where applicable), the recorded
outcome information sufficient to determine success where required, and the
time of the request.

**Required evidence:** Test execution evidence comparing the normalized
event's content, field by field, against FR-017 and the documented
representation required by FR-016.

**Traceability:** UC-001, UC-002; FR-015, FR-016, FR-017; PER-002.

### AC-008 — Normalized-to-source traceability and audit-identity preservation

**Type:** Evidence.

**Context:** A normalized event has been produced from a valid submission.

**Action or condition:** The normalized event is inspected against its
source submission.

**Pass conditions:** The normalized event unambiguously references its
source submission; the source auditID and audit stage are preserved or
unambiguously referenced.

**Required evidence:** Test execution evidence of the normalized event's
source reference and preserved auditID and audit stage, cross-checked
against the original source event.

**Traceability:** UC-002, UC-003; FR-019; PER-002, PER-001.

## Section D — Detection Definitions and Scenario Evaluation

### AC-009 — Detection definitions documented, reviewable, and applied to every normalized event

**Type:** Documentation / Detection.

**Context:** The three approved detection definitions exist as identifiable
product content.

**Action or condition:** Each definition is reviewed, and a sample of
normalized events is evaluated.

**Pass conditions:** Each definition is identifiable and its documented
conditions are available for review; every normalized event in the test set
is evaluated individually against all three definitions, with no stateful,
aggregated, or baseline evaluation observed.

**Required evidence:** Documentation-review evidence confirming each
definition's identity and documented conditions are inspectable; test
execution evidence showing every normalized event in the sample was assessed
against all three definitions.

**Traceability:** UC-002; FR-020, FR-021, FR-022, FR-023; PER-002.

### AC-010 — Scenario 1 match: interactive exec request

**Type:** Detection.

**Context:** A normalized event records a request to the pods/exec
subresource whose recorded characteristics include standard-input streaming
or interactive terminal allocation.

**Action or condition:** The normalized event is evaluated against the
scenario 1 detection definition.

**Pass conditions:** A match is identified regardless of the recorded request
outcome; the match reason records every documented interactive characteristic
actually present in the request.

**Required evidence:** The match reason recorded for the fixture, as required
by FR-028, cross-checked against the source event's exec-request
characteristics.

**Traceability:** UC-002, UC-003; FR-018, FR-024, FR-028; PER-002, PER-001.

### AC-011 — Scenario 2 match: high-risk Pod creation

**Type:** Detection.

**Context:** A normalized event records a Pod-creation request whose
recorded outcome indicates success and whose Pod specification contains at
least one of the five documented high-risk characteristics.

**Action or condition:** The normalized event is evaluated against the
scenario 2 detection definition, alongside a negative-control fixture — a
successful creation with none of the five characteristics.

**Pass conditions:** A match is identified only for successful creations; the
match reason names the specific characteristic or characteristics present;
the negative-control fixture does not match.

**Required evidence:** The match reason recorded for the matching fixture;
the detection-evaluation result confirming that the negative-control fixture
did not match.

**Traceability:** UC-002, UC-003; FR-018, FR-025, FR-028; PER-002, PER-001.

### AC-012 — Scenario 3 match: cluster-admin ClusterRoleBinding grant

**Type:** Detection.

**Context:** A normalized event records successful creation of a
ClusterRoleBinding referencing cluster-admin.

**Action or condition:** The normalized event is evaluated against the
scenario 3 detection definition, alongside a negative-control fixture — a
modification of an existing cluster-admin binding, including a subject
addition.

**Pass conditions:** The creation event matches, with a reason naming the
binding and the referenced role; the negative-control modification fixture
does not match, regardless of whether it adds a subject; deletion of a
ClusterRoleBinding does not match.

**Required evidence:** The match reason recorded for the matching fixture;
the detection-evaluation result confirming that the negative-control fixture
did not match.

**Traceability:** UC-002, UC-003; FR-018, FR-026, FR-028; PER-002, PER-001.

### AC-013 — No alert from legitimate non-matching or non-valid telemetry

**Type:** Detection.

**Context:** Branch (a) — a valid, normalized, scenario-relevant event does
not satisfy any scenario's documented conditions (a legitimate non-match).
Branch (b) — submissions classified invalid, incomplete, or unsupported, as
covered by AC-004 through AC-006.

**Action or condition:** The fixtures are processed through validation,
normalization, and detection.

**Pass conditions:** For branch (a), no matching detection result is
produced, no alert is produced, and the normalized event remains recorded.
This criterion does not require that no internal non-match evaluation result
exists; persistence or product visibility of non-match evaluation results is
not required by v0.1. For branch (b), no alert in the sample is attributable
to any fixture in this set.

**Required evidence:** Test execution evidence that the branch (a) fixture
produced a normalized event with no detection match and no alert;
confirmation that no alert in the sample is attributable to any branch (b)
fixture.

**Traceability:** UC-001, UC-002; FR-014, FR-023, FR-027; PER-002.

## Section E — Alert Generation and Explainability

### AC-014 — Exactly one explainable alert per match, pinned to its detection-definition revision

**Type:** Explainability.

**Context:** A normalized event produced a matching detection result under
any of the three scenarios.

**Action or condition:** The matching detection result is observed, and a
subsequent change is applied to the matched detection definition.

**Pass conditions:** Exactly one alert is produced for the match; it conveys
the matched definition's identity, a statement of what was detected —
subject, operation, target, outcome, and time — the documented conditions
that matched, and the recorded match reason; the alert is available for
review; the alert resolves to the identifiable revision of the detection
definition that produced it, and a later change to that definition does not
alter what the alert resolves to.

**Required evidence:** The alert content checked against the required
elements of FR-029; a revision-resolution check performed before and after a
definition-content change.

**Traceability:** UC-002, UC-003; FR-027, FR-028, FR-029, FR-030; NFR-025;
PER-001, PER-002.

## Section F — Investigation Evidence

### AC-015 — Six-artifact evidence inventory with source-event fidelity and integrity

**Type:** Evidence.

**Context:** An alert has been generated.

**Action or condition:** The evidence inventory and its source-event artifact
are inspected; an out-of-band modification of a stored evidence artifact is
induced.

**Pass conditions:** The evidence inventory accounts for all six
minimum-evidence-set artifacts and states availability for each; every
available artifact is inspectable; the source-event evidence faithfully
preserves the evidentiary content and meaning of the received submission —
byte-for-byte identity is not required; no altered content is represented as
the original received source event; any later presentation or masking
behavior does not alter the underlying evidentiary meaning; out-of-band
modification of a stored evidence artifact is either prevented or detected.

**Required evidence:** The evidence inventory for a sample alert; a
content-and-meaning comparison of the source-event evidence against the
original received submission; test execution evidence of the induced
out-of-band modification showing prevention or detection.

**Traceability:** UC-002, UC-003; FR-031, FR-032; NFR-017; PER-001, PER-002.

### AC-016 — Alert-to-source traceability chain verification, intact and broken

**Type:** Evidence.

**Context:** Branch (a) — an alert with all traceability links intact.
Branch (b) — an alert with a deliberately induced missing or broken link.

**Action or condition:** The traceability links are followed from the alert
to the source audit event, and verification is performed for both branches.

**Pass conditions:** For branch (a), following the links from the alert
resolves correctly through the match reason, the detection definition, and
the normalized event to the source audit event, and verification reports the
chain intact. For branch (b), verification fails visibly, identifies the
specific affected link, and the evidence inventory does not represent the set
as complete.

**Required evidence:** Verification output for branch (a) showing correct
end-to-end resolution; verification output for branch (b) showing visible,
specific failure identification.

**Traceability:** UC-002, UC-003; FR-033, FR-034, FR-035; NFR-007, NFR-029;
PER-001, PER-002.

## Section G — Reliability and Fault Handling

### AC-017 — Deterministic, fixture-driven repeated processing

**Type:** Reliability.

**Context:** A fixture set reproducibly exercising each of the four
data-quality outcomes and each of the three scenarios exists and is run
multiple times.

**Action or condition:** The fixture set is processed at least three times.

**Pass conditions:** Each repeated fixture run produces the same validation
outcome; the same presence or absence of a normalized event and, where one is
produced, identical normalized-event content; and the same presence or
absence of a detection result and, where one is produced, the same detection
result.

**Required evidence:** Test execution evidence comparing outcomes across at
least three repeated runs per fixture, showing identical validation outcomes;
identical presence or absence of a normalized event, and identical content
where one is produced; and identical presence or absence of a detection
result, and identical results where one is produced.

**Traceability:** UC-001, UC-002; NFR-005, NFR-027.

### AC-018 — Fault isolation and artifact integrity, including across restart

**Type:** Reliability.

**Context:** Branch (a) — multiple submissions are presented for processing,
with no concurrency or batching architecture assumed, and one fixture
submission causes a platform processing fault. Branch (b) — the platform
undergoes an orderly shutdown and restart. The two branches verify separate
qualities and are independently verified: restart verification does not need
to occur immediately after the induced processing fault and does not need to
use the same fault-affected test run.

**Action or condition:** For branch (a), the fault is induced and the other
submissions' processing is observed. For branch (b), the platform is stopped
in an orderly manner and restarted.

**Pass conditions:** For branch (a), processing of the other submissions is
not prevented by the fault, their recorded artifacts are not corrupted, and
the fault itself is visible. For branch (b), every recorded artifact within
the scope of NFR-006 is present and uncorrupted after restart. Any artifact
loss or corruption in either branch is observable and never represented as
intact.

**Required evidence:** Test execution evidence isolating the faulted
fixture's effect, or lack of effect, on the other submissions' processing and
recorded artifacts, and showing the fault itself was visible; a separately
conducted before-and-after artifact and link comparison across an orderly
restart.

**Traceability:** UC-001, UC-003; NFR-006, NFR-008, NFR-010.

### AC-019 — Recovery from interruption within the recovery-time objective

**Type:** Recovery.

**Context:** The platform's execution environment is interrupted abruptly —
a non-graceful stop — mid-processing, with recorded persistent state intact;
some submissions were admitted but not yet fully processed at the moment of
interruption.

**Action or condition:** The documented recovery procedure is executed
following the interruption.

**Pass conditions:** Following the documented recovery procedure alone, the
platform is restored to full documented operation within 15 minutes; after
recovery, it is determinable for every pre-interruption admitted submission
whether its processing completed, and any incomplete one is visibly
identifiable rather than silently absent.

**Required evidence:** Timed test execution evidence of the documented
recovery procedure; a test-harness reconciliation of submission completion
states for every pre-interruption admitted submission.

**Traceability:** NFR-009, NFR-011.

## Section H — Approved Quality Targets

### AC-020 — Intake-to-alert latency of 60 seconds or less

**Type:** Performance.

**Context:** A sample of at least 20 matching submissions per approved
detection scenario, at least 60 total, processed within the approved 10
submissions per second capacity envelope.

**Action or condition:** Each sampled submission is admitted and its
resulting alert's availability time is measured.

**Pass conditions:** Every sampled submission's alert is available for review
within 60 seconds of admission. This is an all-samples rule: no sample may
exceed the target.

**Required evidence:** Timestamped measurement evidence, from admission time
to alert-availability time, captured for the full sample.

**Traceability:** UC-002, UC-003; FR-001, FR-027, FR-030; NFR-001, NFR-003.

### AC-021 — Retrieval responsiveness of 5 seconds or less

**Type:** Performance.

**Context:** The reference dataset defined above. The retrieval sample
covers validation outcomes, alerts, and minimum-evidence-set artifacts, each
sampled at least 10 times, including one first retrieval following a
documented idle or reset condition and repeated retrievals under an
already-active condition, per artifact class.

**Action or condition:** Each sampled retrieval is performed and its duration
is measured.

**Pass conditions:** Every sampled retrieval completes within 5 seconds. This
is an all-samples rule: no sample may exceed the target, under both
idle-following and active-condition retrievals.

**Required evidence:** Timestamped measurement evidence captured per
retrieval across the full sample and artifact mix.

**Traceability:** UC-001–UC-003; FR-013, FR-030, FR-031; NFR-002, NFR-003;
PER-001–PER-003.

### AC-022 — Sustained capacity envelope of 10 submissions per second

**Type:** Performance.

**Context:** A 15-minute sustained run at 10 admitted submissions per second,
with a superimposed overload window offering 15 submission attempts per
second for 30 seconds during the run.

**Action or condition:** The sustained run, including the overload window, is
executed and observed.

**Pass conditions:** The admitted rate remains within the supported envelope
throughout; the excess attempts during the overload window are visibly
rejected or deferred for capacity; no admitted submission is silently lost;
NFR-001, NFR-002, NFR-005 through NFR-008, and NFR-035 continue to hold
throughout the run.

**Required evidence:** Throughput measurements captured across the sustained
run; dependent-requirement observations captured during the same run; a
test-harness reconciliation of offered, admitted, and rejected or deferred
attempts during the overload window.

**Traceability:** UC-001; FR-001, FR-004; NFR-003.

### AC-023 — Deployment-lifetime retention and bounded resource consumption, including at resource exhaustion

**Type:** Performance / Operability.

**Context:** The acceptance observation period defined above, during which
resource usage is sampled at regular documented intervals and artifacts are
recorded near the beginning of the period.

**Action or condition:** The platform operates within the approved capacity
envelope for the observation period; a documented resource limit is
deliberately reached during the period.

**Pass conditions:** Non-retention resource consumption remains bounded and
does not grow indefinitely over the observation period; no recorded
submission, validation outcome, or minimum-evidence-set artifact is
automatically deleted or archived during the period; artifacts recorded near
the beginning remain retrievable near the end; no configured or documented
lifecycle behavior contradicts the approved deployment-lifetime retention
model; when the documented resource limit is reached, the platform exhibits
its documented behavior, the condition is observable in diagnostics, no
recorded artifact is silently lost, and the platform does not silently
continue operating in a corrupted state. This finite observation period
demonstrates consistency with the deployment-lifetime retention model; it
does not prove unlimited-duration retention.

**Required evidence:** Resource-utilization measurements sampled at regular
documented intervals across the period; a retrieval confirmation of an
early-recorded artifact near the end of the period; a review confirming no
automatic deletion or archival occurred and no documented lifecycle
configuration contradicts the retention model; test execution evidence of the
induced resource-limit condition and its captured diagnostic output.

**Traceability:** UC-003; FR-013, FR-031; NFR-032, NFR-035, NFR-036.

## Section I — Security and Operability

### AC-024 — Authenticated and authorized access at every product-exposed path

**Type:** Security.

**Context:** Every product-exposed access path or interface that exposes
telemetry, normalized events, detection definitions, match reasons, alerts,
evidence, or validation outcomes.

**Action or condition:** Access is attempted without credentials, with
invalid credentials, and with valid authorized credentials, at each such
path.

**Pass conditions:** Unauthenticated access attempts are denied at every such
path; access attempts with invalid credentials are denied; authenticated,
authorized access succeeds.

**Required evidence:** Captured evidence of access attempts, denied and
allowed, for every documented product-exposed path.

**Traceability:** NFR-012.

### AC-025 — Least-privilege operation, secret handling, and transit protection

**Type:** Security.

**Context:** The documented operating privilege set and secret-supply
procedure; whether a documented external communication boundary exists for
product data in the platform's custody, in the selected architecture.

**Action or condition:** The platform is operated under its documented
privilege set; the repository and diagnostic, alert, and evidence output are
reviewed for secret material; the existence of any external communication
boundary is reviewed.

**Pass conditions:** The platform operates successfully using only the
documented, function-justified privilege set; no platform-managed credential,
key, or token appears in the repository or in diagnostic, alert, or evidence
output; diagnostics do not copy sensitive source-telemetry content beyond
documented diagnostic need. Where a documented external communication
boundary exists, that path is protected against unauthorized disclosure and
undetected modification in transit. Where no such boundary exists in the
selected architecture, this branch is recorded as not applicable, with
evidence that no external communication path exists; this criterion does not
require the architecture to introduce one.

**Required evidence:** The documented privilege set and a successful
operation run under it; a repository and output review for secret material;
either protection evidence for each documented external-transit path, or
evidence supporting the not-applicable determination.

**Traceability:** NFR-014, NFR-015, NFR-016.

### AC-026 — Structured diagnostics, distinct outcome-family reporting, and platform health visibility

**Type:** Operability.

**Context:** Induced authentication and authorization failures, fixture
data-quality submissions, rejected or deferred admission attempts, an induced
platform fault, and both operational and deliberately non-operational
platform states.

**Action or condition:** Each condition is induced or presented, and the
resulting diagnostics and health determination are observed.

**Pass conditions:** Diagnostics are structured and, where applicable,
reference the affected submission or artifact identification; failed
authentication, denied access, and rejected unauthorized submissions appear
in diagnostics; the three outcome families — telemetry data-quality,
admission and security, and platform fault — are reported distinguishably and
never represented as one another; platform health is determinable through a
documented means in both operational states.

**Required evidence:** Captured diagnostic output for each induced condition;
captured health-determination output in both states.

**Traceability:** NFR-019, NFR-020, NFR-021, NFR-022.

### AC-027 — Change isolation, regression validation, and revision identifiability

**Type:** Operability / Maintainability.

**Context:** A representative change confined to one documented workflow
stage; the project's automated test suite; a change to the documented
normalized representation.

**Action or condition:** The confined change is applied and its effects
reviewed; the automated test suite is executed before the change is adopted;
stored normalized events are checked before and after a documented
representation change.

**Pass conditions:** The confined change does not require changes to
unrelated workflow stages, absent an intentional contract change; the
automated test suite exercises all four data-quality outcomes and all three
detection scenarios and is executed before the change is adopted; it is
determinable which documented normalized-representation revision each
normalized event conforms to, and representation changes are recorded in its
documentation.

**Required evidence:** Change-impact review evidence for the representative
change; test-suite execution evidence; a revision-determination check on
stored normalized events across a documented representation change.

**Traceability:** NFR-023, NFR-024, NFR-026.

## Section J — Reproducibility and Documentation

### AC-028 — Clean reference-environment deployment and reproducible three-scenario demonstration

**Type:** Documentation / Recovery.

**Context:** A clean instance of the one documented reference environment;
documented setup and demonstration procedures.

**Action or condition:** The documented setup procedure is followed into a
clean instance; the documented demonstration procedure is followed for each
of the three approved scenarios.

**Pass conditions:** Following the documented setup procedure alone yields an
operational platform; following documented steps alone reproduces the
complete workflow for each of the three approved scenarios, from submission
through full minimum-evidence-set inspection.

**Required evidence:** A clean-environment deployment transcript; a
per-scenario demonstration transcript with the resulting alert and evidence
inventory.

**Traceability:** UC-001–UC-003; NFR-028, NFR-033.

### AC-029 — Third-party documentation walkthrough and self-contained explanations

**Type:** Documentation.

**Context:** A reviewer not involved in authoring the platform, using only
its documentation.

**Action or condition:** The reviewer performs setup, operation, detection
definition review, and the investigation workflow using documentation alone.

**Pass conditions:** The reviewer can set up, operate — including start,
stop, restart, health determination, and recovery — review detection
definitions, and perform the investigation workflow using documentation
alone; validation reasons, match reasons, alert explanations, and
evidence-inventory information are expressed in terms of documented product
concepts, such that each persona's success criteria can be met without
inspecting the implementation.

**Required evidence:** Documented walkthrough session notes from the
uninvolved reviewer; a persona-by-persona mapping of explanation elements to
documented concepts.

**Traceability:** UC-002, UC-003; FR-012, FR-028, FR-029, FR-031; NFR-030,
NFR-031; PER-001–PER-003.

### AC-030 — Dependency and vulnerability-management practice and provider-lock-in avoidance

**Type:** Documentation / Security.

**Context:** The project's dependency inventory and documented
vulnerability-response practice; the documented v0.1 workflow's provider
dependencies.

**Action or condition:** The dependency inventory and vulnerability-response
practice are reviewed, along with evidence of at least one executed cycle;
the documented v0.1 workflow is reviewed against the provider-lock-in
criterion.

**Pass conditions:** A documented, repeatable practice for identifying
dependencies and discovering and responding to known vulnerabilities exists,
with evidence of at least one executed cycle; the documented v0.1 workflow
does not require a capability obtainable only from a single commercial
provider's proprietary service, unless a documented product constraint
justifies it.

**Required evidence:** The dependency inventory and output from one executed
vulnerability-review cycle; a dependency-inventory review against the
lock-in criterion.

**Traceability:** NFR-018, NFR-034.

## Coverage matrix

### Functional requirements

| FR | AC(s) | FR | AC(s) | FR | AC(s) |
| --- | --- | --- | --- | --- | --- |
| FR-001 | AC-001, AC-020, AC-022 | FR-013 | AC-003, AC-004, AC-005, AC-006, AC-021, AC-023 | FR-025 | AC-011 |
| FR-002 | AC-003, AC-004, AC-006 | FR-014 | AC-003, AC-004, AC-005, AC-006, AC-013 | FR-026 | AC-012 |
| FR-003 | AC-001 | FR-015 | AC-007 | FR-027 | AC-013, AC-014, AC-020 |
| FR-004 | AC-003, AC-022 | FR-016 | AC-007 | FR-028 | AC-010, AC-011, AC-012, AC-014, AC-029 |
| FR-005 | AC-003, AC-004, AC-005, AC-006 | FR-017 | AC-007 | FR-029 | AC-014, AC-029 |
| FR-006 | AC-003, AC-004, AC-005, AC-006 | FR-018 | AC-010, AC-011, AC-012 | FR-030 | AC-014, AC-020, AC-021 |
| FR-007 | AC-003 | FR-019 | AC-008 | FR-031 | AC-015, AC-021, AC-023, AC-029 |
| FR-008 | AC-004 | FR-020 | AC-009 | FR-032 | AC-015 |
| FR-009 | AC-005 | FR-021 | AC-009 | FR-033 | AC-016 |
| FR-010 | AC-006 | FR-022 | AC-009 | FR-034 | AC-016 |
| FR-011 | AC-003, AC-004, AC-005, AC-006 | FR-023 | AC-009, AC-013 | FR-035 | AC-016 |
| FR-012 | AC-004, AC-005, AC-006, AC-029 | FR-024 | AC-010 | | |

### Non-functional requirements

| NFR | AC(s) | NFR | AC(s) | NFR | AC(s) |
| --- | --- | --- | --- | --- | --- |
| NFR-001 | AC-020 | NFR-013 | AC-001, AC-002 | NFR-025 | AC-014 |
| NFR-002 | AC-021 | NFR-014 | AC-025 | NFR-026 | AC-027 |
| NFR-003 | AC-020, AC-021, AC-022 | NFR-015 | AC-025 | NFR-027 | AC-017 |
| NFR-004 | AC-002 | NFR-016 | AC-025 | NFR-028 | AC-028 |
| NFR-005 | AC-017 | NFR-017 | AC-015 | NFR-029 | AC-016 |
| NFR-006 | AC-018 | NFR-018 | AC-030 | NFR-030 | AC-029 |
| NFR-007 | AC-016 | NFR-019 | AC-026 | NFR-031 | AC-029 |
| NFR-008 | AC-018 | NFR-020 | AC-026 | NFR-032 | AC-023 |
| NFR-009 | AC-019 | NFR-021 | AC-026 | NFR-033 | AC-028 |
| NFR-010 | AC-018 | NFR-022 | AC-002, AC-026 | NFR-034 | AC-030 |
| NFR-011 | AC-019 | NFR-023 | AC-027 | NFR-035 | AC-023 |
| NFR-012 | AC-024 | NFR-024 | AC-027 | NFR-036 | AC-023 |

## Measurement profiles for approved numerical targets

| Parameter | Approved v0.1 acceptance profile |
| --- | --- |
| NFR-001 rule | All-samples: no sample may exceed 60 seconds. No percentile substitution. |
| NFR-001 sample size | At least 20 matching submissions per approved detection scenario; at least 60 total. |
| NFR-002 rule | All-samples: no sampled retrieval may exceed 5 seconds. No percentile substitution. |
| NFR-002 reference data volume | At least 10,000 accumulated admitted submissions; at least 300 generated alerts total; all four data-quality outcomes represented; artifacts from all three detection scenarios present. |
| NFR-002 retrieval sample | Validation outcomes, alerts, and minimum-evidence-set artifacts all sampled; at least 10 retrievals per named class; each class includes one first retrieval following a documented idle or reset condition and repeated retrievals under an already-active condition. No caching or storage implementation is prescribed. |
| NFR-003 sustained duration | 15 minutes sustained at 10 admitted submissions per second. |
| NFR-003 overload profile | 15 submission attempts per second offered for 30 seconds during the sustained run; admitted rate stays within the supported envelope; excess attempts visibly rejected or deferred for capacity; no admitted submission silently lost. |
| NFR-009 recovery starting condition | Abrupt, non-graceful execution-environment interruption; recorded persistent state remains intact. |
| NFR-035 resource-stability observation | Minimum 30 minutes; measurements sampled at regular documented intervals; may overlap the NFR-003 sustained-capacity run where the resulting evidence remains independently interpretable. |

## Excluded or combined candidates

The following candidate criteria were considered and combined into the AC set
above rather than retained as separate identifiers:

1. Separate criteria for the documented source-event form (FR-002) and
   documented classification criteria and precedence (FR-006) — folded into
   AC-003 through AC-006, since both are observable only through the
   classification outcomes themselves.
2. A separate criterion for classification precedence and exclusivity using
   adversarial boundary fixtures — folded into the pass conditions of AC-003
   through AC-006 using the shared boundary-fixture set.
3. Separate criteria for least-privilege operation, secret handling, and
   transit protection (NFR-014, NFR-015, NFR-016) — combined into AC-025; all
   three share a documentation and engineering-inspection verification
   method with no distinct runtime trigger.
4. Separate criteria for structured diagnostics, distinct outcome-family
   reporting, security-event visibility, and platform health visibility
   (NFR-019 through NFR-022) — combined into AC-026 for the same reason.
5. A separate criterion for interrupted-submission visibility apart from the
   recovery-time objective (NFR-009, NFR-011) — combined into AC-019; both
   are exercised by the same induced-interruption scenario.
6. A separate criterion for fault isolation apart from artifact integrity and
   restart durability (NFR-006, NFR-008, NFR-010) — combined into AC-018 as
   two independently verified branches of the same artifact-integrity
   concern.
7. Separate criteria for alert content and detection-definition-revision
   pinning (FR-029, NFR-025) — combined into AC-014; both are properties of
   the same alert artifact.
8. Separate criteria for evidence-inventory completeness and source-event
   fidelity and integrity (FR-031, FR-032, NFR-017) — combined into AC-015;
   both concern the same six-artifact set for the same alert.
9. Separate criteria for reference-environment deployment and scenario
   demonstration (NFR-028, NFR-033) — combined into AC-028; deployment is the
   necessary first step of the same documented, independently executed
   procedure.
10. Separate criteria for dependency management and provider-lock-in
    avoidance (NFR-018, NFR-034) — combined into AC-030; both are inventory
    and documentation-review checks with no runtime trigger.

## Matters delegated to architecture, threat modeling, or implementation

The following matters are exercised by the criteria above but are not decided
by them and remain delegated to later work:

- The authentication and authorization mechanism and the admission-control
  mechanism (NFR-012, NFR-013).
- Specific encryption, hashing, or transport-protection mechanisms (NFR-016,
  NFR-017).
- The concrete revision-identification mechanism for detection definitions
  and the normalized representation (NFR-025, NFR-026).
- The identity of the one reference environment, the telemetry delivery
  mechanism or protocol, and the detection-definition storage and
  maintenance mechanism.
- Concrete numeric values for the documented resource limit exercised by
  AC-023.
- The automated test-suite framework, continuous-integration tooling, and
  dependency-scanning tool selection (NFR-018, NFR-024).
- Any executable test code, fixture file formats, or load-generation
  tooling.
- Presentation, API, CLI, or user-interface design for any "available for
  review" requirement.

## Unvalidated assumptions

1. At least one product-exposed access path exists to exercise AC-024,
   independent of which presentation, API, or interface mechanism is later
   selected.
2. Synthetic fixture Kubernetes audit events, rather than a live cluster, are
   an acceptable and sufficient basis for the fixtures used across this
   document, including the boundary and negative-control fixtures in AC-003
   through AC-006, AC-011, and AC-012.
3. A documented resource limit exists by the time AC-023's resource-
   exhaustion branch is executed.
4. The reference environment used by AC-028 and AC-029 is a single
   environment that can be reliably reset to a clean instance for repeated
   demonstration runs.
5. The reference dataset for AC-021 is built up through accumulated
   documented operation, which may exceed the duration of a single AC-022
   sustained-capacity run.
6. An abrupt, non-graceful interruption is achievable and safe to induce
   repeatably in the eventual reference environment for AC-019.
7. Architecture will document whether an external communication boundary
   exists for product data in the platform's custody in time for the AC-025
   transit-protection branch to be evidenced rather than left ambiguous.

These assumptions must be validated, refined, or rejected during the
remaining Phase 0 definition work.
