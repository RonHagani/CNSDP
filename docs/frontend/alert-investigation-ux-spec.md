# Cloud-Native Security Telemetry and Detection Platform — Alert Investigation UX Specification

| Field | Value |
| --- | --- |
| Document | CNSDP Alert Investigation UX Specification |
| Version | 0.2 |
| Status | Approved — Phase 1.5 UX baseline |
| Phase | Phase 1.5 — Security Investigation Experience |
| Identifier | Not assigned. Deliberately outside the closed PC-015 identifier namespace (PC-###, PD-###, PER-###, UC-###, FR-###, NFR-###, AC-### are not reopened, renumbered, or extended by this document). Referenced by path only: `docs/frontend/alert-investigation-ux-spec.md`. |
| Relationship to baseline | Extends, and must not contradict, the approved Phase 0 product baseline (`../product.md` and its companions) and the approved Phase 1 architecture baseline (`../architecture.md`, ARCH-01). Defines no new product scope, functional requirement, non-functional requirement, or acceptance criterion. Any backend capability this document's open questions touch requires its own separate architecture and implementation approval before backend code is written (§22). |

**Supersession.** This is a full in-place rewrite, not an amendment. It supersedes v0.1 of this same document in its entirety — including v0.1's "Causal Evidence Dossier" information architecture, its Docket Header / Finding / Evidence Concordance / Evidence Register / Inspection Leaf / Traceability Proof vocabulary, and its document/ledger visual-language constraints (§11). v0.1 was itself an approved, implemented direction — the live implementation under `frontend/src/features/alert-investigation/` currently renders a later, unapproved visual restyling of it (internally called the "Forensic Case Folio" / "Composition Reset," using "folio," "custody," and "evidentiary clause" vocabulary that was never documented in any approved specification). This document approves neither the v0.1 Dossier presentation nor the undocumented Folio restyling. It adopts, as the sole authoritative direction, the refined **Dark Evidence Map** prototype at `frontend/review-artifacts/dark-evidence-map/` and its review document (`frontend/review-artifacts/dark-evidence-map/review.md`), reconciled here against the full approved Phase 0/Phase 1 baseline and the current live domain code.

This document does not supersede `product-experience-brief.md`'s §2 (backend-capability audit) or §11 (backend-capability plan) — those sections state contract facts that remain accurate and are restated, for self-containment, in §2 and §22 below.

**Notation.** Four tags are used throughout to keep the origin of each statement auditable:

- **[Approved]** — restates a decision already binding from `../product.md`, `../use-cases.md`, `../functional-requirements.md`, `../non-functional-requirements.md`, `../scope.md`, or `../architecture.md`. Not a UX choice; cannot be changed by revising this document alone.
- **[UX]** — a decision this document makes. A reasonable alternative existed; this is the chosen one, and future revisions of this document may reconsider it without touching the Phase 0/Phase 1 baseline.
- **[Contract]** — a limit imposed by the current `GET /v1/alerts/{id}` response shape (`frontend/src/types/contract.ts`). Would require a separately approved backend change to lift.
- **[Future]** — explicitly not implemented in v0.1. Named so a later session does not infer it exists.

## 1. Purpose and user outcome

This document defines the v0.2 user experience for the Alert Investigation screen — the platform's one investigation surface.

**[Approved]** The screen serves the Security Analyst (PER-001) performing UC-003: reaching an evidence-backed assessment of a detected activity by reviewing the alert's explanation, inspecting the normalized and source telemetry behind it, and following the traceability chain back to the original event. It also serves the Detection Engineer (PER-002) performing UC-002: verifying that a detection matched for its documented reason.

**[UX]** Stated in product terms, not visual-metaphor language: **the screen lets an analyst see, in one continuous spatial view, how one real Kubernetes audit event became a normalized fact, satisfied a documented detection condition, produced a detection result, and generated an alert — and lets them verify, for any one satisfied condition, exactly which raw field produced it, and for the alert as a whole, whether that entire chain still resolves intact back to the original event.** The screen is not a document, not a dashboard, not a pipeline diagram, and not a node graph. It is an interactive causal map: six real artifacts, connected by real dependency relationships, in which every visible line, shape, and label corresponds to a named field in the response contract (§2) or a documented product concept (`../glossary.md`) — never a decorative one.

The user outcome, restated from UC-003 and UC-002 without reference to any rejected visual metaphor: a reviewer can, without leaving the screen, (1) read what the alert claims happened, (2) see which documented condition(s) it satisfied and which specific observed facts satisfied them, (3) trace any one of those facts back to the exact raw field it was derived from (or see plainly why that raw origin cannot currently be identified), and (4) confirm whether the whole evidentiary chain — alert, detection result, normalized event, source submission — still resolves intact, or see exactly which link in it is broken.

**[Approved]** Every value displayed traces to a named field in the current response contract (§2); nothing is fabricated to fill a visual gap (PC-P-004, NFR-031).

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

- **[Contract]** No alert index or search endpoint exists. The map cannot offer alert browsing, filtering, or discovery; an alert ID must already be known before this screen can render anything.
- **[Contract]** No non-alerting telemetry retrieval path exists. A submission that was rejected, flagged incomplete, classified unsupported, or validly processed without matching a scenario has no read-back path. This screen only ever renders on an alert that already exists.
- **[Contract]** No standalone detection-definition catalog exists. A definition is reachable only through an alert that references it.
- **[Contract]** No per-stage timestamps exist. No response field carries processing-stage timing or duration. This document specifies no timeline, clock, elapsed-time element, or topology diagram of the platform's internal pipeline anywhere.
- **[Contract]** No severity or confidence field exists. This document specifies no severity badge, risk score, or confidence indicator anywhere, including within the Detection Result object (§9). The only quantifiable signal on the screen is the literal declared-vs-satisfied condition count, presented as a plain count, never as a score or percentage.
- **[Approved]** The minimum evidence set is exactly six artifacts (PD-04 scope decision 8; FR-031): the source submission, the validation outcome, the normalized event, the detection definition, the detection result, and the generated alert. This set is closed — no seventh artifact may be introduced by this document.
- **[Approved]** Traceability verification (`traceability.intact` / `traceability.failedLink`) is explicitly **not** a seventh artifact. It describes the integrity and connectivity of the six-artifact set and is always presented as a verification result, never inventoried alongside the six.
- **[Approved]** Each of the six artifacts carries its own independent `available: boolean` (FR-035). No shared or generic "something is missing" placeholder is permitted anywhere in this specification.
- **[Contract]** `traceability.intact` is `true` or `false`; when `false`, `failedLink` is exactly one of `"alert"`, `"source_key"`, `"raw_event_sha256"`. No other failure value exists.
- **[Contract]** `DetectionConditions` (`detectionDefinition.definition.conditions`) may declare any combination of `requires_outcome` (optional), `requires_any` (optional list), and `requires_all` (optional list), including neither list present. Every element of this specification that renders conditions must handle all combinations.
- **[Contract]** `requires_any`'s declared list may legitimately contain characteristics that were **not** satisfied for a given alert — only "at least one" is required for a match. `requires_all`'s declared list is, for any alert that exists, satisfied in full by construction. This declared-vs-satisfied distinction is real product content (FR-021, FR-022; UC-002) and is never collapsed away.

## 3. The governing model: six artifacts, one causal map, unequal visual roles

**[UX]** The investigation is not a document, dashboard, table, or list. It is an interactive causal evidence map showing how one real source event became a normalized event, satisfied a detection condition, produced a detection result, and generated an alert. This is the single governing model for the entire screen; every section below is a part of this one map, not an independent page region.

**[Approved]** The six real artifacts (§2) are placed according to their real causal and structural relationship, not as six identical, equally weighted tiles:

1. **Source Submission** (§5) — the origin. The raw event as received at the intake.
2. **Validation Outcome** (§6) — a small, attached verification fact on the source submission, not a peer-weighted artifact.
3. **Normalized Event** (§7) — the central, structured event representation produced from the source submission.
4. **Detection Definition** (§8) — the reviewable rule, evaluated against the normalized event, exposing its declared characteristics.
5. **Detection Result** (§9) — the resolved outcome of that evaluation: which declared characteristics were satisfied.
6. **Generated Alert** (§10) — the terminal artifact produced for a matching detection result.

**[UX]** Each artifact has a distinct visual form appropriate to its causal role — a dense evidence specimen, a compact stamped checkpoint, a structured field surface, a rule frame with a branching condition bus, a resolution object, and a terminal output marker, respectively. No two artifacts share a silhouette. This is a functional distinction, not decoration: an analyst should be able to tell which of the six artifacts they are looking at from its shape alone, before reading a single label.

**[UX]** A permanent Traceability Rail (§11) connects four of the six artifacts — alert, detection result, normalized event, source submission — along the bottom of the map, always visible, independent of any selection. Validation Outcome and Detection Definition are correctly absent from this rail: they are not part of the alert-to-source identity chain (`../architecture.md` §4; `traceability.ts`), and the rail must not imply that they are.

**[UX]** This is not a generic node-editor canvas. The map's topology is fixed and known (it is always the same six artifacts in the same causal order); nothing about it is a force-directed or user-arranged graph. Every connecting line represents a real, named dependency (§2's field-level relationships), never a decorative or inferred one.

