# Cloud-Native Security Telemetry and Detection Platform — v0.1 Architecture

| Field | Value |
| --- | --- |
| Document ID | ARCH-01 |
| Version | 0.1 |
| Status | Approved – Phase 1 architecture baseline |
| Phase | Phase 1 — Architecture |

## Purpose and scope

This document defines the v0.1 architecture implementing the approved Phase 0
baseline (PD-01 through PD-08, `docs/`). It does not redefine product scope,
functional requirements, non-functional requirements, or acceptance criteria.
Where this document constrains behavior beyond Phase 0, it is because Phase 0
explicitly delegated that decision to architecture (PD-04 "Undecided —
delegated to requirements or architecture"; PD-06 delegated
transport-protection, change-isolation, revision-identification,
reference-environment, and resource-limit mechanisms).

Four decisions with material technical uncertainty or long-lived consequence
are recorded as Architecture Decision Records (ADRs) in `docs/adr/` and
summarized, not restated, here (§9). Lower-stakes or more easily reversible
decisions — reference environment, authentication, observability, testing
direction — are recorded directly in this document (§6–§7) rather than as
ADRs. Implementation-level detail (exact libraries, package layout, migration
tooling, CI, secrets delivery, revision-ID format) is explicitly out of scope
and deferred (§9).

Two architecture spikes preceded this document and are cited by finding,
not reproduced:

- `spikes/01-k8s-audit-intake/FINDINGS.md` — real Kubernetes audit-event
  capture and field-mapping validation.
- `spikes/02-postgres-durable-worker/FINDINGS.md` — real PostgreSQL
  transactional durable-worker crash-recovery validation.

## 1. Architecture drivers

Cited by identifier only; full text lives in the referenced Phase 0 document.

| Concern | Drivers |
| --- | --- |
| Correctness and determinism | NFR-005, NFR-006, NFR-011, NFR-022 |
| Traceability by design | PC-P-005, FR-033, FR-034, NFR-007, NFR-029 |
| Evidence integrity | NFR-017, FR-031, FR-032 |
| Fault isolation and recovery | NFR-008, NFR-009, NFR-010 |
| Operational simplicity, no premature scale | PC-P-006, PC-C-001, NFR-033, NFR-034, NFR-035 |
| Security self-protection | PC-P-008, NFR-012, NFR-013, NFR-014, NFR-015, NFR-016 |
| Capacity and latency envelope | NFR-001, NFR-002, NFR-003 — modest enough to not differentiate among credible technology choices |
| Portfolio credibility without overengineering | PC-G-008, PC-G-009 |

## 2. Architecture style and logical components

**Decision:** one Go deployable, structured as a modular monolith with
explicit internal workflow-stage module boundaries. No service
decomposition, no message broker. See ADR-0001.

Nine internal modules, one per workflow area, each owning its own artifact
table(s) and exposing only a defined interface to the others — no module
reads or writes another module's table directly:

1. Telemetry admission
2. Validation and classification
3. Normalization
4. Detection evaluation
5. Alert generation
6. Evidence inventory
7. Traceability
8. Retrieval and investigation
9. Operational diagnostics

This boundary discipline is what satisfies NFR-023 (change isolation) and is
verified by AC-027 (a confined change must not require changes to unrelated
modules). It is an internal-code convention enforced by module structure and
tests, not a deployment-level boundary.

## 3. End-to-end data flow

Confirmed status vocabulary (Spike 2, `spikes/02-postgres-durable-worker/
FINDINGS.md`, "State transitions and transaction boundaries"):

```
admitted → validated → normalized → evaluated → alerted → evidenced
```

| Transition | Requirement basis |
| --- | --- |
| (intake) → admitted | FR-001, FR-003, NFR-013 |
| admitted → validated | FR-004, FR-005, FR-006, FR-007–FR-010 |
| validated → normalized | FR-015, FR-016, FR-017, FR-018, FR-019 |
| normalized → evaluated | FR-023, FR-024, FR-025, FR-026 |
| evaluated → alerted | FR-027, FR-028, FR-029 |
| alerted → evidenced | FR-031, FR-032, FR-033, FR-034, FR-035 |

Only submissions classified valid proceed past `validated` (FR-014).
Non-valid outcomes are recorded and made reviewable (FR-011, FR-012, FR-013)
but do not advance further.

## 4. Persistence and processing model

**Decision:** one PostgreSQL instance is the platform's sole persistence
store; an in-process, single-threaded, database-backed worker claims and
advances non-terminal submissions. See ADR-0002.

