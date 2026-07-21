# ADR-0002: PostgreSQL persistence and database-backed durable worker

| Field | Value |
| --- | --- |
| Status | Accepted |
| Document | `docs/architecture.md` (ARCH-01) §4 |

## Context

Submissions must be durably recorded before processing (NFR-008, NFR-011),
and every recorded artifact must survive a restart without loss or
duplication (NFR-006, NFR-007, NFR-010). Candidate persistence models
evaluated: PostgreSQL; a document-oriented database; an embedded relational
database; separate persistence technologies per artifact type. Candidate
processing models evaluated: fully synchronous processing; a
database-backed durable worker; an embedded durable queue; an external
message broker; an event-streaming platform.

## Decision

One PostgreSQL instance is the platform's sole persistence store. An
in-process, single-threaded worker claims and advances non-terminal
submissions — no `SKIP LOCKED`, no concurrent consumers, no external
broker.

Every stage's artifact insert(s) and its submission-status advance occur in
**one transaction**. Artifact inserts are conflict-safe (idempotent
re-insertion on retry, not an unconditional insert). Each status `UPDATE`
is guarded by the submission's expected prior status.

Per-artifact uniqueness (confirmed pattern, not a blanket rule):

| Artifact | Uniqueness |
| --- | --- |
| Validation outcome | one per submission |
| Normalized event | one per submission |
| Detection result | one per submission and detection-definition revision |
| Alert | one per matching detection result |
| Traceability link | unique per (source, target, relation) |

## Consequences

- Restart-safe recovery and duplicate-free retry were confirmed
  empirically, not just reasoned about: a 5-stage × 3-crash-timing matrix
  (16 scenarios) all passed, with zero submissions left non-terminal and
  zero duplicate artifacts across the run
  (`spikes/02-postgres-durable-worker/FINDINGS.md`, "Observed results").
- Crash-before and crash-mid-transaction were confirmed to converge to the
  identical recoverable state, so the recovery model has exactly two
  distinguishable cases — stage committed, or stage not committed — never a
  third "half-applied" state.
- No row-locking primitive was needed at this scale or concurrency level.
- Single-worker only: a future need for concurrent workers would require
  its own dedicated validation (`SKIP LOCKED` or equivalent claim-locking
  is explicitly not introduced by this decision).
- The schema exercised by the spike is a stub sufficient to validate the
  pattern; exact product column names, types, and the full nine-module
  artifact set remain implementation detail.

## Alternatives considered

- **SQLite** — a credible, lower-operational-footprint alternative; not
  further compared against PostgreSQL by spike, per explicit direction to
  skip that comparison. PostgreSQL's foreign-key-enforced referential
  integrity for traceability links (NFR-007) was the deciding factor from
  the prior evaluation.
- **Document-oriented database, split persistence per artifact type** —
  rejected: no requirement calls for schema flexibility, and both options
  weaken referential-integrity enforcement exactly where NFR-007 needs it
  most.
- **External message broker, event-streaming platform** — rejected: the
  approved 10 submissions/sec envelope (NFR-003) does not justify one, and
  both reintroduce at-least-once duplicate-delivery handling that this
  design avoids by construction.
- **Embedded durable queue** — rejected: introduces a second durability
  domain separate from the primary store, with no compensating benefit
  over the transactional pattern above.

## References

NFR-003, NFR-006, NFR-007, NFR-008, NFR-009, NFR-010, NFR-011; FR-015,
FR-023, FR-027; AC-018, AC-019;
`spikes/02-postgres-durable-worker/FINDINGS.md` ("Exact schema", "State
transitions and transaction boundaries", "Observed results",
"Architectural implications").