## 4. Permanent alert identity header

**[UX]** A compact, always-visible header — not a masthead, not a hero heading, not a document title block — reads as one composed sentence plus quiet technical metadata:

- `alertId` (e.g., "Alert #4").
- The matched detection's name, from `detectionResult.matchReason.definitionName` — with an explicit "detection identity unavailable" statement (FR-035) if `detectionResult.available` is `false`.
- The actor, operation, and target, from `alert.summary` (`subject.username`, `operation.verb`, `target`) — always stated (FR-029), independent of whether outcome was itself a match condition (§8).
- The recorded outcome code (`alert.summary.outcome.code`).
- The request time (`alert.summary.requestTime`).
- The pinned detection-definition revision (`detectionDefinition.revision`, truncated for display), labeled as pinned: a later definition edit does not change what this alert resolves to (NFR-025).
- A compact traceability-state indicator (intact / broken, plus the specific `failedLink` when broken) — a pointer to §11's full rail state, not a duplicate explanation of it.

**[Approved]** This header carries what v0.1 (§3.1 Docket Header, §3.2 Finding) split across two separate, stacked sections. There is no reason to keep them apart: both are drawn from the same alert-identity data (`alert.summary` plus `detectionResult.matchReason`), and combining them into one line is what keeps this header genuinely compact rather than a second document section.

Explicitly excluded from this header: severity, confidence, any decorative verification seal, any processing-stage timeline.

If `alert.available` is `false`, the header renders an explicit "alert summary unavailable" statement (FR-035) in place of the composed sentence, rather than a blank region.

## 5. Source Submission specimen

**[UX]** The Source Submission is the origin object and the visually densest artifact on the map — a raw-event specimen, not a generic card. It is recomposed into three visual tiers, not one undifferentiated field dump:

1. **Identity and request essentials** (most prominent tier): `auditID`, `verb`, `user.username`, `objectRef` (resource, name, namespace, subresource), and the recorded response code. Set at a value-scale typographic weight — this is what an analyst reads first.
2. **Secondary ingestion metadata** (visually quieter): `level`, `stage`, `sourceIPs`, `userAgent`, `requestReceivedTimestamp`. Present, legible, and real, but subordinate to tier 1.
3. **Raw payload** (last, quietest tier, but never hidden or truncated away): the full `requestURI` (scenario 1) or a real excerpt of `requestObject` (scenarios 2 and 3), labeled plainly as raw payload.

**[UX]** The raw payload must remain available and fully legible, but visually subordinate — never converted into tabs, an accordion, or a conventional label/value form table. Subordination is expressed through position (last), typographic weight, and reduced-opacity treatment of its surrounding container, never by shrinking its text below the same legibility floor the rest of the map uses (§16).