Confirmed transaction rule (Spike 2, "State transitions and transaction
boundaries"; "Observed results"): every stage's artifact insert(s) and its
submission-status advance occur in **one transaction**. Artifact inserts are
conflict-safe (idempotent re-insertion on retry). Each status `UPDATE` is
guarded by the expected prior status, so a stage cannot silently be applied
twice.

Confirmed per-artifact uniqueness semantics (not a blanket rule):

| Artifact | Uniqueness |
| --- | --- |
| Validation outcome | one per submission |
| Normalized event | one per submission |
| Detection result | one per normalized event and detection-definition revision (equivalent to one per submission and revision, since a normalized event is itself one per submission) |
| Alert | one per matching detection result |

The alert-to-source chain (alert → detection result → normalized event →
submission) is represented entirely by enforced, `NOT NULL` foreign-key
relationships — `detection_results` references `normalized_events`
directly rather than storing a separately mutable submission reference, so
the chain has exactly one path and cannot become internally inconsistent.
No redundant `traceability_links` table is persisted: a stored copy of a
relationship the foreign keys already state unambiguously would itself be
the kind of duplicated, independently-mutable reference this design avoids
elsewhere. The traceability module (module 7) derives and exposes the
chain by joining across these foreign keys rather than reading a
denormalized table. Because every link in the chain is `NOT NULL` and
FK-enforced, a broken or missing link cannot arise from ordinary valid
operation — the chain verifier's role (NFR-029, AC-016) is to detect
missing, corrupted, or out-of-band-damaged relationships (an AC-018-class
platform fault), not to check for a routine consistency gap the schema
already makes structurally impossible.

Confirmed restart behavior: across a 5-stage × 3-crash-timing matrix (16
scenarios, all passing), every submission reached a determinable state after
an abrupt process crash, and no duplicate artifact was ever produced on
resume (Spike 2, "Observed results"). Crash-before and crash-mid-transaction
were confirmed to converge to an identical, safely-resumable state — the
recovery model has exactly two distinguishable cases (stage committed / stage
not committed), not three. This satisfies NFR-006, NFR-008, NFR-009,
NFR-010, and NFR-011, and is exercised by AC-018 and AC-019.

No `SKIP LOCKED`, no concurrent workers, and no external broker are
introduced — the approved 10 submissions/sec envelope (NFR-003) does not
justify them, and none was needed to pass the Spike 2 matrix.

## 5. Intake and external contracts

**Decision:** a dual-layer contract — an external HTTP(S) endpoint
compatible with the Kubernetes audit-webhook backend's `EventList` wire
format, mapped into a small canonical internal submission model consumed by
the rest of the pipeline. See ADR-0003.

Fixture-based delivery (tests, demonstrations) POSTs recorded or synthetic
payloads at this same endpoint — there is no separate ingestion path,
preserving FR-001's single defined intake.

Confirmed field-mapping realities (Spike 1, `spikes/01-k8s-audit-intake/
FINDINGS.md`):

- **Q1** — exec interactive signals (`tty`, `stdin`) are `requestURI` query
  parameters, not `requestObject` fields; the adapter must parse the URI,
  not only the body. Detection logic must not key on `verb == "connect"`
  for scenario 1 — the captured cluster recorded `verb == "get"` for a
  genuine, successful, TTY-allocated exec session. This is version-specific
  and not reverified beyond the tested cluster (§9).
- **Q2** — `requestObject`/`responseObject` are populated only at the
  `ResponseComplete` audit stage in the tested configuration, not
  `RequestReceived`. Any component needing object content must consume
  `ResponseComplete`-stage events.
- **Q3** — Scenario 3 evaluates ClusterRoleBinding **creation only** in
  v0.1. Modification (including subject addition) is not evaluated and no
  modification-detection code path exists. This is already reflected in
  `docs/scope.md`, `docs/functional-requirements.md` (FR-018, FR-026), and
  `docs/acceptance-criteria.md` (AC-012); this document does not restate
  that content, only enforces it architecturally.

## 6. Security and trust boundaries

**Authentication:** a shared bearer token / API key represents the single
approved trust level (NFR-012). Checked at the intake boundary (NFR-013)
and at every product-exposed access path (AC-024). No user or role model,
no RBAC — an explicit NFR-012 waiver, not an oversight.

**Transit protection:** TLS is required wherever the deployment exposes a
network boundary beyond localhost (NFR-016); where the reference environment
has no such boundary, this is recorded as not-applicable per AC-025's
explicit branch, not treated as a gap.

**Secrets:** the bearer token must never be committed to the repository or
appear in diagnostic, alert, or evidence output (NFR-015, AC-025). The exact
delivery mechanism (environment variable, mounted file, secret store) is
deferred (§9).

**Upgrade triggers, not current decisions:** mutual TLS and OIDC remain
documented future options if per-submitter cryptographic identity or
federated/role-based access is ever required by an approved later release.
Neither is needed by the current baseline.

## 7. Reference environment, operability, and testing

**Reference environment:** Docker Compose, exactly two services — the
application and PostgreSQL — on a single host (NFR-033, NFR-034, AC-028). A
separate, optional local Kubernetes cluster may be used purely as a
telemetry-fixture-generation tool, exactly as Spike 1 used one — it is never
the platform's deployment target.

**Operability:**
- Structured, submission/artifact-correlated logs (NFR-021), distinguishing
  the three outcome families — data-quality, admission/security, platform
  fault — without conflating them (NFR-022, AC-026).
- A health/readiness endpoint (NFR-020).
- A dedicated, independently testable traceability-chain verifier
  implementing NFR-029: it must fail visibly and identify the specific
  affected link (AC-016).
- No metrics or distributed-tracing stack in v0.1. Nothing in the approved
  baseline requires continuous dashboards or cross-process tracing for a
  single-deployable system; latency and capacity targets (NFR-001, NFR-002,
  NFR-003) are acceptance-tested with point-in-time measurement, not a
  monitoring product (AC-020–AC-023).

**Testing strategy:**
- The language's native test framework for unit, integration, and
  end-to-end tests.
- Integration tests run against a real, ephemeral PostgreSQL instance —
  not mocked — because correctness here is substantially transactional and
  referential (NFR-006, NFR-007; AC-017, AC-018).
- Property-based or fuzz testing applied to the four-way
  classification-precedence logic (FR-005), beyond the boundary-fixture set
  (AC-003–AC-006).
- A lightweight load-generation script for AC-020–AC-023; no dedicated
  load-testing product, matching NFR-003's modest envelope.
- Exact frameworks and tools are deferred (§9).

## 8. Walking-skeleton definition

The smallest slice proving the architecture above works end to end:

1. One real (or Spike-1-realistic) `EventList` containing a single
   `pods/exec` event with TTY allocation (scenario 1), POSTed with a valid
   bearer token to the intake endpoint.
2. The endpoint authenticates the token and durably admits the submission.
3. The worker advances it through `validated → normalized → evaluated →
   alerted → evidenced`, using exactly the transaction pattern confirmed in
   Spike 2 — all three loaded detection definitions are evaluated (FR-023),
   only scenario 1's matches.
4. Exactly one alert is produced, citing the matched definition's revision
   id and a match reason (FR-028, FR-029).
5. A bearer-token-authenticated retrieval walks the traceability chain from
   the alert back to the source event (FR-034).

**Explicitly excluded from the skeleton:** scenario 2/3 full detection logic
beyond definition loading, invalid/incomplete/unsupported paths, full
capacity/admission-control enforcement, restart/recovery demonstration,
performance measurement, any UI, definition revisioning beyond a hardcoded
revision-1 id, negative-control fixtures.

## 9. Traceability, ADR index, and unresolved assumptions

### Delegated-decision closure

| PD-04 / PD-06 delegated item | Resolved by |
| --- | --- |
| Telemetry delivery mechanism/protocol (PD-04 deleg. 1) | ADR-0003 |
| Detection-definition storage/maintenance mechanism (PD-04 deleg. 2) | ADR-0004 |
| Normalized-representation format (PD-04 deleg. 3) | §3, §4 (no dedicated ADR) |
| Exact Kubernetes audit-event fields required (PD-04 deleg. 9) | ADR-0003, spike 1 |
| Transport-protection mechanism (NFR-016) | §6 |
| Change-isolation mechanism (NFR-023) | ADR-0001, §2 |
| Revision-identification mechanisms (NFR-025, NFR-026) | ADR-0004; §3 |
| Reference-environment identity (NFR-033) | §7 |
| Resource-limit values (NFR-035, NFR-036) | Deferred — implementation value, not architecturally fixed |

### ADR index

| ID | Title | Status |
| --- | --- | --- |
| ADR-0001 | Modular monolith architecture in Go | Accepted |
| ADR-0002 | PostgreSQL persistence and database-backed durable worker | Accepted |
| ADR-0003 | Kubernetes audit-webhook-compatible dual-layer intake | Accepted |
| ADR-0004 | Version-controlled declarative detection definitions | Accepted |

### Deferred implementation choices — not silently decided

- HTTP router/library selection.
- Structured-logging library selection.
- Schema-migration tool.
- Detection-definition revision-ID exact format (content hash, git tag, or
  sequence number).
- Go package layout and naming for the internal module boundaries in §2.
- Secrets-supply mechanism for the bearer token.
- CI tooling and dependency-scanning tool selection (NFR-018).

### Open assumptions

- Whether `verb == "get"` for a TTY-allocated exec request holds on
  Kubernetes versions other than the one tested (v1.34.0) — not
  reverified.
- Live audit-webhook network delivery mechanics (retries, backpressure,
  TLS) were not exercised — Spike 1 substituted the audit log-file backend
  by disclosed necessity; wire-format shape only was validated.
