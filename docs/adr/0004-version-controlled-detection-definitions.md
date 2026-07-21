# ADR-0004: Version-controlled declarative detection definitions

| Field | Value |
| --- | --- |
| Status | Accepted |
| Document | `docs/architecture.md` (ARCH-01) §9 |

## Context

FR-020 through FR-022 require identifiable, documented, reviewable
detection definitions. NFR-025 requires every alert to remain pinned to the
detection-definition revision that actually produced it, even after a later
edit to that definition. PD-04 scope decision 3 explicitly excludes
in-product authoring, maintenance workflows, and content lifecycle
management from v0.1. Candidate representations evaluated: version-controlled
declarative files; database-resident mutable definitions; compiled-only
definitions; in-product authoring and storage.

## Decision

Detection definitions are version-controlled declarative YAML files, one
per approved scenario, loaded at startup into the persistence store as
immutable, revision-identified records. No code path exists to edit a
definition through the product itself — a definition change ships as an
ordinary code change through the same review, test, and deploy discipline
as everything else.

## Consequences

- Revision identity and reviewability come from version control directly;
  no bespoke revisioning mechanism needs to be built to satisfy NFR-025.
- Structurally excludes in-product authoring by construction, matching PD-04
  scope decision 3 without relying on a policy convention that could be
  bypassed later.
- Immutable, load-once definitions directly support NFR-005 (deterministic
  processing) — a definition cannot change mid-evaluation.
- A definition change always requires a redeploy. This is a deliberate
  consequence of the PD-04 exclusion, not a limitation to be engineered
  around.

## Alternatives considered

- **Database-resident mutable definitions** — weaker reviewability than
  files in version control (no native diff/history) and would require
  building a parallel change-log mechanism to approximate what version
  control already provides for free.
- **Compiled-only definitions** (logic embedded directly in source, no
  separate declarative artifact) — weaker separation between the
  human-reviewable "documented conditions" FR-021 requires and the
  evaluation code that interprets them.
- **In-product authoring and storage** — excluded outright by PD-04 scope
  decision 3; not a live alternative.

## Deferred

The exact revision-ID format (content hash, git tag/commit, or sequence
number) is not decided by this ADR — see `docs/architecture.md` §9,
"Deferred implementation choices."

## References

FR-020, FR-021, FR-022; NFR-025; PD-04 scope decision 3, PD-04 "Deferred to
later releases".