**[UX] Full raw record.** `RawAuditEvent` (`contract.ts`) may carry fields beyond what the three tiers individually promote (e.g., `annotations`, `sourceIPs` as a full list, the complete `requestObject` for scenarios 2 and 3 rather than the curated excerpt). An on-demand "view full raw record" affordance, reusing the existing `JsonTree` component unchanged, must be reachable from the Source Submission object for complete evidentiary fidelity (FR-032). This affordance is explicit and on-demand — never auto-expanded — consistent with the general rule that voluminous content is available, not thrust in front of the analyst by default.

**[UX]** When a satisfied characteristic has Verified provenance (§13), the exact substring of the raw payload that produced it (e.g., the `tty=true` fragment of `requestURI`) is visually marked directly within the specimen on selection (§12) — this is the literal mechanism by which "select a condition, see its raw origin" is satisfied, not a separate lookup.

## 6. Validation Outcome

**[UX]** A small, compact, visually attached object — a stamped checkpoint, not a peer-weighted artifact — positioned directly against the Source Submission, connected by a short structural stub, never floating independently on the map.

**[Approved]** Content: the outcome value (`valid` / `invalid` / `incomplete` / `unsupported`) and, when present, its stated `reason`. **[Approved]** A `valid` outcome legitimately carries no `reason` (FR-011, FR-012) — this is not a missing field. Every alert this screen can ever show has a `valid` validation outcome by construction (FR-014 gates non-valid telemetry before detection ever runs) — the object still renders generically for all four contract-permitted values, never hardcoded to assume `valid` is the only reachable value.

## 7. Normalized Event field surface

**[UX]** The central artifact on the map — a structured field surface, not a form and not a JSON viewer. Every field `NormalizedEvent` (`contract.ts`) carries is rendered as one row: `subject.username`, `operation.verb`, `operation.requestURI`, `target.*`, `outcome.code`, `requestTime`, and whichever single scenario-specific block (`exec` | `podCreation` | `clusterRoleBinding`) is populated.

**[UX] Generic scenario-block rendering.** The scenario block's fields render as one row per top-level field, generically, with no scenario-specific branch in the rendering mechanism itself:

