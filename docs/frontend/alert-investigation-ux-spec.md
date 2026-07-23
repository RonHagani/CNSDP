# Cloud-Native Security Telemetry and Detection Platform — Alert Investigation UX Specification

| Field | Value |
| --- | --- |
| Document | CNSDP Alert Investigation UX Specification |
| Version | 0.1 |
| Status | Approved — Phase 1.5 UX baseline |
| Phase | Phase 1.5 — Security Investigation Experience |
| Identifier | Not assigned. Deliberately outside the closed PC-015 identifier namespace (PC-###, PD-###, PER-###, UC-###, FR-###, NFR-###, AC-### are not reopened, renumbered, or extended by this document). Referenced by path only: `docs/frontend/alert-investigation-ux-spec.md`. |
| Relationship to baseline | Extends, and must not contradict, the approved Phase 0 product baseline (`../product.md` and its companions) and the approved Phase 1 architecture baseline (`../architecture.md`, ARCH-01). Defines no new product scope, functional requirement, non-functional requirement, or acceptance criterion. Any backend capability this document's open questions touch requires its own separate architecture and implementation approval before backend code is written (§15). |

**Supersession.** This document supersedes `docs/frontend/product-experience-brief.md` §1 (product ambition), §4 (creative direction — "The Signal Path"), §5 (flagship screen specification), §6 (information architecture, as it pertains to the Alert Investigation route), §7 (design system), and §8 (motion system). It does not supersede that document's §2 (backend-capability audit) or §11 (backend-capability plan) — those sections state contract facts that remain accurate today and are restated, for self-containment, in §2 and §15 below. `product-experience-brief.md` is **not deleted or edited by this task**. It remains in the repository for historical reference. A later documentation task should mark it explicitly superseded (e.g., a status-field change or an in-place "Superseded by" note); leaving two contradictory approved creative directions live and unmarked is not the intended end state, but resolving that is out of scope for this document (§14).

**Notation.** Four tags are used throughout to keep the origin of each statement auditable:

- **[Approved]** — restates a decision already binding from `../product.md`, `../use-cases.md`, `../functional-requirements.md`, `../non-functional-requirements.md`, `../scope.md`, or `../architecture.md`. Not a UX choice; cannot be changed by revising this document alone.
- **[UX]** — a decision this document makes. A reasonable alternative existed; this is the chosen one, and future revisions of this document may reconsider it without touching the Phase 0/Phase 1 baseline.
- **[Contract]** — a limit imposed by the current `GET /v1/alerts/{id}` response shape (`frontend/src/types/contract.ts`). Would require a separately approved backend change to lift.
- **[Future]** — explicitly not implemented in v0.1. Named so a later session does not infer it exists.

## 1. Purpose and user outcome

This document defines the v0.1 user experience for the Alert Investigation screen — the platform's one investigation surface — replacing the "Signal Path" schematic/pipeline direction previously defined in `product-experience-brief.md`.

**[Approved]** The screen serves the Security Analyst (PER-001) performing UC-003: reaching an evidence-backed assessment of a detected activity by reviewing the alert's explanation, inspecting the normalized and source telemetry behind it, and following the traceability chain back to the original event. It also serves the Detection Engineer (PER-002) performing UC-002: verifying that a detection matched for its documented reason. The primary task is not observing how an event moved through the platform's processing stages — it is verifying, from evidence, that an existing alert's claim is true.

**[UX]** The screen is a **Causal Evidence Dossier**: a structured record organized around one causal chain, not a dashboard, not a pipeline visualization, not a processing-stage presentation, not a generic SIEM layout, and not an editorial landing page. Every element on the screen exists to answer one of five questions the analyst actually has:

1. **What happened** — the plain-language claim (§3.2, Finding).
2. **Why the detection matched** — which documented conditions were evaluated (§3.3, Evidence Concordance).
3. **Which documented conditions were satisfied** — the concordance between declared conditions and the recorded match (§3.3).
4. **Which observed event facts satisfied them** — the specific normalized-event values behind each satisfied condition (§3.3, §4).
5. **Whether every claim resolves to original source telemetry** — the field-level provenance record (§3.5) and the traceability verification result (§3.6, §7).

The organizing model is a single causal chain, always read in the same direction:

```
CLAIM → DOCUMENTED CONDITION → OBSERVED FACT → SOURCE EVIDENCE
```

- **CLAIM** — the Finding (§3.2): what the platform asserts happened and what it detected, stated in plain language, sourced only from `alert.summary` and `detectionResult.matchReason`.
- **DOCUMENTED CONDITION** — the matched detection definition's declared conditions (§3.3): `operation`, `requires_outcome`, `requires_any`, `requires_all`, exactly as authored and reviewable (PD-04 scope decision 3 — detection definitions are read-only product content).
- **OBSERVED FACT** — the specific `normalizedEvent` values and `detectionResult.matchReason.satisfiedCharacteristics` entries that satisfied those conditions (§3.3, §4.3).
- **SOURCE EVIDENCE** — the original `sourceEvent.rawEvent` field(s) each observed fact was derived from, with the derivation behavior stated explicitly — a verbatim carry or a parsed transformation (§3.5, §4.3).

**[Approved]** Success criterion, restated from UC-003: a reviewer can move from the Finding to a specific matched condition, to the observed fact that satisfied it, to the source evidence it was derived from, to the traceability proof of the whole chain — without leaving the screen (UC-003 main flow; PC-G-005, PC-G-006, PC-G-007). Every value displayed traces to a named field in the current response contract (§2); nothing is fabricated to fill a visual gap.

## 2. Current v0.1 contract boundaries

**[Contract]** The one rich data source for this screen is `GET /v1/alerts/{id}`, returning `AlertInvestigationResponse` (`frontend/src/types/contract.ts`):

```
alertId
sourceEvent          { available, rawEvent? }
validationOutcome    { available, outcome?, reason? }
normalizedEvent      { available, event? }
detectionDefinition  { available, revision?, definition? }
detectionResult      { available, matchReason? }
alert                { available, summary? }
traceability         { intact, failedLink? }
```

The following are binding limits on everything this document specifies. No section below may describe UI that implies a capability beyond this list exists.

- **[Contract]** No alert index or search endpoint exists. The dossier cannot offer alert browsing, filtering, or discovery; an alert ID must already be known (direct navigation) before this screen can render anything.
- **[Contract]** No non-alerting telemetry retrieval path exists. `POST /v1/audit-events` returns only an admission result. A submission that was rejected, flagged incomplete, classified unsupported, or validly processed without matching a scenario has no read-back path. This screen only ever renders on an alert that already exists — it never needs to represent a non-match.
- **[Contract]** No standalone detection-definition catalog exists. A definition is reachable only through an alert that references it, via `detectionDefinition`.
- **[Contract]** No per-stage timestamps exist. No response field carries processing-stage timing or duration. This document specifies no timeline, clock, or elapsed-time element anywhere.
- **[Contract]** No severity or confidence field exists. This document specifies no severity badge, risk score, or confidence indicator anywhere. The only quantifiable signal on the screen is the literal declared-vs-satisfied condition count (§3.3), and it is presented as a checklist, never as a score.
- **[Approved]** The minimum evidence set is exactly six artifacts (PD-04 scope decision 8; FR-031): `sourceEvent`, `validationOutcome`, `normalizedEvent`, `detectionDefinition`, `detectionResult`, `alert`. This set is closed — no seventh artifact may be introduced by this document.
- **[Approved]** Traceability verification (`traceability.intact` / `traceability.failedLink`) is explicitly **not** a seventh artifact (PD-04 scope decision 8; `product-experience-brief.md` §3.3, stated three times in the approved baseline). It describes the integrity and connectivity of the six-artifact set and is always presented as a verification result, never inventoried alongside the six.
- **[Approved]** Each of the six artifacts carries its own independent `available: boolean` (FR-035). No shared or generic "something is missing" placeholder is permitted anywhere in this specification.
- **[Contract]** `traceability.intact` is `true` or `false`; when `false`, `failedLink` is exactly one of `"alert"`, `"source_key"`, `"raw_event_sha256"`. No other failure value exists.
- **[Contract]** `DetectionConditions` (`detectionDefinition.definition.conditions`) may declare any combination of `requires_outcome` (optional), `requires_any` (optional list), and `requires_all` (optional list), including neither list present. Every element of this specification that renders conditions must handle all combinations, not only the one exercised by the currently committed scenario-1 fixture.

## 3. Information architecture

Six elements, always present on a successfully loaded dossier (individual artifacts may still report `available: false` — see §6). Order follows investigative primacy — Finding and Concordance first, because they carry the CLAIM and its DOCUMENTED CONDITION/OBSERVED FACT reasoning — not workflow chronology. **No processing-stage order governs layout or navigation** (§1; carried through every later section).

### 3.1 Docket Header

**[UX]** A compact identity strip, not a masthead. Content, each field independently guarded by its source artifact's availability:

- `alertId` (e.g. "Alert #1").
- The matched scenario id and definition name, from `detectionResult.matchReason.scenario` / `.definitionName` — omitted, with an explicit "detection identity unavailable" statement (FR-035), if `detectionResult.available` is `false`.
- The pinned detection-definition revision (`detectionDefinition.revision`, truncated for display), labeled explicitly as **pinned** — a later definition edit does not change what this alert resolves to (NFR-025).
- A compact traceability-state indicator (intact / broken word plus glyph), which is a pointer to §3.6's full statement, not a duplicate explanation of it.

Explicitly excluded from the Docket Header: severity, confidence, any timestamp/duration figure, a decorative verification seal.

### 3.2 Finding

**[UX]** The CLAIM, in full plain-language prose. Built entirely from `alert.summary` (`subject`, `operation`, `target`, `outcome`, `requestTime`) and `detectionResult.matchReason.definitionName` / `.scenario`. **Always visible on load** — never behind a click, never collapsed by default.

- **[Approved]** The recorded outcome and time are always stated (FR-029), even for scenario 1, whose match condition does not depend on outcome (FR-024) — the outcome is still part of "what was detected," independent of whether it was a match condition.
- If `alert.available` is `false`, the Finding renders an explicit "alert summary unavailable" statement (FR-035) rather than a blank region.
- **[Approved]** Tone constraint (NFR-031): declarative, sourced from the response only. No adjective implying a risk level absent from the data ("critical," "high-risk" as invented labels). Only the platform's own documented vocabulary is used: `valid` / `invalid` / `incomplete` / `unsupported`, matched / not matched, `intact` / broken (named `failedLink`).

### 3.3 Evidence Concordance

**[UX]** Renders DOCUMENTED CONDITION → OBSERVED FACT as a structured comparison, not a chain visualization, not a ribbon, not a decorative checklist icon set. This is the screen's causal core, and it contains **only conditions actually satisfied by the current detection result** — it is a record of matched evidence, not a rendering of the full definition text (the full literal text, satisfied or not, lives in the Detection Definition Inspection Leaf — §6 item 4).

**Terminology.** Two separate concepts, never conflated:

- **Declared** — a condition or characteristic named in `detectionDefinition.definition.conditions` (`operation`, `requires_outcome`, `requires_any`, `requires_all`). Declaring a characteristic does not mean this alert's event satisfied it.
- **Satisfied** — a characteristic whose `id` appears in `detectionResult.matchReason.satisfiedCharacteristics` for this specific detection result. Only satisfied characteristics are matched evidence.

**[Approved]** Declared clauses combine with logical AND (FR-024–FR-026): a matching detection result requires the operation to match, `requires_outcome` (if declared) to hold, `requires_any` (if declared) to have at least one satisfied entry, and `requires_all` (if declared) to have every entry satisfied. Because a produced alert already proves this, every row the Concordance renders is, by construction, satisfied — there is no per-row satisfied/unsatisfied marking within the Concordance itself.

An unsatisfied declared characteristic (possible within a `requires_any` group, whose semantics require only *at least one* declared option to be present) is real, reviewable product content — but it belongs in the Detection Definition Inspection Leaf's full literal text, never in the Evidence Concordance. The Concordance would misrepresent an unsatisfied declared condition as matched evidence if it rendered one.

**Rendering algorithm.** Applies identically to every scenario, with no scenario-specific UI branch (§5, §15 item 7). Every declared-condition combination the contract permits (§2) is enumerated below, so no ambiguity remains for implementation:

| Declared shape | Evidence Concordance content |
| --- | --- |
| `requires_any` only | Operation row + one row per **satisfied** entry from the declared `requires_any` list. |
| `requires_all` only | Operation row + one row per entry from the declared `requires_all` list (by construction, satisfied — see AND-combination, above). |
| `requires_outcome` only | Operation row + outcome row. No characteristic rows; no "N of M" summary line (nothing to count). |
| `requires_any` + `requires_outcome` | Operation row + outcome row + satisfied `requires_any` rows. |
| `requires_all` + `requires_outcome` | Operation row + outcome row + `requires_all` rows. |
| `requires_any` + `requires_all` together | Operation row + **two separately labeled groups**, one per clause — never merged into one undifferentiated list, since "at least one" and "all of these" are different satisfaction semantics and merging them would misstate which rule governed the match. |
| Neither `requires_any` nor `requires_all`, no `requires_outcome` | Operation row only. No summary line. |

Row-level detail:

1. **Operation-match row** — always rendered when `detectionDefinition.available`: the declared `conditions.operation` (`resource`, `subresource?`, `verb?`). Not a characteristic with an `id`; it is a structural fact about which operation this detection evaluates, not something individually marked satisfied/unsatisfied.
2. **Outcome row** — rendered only if `conditions.requires_outcome` is declared; states the required outcome. **[UX]** Outcome is shown as a causal condition in the Concordance *only* when the definition declares it as required — this is distinct from the Finding (§3.2), which always states the recorded outcome as part of "what happened" regardless of whether it is a match condition (FR-029). The same underlying `outcome` value serves two different purposes in two different places; neither substitutes for the other.
3. **`requires_any` / `requires_all` rows** — every row shown is, by definition, satisfied (per the table above); each states the characteristic's declared `id` and `description` from `detectionDefinition`, plus the corresponding entry from `detectionResult.matchReason.satisfiedCharacteristics`. **The number of rows is never assumed** — it may be one, two, five, or any other count the response actually contains; nothing in this algorithm is sized to exactly two.

A summary line ("N of M declared characteristics satisfied") may caption the row list when `M > 0` — a literal count derived from the declared-list length and the satisfied-row count, never a percentage, never styled as a score, and never implying the unsatisfied `M − N` are shown anywhere in the Concordance (they are not — see Terminology, above).

- **[UX] Selection.** Every row in the Concordance is, by construction, satisfied, so every row is selectable into a provenance Inspection Leaf (§3.5, §4.3). Selectability depends only on `detectionResult` reporting the characteristic as satisfied — it does **not** depend on whether `sourceEvent` or `normalizedEvent` are independently available. Selecting a row when one of those artifacts is unavailable still opens a leaf; it reports Unavailable provenance (§3.5) rather than fabricating content, consistent with FR-035's requirement that a limitation be visible, not silently withheld from selection entirely.
- **[UX] Default state.** Because Detection Result is the default-selected Evidence Register entry (§3.4, §4.1), its Evidence Concordance is what is visible on load, satisfying the requirement that the Finding and the matched-condition concordance are both visible without interaction.
- **[UX]** The Evidence Concordance remains visible regardless of which Evidence Register entry the analyst later selects — it is pinned, causal-core content, not a per-artifact detail pane that disappears when inspecting a different artifact.

### 3.4 Evidence Register

**[UX]** The six-artifact ledger — literal FR-031/FR-035 visibility, rendered as a list or table with hairline rule dividers, never a card grid, never a "ribbon," never a staggered-entrance procession.

Exactly six ordered entries, each showing a fixed accession index (1–6, never reordered), a name, a one-line real-content caption drawn from actual field values, and an availability state:

| # | Entry | Source | Caption content (illustrative, not literal copy) |
| --- | --- | --- | --- |
| 1 | Source event | `sourceEvent` | operation verb + truncated `auditID` |
| 2 | Validation outcome | `validationOutcome` | the outcome value |
| 3 | Normalized event | `normalizedEvent` | operation verb + target resource |
| 4 | Detection definition | `detectionDefinition` | truncated pinned revision |
| 5 | Detection result | `detectionResult` | matched scenario id |
| 6 | Alert | `alert` | `#{alertId}` |

- **[UX] Selection model.** Exactly one entry is "current" at a time. Selecting an entry updates the current selection and governs what the Inspection Leaf shows for it (§4.2). **Detection Result (#5) is current by default.**
- **[Approved]** Traceability is deliberately absent from this register — it is not a seventh artifact (§2). A single reference line pointing to §3.6 may appear as a register footer, never as a numbered seventh row.
- Unavailable entries (`available: false`) remain in the register at their fixed accession index, visibly marked unavailable — never omitted (FR-035).

### 3.5 Inspection Leaf

**[UX]** The single, on-demand, secondary detail surface. This is the only place large or literal content (full raw JSON, full field lists, full definition text) is shown. **Only one Inspection Leaf is open at a time**; opening a new one closes whichever was previously open.

An Inspection Leaf takes one of two forms:

**a) Artifact detail**, for whichever Evidence Register entry is current (§3.4, §6):
- Compact artifacts (validation outcome, detection definition, detection result, alert) render their detail directly when selected — no further toggle needed.
- Voluminous artifacts (source event, normalized event) render a compact summary plus an explicit, separate "Inspect full source record" / "Inspect full normalized record" affordance; the full record (via `JsonTree` for source, a field list for normalized) opens only on that explicit action — **full records are secondary, on-demand views, never auto-expanded.**

**b) Matched-condition provenance record**, opened by selecting a satisfied row in the Evidence Concordance (§3.3). It contains exactly four things, and nothing else:
1. The raw path and value (from `sourceEvent.rawEvent`).
2. The transformation-or-preservation behavior — stated as either a verbatim carry or a specific parsed transformation (e.g., "parsed from the `requestURI` query parameter," never a generic "processed").
3. The normalized path and value (from `normalizedEvent.event`) that resulted.
4. The documented detection condition text (from `detectionDefinition.definition.conditions`) that this observed fact satisfies.

