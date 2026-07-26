# Frontend documentation — source-of-truth index

This is an index, not a specification. It exists so a reader can tell, in one
place, which frontend document currently governs implementation and which do
not. It defines no product, UX, or implementation decision itself — every
claim below points to the document that actually makes it.

## Current authoritative documents

Read these, and only these, to build the Alert Investigation screen today.

| Document | Governs |
| --- | --- |
| [`alert-investigation-ux-spec.md`](./alert-investigation-ux-spec.md) (v0.2) | The approved UX direction: the **Dark Evidence Map** — the six evidence artifacts, the permanent traceability rail, condition selection and provenance tracing, provenance/traceability states, keyboard behavior, and the desktop-first responsive policy. |
| [`alert-investigation-implementation-plan.md`](./alert-investigation-implementation-plan.md) (v0.3) | The concrete, phased build plan against the current codebase: what domain code is preserved, what is retired, in what order, with what verification at each step. |

Both extend, and must not contradict, the approved Phase 0 product baseline
(`../product.md` and its companions) and the approved Phase 1 architecture
baseline (`../architecture.md`). Neither defines new product scope, a new
requirement, or a new acceptance criterion — see each document's own front
matter for its exact relationship to that baseline.

## The selected prototype

**[`frontend/review-artifacts/dark-evidence-map/`](../../frontend/review-artifacts/dark-evidence-map/)** is the approved visual and interaction direction referenced by `alert-investigation-ux-spec.md` v0.2. It is a local, isolated, gitignored HTML/CSS/JS prototype — not live application code, not itself a specification. Its `review.md` documents the design rationale; `alert-investigation-ux-spec.md` v0.2 is the authoritative, reconciled specification derived from it. If the two ever disagree, the UX spec governs implementation, not the prototype.

## Superseded historical documents

Retained for historical reference only. **Must not guide implementation.**

| Document | Status |
| --- | --- |
| [`product-experience-brief.md`](./product-experience-brief.md) | Superseded by `alert-investigation-ux-spec.md` (first by v0.1, restated by v0.2). Its "Signal Path" creative direction, flagship screen spec, and design system are rejected. Its §2 (backend-capability audit) and §11 (backend-capability plan) remain accurate contract reference material, restated for self-containment in the current UX spec's §2 and §22. |
| `alert-investigation-ux-spec.md` v0.1 | Superseded in place by v0.2 (same file, no separate v0.1 file remains on disk — recoverable from version control). Its "Causal Evidence Dossier" information architecture (Docket Header, Finding, Evidence Concordance, Evidence Register, Inspection Leaf, Traceability Proof) is retired vocabulary; see v0.2 §21. |
| `alert-investigation-implementation-plan.md` v0.1 | Superseded in place by v0.2 (same file). Its build plan targeted the now-retired Causal Evidence Dossier vocabulary. |

## Local review and prototype artifacts

`frontend/review-artifacts/` (gitignored; never a template to copy DOM or CSS
from) holds design-exploration history. All prototype directories below are
**visually rejected** — none of them, including their silhouettes, colors, or
layout patterns, may be refined, hybridized, or reused. Only the Dark
Evidence Map (above) is approved.

| Path | What it is |
| --- | --- |
| `dark-evidence-map/` | **Approved.** See "The selected prototype," above. |
| `alert-investigation-design-decision.md` | A design investigation comparing three now-rejected concepts ("Line of Inquiry," "The Condition Board," "The Cited Claim") against the then-live Forensic Case Folio presentation. Its recommendation (Line of Inquiry) was **not** carried forward — the Dark Evidence Map is a later, independent direction that supersedes this investigation's conclusion. Retained only as a record of that investigation's product-understanding research (its Part 1), which remains factually accurate. |
| `forensic-workbench-prototype/`, `line-of-inquiry-prototype/`, `evidence-lens-study.*`, `evidence-argument-surface.*`, `alert-investigation-visual-reset/` (Directions A, B, C) | Rejected prototypes. Not superseded-with-value the way the documents above are — simply rejected. |

## What is live on disk but not documented by any approved specification

`frontend/src/features/alert-investigation/` still contains a presentation
layer internally called the "Forensic Case Folio" / "Composition Reset" —
built after `alert-investigation-ux-spec.md` v0.1 was approved, but never
itself specified or approved by any document in this index. It is superseded
by `alert-investigation-ux-spec.md` v0.2. The live route (`/alerts/:alertId`)
no longer renders it — the Dark Evidence Map (`InvestigationMap`) replaced it
on the shipped page — but the Folio's files remain on disk, unreferenced,
pending deletion by `alert-investigation-implementation-plan.md` v0.3's Pass 8,
which is itself gated behind that plan's Pass 7 ("Track B Structural Fidelity
Completion"): the shipped Dark Evidence Map is content-correct but not yet
structurally faithful to the UX spec's connector-line, characteristic-bus,
selection-dimming, and provenance-anchoring requirements, and the Folio may
not be deleted until that gap is closed. This is a known, tracked gap, not a
silent one.

The domain and view-model layer beneath that presentation (`lib/*.ts`) is
sound, contract-faithful, and preserved almost unchanged by the current
implementation plan — see that plan's §3.

## Documents this index does not cover

The Phase 0 product baseline (`docs/product.md`, `personas.md`, `use-cases.md`,
`scope.md`, `functional-requirements.md`, `non-functional-requirements.md`,
`acceptance-criteria.md`, `glossary.md`) and the Phase 1 architecture baseline
(`docs/architecture.md`, `docs/adr/*`) are out of scope for this index — they
are not frontend-specific and are not touched by any document listed above.