- Scenario 1 (`exec`): `exec.stdin`, `exec.tty` — boolean rows.
- Scenario 2 (`podCreation`): `podCreation.privileged`, `.hostNetwork`, `.hostPID`, `.hostIPC`, `.hostPathVolume` — boolean rows.
- Scenario 3 (`clusterRoleBinding`): `clusterRoleBinding.bindingName`, `.roleRef` (rendered as `kind/name`, e.g., `ClusterRole/cluster-admin`), `.subjects` (an array — rendered as a compact, comma-joined list within the value cell, never as nested sub-rows; the field surface's row mechanism stays flat regardless of which scenario block populates it).

**[UX]** Each row that corresponds to a characteristic evaluated by the matched detection definition is a **selectable trace target**: it is the destination of the row-to-condition trace line described in §12, and the origin of the row-to-source trace segment for Verified provenance. Rows that are not evaluated by any declared characteristic (identity fields such as `subject.username`, or scenario 3's `bindingName`/`subjects` when not themselves a declared characteristic) render as plain, non-selectable field rows.

**[Contract]** `NormalizedEvent` is a small, fully enumerable interface — its complete content is exactly what the field surface already renders. No separate "full normalized record" affordance is required the way the Source Submission needs one for its unbounded raw payload (§5).

## 8. Detection Definition: rule frame, characteristic bus, and grouping

**[UX]** A rule frame — an open, corner-bracketed instrument boundary, never a filled card — containing the definition's reviewable content and a branching characteristic bus, never a conventional vertical checklist.

**Frame content**, all drawn directly from `detectionDefinition.definition` and always visible (never behind a click):

- `name` and `description`.
- The operation clause (`conditions.operation`: resource, subresource, verb — whichever are declared).
- The outcome clause (`conditions.requires_outcome`), rendered only when declared — scenario 1 has none (FR-024 matches independent of outcome); scenarios 2 and 3 both declare `success` (FR-025, FR-026).
- A group label naming which combinator(s) govern the declared characteristics (`requires_any`, `requires_all`, or — when the contract-permitted case of both being declared together occurs — both, clearly distinguished, per the next paragraph).

**[UX] The characteristic bus.** Each declared characteristic (§2's `Characteristic[]` entries in `requires_any` and/or `requires_all`) is rendered as an independently selectable pin, branching left or right off a central vertical bus inside the frame — never a stacked list, never a repeated label/value/description block per characteristic. This directly satisfies the requirement that Scenario 2's five declared characteristics read as a branching structure, not five repeated rows or cards.

**[UX] Declared vs. satisfied.** A pin whose characteristic id appears in `detectionResult.matchReason.satisfiedCharacteristics` renders in its normal, full-weight state and is selectable into the trace-and-provenance interaction (§12). A pin declared but **not** satisfied (only possible within a `requires_any` group, whose semantics require only *at least one* declared option) renders in a visually recessed state — muted weight, no "satisfied" sub-label, no `--selected`/`--verified` accent eligibility — and remains independently selectable for review (FR-022), but selecting it opens a plain declared-only statement ("Declared in this definition; not satisfied by this alert's recorded event") rather than a provenance trace, since there is no matched evidence to trace. **This distinction must never be blurred**: only satisfied characteristics are matched evidence; only matched evidence may be traced to source (§12).

**[UX] Subgroup clustering.** Where the real declared conditions justify it, characteristics cluster visually along the bus with a wider gap and a small rotated bracket label at the boundary — proximity, not a panel, carries the grouping. Two real, contract-grounded cases use this mechanism:

1. **A genuine semantic grouping already implied by the characteristics' own descriptions** — e.g., scenario 2's four host-namespace/access characteristics (`host_ipc`, `host_network`, `host_pid`, a host-path-volume characteristic) clustering apart from its one privilege characteristic (`privileged_container`). This restates a real distinction already present in the declared `description` text; it never introduces a domain category the definitions do not already imply.
2. **`requires_any` and `requires_all` declared together** (§2) — the same bracket mechanism separates the two combinator groups, since "at least one" and "all of these" are different satisfaction semantics that must never merge into one undifferentiated list.

Every characteristic, in every group, remains independently selectable regardless of clustering.

## 9. Detection Result: a convergence/resolution object, never a score

**[UX]** A compact resolution object — a wedge that geometrically converges from a wide edge (facing the Detection Definition's characteristic bus) to a narrow edge (feeding the Generated Alert) — communicating four things and nothing else:

1. **Resolved match** — this object exists at all only because a matching detection result was produced (FR-027); its very presence on the map states that a match occurred.
2. **Satisfied count** — one tally mark per **declared** characteristic (the wide edge), filled/accented for each **satisfied** one, unfilled for any declared-but-unsatisfied `requires_any` option. This generalizes correctly to every real case, including one where a `requires_any` group's satisfied count is genuinely less than its declared count.
3. **Convergence of condition inputs** — the geometric narrowing itself, not a caption, communicates that the declared characteristic set resolves to one settled verdict.
4. **Production of the alert** — the narrow edge is the object's only outgoing connection, feeding directly into the Generated Alert.

**[Approved] No invented confidence score, anywhere.** The tally is a literal, countable fact — declared count and satisfied count, drawn directly from `detectionDefinition`/`detectionResult` — never rendered as a percentage, a star rating, a color-coded risk level, or any other construct that would imply a confidence or severity judgment the contract does not carry (§2). When no characteristics are declared at all (an operation-and-outcome-only definition, a real contract-permitted shape), the object renders with zero tally ticks and no fabricated "0 of 0" — only the resolved-match fact and the convergence geometry apply.

**[UX]** Deliberately not a circle, a generic card, a badge, a hexagon, or a standard flowchart decision diamond — every one of those reads as either a decorative UI ornament or a borrowed symbol from a different visual grammar (flowcharting) that this product does not otherwise use.

## 10. Generated Alert: the terminal result

**[UX]** A small, compact terminal-output marker — the map's rightmost/terminal object, fed by exactly one incoming connection from the Detection Result. Content: `alertId` and a one-line summary composed from `alert.summary` (actor, operation, target, outcome) — the same underlying facts the identity header (§4) states in full; this object is the literal artifact view of that same data, not a re-derivation of it.

## 11. Traceability Rail: a permanent, literal path

**[Approved]** The real four-artifact identity chain (`../architecture.md` §4; `traceability.ts`'s `TRACEABILITY_CHAIN_ORDER`), read left to right in causal order: **Source Submission → Normalized Event → Detection Result → Generated Alert.**

**[UX]** Rendered as a permanent rail along the base of the map — always visible, independent of any selection, never tucked behind a click and never appearing only when something is wrong. This is a structural, binding change from v0.1's Traceability Proof (a plain statement, reachable only as the last item in reading order): traceability must appear as a real path across the relevant artifact objects, not as a footer paragraph, an ordered text list, a generic timeline, or a set of status cards.

**[Approved]** Validation Outcome and Detection Definition are not part of this chain (`traceability.ts`) — the rail passes beneath their map positions without a tick at either, matching the real model exactly. This is not an oversight; a later reviewer must not "fix" the rail by adding ticks for either artifact.

**[Contract]** The rail has exactly two states:

- **Intact** (`traceability.intact === true`): every segment renders in its normal healthy state. The identity header's traceability indicator (§4) reads "intact."
- **Broken** (`traceability.intact === false`): **only** the specific segment corresponding to the named `failedLink` renders in the broken state (a distinct color plus an explicit break marker at the exact point of failure); every other segment remains in its normal healthy state, unchanged. The whole rail — and the whole screen — must never turn uniformly red. A plain-language explanation of the specific failure accompanies the broken segment, reusing the platform's own established wording:
  - `alert` — the alert itself does not resolve through the chain.
  - `source_key` — the submission's re-derived source identity no longer matches its stored record.
  - `raw_event_sha256` — the stored raw event no longer matches its recorded integrity digest.

**[UX] Failed-link localization.** `raw_event_sha256` and `source_key` both concern the integrity of the Source Submission → Normalized Event segment specifically (the identity and content of the source event itself); `alert` concerns the Detection Result → Generated Alert segment. The rail must localize the broken-state rendering to the one segment the named `failedLink` actually concerns, never to a segment chosen arbitrarily or to the whole rail.

**[Approved]** Framed explicitly as **live-verified on every retrieval**, never a cached flag (`../architecture.md` §4: traceability is recomputed independently on every `GET /v1/alerts/{id}` call). The identity header's compact indicator and the rail's own state must never contradict each other, since both are read from the same `traceability` field on the same response.

## 12. Condition selection and raw-to-normalized provenance tracing

**[UX]** Selecting a satisfied characteristic pin (§8) is the screen's primary interaction. Nothing is preselected on load — the map's neutral state has no active selection.

Selecting a pin must, together, on the same screen, without navigating away:

1. **Preserve the entire map.** Nothing is hidden, replaced, or navigated away from.
2. **Trace the relevant path**, drawn as a real line (not a caption) from the Normalized Event field row the pin's characteristic evaluates, into the pin itself, and — for Verified provenance only — onward to the exact substring of the Source Submission's raw payload it was derived from (§5, §13).
3. **Dim, never hide, unrelated content.** Other pins, other field rows, and unrelated connectors recede (a visible-but-quiet opacity level, not near-invisible) so the selected path reads as dominant within one second, without erasing the rest of the map's context.
4. **Reveal an anchored provenance annotation** (§13) attached to the selected path by a short leader line that lands at wherever the trace actually terminates — not a fixed-position panel, not a right-side inspector, not a modal, not a drawer, not a table, and not a card stack. The annotation must read as physically part of the map, not as a floating, disconnected panel.
5. **Remain identifiable without depending only on color.** The selected pin carries its own shape-based marker (a border-weight change plus a small filled corner mark) in a single warm-neutral accent reserved for selection state — distinct from the turquoise/amber/red vocabulary reserved for provenance and traceability state (§13, §16). Selection state and provenance state are visually independent facts and must never be collapsed into the same visual signal.

**[UX]** Selecting an unsatisfied, declared-only pin (§8) performs steps 1 and 3–5 identically, but step 2's trace terminates at the pin itself (no Source Submission segment, since there is no matched evidence to trace), and step 4's annotation states the plain declared-only fact instead of a provenance record.

## 13. Provenance states: Verified, Partial, and Unavailable

**[Approved]** Every satisfied characteristic's provenance is computed deterministically by the existing, unchanged `lib/provenance.ts` (§17), from artifact availability and whether a concrete raw-to-normalized field mapping is known for that characteristic. **No raw field path, value, transformation, or source mapping is ever invented** to fill a state the data does not support.

- **Verified** — `sourceEvent.available` and `normalizedEvent.available` are both `true`, and a concrete lineage mapping identifies a specific raw field for this characteristic (today: scenario 1's `exec.stdin`/`exec.tty`, both parsed from the `requestURI` query string). Rendered as a **continuous, solid line** from the Source Submission's exact raw substring, through the Normalized Event's field row, into the pin — plus a **turquoise/blue-green status accent** on the trace and the annotation, and a **complete source locator** (the exact raw field path and value) in the annotation.
- **Partial** — both artifacts are available, and the documented condition and observed normalized fact are both genuinely known, but no lineage mapping identifies a guaranteed raw field path (today: all of scenario 2's `podCreation.*` and scenario 3's `clusterRoleBinding`-derived characteristics, because `RawAuditEvent.requestObject` is typed `unknown` — §18 item 6). Rendered as a **structurally interrupted** source-side segment — a dashed or visibly broken connector between the Source Submission and the Normalized Event row, with an explicit gap marker — plus a **muted-amber status accent**, and the annotation's raw-path field is replaced by a single explanatory sentence naming the real limitation, never a fabricated or guessed path. The normalized-to-pin segment beyond the gap remains solid amber, since that portion of the fact (the normalized value and the condition it satisfies) genuinely is known.
- **Unavailable** — `sourceEvent.available` or `normalizedEvent.available` is `false`, so the relationship cannot be inspected at all. The annotation states plainly which artifact is unavailable, reusing §14's per-artifact unavailable wording, rather than attempting a partial render.

**[UX]** None of the three states is ever rendered with a generic warning card, badge, icon, or severity level. Verified and Partial differ **structurally** — a continuous line versus a visibly interrupted one — not only by accent color; color reinforces the distinction, it does not solely carry it.

## 14. Distinguishing per-condition provenance from whole-chain traceability

**[Approved]** Provenance (§13) and traceability (§11) are independent concepts, computed at different levels, and must never be conflated in rendering, copy, or interaction:

- **Traceability** verifies the alert-to-source **artifact chain as a whole** — whether the four chained artifacts resolve and remain unmodified.
- **Provenance** describes whether **one specific matched characteristic's raw field** can be identified within an already-available source artifact.

A row reporting Partial provenance never implies `traceability.intact === false`. Intact traceability never implies every characteristic has Verified provenance — scenario 2's fixture is the concrete, real proof: five satisfied characteristics, every one Partial, on an alert whose traceability is fully intact. The two axes render with visually distinct mechanisms (§11's rail vs. §13's trace-and-annotation) specifically so they can never be mistaken for one another.

## 15. Unavailable-artifact rendering

**[Approved]** Each of the six artifacts independently carries `available: boolean` (FR-035). An unavailable artifact remains in its fixed causal position on the map — it is never removed, collapsed, or omitted.

**[UX]** Each artifact's shape (§5–§10) has its own distinct unavailable rendering, consistent with that shape's own visual language, never a shared generic blank or placeholder:

- The shape's outline renders in a dimmed, outline-only treatment (no fill, muted border).
- An explicit, artifact-named statement replaces its normal content (e.g., "Source event unavailable," "Detection definition unavailable") — reusing the same wording pattern across all six, varied only by artifact name.
- Any connector that would normally originate from or terminate at that artifact renders in the same dimmed treatment, never simply vanishing.
- If the unavailable artifact is one whose declared characteristics the Traceability Rail or the characteristic bus depends on for full rendering (e.g., `detectionDefinition` unavailable while `detectionResult` is available), the dependent surface renders its own explicit degraded statement (e.g., the characteristic bus states that declared-condition text cannot be shown) rather than silently omitting the dependency.

**[UX]** Partial availability (some but not all six `available: false`) is simply the composition of each artifact's own state within the normal map layout — there is no separate "partial" screen or mode.

## 16. Keyboard interaction and focus behavior

**[UX]** The full map is reachable and operable without a mouse:

- Every selectable object — each characteristic pin (satisfied or declared-only), and any artifact-level "view full raw record" affordance (§5) — is a real, natively focusable control (`tabIndex="0"`, `role="button"`, `aria-pressed` reflecting selection state), reachable by Tab in document order and activated by Enter or Space exactly as a click would.
- A visible, high-contrast `:focus-visible` treatment applies to every one of these controls, tuned against the dark graphite background (§17) — this is a standard accessibility affordance, not a decorative "glow," and is not excluded by §17's prohibition on glow/neon treatments.
- Selecting a pin via keyboard produces the identical trace-and-annotation result as a pointer click (§12) — no keyboard-only degraded path exists.
- Reduced-motion users receive an instant equivalent for every selection/trace transition, carrying identical information — no animation is the sole carrier of any fact.

**[Future]** A command-palette or cross-alert jump mechanism is not specified by this document. §2 already establishes that no alert index exists to jump within; any command surface for this screen would be scoped to the current alert's own map elements only, and is left to implementation judgment rather than mandated here.

## 17. Desktop-first responsive policy

**[UX] v0.1 (this specification) is desktop-first.** This is a deliberate, binding change from the prior version's mobile-parity requirement, made for this phase only:

- **Full investigation support — the complete spatial map, condition selection, provenance tracing, and the traceability rail — is required at viewport widths of 1024px and above.**
- **The principal design targets are 1440–1600px desktop workstations.** Visual density, typography scale, and spacing are tuned for this range first; 1024–1440px must remain fully functional and readable, but is not the primary design target.
- **A dedicated phone/mobile investigation experience is explicitly out of scope for this phase.** No mobile causal-traversal mode, no swipe/carousel/stepper navigation, no phone-specific screenshot or acceptance requirement exists anywhere in this document. This reverses v0.1's mobile-parity requirement (its former §9's "no information is dropped at any breakpoint" mobile rule, and its dedicated mobile composition) — that requirement is retired, not merely deferred silently; a later phase may reinstate dedicated mobile support as its own explicitly scoped decision.
- **Below 1024px, a deliberate, readable fallback is required — not the full spatial map interaction.** The fallback must: avoid catastrophic horizontal overflow; avoid inaccessible or hidden content (every artifact's availability state and the traceability state must remain reachable, even if not spatially arranged); use the same dark visual system and the same real data as the desktop map (no separate design language, no fabricated content). It is explicitly **not** required to preserve the full interactive causal-map experience, the pin-level selection/trace mechanism, or the exact spatial layout — a simplified, vertically ordered presentation of the same six artifacts and the same traceability state, in the same causal order, satisfies this requirement. The exact fallback composition is left to implementation judgment within these constraints; this document does not mandate a specific stacked layout, since doing so would risk re-specifying a small mobile experience this phase has explicitly decided not to invest in.

**[UX] Desktop viewport checks.** The full map must be verified at 1600px, 1440px, 1280px, and 1024px — the four widths spanning the principal target range down to the desktop-support floor. No horizontal page-level scrolling is permitted at any of these four widths; technical values that could overflow (revision hashes, audit IDs, request URIs) wrap or truncate within their own bounded containers, never forcing page-level overflow.

## 18. Visual-language constraints

Binding on every element specified above. These are the Dark Evidence Map's own established constraints (`frontend/review-artifacts/dark-evidence-map/review.md`), reconciled here as the approved product direction.

**Required tonal direction:**

- Deep graphite application background (not pure black); a slightly lighter primary working surface; a controlled steel secondary surface.
- Soft off-white primary text; cool desaturated gray secondary text.
- Exactly three status accents, each meaning exactly one thing: restrained turquoise/blue-green for Verified provenance and intact traceability; muted amber for Partial provenance; controlled coral/red for Broken traceability and unavailable-state emphasis. No other UI state borrows these hues.
- One precise warm-neutral accent, reserved solely for selection state (§12), distinct from the three status accents above.

**Prohibited, without exception:**

- White, off-white, beige, or paper page backgrounds; lavender or purple SaaS palettes; pastel colors.
- Gradients, glassmorphism, glow, and neon.
- Large drop shadows; rounded dashboard cards; card grids; KPI tiles.
- Decorative icons and playful illustrations.
- A curved conduit, stage rail, or any SVG-path-based pipeline navigation spine.
- Evidence presented as a card grid, a bordered-row ledger, or a document-style stack of headed sections.
- Any chain-link graphic, glowing "proof chain" pill, or decorative verification seal for traceability — the Traceability Rail (§11) is a literal path across real artifact positions, not a graphic bolted onto a footer.
- A generic warning card, badge, icon, or severity level for a provenance limitation (§13) — Verified, Partial, and Unavailable are distinguished structurally and by label/copy, never by a new color or icon vocabulary layered on top of the three status accents above.
- Severity, confidence, risk score, or any other indicator not present in the contract (§2, §9).
- Oversized display headings, hero sections, or large, empty, purposeless canvas regions.
- Serif typography anywhere.

**Prescribed instead:**

- A highly readable sans-serif for prose, headings, and interface text; monospace reserved for technical/verbatim values — raw field values, revision hashes, audit IDs, dot-paths, request URIs.
- Sharp or minimally softened geometry; functional shapes tied to each artifact's causal role (§3), never decorative ones.
- Orthogonal, schematic-style connector lines with small, precise junctions — not smooth node-editor bezier curves.
- Empty space used only to separate evidence relationships (breathing room around causal connections, around the traceability rail), never left over merely to appear minimal.

**Status language:** exactly the platform's own real vocabulary — `valid` / `invalid` / `incomplete` / `unsupported`; satisfied / declared-only (per characteristic); `intact` / broken, named by `failedLink` — never invented synonyms, never a traffic-light or generic severity metaphor (NFR-031).

## 19. Frontend acceptance criteria

Frontend-scoped, locally numbered `UX-AC-#` criteria — **not** part of the closed PD-07 `AC-###` namespace. Each is independently checkable against the rendered screen.

1. On load, the identity header, all six artifacts (each in its available or unavailable state), and the Traceability Rail are all visible in their neutral state; no characteristic pin is preselected.
2. Every satisfied characteristic pin is independently selectable by pointer and by keyboard (Tab + Enter/Space), producing an identical result either way.
3. Selecting a satisfied pin preserves the full map, traces the real path from the relevant Normalized Event row into the pin (and onward to the Source Submission for Verified provenance only), dims unrelated content without hiding it, and reveals an anchored provenance annotation attached to the selected path.
4. Selecting a declared-but-unsatisfied pin (where one exists in the response) is possible, dims unrelated content identically, and reveals a plain declared-only statement, never a fabricated provenance record.
5. Verified provenance renders a continuous, solid trace with a complete raw source locator; Partial provenance renders a visibly interrupted source-side segment plus an explanatory limitation sentence, with no raw path ever fabricated; Unavailable provenance names the specific missing artifact.
6. The Detection Result object renders a literal declared/satisfied tally — never a percentage, score, or color-coded risk level — and renders correctly with zero declared characteristics (no fabricated "0 of 0").
7. The Traceability Rail renders `intact: true` with every segment healthy, and each of the three `failedLink` values with only its own specific, correctly localized segment in the broken state — never the whole rail or the whole screen turning uniformly red.
8. Provenance state and traceability state render independently: a satisfied pin's provenance and the rail's state never imply or contradict one another, and both remain simultaneously visible and correct on an alert exhibiting both a Partial-provenance characteristic and intact traceability.
9. Each of the six artifacts renders its own distinct, artifact-named unavailable statement when `available: false`, remaining in its fixed map position — never a shared blank or an omitted artifact.
10. Scenario 1, Scenario 2, and Scenario 3 each render correctly through the same generic mechanisms (§7's field-surface rule, §8's bus/grouping rule, §9's tally rule) with no scenario-specific branch anywhere in the implementation.
11. Scenario 2's five declared characteristics read as a branching, clustered structure — never five repeated cards or rows — with the host-access/privilege grouping visible and every characteristic still independently selectable.
12. No horizontal page-level overflow exists at 1600px, 1440px, 1280px, or 1024px.
13. Below 1024px, a readable fallback renders with no catastrophic overflow and no artifact or traceability state hidden or inaccessible; the full spatial map interaction is not required at this width.
14. No severity, confidence, per-stage timestamp, alert-count, index, search affordance, or pipeline-topology diagram is rendered anywhere on the screen.
15. No card grid, KPI tile, chain-link graphic, decorative seal, glow, gradient, or serif typeface exists anywhere in the rendered output.
16. All rendered text traces to a named field in `AlertInvestigationResponse`, `RawAuditEvent`, `NormalizedEvent`, `DetectionDefinition`, or `MatchReason` — zero fabricated content.

## 20. Mapping from current reusable implementation

| Existing module | Disposition | New role |
| --- | --- | --- |
| `frontend/src/types/contract.ts` | Keep as-is, minus one dead export | Unchanged wire-type contract (§2). The trailing `SignalPathStageId` type (lines 226–240) is retired vocabulary from the already-superseded Signal Path direction (§21) — remove it; nothing in this specification's model is a "stage." |
| `lib/alertSource.ts` (`fetchAlertInvestigation`, `AlertFetchError`) | Keep as-is | Unchanged fetch seam and three-way error taxonomy; route-level loading/not-found/unauthorized/unavailable states (unspecified visually by this document beyond reusing the dark tonal system, §18) map 1:1 to `AlertFetchError.kind`. |
| `hooks/useAlertInvestigation.ts` | Keep as-is | Unchanged TanStack Query hook. |
| `fixtures/alert-investigation/v1.ts` | Keep as-is | The five existing fixtures (scenario-1 intact/partial-availability/broken-traceability, scenario-2 intact, scenario-3 intact) already cover most of §19's scenario coverage; broken-traceability variants for the other two `failedLink` values remain a genuine gap (§21, implementation plan). |
| `lib/lineage.ts` (`buildLineageLinks`, `resolveHighlights`) | Keep as-is | Unchanged. Its field-based construction already produces Verified provenance for scenario 1's `requestURI`-derived fields and correctly produces no link (hence Partial) for scenario 2/3's `requestObject`-derived fields (§13). |
| `lib/provenance.ts` | Keep as-is | Unchanged. Already implements exactly the Verified/Partial/Unavailable engine §13 specifies. |
| `lib/traceability.ts` | Keep as-is | Unchanged. Already implements exactly the intact/broken model §11 specifies, including the real `FAILED_LINK_EXPLANATION` copy this document reuses verbatim. |
| `lib/evidenceRegister.ts` | Keep, reframe only | The six-artifact availability/default-selection logic is sound and reused; only its consumers change — there is no separate "register" section in this document's model (§15), so its output feeds each artifact's map-position rendering directly instead of a ledger list. |
| `lib/finding.ts` | Keep as-is | The CLAIM view-model this document's identity header (§4) consumes. |
| `lib/concordance.ts`, `lib/detectionConditions.ts` | Keep, reframe | The declared-vs-satisfied logic (§8's core distinction) is sound and fully reusable; its row output feeds the characteristic bus's pins instead of a concordance list. |
| `lib/artifactInspection.ts` | Keep as-is | The six typed, presentation-neutral artifact views this document's map objects (§5–§10) render directly. |
| `lib/investigationViewModel.ts` | Keep, rename fields | The single root view-model composition is sound; `DocketHeaderViewModel` is renamed to match §4's vocabulary (an implementation detail, not a new computation). |
| `components/JsonTree.tsx` | Keep as-is | Unchanged; becomes the Source Submission's on-demand full-record renderer (§5). |
| `components/CommandPalette/CommandPalette.tsx` | Keep as-is, optional | Unchanged mechanism, if retained per §16's [Future] note; not mandated by this document. |
| `AlertInvestigationPage.tsx` (query/error branching) | Keep, recompose | Unchanged `isPending`/`isError`/`isSuccess`/`AlertFetchError.kind` orchestration; only the success-branch composition changes to the map described in §3–§11. |
| `app/router.tsx` | Keep as-is | Unchanged single-route model. |
| Existing domain test suites (`lib/*.test.ts`) | Keep, extend | The same states and behaviors under test remain the correct acceptance surface; new tests are additive (declared-but-unsatisfied rendering, §8) rather than replacing sound existing assertions. |

A full, phased build sequence for the presentation layer is defined separately in `docs/frontend/alert-investigation-implementation-plan.md`.

## 21. Vocabulary and components explicitly retired by this document

**No deletion is performed by this document.** This section records what a later implementation task must retire, so removal is deliberate and traceable. It supersedes, and folds into one list, both v0.1's own §14 retirement list (the Signal Path era, already mostly dead code — see the implementation plan's cleanup inventory) and the undocumented Forensic Case Folio restyling currently live on disk.

**Section/IA vocabulary retired** (this was v0.1's own approved vocabulary; it is retired by this revision, not by an earlier one):

- "Docket Header" — replaced by the permanent alert identity header (§4).
- "Finding" — folded into the identity header (§4); no separate CLAIM-prose section exists.
- "Evidence Concordance" — replaced by the characteristic bus and condition-selection interaction (§8, §12).
- "Evidence Register" — replaced by the six artifacts' own fixed positions on the map (§3); there is no separate ledger section.
- "Inspection Leaf" — replaced by each artifact's own always-visible map content (§5–§10) plus the on-demand full-record affordance (§5).
- "Traceability Proof" — replaced by the Traceability Rail (§11).

**Forensic Case Folio / "Composition Reset" vocabulary retired** (undocumented, currently live in `components/dossier/dossier.module.css` and its consuming components):

- "Folio," "folio index," "folio row," "folio split."
- "Case Opening," "case statement," "case meta."
- "Custody," "custody split," "custody margin," "custody terminal," "custody closure."
- "Evidentiary clause," "admission threshold," "admission rule."
- "Accession index," "accession gutter."
- "Rust mark" (the CSS class used for both the identity header's and the traceability section's broken-state accent).

**Signal Path vocabulary retired** (already dead code on disk, confirmed unreferenced by the live page — see the implementation plan's cleanup inventory for the exact file list):

- "Signal Path," "stage rail," "conduit," "tendril," "junction."
- "Exhibit," "exhibit procession," "the filmstrip."
- "Proof chain," "forge," "spark," "severed connector," "gate/jaws."
- "Decoder Strip," "instrument," "read head."
- The `StatusTone` vocabulary `"signal" | "branch" | "severed" | "neutral"` — this one is **not** fully dead: `components/StatusBadge/StatusBadge.tsx` and `components/StateScreens.tsx` still use it live today. It is retired by this document regardless of which era's component currently carries it, and must be replaced by state names matching this document's own real vocabulary (§18: `intact`/`broken`, `available`/`unavailable`, `verified`/`partial`/`unavailable`).

**[UX]** `docs/frontend/product-experience-brief.md` is not deleted by this document (front matter, above); it remains superseded historical material. The v0.1 version of this same document is overwritten in place by this revision, per the task that produced it — there is no separate v0.1 file retained on disk; its content is recoverable from version control if ever needed for historical reference.

## 22. Open constraints caused by the current backend contract

1. **[Contract]** No alert index or search endpoint (§2) — the map cannot offer alert-to-alert browsing or a "next alert" affordance.
2. **[Contract]** No non-alerting telemetry retrieval (§2) — UC-001's classification-visibility outcomes remain wholly outside this screen.
3. **[Contract]** No standalone detection-definition catalog (§2) — the Detection Definition object (§8) is the only way to read a definition, and only for one already referenced by an existing alert.
4. **[Contract]** No per-stage timestamps (§2) — confirmed absent from every section above; no timeline, duration, or elapsed-time content exists anywhere in this specification.
5. **[Contract]** No severity or confidence field (§2, §9) — confirmed absent from every section above.
6. **[Contract]** `RawAuditEvent.requestObject` is typed `unknown` in the current frontend contract. Scenario 2 and scenario 3's observed facts derive from this field, but the contract does not structurally guarantee a specific raw JSON path for a given normalized characteristic the way scenario 1's `requestURI` (a plain string) does. This is exactly the condition §13 defines as Partial provenance — the full raw `requestObject` content remains inspectable generically via the Source Submission's full-record affordance (§5), but no fabricated raw path or transformation label is ever rendered for these two scenarios without either a documented/typed `requestObject` shape or an unverified heuristic mapping, which this document does not authorize.
7. **[Contract]** Scenario 3's exact declared-condition shape is confirmed by direct inspection of the real fixture (`frontend/src/fixtures/alert-investigation/v1.ts`): a `requires_all` group of exactly one characteristic (`role_ref_cluster_admin`), plus `requires_outcome: "success"`. This resolves what v0.1 had left as an open assumption.
8. **[Future]** Alert list/index, non-matching/non-valid submission retrieval, a standalone detection-definition catalog, and processing-stage timing remain classified as requiring separate architecture approval before any corresponding UI is built (`product-experience-brief.md` §11.3). This document does not change that classification.
9. **[Future]** Any telemetry source family beyond Kubernetes audit events (PD-04 exclusion 12), and any dedicated mobile/phone investigation experience (§17) — both explicitly out of scope for this phase, not decided here.

## Traceability

This specification presents, and must remain faithful to, product behavior already approved elsewhere; it defines no new product behavior itself.

- **Use cases served:** `UC-002` (verify a detection match and evaluate alert explainability), `UC-003` (investigate an alert using its explanation and supporting evidence) — `../use-cases.md`.
- **Product goals served:** `PC-G-005` (explainable alert generation), `PC-G-006` (evidence-based investigation), `PC-G-007` (end-to-end traceability) — `../product.md`.
- **Requirements the presented data must remain faithful to:** `FR-029` (alert explainability content), `FR-031`/`FR-035` (evidence inventory and visible absence), `FR-033`/`FR-034` (traceability links and navigation), `NFR-025` (definition-revision pinning), `NFR-031` (self-contained understandability in the product's own documented terms) — `../functional-requirements.md`, `../non-functional-requirements.md`.
- **Scope boundary this document does not reopen:** the approved minimum evidence set (`../scope.md` scope decision 8) and the PC-011 non-goals (no SIEM-style search, no case management) remain binding on this screen.
- **Responsive-policy change of record:** §17 revises v0.1's mobile-parity requirement for this phase only, per explicit product direction received for this documentation task. This is a UX-scope decision within this document's own authority (front matter) and does not alter any Phase 0 requirement, acceptance criterion, or persona definition — mobile/phone use of the platform was never a Phase 0 functional requirement in the first place (no FR-### specifies a viewport or device class).
