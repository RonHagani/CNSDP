# CLAUDE.md — Cloud-Native Security Telemetry and Detection Platform

Persistent instructions for all work in this repository. Product detail lives in
`docs/product.md` (Product Charter, PD-01) — the approved source of truth for
product direction. Do not duplicate its content here.

## Project purpose

A portfolio-grade, production-oriented platform demonstrating an end-to-end
security workflow: telemetry collection → validation and normalization →
detection → explainable alerts → evidence-based investigation. The central
value is explainable and traceable detection, not alert volume or feature
breadth. See `docs/product.md` for the full charter.

## Current phase

Phase 1 — Architecture and Implementation. Phase 0 (Product Definition and
Requirements) is complete and approved; the Phase 1 architecture baseline
(`docs/architecture.md`, ARCH-01) is approved, and the walking-skeleton
implementation defined by ARCH-01 §8 is in progress.

Create each project artifact only when it becomes necessary in the approved
project sequence. Never create empty placeholder documents. Update this section
only after an explicitly approved phase transition.

## Source of truth and change control

- Approved documents override personal or model preferences. Do not redesign
  the product or replace approved decisions.
- Substantive changes to approved content must be proposed separately and
  explicitly approved before being applied — never made silently.
- Editorial fixes (formatting, grammar, structure) must not alter meaning.

## Working principles

Follow the charter principles (PC-P-001…PC-P-008 in `docs/product.md`), in
summary:

- A complete end-to-end workflow beats broad, incomplete coverage.
- Evidence and explainability come before automation and detection volume.
- Invalid, incomplete, or unsupported data is handled visibly, never silently.
- Traceability by design across the telemetry and detection lifecycle.
- Production-oriented engineering without premature enterprise scale.
- The platform itself is a security-sensitive system and must be protected.

## Decision boundaries

- No technology, architecture, protocol, database, messaging, infrastructure,
  or deployment decisions during Phase 0 unless a product constraint genuinely
  requires them (PC-C-004).
- Every capability must be justified by documented users, use cases, or
  requirements (PC-C-003).
- The non-goals in PC-011 are binding: do not implement SIEM, SOAR,
  incident-response, case-management, commercial SaaS, or multi-tenant
  capabilities unless the approved product scope changes.

## Scope discipline

- No unjustified scope expansion and no overengineering. Prefer the smallest
  change that satisfies a documented requirement.
- Do not add capabilities, compliance requirements, or enterprise features
  that are not in approved documents.
- When scope is ambiguous, ask instead of expanding.

## Documentation rules

- Product and specification content lives under `docs/`. CLAUDE.md carries
  only persistent instructions.
- Preserve stable identifiers (PC-###, PC-G-###, PC-P-###, PC-C-###, PC-A-###,
  and future PER-###, UC-###, FR-###, NFR-###, AC-###). Never renumber or
  reuse an identifier.
- Follow the traceability rules in PC-015: requirements trace to use cases;
  use cases trace to personas and product goals; acceptance criteria trace to
  requirements.
- Markdown conventions: one `#` title per document; keep identifiers verbatim
  in section headings; keep documents single-purpose and free of duplication.

## Execution discipline

- Inspect relevant existing files and approved documentation before changing them.
- Keep each task narrowly scoped to its stated objective.
- For non-trivial changes, present a plan before implementation when requested.
- Run relevant tests, checks, or validation before claiming completion.
- Do not claim that a change works without reporting the evidence used to verify it.
- Summarize changed files, validation performed, assumptions, and unresolved issues.
