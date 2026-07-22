# Cloud-Native Security Telemetry and Detection Platform — Glossary

| Field | Value |
| --- | --- |
| Document ID | PD-08 |
| Version | 0.1 |
| Status | Approved – Phase 0 baseline |
| Phase | Phase 0 — Product Definition and Requirements |

## Purpose and scope

This document standardizes the product-specific terminology already
established by the approved Phase 0 baseline: the Product Charter (PD-01),
Personas (PD-02), Use Cases (PD-03), Scope and Non-Goals (PD-04), Functional
Requirements (PD-05), Non-Functional Requirements (PD-06), and Acceptance
Criteria (PD-07). It defines existing terms only and introduces no new
product behavior, requirements, scope decisions, personas, use cases,
acceptance criteria, or identifier namespaces.

This is not a general cybersecurity glossary. It includes only the terms
whose consistent interpretation is necessary for understanding the approved
product scope, requirements, traceability, and acceptance criteria. Ordinary
words that need no project-specific clarification are intentionally omitted,
as is any wording that would select a technology, architecture, protocol,
storage mechanism, user interface, or other implementation decision not yet
made by the approved baseline.

Where the baseline has used more than one phrase for the same concept, this
document selects one canonical term for that entry and records the other
phrase as a related or documented synonym. It does not change the meaning
established by PD-01 through PD-07.

## Terms

### Telemetry intake and admission

**Defined intake** — The platform's designated point of entry for telemetry
submissions of the supported source family. Distinguished from the admission
boundary, the earlier point at which a submission attempt first arrives and
is subject to authorization and capacity admission control before being
admitted into the defined intake. *Source:* PD-05 FR-001; PD-06 Definitions.

**Submission attempt** — An attempted delivery presented to the platform's
admission boundary, before authorization and capacity admission control are
applied. *Source:* PD-06 Definitions.

**Admitted submission** — A submission attempt accepted through the
platform's authorization and capacity admission controls into the defined
intake. Equivalent, for validation and classification purposes, to the
"submission" referred to throughout PD-05. *Source:* PD-06 Definitions;
PD-05 FR-004 through FR-014.

**Authorized submission path or submitter** — A submission origin or channel
that the platform's admission controls recognize as authorized to deliver
telemetry at the defined intake. Submission attempts from any other path or
submitter are rejected as an admission and security outcome, before
telemetry validation. The concrete authentication or authorization mechanism
is not selected at this phase. *Source:* PD-06 NFR-013.

**Source family** — The category of telemetry origin the platform is
designed to accept submissions from. v0.1 supports exactly one source
family: Kubernetes API server audit events. Distinguished from an individual
source event and from the supported source-event form. *Source:* PD-04
"Selected telemetry source family."

**Source event** — The Kubernetes API server audit event carried by a
submission, in the content and form received at the defined intake.
*Related:* source-event evidence — the same source event as made available
for alert investigation, subject to the evidentiary-fidelity guarantee that
it faithfully represents the originally received submission content.
*Source:* PD-05 Definitions; FR-032; PD-07 AC-015.

**Supported source-event form** — The documented shape of a Kubernetes API
server audit event — a Kubernetes audit.k8s.io/v1 Event — that the platform
supports as input, including its structural constraints and required
information. Defines the product's input contract only; it does not select
any audit configuration, collection mechanism, or delivery protocol.
*Source:* PD-05 FR-002.

### Data-quality and outcome families

**Telemetry data-quality outcome** — One of exactly four mutually exclusive
outcomes assigned to every admitted submission: valid — supported,
parseable, structurally conformant, and containing all required information;
invalid — attributable to the supported source family and form but
unparseable or structurally nonconformant; incomplete — parseable and
structurally valid but missing required information; unsupported — not
attributable to the supported source family or form, or belonging to an
explicitly unsupported variant. Assessed in that precedence order; each
submission receives exactly one outcome. Also referred to in PD-05 as the
submission's "validation outcome" — the two phrases name the same
classification. Distinguished from admission and security outcome and
platform fault; together these are the three families of outcome the
platform reports distinguishably. *Source:* PD-05 FR-005 through FR-010;
PD-06 Definitions, NFR-022; PD-07 AC-003 through AC-006, AC-026.

**Admission and security outcome** — An outcome of the admission boundary
itself — unauthorized-submission rejection, over-capacity rejection, or
capacity deferral — assessed before and independently of telemetry
data-quality classification. Never represented as a valid, invalid,
incomplete, or unsupported outcome. *Source:* PD-06 Definitions.

**Platform fault** — An internal processing, availability, integrity, or
recovery failure of the platform, distinct from both telemetry data-quality
outcomes and admission and security outcomes. *Source:* PD-06 Definitions.

### Normalization

**Normalized event** — The representation of a valid source event after
transformation into the documented normalized representation. Exactly one
normalized event is produced per valid submission. *Source:* PD-05
Definitions; FR-015.

**Documented normalized representation** — The documented specification of
what a normalized event must convey and the meaning of each element it
carries. Its concrete form or format is delegated to architecture; only its
required content is fixed at this phase. Individual normalized events are
traceable to an identifiable revision of this representation. *Source:*
PD-05 FR-016 through FR-018; PD-06 NFR-026.

### Detection

**Detection definition** — The identifiable definition of one of the three
approved v0.1 detection scenarios, including its documented detection
conditions. *Source:* PD-05 Definitions; FR-020, FR-021.

**Detection-definition revision** — A specific, identifiable version of a
detection definition. Every alert and match reason is pinned to the
revision that actually produced it, so a later edit to a detection
definition does not retroactively change what a previously generated alert
is traceable to. *Source:* PD-06 NFR-025; PD-07 AC-014.

