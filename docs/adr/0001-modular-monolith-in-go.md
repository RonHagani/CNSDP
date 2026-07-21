# ADR-0001: Modular monolith architecture in Go

| Field | Value |
| --- | --- |
| Status | Accepted |
| Document | `docs/architecture.md` (ARCH-01) §2 |

## Context

v0.1 needs the smallest credible architecture satisfying FR-001–FR-035,
NFR-001–NFR-036, and AC-001–AC-030 without unjustified distribution
(PC-P-006, PC-C-001). Candidate architecture styles evaluated: modular
monolith; independently deployable services; a deployment-level hybrid; and
an event-driven pipeline with separately deployable stages. Candidate
languages evaluated: Go, Rust, Kotlin (JVM), Python, TypeScript/Node.js.

## Decision

A single Go deployable, structured as a modular monolith with explicit
internal workflow-stage module boundaries (`docs/architecture.md` §2). No
service decomposition and no message broker in v0.1.

## Consequences

- Simplest deployment, testing, and recovery story, directly serving
  NFR-005, NFR-006, NFR-009, and NFR-033.
- Go's explicit-error-return idiom aligns with the "never silently lose or
  misclassify an outcome" language repeated across NFR-004, NFR-006,
  NFR-011, and NFR-022.
- The official Kubernetes Go audit types gave canonical field fidelity for
  FR-002, confirmed empirically: real captured payloads for all three
  scenarios parsed with zero unmarshal errors
  (`spikes/01-k8s-audit-intake/FINDINGS.md`, Q1).
- Weaker compiler-enforced exhaustiveness than Kotlin for the four-way
  validation-outcome and detection-result domain model — mitigated by
  lint-level exhaustiveness checks and the mandatory NFR-024 regression
  suite, not by a language feature.
- NFR-023 (change isolation) is satisfied by module discipline, not by a
  physical deployment boundary, and is verified by AC-027 rather than
  guaranteed structurally.

## Alternatives considered

- **Independently deployable services** and **event-driven pipeline with a
  broker** — both rejected: neither is justified by the approved 10
  submissions/sec, single-tenant v0.1 envelope (NFR-003), and both would
  work against NFR-005 (determinism), NFR-007 (link integrity across
  stores), and NFR-009 (multi-service RTO).
- **A deployment-level hybrid** (monolith plus one arbitrarily separated
  component) — no FR/NFR identifies which component would need separating;
  rejected as unjustified complexity.
- **Kotlin/JVM** — a near-tie with Go on the weighted evaluation matrix
  (strongest ecosystem-maturity and domain-modeling story); Go's canonical
  Kubernetes audit types and minimal operational footprint were the
  deciding factors.
- **Rust** — best correctness ceiling, but the highest realistic
  solo-developer delivery risk for shipping the full v0.1 scope, and its
  distinguishing strength (compile-time memory/data-race safety) is
  orthogonal to this product's problem shape.
- **Python, TypeScript/Node.js** — viable but weaker on static domain-model
  safety (Python) or supply-chain risk profile for a self-described
  security-sensitive platform (Node, PC-P-008).

## References

FR-002; NFR-003, NFR-004, NFR-005, NFR-006, NFR-008, NFR-009, NFR-011,
NFR-022, NFR-023, NFR-033, NFR-034; PC-P-006, PC-C-001; AC-027;
`spikes/01-k8s-audit-intake/FINDINGS.md` (Q1).
