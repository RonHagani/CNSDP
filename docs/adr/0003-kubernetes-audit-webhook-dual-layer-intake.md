# ADR-0003: Kubernetes audit-webhook-compatible dual-layer intake

| Field | Value |
| --- | --- |
| Status | Accepted |
| Document | `docs/architecture.md` (ARCH-01) §5 |

## Context

FR-001 and FR-002 require a defined intake accepting the documented
Kubernetes `audit.k8s.io/v1 Event` supported form. Candidate contracts
evaluated: a Kubernetes audit-webhook-compatible contract; a simplified
project-specific contract; a dual-layer design (webhook-compatible external
adapter plus a canonical internal model); and fixture-file-based ingestion
as the primary mechanism.

## Decision

An external HTTP(S) endpoint compatible with the Kubernetes audit-webhook
backend's `EventList` wire format, mapping each `Event` onto a small
canonical internal submission model consumed by the rest of the pipeline
(validation, normalization, detection). Fixture-based delivery for tests and
demonstrations POSTs recorded or synthetic payloads at this same endpoint —
no separate ingestion path exists, preserving FR-001's single defined
intake.

**Scenario 3 boundary:** detection evaluates ClusterRoleBinding **creation
only** in v0.1. Modification, including subject addition, is not evaluated.
This is already reflected in `docs/scope.md`, `docs/
functional-requirements.md` (FR-018, FR-026), and `docs/
acceptance-criteria.md` (AC-012); this ADR records the architectural
consequence — no modification-detection code path is built — without
restating that content.

## Consequences

- Real captured payloads for all three approved scenarios parsed cleanly
  with the official `k8s.io/apiserver/pkg/apis/audit/v1` Go types, with
  zero unmarshal errors (`spikes/01-k8s-audit-intake/FINDINGS.md`, Q1).
- The internal canonical model insulates the rest of the pipeline from any
  future wire-format or source-family change (NFR-023).
- Exec's interactive signal (`tty`, `stdin`) lives in `requestURI` query
  parameters, not `requestObject` — the adapter must parse the URI, not
  only the request body (Q1). Detection must not key on `verb ==
  "connect"`; the captured cluster recorded `verb == "get"` for a genuine,
  successful, TTY-allocated exec session — confirmed via
  `responseStatus.code: 101` across all three audit stages, not a parsing
  artifact.
- `requestObject`/`responseObject` are populated only at the
  `ResponseComplete` audit stage in the tested configuration, not
  `RequestReceived` (Q2). Any component needing object content must
  consume `ResponseComplete`-stage events.
- Scenario 3 modification detection was found empirically unreliable for
  the two dominant real-world modification techniques (`kubectl apply` and
  merge-patch) — neither produces a `requestObject`/`responseObject` that
  distinguishes a newly added subject from one already present (Q3). Only
  an explicit RFC 6902 JSON-Patch "add" operation self-proves addition, and
  is not retained as a special-case detection branch — see `docs/scope.md`
  "Deferred to later releases" for the product-level rationale.

## Alternatives considered

- **Simplified project-specific contract** — rejected: breaks
  real-cluster wire-format compatibility and future live-integration
  credibility for a small implementation saving.
- **Fixture-file-based ingestion as the primary mechanism** — rejected as
  primary: bypasses the FR-001 admission boundary and NFR-013's authorized
  submission path; retained only as the test/demonstration delivery method
  riding on the real endpoint.

## References

FR-001, FR-002, FR-007, FR-018, FR-023, FR-024, FR-025, FR-026; AC-010,
AC-011, AC-012; PD-04 Scenario 3, PD-04 "Deferred to later releases" item 7;
`spikes/01-k8s-audit-intake/FINDINGS.md` (Q1, Q2, Q3).