**Provenance states.** Every matched-condition provenance record (b) renders in exactly one of three states, computed deterministically from artifact availability and whether a concrete lineage mapping exists for the normalized field involved (§13). **No raw field path, value, transformation, or source mapping is ever invented** to fill a state the data does not support.

- **Verified provenance** — `sourceEvent.available` and `normalizedEvent.available` are both `true`, and a concrete lineage mapping identifies a specific raw path for the normalized field involved. All four items of (b) render in full.
- **Partial provenance** — `sourceEvent.available` and `normalizedEvent.available` are both `true`, the documented condition and the observed normalized fact are both known, but no lineage mapping identifies a guaranteed raw field path for it (today: scenario 2 and 3's `requestObject`-derived characteristics — §15 item 6). Items 3 and 4 of (b) — normalized path/value and documented condition — still render in full, because both are genuinely known. Item 1 (raw path/value) is replaced by a single explanatory sentence, in the platform's own restrained product language, naming the limitation (in substance: this field's source location cannot be structurally identified in the current raw-event contract). Item 2 (transformation behavior) is omitted rather than guessed — asserting a verbatim-carry or parsed-transform behavior for an unidentified mapping would itself be an invented claim. **Partial provenance is a visible limitation and must never be rendered as if it were a complete, successful end-to-end mapping.**
- **Unavailable provenance** — `sourceEvent.available` or `normalizedEvent.available` is `false`, so the relationship cannot be inspected at all, regardless of whether a lineage mapping exists. The leaf states plainly which artifact is unavailable (reusing §6's per-artifact unavailable wording) rather than attempting a partial render.

**[UX]** None of the three states is rendered with a generic warning card, badge, icon, or severity level (§11). Differentiation between Verified and Partial is carried entirely by label and copy — a Partial record reads visibly differently in prose from a Verified one — not by a new color or icon vocabulary layered on top of §11's single restrained accent.

**[Approved] Provenance is not traceability.** A missing field-level raw-path mapping (Partial provenance) is a distinct fact from a broken alert-to-source artifact link (`traceability.intact === false`, §7). Partial provenance can occur on an alert whose traceability is fully intact — it means the platform has not identified *which specific raw field* produced a normalized fact, not that the source event's integrity or connectivity is in question. The two are never conflated in rendering or in copy (§7).

- **[UX] Nothing is preselected on load.** On initial render, zero Inspection Leaves are open. The Evidence Concordance (§3.3) is not itself an Inspection Leaf — it is pinned, always-visible content — so this constraint and "Detection Result is default-selected" (§3.4) are both satisfied simultaneously without contradiction: the register marks Detection Result current, but opening its literal artifact detail (the raw `matchReason` payload, distinct from the interpreted Concordance) remains a deliberate, on-demand action.

### 3.6 Traceability Proof

**[UX]** A plain, declarative verification statement — never a chain-link graphic, never a glowing pill, never a decorative seal.

- **[Approved]** Content: whether `traceability.intact` is `true` or `false`; when `false`, the specific named `failedLink` value with its plain-language meaning (§7). Framed explicitly as live-verified on every retrieval, not a cached flag (ARCH-01 §4 — traceability is recomputed independently on every `GET /v1/alerts/{id}` call).
- **[UX]** Position: last in reading order — the terminal verification of the whole dossier's SOURCE EVIDENCE resolution, matching UC-003's main flow ("follows the traceability chain … reaches an evidence-backed assessment").
- Not selectable or expandable. It is a terminal statement, not a navigation entry, and it is not one of the six Evidence Register rows (§2, §3.4).

## 4. Detailed interaction behavior

### 4.1 Load

On a successful retrieval: Docket Header, Finding, and the Evidence Concordance for the default-selected Detection Result artifact render immediately, alongside the Evidence Register (Detection Result marked current) and Traceability Proof. Zero Inspection Leaves are open. No field is preselected within any concordance row or record.

### 4.2 Evidence Register selection

Selecting any of the six entries:

1. Updates the "current" marker to that entry.
2. Closes any currently open Inspection Leaf.
3. For a compact artifact (validation outcome, detection definition, detection result, alert): opens that artifact's literal-content Inspection Leaf immediately.
4. For a voluminous artifact (source event, normalized event): shows the compact summary already visible in the register row, plus the on-demand "Inspect full record" affordance (§3.5a) — the full record itself opens only on a further explicit action.
5. Selecting an **unavailable** entry (`available: false`) still opens an Inspection Leaf — one that states, per FR-035, exactly which artifact is unavailable and does not attempt to render partial or reconstructed content.

### 4.3 Evidence Concordance row selection

Selecting a row (every rendered row is satisfied — §3.3):

1. Closes any currently open Inspection Leaf.
2. Opens the matched-condition provenance record (§3.5b), rendered per its computed provenance state (§3.5) — Verified, Partial, or Unavailable.
3. Does not change the Evidence Register's "current" marker — the Register and the Concordance are independent selection axes; only one Inspection Leaf may be open regardless of which axis triggered it.

Unsatisfied and declared-but-absent characteristics never render as Evidence Concordance rows (§3.3) — there is nothing to select. They remain inspectable, in full, via the Detection Definition Inspection Leaf (§6 item 4).

### 4.4 Mutual exclusivity

Exactly one Inspection Leaf is rendered at any time, regardless of whether it was opened from the Evidence Register (§4.2) or the Evidence Concordance (§4.3). Opening any new leaf closes the previous one unconditionally.

### 4.5 Traceability Proof

Read-only. Not selectable, not expandable, always rendered in full once the response has loaded, independent of which artifact is current or which leaf is open (§3.6).

### 4.6 Command palette

See §10 for full behavior. In summary: jump to a specific Evidence Register entry (opens it exactly as a direct selection would, §4.2), jump to the Traceability Proof, jump to the Finding/top of the dossier, or clear the currently open Inspection Leaf. No command references a processing stage (§1, §14).

### 4.7 Retrieval retry

Route-level retrieval failure (§8) offers a retry action that re-issues the same `GET /v1/alerts/{id}` request; this is independent of, and does not reset, in-dossier selection state, since a retrieval failure means no dossier has rendered yet.

## 5. Scenario-aware content model

**[UX]** The Evidence Concordance (§3.3) and Inspection Leaf (§3.5) mechanisms are generic: driven entirely by whichever `conditions` and `satisfiedCharacteristics` the response actually contains, and by whichever single normalized-event scenario block (`exec` | `podCreation` | `clusterRoleBinding`) is populated. No scenario-specific UI branch exists anywhere in this specification. This section documents how the generic mechanism instantiates for each of the three currently supported scenarios, as a truthfulness and testability aid — not as three separate implementations. **[Approved]** This remains truthful to the single currently supported telemetry source family (Kubernetes API-server audit events, PD-04) while remaining structurally extensible: a future fourth normalized-event characteristic block or scenario id requires no redesign of the Concordance, Register, or Inspection Leaf mechanisms themselves (§15 item 9).

### 5.1 Scenario 1: interactive `pods/exec`

- Operation match: `resource: "pods"`, `subresource: "exec"`.
- **[Approved]** No `requires_outcome` is documented — this scenario matches regardless of the recorded outcome (FR-024). The Concordance correctly omits the outcome row (§3.3 step 2) rather than inventing one; the recorded outcome is still shown in the Finding (§3.2) because FR-029 requires it in the alert statement regardless of match logic.
- `requires_any`: up to two declared characteristics (`stdin_streaming`, `tty_allocation`). The Concordance renders however many are actually declared and satisfied for this alert — zero, one, or two — never assuming exactly two.
- Observed facts: `normalizedEvent.event.exec.{stdin, tty}` (booleans).
- **[Approved]** Source evidence: both derived by parsing the `requestURI` query string (ADR-0003 Q1), not from `requestObject`. The provenance record's transformation-behavior text must say "parsed from the `requestURI` query parameter," not "carried verbatim."
- **[UX] Provenance state (§3.5):** Verified, whenever both `sourceEvent` and `normalizedEvent` are available — the `requestURI` query string is a plain, always-addressable string field, so a concrete raw path is always identifiable for these two characteristics.

### 5.2 Scenario 2: high-risk Pod creation

- Operation match: Pod-creation request.
- **[Approved]** `requires_outcome`: successful completion is documented (FR-025) — the Concordance renders this row and marks it satisfied only when the recorded outcome indicates success.
- `requires_any`: up to five declared characteristics (`privileged`, `hostNetwork`, `hostPID`, `hostIPC`, a host-path-volume characteristic). The Concordance renders however many are declared and satisfied — this scenario is the concrete case that makes "never assume exactly two" a real constraint, not a hypothetical one.
- Observed facts: `normalizedEvent.event.podCreation.{privileged, hostNetwork, hostPID, hostIPC, hostPathVolume}` (booleans).
- **[Approved]** Source evidence: each characteristic is carried, not parsed, from the corresponding field of the Pod specification (FR-018).
- **[UX] Provenance state (§3.5):** Partial, whenever both `sourceEvent` and `normalizedEvent` are available — `RawAuditEvent.requestObject` is typed `unknown` (§15 item 6), so no structurally verified raw path exists for these characteristics today. The Concordance row and the normalized fact still render in full; the raw-path/value and transformation-behavior items are replaced by the explanatory sentence §3.5 defines, never fabricated.

### 5.3 Scenario 3: cluster-admin `ClusterRoleBinding` creation

- Operation match: `ClusterRoleBinding`-creation request.
- **[Approved]** `requires_outcome`: successful completion is documented (FR-026).
- **[UX]** This document does not assume whether the cluster-admin role match is expressed as a declared `requires_any`/`requires_all` characteristic or determined purely by the operation-and-outcome match with the role check embedded in matching logic not further decomposed into a characteristic (§15 item 7). The Concordance's generic algorithm (§3.3) handles either shape without modification: if characteristics are declared, they render as rows; if none are declared, the Concordance shows only the structural rows.
- Observed facts / identity fields: `normalizedEvent.event.clusterRoleBinding.{bindingName, roleRef, subjects}`. These are **identity facts** (which binding, which role, which subjects), not boolean flags — they render within the Finding and the Normalized Event Inspection Leaf as identity fields, the same way subject/operation/target are always shown without being conditions themselves; they are not automatically rendered as Concordance rows unless the definition actually declares them as characteristics.
- **[UX] Provenance state (§3.5):** Partial, for the same reason as scenario 2 (§5.2, §15 item 6), whenever a declared characteristic exists and both `sourceEvent` and `normalizedEvent` are available. Where scenario 3's definition declares no characteristics at all (§3.3's "neither declared" table row), the Concordance renders only structural rows and no provenance record applies — there is no matched-condition row to select.

**[Approved]** No scenario requires this screen to represent a non-match. Only matching detection results produce alerts (FR-027), and this screen only ever renders on an existing alert — AC-013 branch (a)'s non-match case has no corresponding content model here by design, not by omission.

## 6. Evidence-artifact behavior

Each of the six artifacts is independently available (§2). This section defines Evidence Register caption content and Inspection Leaf detail content for each; §8 defines the shared unavailable-state wording pattern.

1. **Source event** — Register caption: operation verb + truncated `auditID`. Inspection Leaf: the full raw event via `JsonTree` (§13), collapsible, including whichever fields are present (`user`, `objectRef`, `responseStatus`, `requestObject`, `annotations`, etc.).
2. **Validation outcome** — Register caption: the outcome value. Inspection Leaf: outcome plus `reason` when present. **[Approved]** A `valid` outcome legitimately carries no `reason` (FR-011, FR-012) — this is not treated as a missing field. The Leaf renders all four possible outcome values generically (`valid`/`invalid`/`incomplete`/`unsupported`); it does not assume `valid` is the only value the contract can return for this field, even though every real alert-producing submission is expected to have been classified valid (FR-014) before reaching detection.
3. **Normalized event** — Register caption: operation verb + target resource. Inspection Leaf: compact summary (`subject`, `operation`, `target`, `outcome`, `requestTime`) with an on-demand full-record toggle (§3.5a) revealing every present field, including whichever single scenario-specific block (`exec` | `podCreation` | `clusterRoleBinding`) is populated, rendered generically rather than hardcoded to one.
4. **Detection definition** — Register caption: truncated pinned revision + scenario id. Inspection Leaf: `name`, `description`, and the full literal `conditions` object (`operation`, `requires_outcome`, `requires_any`, `requires_all`, whichever are present), labeled pinned (NFR-025). This is the only place an unsatisfied declared characteristic is inspectable — the Evidence Concordance (§3.3) never renders one.
5. **Detection result** — Register caption: matched scenario id. Inspection Leaf (opened on explicit selection, not auto-opened by default — §3.5): the literal `matchReason` payload (`scenario`, `definitionName`, `definitionRevision`, full `satisfiedCharacteristics` list). This is the raw artifact; the interpreted declared-vs-satisfied comparison lives in the Evidence Concordance (§3.3), not here.
6. **Alert** — Register caption: `#{alertId}`. Inspection Leaf: the full `alert.summary` fields as a literal artifact view, distinct from the Finding's plain-language prose rendering of the same underlying data.

## 7. Traceability behavior

- **[Contract]** `traceability.intact: boolean`; when `false`, `traceability.failedLink` is exactly one of `"alert"`, `"source_key"`, `"raw_event_sha256"`.
- **[Approved]** Traceability is computed independently of the six artifacts' individual availability, and is recomputed on every retrieval (ARCH-01 §4). Traceability Proof (§3.6) always renders once the response has loaded, regardless of which artifacts are `available`.
- **Intact:** states plainly that the alert resolves through its detection result and normalized event to the source submission, and that the re-derived source identity and raw-event digest both match their stored records (NFR-017).
- **Broken:** states the specific `failedLink` with its plain-language meaning — reused verbatim from the legacy implementation's proven-correct mapping (`ProofChain.tsx`'s `FAILED_LINK_EXPLANATION`), as text, not as a chain-link graphic:
  - `alert` — the alert itself does not resolve through the chain.
  - `source_key` — the submission's re-derived source identity no longer matches its stored record.
  - `raw_event_sha256` — the stored raw event no longer matches its recorded integrity digest.
- **[UX]** Framing requirement: the statement must say the verification is live, computed on this retrieval, and is not a cached flag — this is a factually correct description of ARCH-01 §4's behavior and must be preserved.
- **[Approved]** Traceability and artifact availability are independent facts and must never be conflated. A 6-of-6-available alert can still have broken traceability (this is exactly fixture 3's documented case); the UX shows both facts distinctly, side by side, never letting a complete Evidence Register imply intact traceability or vice versa (FR-035).
- **[Approved]** Traceability (this section) and per-condition provenance state (§3.5) are independent concepts computed at different levels: traceability verifies the alert-to-source artifact chain as a whole (`traceability.intact`/`failedLink`); provenance state describes whether one specific matched condition's raw field can be identified within an already-available source artifact. A row reporting Partial provenance never implies `traceability.intact === false`, and intact traceability never implies every condition row has Verified provenance — the two axes render independently and must never be conflated in copy or presentation (§3.5).

## 8. Loading, unavailable, partial, broken, unauthorized, not-found, and retrieval-failure states

Two tiers of state, rendered differently:

**Route-level states** (no dossier content exists yet) — each gets its own distinct full screen:

| State | Trigger | Rendering |
| --- | --- | --- |
| Loading | Query pending | A plain, non-decorative loading indicator with an `aria-live` status region. No schematic or rail-based loading animation. |
| Not found | `AlertFetchError.kind === "not-found"` | Distinct screen naming the specific requested alert id. |
| Unauthorized | `AlertFetchError.kind === "unauthorized"` | Distinct screen (NFR-012, NFR-013). No retry action — retrying with the same missing or invalid credential cannot succeed. |
| Unavailable / retrieval failure | `AlertFetchError.kind === "unavailable"`, or any other thrown error | Distinct screen with a working Retry action, and an explicit statement that no internal error detail is available for display (NFR-022 — never leak implementation detail). |

**Artifact-level and chain-level states** (a dossier has rendered) — rendered inline, never as a separate screen:

| State | Trigger | Rendering |
| --- | --- | --- |
| Artifact unavailable | Any of the six `available: false` | That artifact's Evidence Register row and Inspection Leaf render an explicit, artifact-named unavailable statement (§6), never a blank or a shared placeholder. |
| Partial availability | Some but not all six `available: false` | No dedicated "partial" screen — this is simply the composition of each artifact's own state (§6) within the normal dossier layout. |
| Broken traceability | `traceability.intact === false` | Rendered entirely by Traceability Proof (§3.6, §7) within the normal dossier layout; the six artifacts continue to render per their own individual availability. |

**[Approved]** Every one of the seven `available`-guarded states plus the two traceability states has its own distinct, named rendering (FR-035). No state — route-level or artifact-level — is ever collapsed into a shared generic "something went wrong" placeholder.

## 9. Desktop and mobile responsive behavior

- **[UX]** Single reading order at every width: Docket Header → Finding → Evidence Concordance → Evidence Register → Traceability Proof, with the active Inspection Leaf appearing inline, adjacent to whatever triggered it — never as a separate page or route.
- **[UX]** No sidebar navigation is introduced anywhere in this document — this remains a single-route v0.1 experience (§2).
- **Desktop (wide):** the Evidence Register may render with more caption detail per row; the Inspection Leaf may render in a secondary column beside the Register when width allows. Exactly one leaf is still open.
- **Narrow / mobile:** the Evidence Register collapses to a single-column list, six ordered rows with accession numerals preserved; the Inspection Leaf renders as an inline expanding region directly beneath the selected Register row — a deliberately simpler collapse than a bottom-sheet reinterpretation, since there is no stage rail to collapse from in the first place (§14).
- **[Approved]** No information is dropped at any breakpoint: every artifact's availability state, every Concordance row, and the Traceability Proof statement remain reachable at every tested width.
- No layout introduces page-level horizontal scrolling; the raw-event `JsonTree` content scrolls within its own bounded region, as it does today.

## 10. Keyboard and command-palette behavior

- **[UX]** The full dossier is reachable and operable without a mouse: Tab / Shift+Tab moves through Register entries and Concordance rows in document order; Enter or Space activates a focused entry exactly as a click does.
- Escape closes the topmost open interactive surface: the command palette if it is open, otherwise the currently open Inspection Leaf, returning the dossier to its zero-leaves-open default (§4.1).
- `⌘K` / `Ctrl+K` opens the command palette. **[UX]** The palette mechanism itself (`components/CommandPalette/CommandPalette.tsx` — Radix Dialog primitive, filterable list, Arrow-key selection, Enter to run, mouse-hover sets active index) is reused unchanged (§13); only its command list is rebuilt.
- **[UX]** Command list for this dossier: one jump command per Evidence Register entry (by artifact name, e.g. "Focus evidence: Source event" — opens it exactly as §4.2 describes), one jump-to-Traceability-Proof command, one jump-to-Finding command, and one clear-active-Inspection-Leaf command. **No command references a processing stage or a capability this screen does not have** — carried forward as a binding rule from the superseded brief because it was correct and is not tied to the rejected visual system. Given §2's constraints, the palette does not offer cross-alert navigation — no alert list exists to jump within.
- **[UX]** Every interactive Register row, Concordance row, on-demand toggle, and palette entry has a visible, high-contrast focus state. This is a standard accessibility affordance, not a "glow" — it is not excluded by §11's prohibition on decorative glow.
- **[UX]** Reduced-motion parity: any transition used to open or close an Inspection Leaf has an instant-change fallback carrying the identical information, using the existing `useReducedMotion` pattern already present throughout the codebase (§13).

## 11. Visual-language constraints

Binding on every element specified above.

**Prohibited, without exception:**

- Serif typography anywhere.
- Glow, neon, or gradient treatments anywhere.
- A curved conduit or stage rail — no SVG-path-based navigation spine.
- Evidence cards — the Evidence Register is a ledger with hairline rules, never a card grid or tile procession.
- Glowing proof-chain pills or any chain-link graphic for traceability — Traceability Proof is a text statement (§3.6, §7).
- A decorative verification seal.
- KPI cards, invented metrics, or any severity/confidence indicator (§2 — none exists in the contract).
- A generic warning card, badge, icon, or severity level for a provenance limitation (§3.5) — Partial and Unavailable provenance are distinguished by label and copy only, within the same single-accent constraint below, never by a new color or icon vocabulary.
- Sidebar navigation, for this single-route v0.1 experience.
- Oversized display headings or large, empty, vertically separated presentation sections.

**Prescribed instead:**

- One sans-serif typeface for interface text.
- Monospace reserved for technical/verbatim values: raw field values, revision hashes, audit IDs, dot-paths, accession indices.
- Accession indices (1–6, fixed — §3.4).
- Citation-style leaders/labels (e.g., the Docket Header's compact identity line, §3.1).
- Hairline rule dividers, not shadows or elevation, to express structure.
- Controlled, deliberate indentation expressing the raw → normalized → condition hierarchy within a provenance record (§3.5b).
- Exactly one restrained accent treatment, reserved solely for whichever element is currently active or selected (a Register row, a Concordance row, or an open Inspection Leaf) — not a reused semantic-status color system applied decoratively elsewhere.

**Status language:** exactly the platform's own real vocabulary — `valid` / `invalid` / `incomplete` / `unsupported`; satisfied / not satisfied (per condition); `intact` / broken, named by `failedLink` — never invented synonyms, never a traffic-light or severity metaphor (NFR-031).

**[UX] Distinctiveness principle.** The screen's identity comes from making causal evidence structure and field-level provenance literally inspectable as the primary interaction — not from a cyber-security aesthetic. This replaces the superseded brief's claim ("no other product renders its own state machine as a schematic") with the corresponding claim for this direction: no other product lets an analyst select a matched condition and see its exact raw-to-normalized derivation inline, as the primary interaction, without leaving the screen.

## 12. Frontend acceptance criteria

These are frontend-scoped, locally numbered `UX-AC-#` criteria — **not** part of the closed PD-07 `AC-###` namespace and not a proposal to extend it. Each is independently checkable against the rendered screen.

1. On load, Docket Header, Finding, Evidence Concordance (for Detection Result), Evidence Register (Detection Result marked current), and Traceability Proof are all visible; zero Inspection Leaves are open; no field is preselected.
2. Selecting any Evidence Register entry updates the current selection; compact artifacts open their Inspection Leaf immediately; source/normalized event reveal an on-demand full-record toggle rather than auto-expanding.
3. At most one Inspection Leaf is ever rendered open; opening a new one closes any previously open leaf.
4. Selecting a Concordance row opens a provenance leaf whose content matches its computed provenance state (§3.5): Verified renders all four items (raw path/value, transformation behavior, normalized path/value, documented condition); Partial renders the normalized path/value and documented condition plus a single explanatory limitation sentence in place of the raw path/value, with no transformation behavior claimed; Unavailable renders an explicit named-artifact-unavailable statement.
5. Unsatisfied declared characteristics never render as Evidence Concordance rows — they are inspectable only via the Detection Definition Inspection Leaf's full literal text, never as matched evidence (§3.3).
6. The Evidence Concordance renders correctly for a definition declaring only `requires_outcome` with no `requires_any`/`requires_all` — no fabricated "0 of 0" count.
7. The Evidence Concordance renders correctly for each of the seven declared-condition combinations in §3.3's table (`requires_any` alone, `requires_all` alone, `requires_outcome` alone, each paired with `requires_outcome`, `requires_any` and `requires_all` together as two separately labeled groups, and no declared characteristic list at all), for any satisfied-row count including counts other than two.
8. Each of the six Evidence Register entries renders its own explicit, artifact-named unavailable state when `available: false` — never a shared blank.
9. Traceability Proof correctly renders `intact: true` and each of the three `failedLink` values with the correct plain-language explanation.
10. Traceability state and artifact availability render independently — a 6-of-6-available alert with broken traceability shows both facts, and neither masks the other.
11. The four route-level states (loading, not-found, unauthorized, unavailable) each render a distinct screen; unavailable includes a working Retry action; unauthorized does not offer a retry.
12. No severity, confidence, per-stage timestamp, alert-count, index, or search affordance is rendered anywhere on the screen.
13. Every interactive element (Register row, Concordance row, toggle, palette entry) is reachable and operable by keyboard alone, with a visible focus state.
14. `⌘K` / `Ctrl+K` opens the command palette; Escape closes the topmost open surface (palette, else the active Inspection Leaf).
15. No stage-rail, conduit, ribbon, chain-graphic, card-grid, or KPI element exists in the rendered DOM.
16. Reduced-motion users receive an instant equivalent for every open/close transition, carrying identical information.
17. No tested breakpoint drops an artifact's availability state or a Concordance row from reachability.
18. All rendered text traces to a named field in `AlertInvestigationResponse`, `RawAuditEvent`, `NormalizedEvent`, `DetectionDefinition`, or `MatchReason` — zero fabricated content.
19. A satisfied Concordance row with a structurally known raw-to-normalized mapping, and both `sourceEvent` and `normalizedEvent` available, renders Verified provenance with all four items populated.
20. A satisfied Concordance row whose normalized field has no known raw-path mapping, with both artifacts available, renders Partial provenance: the normalized path/value and documented condition populate; the raw-path/value and transformation-behavior items are replaced by a single explanatory sentence; nothing is fabricated.
21. A satisfied Concordance row where `sourceEvent.available` or `normalizedEvent.available` is `false` renders Unavailable provenance: an explicit statement naming which artifact is unavailable and that the relationship cannot currently be inspected.
22. No provenance state — Verified, Partial, or Unavailable — is ever rendered with a generic warning card, badge, icon, or severity level; differentiation is by label and copy only, within §11's single-accent constraint.
23. Partial and Unavailable provenance are never rendered or worded in a way that implies `traceability.intact` is `false`; a row's provenance state and the alert's traceability result render independently (§7).
24. Scenario 1's `exec`-derived rows render Verified provenance whenever both `sourceEvent` and `normalizedEvent` are available; scenario 2's `podCreation`-derived and scenario 3's `clusterRoleBinding`-derived rows render Partial provenance under the same availability condition, per the current lineage-mapping limits (§5, §15 item 6).

## 13. Mapping from current reusable implementation to the new UX

| Existing module | Disposition | New role |
| --- | --- | --- |
| `frontend/src/types/contract.ts` | Keep as-is | Unchanged wire-type contract; this specification introduces no new field. |
| `lib/alertSource.ts` (`fetchAlertInvestigation`, `AlertFetchError`) | Keep as-is | Unchanged fetch seam and three-way error taxonomy; §8's route-level states map 1:1 to `AlertFetchError.kind`. |
| `hooks/useAlertInvestigation.ts` | Keep as-is | Unchanged TanStack Query hook. |
| `fixtures/alert-investigation/v1.ts` | Keep as-is | The intact/partial/broken-traceability fixtures already cover §8's artifact- and chain-level states; no new fixture variant is required by this document. |
| `lib/lineage.ts` (`buildLineageLinks`, `resolveHighlights`) | Keep, extend | Becomes the provenance-state engine behind §3.5's Verified/Partial/Unavailable model (§3.5, §4.3). Its field-based construction already produces Verified provenance for scenario 1's `requestURI`-derived fields; scenario 2/3's `podCreation`/`clusterRoleBinding` fields correctly resolve to Partial provenance today, per §15 item 6's `requestObject`-typing gap, and move to Verified only if a documented/typed `requestObject` shape becomes available — not by frontend mapping effort alone. |
| `lib/deriveStageStatus.ts` | Extract logic, rename | Its availability/outcome boolean logic (all-six-available check, per-artifact availability, validation-outcome branching) is preserved. Its `SignalPathStageId` keying and `signal`/`branch`/`severed` vocabulary are dropped — no stage concept survives (§14). |
| `components/JsonTree.tsx` | Keep as-is | Unchanged; becomes the Source Event Inspection Leaf's full-record renderer (§6), and remains available generically for `requestObject`-derived scenario blocks (§15 item 6). |
| `components/CommandPalette/CommandPalette.tsx` | Keep as-is | Unchanged mechanism (§10); only its command list (`useInvestigationCommands.ts`) is rewritten. |
| `AlertInvestigationPage.tsx` query/error branching (`isPending`/`isError`/`isSuccess`, `AlertFetchError.kind` switch) | Keep, recompose | Unchanged orchestration logic; only the success-branch composition changes to the six §3 elements. |
| `app/router.tsx` | Keep as-is | Unchanged single-route model; this document adds no route (§2, §9). |
| Existing test intent (`alertSource.test.ts`, `deriveStageStatus.test.ts`, `lineage.test.ts`, `AlertInvestigationPage.test.tsx`'s seven-state coverage, `LineageInteraction.test.tsx`'s provenance-selection coverage, `CommandPalette.test.tsx`, `flagship.spec.ts`'s scenario list) | Keep intent, rewrite assertions | The same states and behaviors under test remain the correct acceptance surface (§12); selectors and component references must be rewritten against the new component tree in a later implementation task. |

## 14. Explicit legacy components and vocabulary to retire later

**No deletion or retirement is performed by this task.** This section records what a later implementation task must retire, and why, so removal is deliberate and traceable rather than incidental.

**Components to retire:**

- `ConduitField.tsx` / `.module.css` — the curved SVG stage rail. Replaced by no navigation-rail concept at all (§3, §9).
- `ConduitMobileStrip.tsx` / `.module.css` — the rail's mobile counterpart.
- `EvidenceRibbon.tsx` / `.module.css` — the "exhibit procession." Replaced by the Evidence Register (§3.4), a ledger, not a card rail.
- `ProofChain.tsx` / `.module.css` — the chain-link SVG traceability graphic. Replaced by Traceability Proof (§3.6, §7) as text; its `FAILED_LINK_EXPLANATION` copy is worth reusing verbatim (§7).
- `components/artifactIcons.tsx` — the six hand-authored per-artifact icon glyphs. The Evidence Register (§3.4, §11) does not specify icon-driven differentiation.
- The `DecoderStrip` sub-component specifically, inside `LineageInteraction.tsx` — the "Decoder Strip" instrument, explicitly named as the rejected central concept. The surrounding provenance-selection interaction in the same file is preserved (§13); only this sub-component is retired.
- `StageContent.tsx`'s outer "editorial register" composition (`StageHead`, the single-focus canvas, the `selectedStage`-driven switch) — replaced by the six fixed IA elements of §3, which are not stage-selected panes.
- `lib/stageMapping.ts`'s `STAGE_ORDER`, `STAGES`, and `SignalPathStageId` — the eight-stage pipeline model. No stage concept survives in this document. Its `EVIDENCE_ARTIFACT_LABELS` export is separable and may be preserved (§13).
- `lib/InvestigationUIContext.tsx`'s current shape (`selectedStage: SignalPathStageId`) — replaced by state matching §3/§4 (current Register entry, active Inspection Leaf, palette open). The context-over-Zustand mechanism itself is not retired, only its shape.

**Vocabulary to retire** (code, comments, `aria-label`s, class names, and copy):

- "Signal Path," "stage rail," "conduit," "tendril," "junction."
- "Exhibit," "exhibit procession," "the filmstrip."
- "Proof chain," "forge," "spark," "severed connector," "gate/jaws."
- "Decoder Strip," "instrument," "read head."
- Status tone names "signal / branch / severed" — replaced by the platform's own real vocabulary (§11).
- `tokens.css`'s cyan/blue "signal," amber "branch," red "severed" palette, and its Fraunces serif display-font role — retired per §11's visual-language constraints. A replacement token set is implementation work, not defined by this document.

**[UX]** `docs/frontend/product-experience-brief.md` itself is not deleted by this task (front matter, above). This section records only that its creative direction is superseded; marking it explicitly superseded in place is a follow-up documentation task, not performed here.

## 15. Open constraints caused by the current backend contract

1. **[Contract]** No alert index or search endpoint (§2) — the dossier cannot offer alert-to-alert browsing. The command palette (§10) is scoped to the current alert's own dossier elements only; it offers no cross-alert jump list, because no capability produces one.
2. **[Contract]** No non-alerting telemetry retrieval (§2) — UC-001's classification-visibility outcomes remain wholly outside this screen; §5's closing note applies.
3. **[Contract]** No standalone detection-definition catalog (§2) — the Detection Definition Inspection Leaf (§6) is the only way to read a definition, and only for one already referenced by an existing alert.
4. **[Contract]** No per-stage timestamps (§2) — confirmed absent from every section above; no timeline or duration content exists anywhere in this specification.
5. **[Contract]** No severity or confidence field (§2) — confirmed absent from every section above.
6. **[Contract]** `RawAuditEvent.requestObject` is typed `unknown` in the current frontend contract (`frontend/src/types/contract.ts`). Scenario 2 (Pod creation) and scenario 3 (`ClusterRoleBinding` creation) observed facts derive from this field, but the contract does not structurally guarantee a specific raw JSON path for a given normalized characteristic the way scenario 1's `requestURI` (a plain string) does. This is exactly the condition §3.5 defines as **Partial provenance**: the provenance Inspection Leaf can still show the full raw `requestObject` content generically (via `JsonTree`) alongside the normalized characteristic value and the documented condition, but must not render a fabricated raw path or transformation label for these two scenarios without either a documented/typed `requestObject` shape or an unverified heuristic mapping. This document does not authorize the latter, since it would not be a structurally verified fact (§5.2, §5.3).
7. **[Contract]** This document does not know, and does not assume, the exact literal shape of scenario 3's `conditions` object — whether the cluster-admin role match is expressed as a declared characteristic or determined purely by operation-and-outcome matching. The Evidence Concordance's generic rendering (§3.3) is designed to handle either shape without a scenario-specific branch, but this remains an assumption pending direct observation of a real scenario-3 response (§5.3).
8. **[Future]** Alert list/index, non-matching/non-valid submission retrieval, a standalone detection-definition catalog, and processing-stage timing were previously classified in `product-experience-brief.md` §11.3 as requiring separate architecture approval before any corresponding UI is built. This document does not change that classification; it only redefines the UX for the one screen buildable on the current contract — `product-experience-brief.md` §11.2's conclusion ("None beyond §11.1") remains accurate.
9. **[Future]** Any telemetry source family beyond Kubernetes audit events (PD-04 exclusion 12). §5's scenario model is written to remain structurally extensible — a fourth normalized-event characteristic block and a fourth scenario id would not require redesigning the Concordance, Register, or Inspection Leaf mechanisms — but this document does not define what a non-Kubernetes scenario's Finding or Concordance content would say. That is future scope, not decided here.

## 16. Implementation-readiness constraints

Binding on the first implementation pass, before any visual polish begins.

- **[UX]** The first implementation replaces the page composition as **one coherent shell** — `AlertInvestigationPage.tsx`'s success-branch composition changes to the six §3 elements in a single change, not through migrating one legacy presentation section at a time. A commit that mixes, e.g., a new Evidence Register alongside the legacy `ConduitField` rail is not an acceptable intermediate state.
- **[UX]** **No legacy and new visual systems may coexist** in the approved implementation, at any point in the branch's history that is presented as reviewable. Legacy presentation components (§14) are removed in the same change that introduces their §3 replacement, not left dormant beside it.
- **[UX]** Domain and data behavior is reused **separately from**, and independently of, the rejected presentation components currently wrapping it (§13) — e.g., `lib/lineage.ts`'s field-mapping logic is extracted and reused on its own; `LineageInteraction.tsx`'s `DecoderStrip` sub-component is not carried forward even temporarily as a placeholder shell around the reused logic.
- **[UX]** The first implementation pass is **grayscale, structural work only**: the six IA elements (§3), their interaction behavior (§4), and their state handling (§6–§8) are built and functionally verified before any typography, color, or motion authorship beyond §11's minimum functional requirements (visible focus states, reduced-motion parity) is applied.
- **[UX]** **Visual authorship and polish occur only after** interaction and scenario coverage are reviewed and approved against §12's acceptance criteria — the specific sans-serif/monospace pairing, the single restrained accent treatment, spacing rhythm, and any motion beyond functional necessity are a later, separately reviewed pass, not part of the structural build.
- **[UX]** Before visual approval is sought, the grayscale structural build must demonstrably cover, from a real or fixture response rather than asserted from this document's text alone: **Scenario 1** (§5.1), **Scenario 2** (§5.2), **Scenario 3** (§5.3), **partial artifact availability** (§8), and **broken traceability** (§8) — including the provenance-state behavior (§3.5) each scenario exercises.

## Traceability

This specification presents, and must remain faithful to, product behavior already approved elsewhere; it defines no new product behavior itself.

- **Use cases served:** `UC-002` (verify a detection match and evaluate alert explainability), `UC-003` (investigate an alert using its explanation and supporting evidence) — `../use-cases.md`.
- **Product goals served:** `PC-G-005` (explainable alert generation), `PC-G-006` (evidence-based investigation), `PC-G-007` (end-to-end traceability) — `../product.md`.
- **Requirements the presented data must remain faithful to:** `FR-029` (alert explainability content), `FR-031`/`FR-035` (evidence inventory and visible absence), `FR-033`/`FR-034` (traceability links and navigation), `NFR-025` (definition-revision pinning), `NFR-031` (self-contained understandability in the product's own documented terms) — `../functional-requirements.md`, `../non-functional-requirements.md`.
- **Scope boundary this document does not reopen:** the approved minimum evidence set (`../scope.md` scope decision 8) and the PC-011 non-goals (no SIEM-style search, no case management) remain binding on this screen.