**Detection result (match / non-match)** — The outcome of evaluating one
normalized event against one detection definition. A match — also described
in the baseline as a "matching detection result" or "detection-evaluation
result" — occurs when the documented detection conditions are satisfied and
produces an alert; otherwise the evaluation is a non-match. Because a valid,
scenario-relevant submission is guaranteed to carry the information its
scenario requires, a non-match occurs only when that information was
available and the conditions were genuinely not met, never from missing
data. Whether a non-match is itself recorded or made visible is not
required by v0.1. *Source:* PD-05 Definitions, FR-023; PD-04 scope decision
7; PD-07 AC-011, AC-012, AC-013.

**Match reason** — The recorded explanation of why a matching detection
result matched: it identifies the detection definition, the documented
conditions that were satisfied, and the event information that satisfied
them. Persisted as part of the detection result, one of the six
minimum-evidence-set artifacts. *Source:* PD-05 Definitions; FR-028.

### Alerts and explainability

**Alert** — The structured product artifact produced for every matching
detection result, exactly one per match. Conveys the matched definition's
identity, what was detected, the documented conditions that matched, and
the recorded match reason. *Source:* PD-05 Definitions; FR-027, FR-029.

**Explainability** — The product quality by which an alert's, match
reason's, or telemetry data-quality outcome's stated reasoning is expressed
entirely in terms of the platform's own documented concepts — the supported
source-event form, the documented normalized representation, and documented
detection conditions — so that a persona can assess it without inspecting
the platform's internal implementation. *Source:* PD-01 PC-G-005; PD-06
NFR-031.

### Evidence and traceability

**Minimum evidence set** — The fixed set of six persisted artifacts
approved for v0.1: (1) the source submission (raw event), (2) the
validation outcome, (3) the normalized event, (4) the detection
definition and its documented conditions, (5) the detection result,
including its recorded match reason, and (6) the generated alert.
Traceability verification — confirming these six artifacts connect
correctly and remain unmodified — describes the integrity and
connectivity of this set; it is not a seventh artifact. This boundary is
fixed for v0.1; broader contextual evidence is deferred to later
releases. *Source:* PD-04 scope decision 8; PD-05 FR-031.

**Evidence inventory** — The per-alert account of the six
minimum-evidence-set artifacts, stating which are available for inspection.
The inventory is a visibility mechanism over the approved evidence
boundary, not an additional or broader form of contextual evidence.
*Source:* PD-05 Definitions; FR-031, FR-035.

**Traceability link** — A recorded association between two artifacts of the
minimum evidence set. *Source:* PD-05 Definitions.

**Alert-to-source traceability chain** — The ordered sequence of
traceability links connecting a generated alert, through its match reason
and the detection definition that matched, to the normalized event and the
source audit event. Distinguished from an individual traceability link,
which is a single association between two artifacts. *Source:* PD-05
FR-033, FR-034; PD-07 AC-016.

**Recorded artifact** — Any of the platform's persisted product records,
considered collectively for integrity, durability, and retention:
submission records, telemetry data-quality outcomes, normalized events,
detection-definition revisions, match reasons, alerts, evidence
inventories, and traceability links. *Source:* PD-06 NFR-006.

**Recorded persistent state** — The stored representation of recorded
artifacts and traceability links that the platform maintains across
restarts of its execution environment. *Source:* PD-06 Definitions.

### Operating envelope and environment

**Reference environment** — The single documented environment into which
the platform is deployed for reproducible setup and demonstration in v0.1.
Its concrete technical identity is delegated to architecture; v0.1 requires
exactly one such environment, not multiple. *Source:* PD-06 NFR-033.

**Capacity envelope** — The bounded admitted load — 10 submissions per
second sustained in v0.1 — within which the platform's documented
performance, reliability, and resource-consumption requirements are
guaranteed to hold. Load beyond the envelope produces a visible admission
and security outcome, capacity rejection or deferral, not a data-quality
outcome. *Source:* PD-06 NFR-003.

**Recovery-time objective** — The maximum documented time, 15 minutes in
v0.1, to restore the platform to full documented operation after an
interruption of its execution environment, using the documented recovery
procedure alone, provided recorded persistent state remains available.
Restoration after total loss of recorded persistent state is out of scope
for v0.1. *Source:* PD-06 NFR-009.

**Product-exposed access path** — Any interface, path, or surface —
regardless of its eventual technical form — through which product data or
product functions are exposed to a requesting party. The concrete
mechanism, whether an API, a user interface, a CLI, or otherwise, is not
selected at this phase. *Source:* PD-06 NFR-012; PD-07 AC-024.

## Terms considered and not included

The following terms were evaluated against the approved Phase 0 baseline and
intentionally excluded as standalone entries:

- **Valid, invalid, incomplete, unsupported** — folded into *Telemetry
  data-quality outcome* as its four mutually exclusive values, rather than
  four separate entries.
- **Match, non-match, detection-evaluation result, matching detection
  result** — combined into the single *Detection result (match /
  non-match)* entry rather than treated as separate entries.
- **Validation outcome** — retained only as the documented PD-05 synonym
  within *Telemetry data-quality outcome*, not as a separate entry.
- **Evidence** — not given a standalone entry; its project-specific meaning
  in v0.1 is exactly the approved minimum evidence set (see *Minimum
  evidence set*).
- **Source-event evidence** — folded into *Source event* as a related term
  rather than a full entry.
- **Unvalidated assumption** — a documentation convention used throughout
  PD-01 through PD-07, not project-specific product terminology; excluded
  as an ordinary term needing no clarification.
- **Reference dataset** (PD-07 AC-021) — an acceptance-testing data-volume
  parameter, not product terminology; noted here only to distinguish it
  from *Reference environment*, which is a deployment concept.
