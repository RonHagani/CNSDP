# Cloud-Native Security Telemetry and Detection Platform — Alert Investigation UX Implementation Plan

| Field | Value |
| --- | --- |
| Document | CNSDP Alert Investigation UX Implementation Plan |
| Version | 0.1 |
| Status | Approved — Phase 1.5 implementation baseline |
| Phase | Phase 1.5 — Security Investigation Experience |
| Identifier | Not assigned. Outside the closed PC-015 namespace (PC-###, PD-###, PER-###, UC-###, FR-###, NFR-###, AC-### are not reopened, renumbered, or extended). Referenced by path only: `docs/frontend/alert-investigation-implementation-plan.md`. |
| Authoritative sources | `../product.md`, `../personas.md`, `../use-cases.md`, `../scope.md`, `../functional-requirements.md`, `../non-functional-requirements.md`, `../acceptance-criteria.md`, `../architecture.md`, `../reference-environment.md`, and — for this document's UX authority — `alert-investigation-ux-spec.md` (all 16 sections plus its Traceability footer). `product-experience-brief.md` is superseded historical direction and is cited below only to identify what must not be reintroduced. |
| Relationship to baseline | This document proposes no new product scope, requirement, or acceptance criterion, and no backend change. It is an implementation plan only: it translates the approved UX specification into a concrete file-by-file, component-by-component, step-by-step build plan. Every UX decision cited below traces to `alert-investigation-ux-spec.md`; where this document makes a decision the spec left to implementation (file layout, exact component boundaries, exact state shape), that is marked **[Plan]**, distinct from **[Spec]** citations of binding UX decisions and **[Finding]** facts discovered by inspecting the current codebase and backend fixtures during this planning pass. |

**Scope discipline, restated as binding constraints on this plan:** no backend change; no alert index, search, or detection-definition catalog; no severity, confidence, case status, assignment, or disposition; no new telemetry source family; the plan preserves structural extensibility (§6, §13) without pretending unsupported capabilities exist; the static design reference at `docs/design-reference/alert-investigation/flagship-assembly.html` is legacy visual-direction material and is not a template for the new implementation's DOM or CSS; nothing already built is preserved merely because it exists — every retained file in §2 is retained for a stated, specific reason, not by default.

## 1. Implementation objectives

**What the first structural implementation must accomplish.** In one coherent change (UX spec §16), replace `AlertInvestigationPage.tsx`'s entire success-branch composition with the six-element Causal Evidence Dossier — Docket Header, Finding, Evidence Concordance, Evidence Register, Inspection Leaf, Traceability Proof — such that, at the end of the change:

- All three supported scenarios (interactive `pods/exec`; high-risk Pod creation; cluster-admin `ClusterRoleBinding` creation) render correctly through the same generic Concordance/Register/Leaf mechanisms, with no scenario-specific branch in any component (spec §5).
- All six evidence artifacts render both their Register entry and their Inspection Leaf correctly in both the available and unavailable state (spec §6), independently of one another.
- All three provenance states — Verified, Partial, Unavailable — render correctly for a matched-condition row (spec §3.5).
- Both traceability states render correctly, including each of the three named `failedLink` values (spec §7).
- All four route-level states (loading, not-found, unauthorized, unavailable/retrieval-failure) render distinctly (spec §8).
- The full dossier is keyboard-operable, including the command palette, with visible focus and reduced-motion parity (spec §10).
- Every one of the 24 criteria in spec §12 passes.
- No legacy visual system (the curved rail, the exhibit ribbon, the chain graphic, the Decoder Strip, the eight-stage model, the serif/cyan/glow token set) remains reachable from the shipped page (spec §14, §16).

**What is deliberately deferred to the visual-authorship pass.** Per spec §16, this plan produces only the **grayscale structural build**. Deferred:

- The specific typeface pairing (a named sans-serif and a named monospace face). The structural build uses the browser's generic `sans-serif` and `monospace` keyword stacks to establish *typographic role* (prose vs. technical value) without committing to final fonts.
- The single restrained accent color's exact hue (spec §11). The structural build uses one flat, functional neutral (e.g., a mid-gray background/border shift) to mark "current" or "selected," proving the *mechanism* without authoring the *palette*.
- Spacing-rhythm refinement beyond what is needed for legibility and touch-target size.
- Any motion beyond the functional open/close feedback and its reduced-motion fallback already required by spec §10.
- Final copywriting polish beyond content that is complete, truthful, and traceable to a named field (spec §12 item 18) — exact sentence phrasing for Partial-provenance and Unavailable-provenance explanations may be refined later without being a blocker now.

**Definition of grayscale structural completion.** The build is structurally complete when:

1. It passes every criterion in spec §12 (24 items) under a plain, functional stylesheet — real layout (flex/grid), real spacing, real borders/hairlines, one neutral placeholder accent, generic font-role stacks, a single consistent `:focus-visible` treatment — with **zero** of the prohibited legacy visual elements (spec §11: no serif, no glow/gradient, no conduit/rail, no cards, no chain graphic, no seal, no KPI tiles, no sidebar, no oversized headings).
2. It is not "unstyled HTML" — the structural CSS is real and sufficient to read the dossier comfortably at desktop and mobile widths (§9) — but it is also not final: color, type, and motion authorship are a distinct, separately reviewed follow-on pass, not part of this plan's deliverable.
3. Automated verification (typecheck, lint, unit tests, production build, Playwright) all pass, and a manual keyboard-only walkthrough of all three scenarios plus every failure state succeeds.

## 2. Current implementation inventory

**Classification legend [Plan]:**

- **Preserve unchanged** — no edit required.
- **Preserve after extraction** — specific, named logic is pulled out into a new or existing module, largely or entirely as-is; the original file is then retired.
- **Rewrite** — the file keeps its current path and primary export; its internal content changes substantially.
- **Replace** — the file's role is taken over by a differently named/organized file; the original is retired once the replacement exists.
- **Retire after migration** — no meaningful code is carried forward; the file is deleted once its replacement in §3 is in place and verified.
- **Test-only adjustment** — the subject under test is unchanged; only test wiring (imports, provider wrapping) needs updating.

| File | Classification | Reason |
| --- | --- | --- |
| `src/types/contract.ts` | Preserve unchanged | The wire contract; spec §13 keeps it as-is, and this plan introduces no new field (constraint, front matter). |
| `src/fixtures/alert-investigation/v1.ts` | Preserve unchanged, **extend** | The three existing scenario-1 fixtures (intact / partial-availability / broken-traceability) remain valid and are reused as-is (spec §13). New scenario-2 and scenario-3 fixtures must be *added* to this file (§6, §11) — an addition, not a rewrite of existing content. |
| `src/features/alert-investigation/lib/alertSource.ts` (+ `.test.ts`) | Preserve unchanged | Fetch seam and `AlertFetchError` taxonomy are unaffected by the UI redesign (spec §13). |
| `src/features/alert-investigation/hooks/useAlertInvestigation.ts` | Preserve unchanged | Thin TanStack Query wrapper; unaffected (spec §13). |
| `src/features/alert-investigation/lib/lineage.ts` (+ `.test.ts`) | **Preserve unchanged** | **[Finding]** `buildLineageLinks`/`resolveHighlights` already produce links *only* for fields with a genuinely verifiable raw path (the four always-present links plus the two `requestURI`-derived exec links). This is exactly the Verified/Partial boundary spec §3.5 requires — a characteristic with no entry in this function's output *is* Partial provenance, by construction. No code change is needed; see §4's `deriveProvenanceState`. |
| `src/features/alert-investigation/lib/deriveStageStatus.ts` (+ `.test.ts`) | **Replace** | Its per-artifact boolean logic (`available` checks, validation-outcome branching) is sound and reused, but its output is keyed by the retiring `SignalPathStageId` (eight stages, not six artifacts) and its `StatusTone` vocabulary (`signal`/`branch`/`severed`) is retired vocabulary (spec §14). Replaced by `lib/evidenceRegister.ts` (§4). |
| `src/features/alert-investigation/lib/stageMapping.ts` | **Replace** (mixed) | `STAGE_ORDER`, `STAGES`, `SignalPathStageId` retire outright — no stage concept survives (spec §14). `EVIDENCE_ARTIFACT_LABELS` is genuinely reusable, presentation-neutral data and is preserved after extraction into `lib/artifacts.ts` (§4). |
| `src/features/alert-investigation/lib/InvestigationUIContext.tsx` | **Replace** | The plain-React-Context *pattern* is correct and reused (§5), but the state shape (`selectedStage: SignalPathStageId`) is retiring vocabulary tied to a model this plan discards. Replaced by `lib/InvestigationStateContext.tsx` (§5). |
| `src/features/alert-investigation/hooks/useInvestigationCommands.ts` | Rewrite | Same path, same export shape (`useInvestigationCommands(): Command[]`); the command list itself is fully replaced (register-entry jump commands, not stage-focus commands — spec §10). |
| `src/features/alert-investigation/AlertInvestigationPage.tsx` (+ `.module.css`) | Rewrite | The route-param handling, `⌘K` keyboard effect, and the `isPending`/`isError`/`isSuccess`/`AlertFetchError.kind` branching (spec §13) are preserved verbatim; only the success-branch JSX composition and the page-level CSS (currently built entirely around `ConduitField`'s `stageFloor`/`data-spotlight` layering — see the CSS's own comment citing "the ConduitField... threads through") are replaced. |
| `src/features/alert-investigation/AlertInvestigationPage.test.tsx` | Rewrite | Same seven route-level/state-coverage *intent* (spec §13's mapping table), new selectors and copy against the new DOM. |
| `src/features/alert-investigation/components/JsonTree.tsx` (+ `.module.css`) | Preserve unchanged | Presentation-neutral recursive renderer with per-path linkable/highlighted/dimmed hooks; spec §13 keeps it as-is. Becomes the Source Event Inspection Leaf's full-record view and the generic `requestObject` viewer for scenario 2/3 (§7, §15 item 6 of the spec). |
| `src/components/CommandPalette/CommandPalette.tsx` (+ `.test.tsx`) | Preserve unchanged | Radix-based mechanism; spec §13 and §10 keep it as-is. |
| `src/components/CommandPalette/CommandPalette.module.css` | Rewrite (minor) | **[Finding]** References `--cnsdp-signal-surface` for the active-item background and sets every text role (input, list item, hint, empty state) to `--cnsdp-font-mono`. The active-state token must be repointed at the new neutral accent (§9); command *labels* are interface text and should move to the sans role, keeping mono only for the module/hint sub-label, which is closer to a technical value. |
| `src/app/router.tsx` | Preserve unchanged | Single-route model; unaffected (spec §13, §9). |
| `src/app/RootErrorBoundary.tsx` | Rewrite (minimal) | **[Finding]** Contains the literal string `"CNSDP — Signal Path"` as its eyebrow text — retiring vocabulary (spec §14) even though this component (the router-level crash boundary, distinct from in-page error states) is otherwise unrelated to the six-element IA. One-line text change. |
| `src/app/RootErrorBoundary.module.css` | Rewrite (minor) | `.eyebrow` sets `color: var(--cnsdp-severed)` — a retiring token reference; layout is otherwise reusable. |
| `src/components/StatusBadge/StatusBadge.tsx` (+ `.module.css`) | Rewrite | The glyph-plus-label accessibility pattern ("color is never the sole carrier of meaning") is worth keeping, but `StatusTone = "signal" \| "branch" \| "severed" \| "neutral"` is retired vocabulary in the type system itself, not just in prose (spec §14), and the CSS's pill shape (`border-radius: var(--cnsdp-radius-pill)`) reads as exactly the badge/pill ornament spec §11 excludes for provenance and, by the same reasoning, should not survive generally. Re-scoped to the small set of real binary states this UI needs (e.g. available/unavailable, intact/broken) and re-rendered as plain glyph+text, not a pill container (§9). |
| `src/features/alert-investigation/components/StateScreens.tsx` (+ `.module.css`) | Rewrite | The four states (Loading, Unauthorized, NotFound, Unavailable-with-retry) and their copy are correct and reused (spec §8, §13's mapping table); `.title`'s `font-family: var(--cnsdp-font-display); font-style: italic` is the Fraunces serif display face — a direct, explicit violation of "no serif typography anywhere" (spec §11) — and every token reference needs repointing. |
| `src/features/alert-investigation/components/LineageInteraction.tsx` (+ `.module.css`, `.test.tsx`) | **Preserve after extraction, then retire** | `ProvenanceRecord`'s four-field rendering (raw path/value, behavior label, normalized path/value(s), and its record-toggle mechanism, `resolveHighlights`/`JsonTree` wiring) is near-mechanically reusable and becomes the basis for the new `ProvenanceRecord` component and the Source/Normalized Event Inspection Leaves' full-record toggle (§3, §4). The `DecoderStrip` sub-component (≈100 of the file's ≈400 lines) is discarded entirely — it is the explicitly named rejected central concept (spec §14). |
| `src/features/alert-investigation/components/StageContent.tsx` (+ `.module.css`) | Retire after migration | Its outer shell (`StageHead`, the single-focus canvas, the `selectedStage` switch) retires outright — no stage-selected pane survives. Its four inner render functions (`ValidationStage`'s outcome-tone mapping, `DetectionStage`'s characteristic-checklist rendering, `AlertingStage`'s key/value table, `MetaStage`) inform, but are not mechanically copied into, the new artifact-specific Inspection Leaf views (§3), because those views must additionally carry provenance-state awareness and register-index labeling this file never needed. |
| `src/features/alert-investigation/components/EvidenceRibbon.tsx` (+ `.module.css`) | Retire after migration | The "exhibit procession" card rail is explicitly rejected (spec §14); its per-artifact caption derivation is simple enough to be written fresh in `lib/evidenceRegister.ts` rather than extracted. |
| `src/features/alert-investigation/components/ProofChain.tsx` (+ `.module.css`) | Retire after migration (**extract first**) | The chain-link SVG graphic retires outright (spec §11, §14). Its `FAILED_LINK_EXPLANATION` constant is explicitly identified by the spec (§7, §14) as worth reusing verbatim — extract it into `lib/traceability.ts` before deleting this file. |
| `src/features/alert-investigation/components/InvestigationHero.tsx` (+ `.module.css`) | Retire after migration (**extract first**) | Replaced conceptually by Docket Header (spec §3.1); its `truncateMiddle` helper is small, correct, and needed again (audit IDs, revision hashes, truncated captions across Docket Header and the Evidence Register) — extract it into a shared formatting helper before deleting this file. |
| `src/features/alert-investigation/components/ConduitField.tsx` (+ `.module.css`) | Retire after migration | The curved SVG stage rail — explicitly named as rejected (spec §11, §14); zero reuse. |
| `src/features/alert-investigation/components/ConduitMobileStrip.tsx` (+ `.module.css`) | Retire after migration | The rail's mobile counterpart; zero reuse. |
| `src/features/alert-investigation/components/artifactIcons.tsx` | Retire after migration | The Evidence Register does not specify icon-driven differentiation (spec §3.4, §11); zero reuse. |
| `src/test/setup.ts` | Preserve unchanged | jsdom `IntersectionObserver`/`ResizeObserver` polyfills and RTL cleanup are theme- and structure-independent. |
| `src/test/renderWithProviders.tsx` | Rewrite (minimal) | Wraps children in `InvestigationUIProvider`; swap the import/wrapped component for the new `InvestigationStateProvider` (§5). |
| `src/styles/tokens.css` | Rewrite | Retire: the Fraunces display-font role, the `signal`/`branch`/`severed` color triad and their `*-dim`/`*-surface` variants, `--cnsdp-glow-*`, `--cnsdp-relief-*`, `--cnsdp-seal-*`, `--cnsdp-conduit-*`. Reuse structurally: the spacing scale (`--cnsdp-space-1`…`11`), `--cnsdp-radius-sm`/`-md` (already documented as "minimal, structural, never soft-SaaS"), the duration/ease pair, and the z-index scale minus the retiring conduit/spotlight layers — these are exactly the primitives §9 needs and are not themselves tied to the rejected aesthetic. |
| `src/styles/global.css` | Rewrite | Retire the Fraunces `@import` lines and the `.cnsdp-grain` decorative texture overlay (pure ornament with no functional grounding — out of scope for a grayscale structural pass regardless of the final visual direction). Reuse: the CSS reset, `box-sizing`, body defaults, the single `:focus-visible` mechanism (repoint its color reference only), `.cnsdp-visually-hidden`, `.cnsdp-scrollbar`. |
| `src/features/alert-investigation/components/LineageInteraction.test.tsx` | **Replace** | The unit under test is retired; its assertions (open record → select field → see provenance) map onto the new `ProvenanceRecord` component and the new Source/Normalized Event Inspection Leaf tests (§10) — new test files, same interaction *intent*. |
| `e2e/flagship.spec.ts` | Rewrite | Same production-preview acceptance-gate role; nearly every current locator targets legacy copy or structure ("Alert no. 0001" citation matcher, "N / 6 present" ribbon matcher, staggered-tile opacity waits) and must be rewritten (§10). |
| `docs/design-reference/alert-investigation/flagship-assembly.html` / `.css` | Not touched by this plan | Historical design reference for the superseded direction; explicitly not a template to copy DOM or CSS from (front matter, above). Retirement of this reference material is a separate documentation decision, out of scope here. |

## 3. Proposed component architecture

**[Plan] Component tree:**

```
AlertInvestigationPage                              (route component; unchanged name/path)
└── InvestigationStateProvider                       (new context — replaces InvestigationUIProvider)
    └── AlertInvestigationPageInner
        ├── route-level state (query not yet successful):
        │   LoadingScreen | NotFoundScreen | UnauthorizedScreen | UnavailableScreen
        │   (StateScreens.tsx, rewritten — reused unchanged in role, spec §8)
        └── on success:
            ├── DocketHeader
            ├── InvestigationFinding
            ├── EvidenceConcordance
            │   └── ConcordanceConditionRow  × N   (N = satisfied rows, §3.3's table; zero rows is valid)
            ├── EvidenceRegister
            │   └── EvidenceRegisterEntry  × 6      (fixed count, never conditional)
            ├── InspectionLeaf                       (single mount point — see rationale below)
            │   ├── (artifact mode) one of:
            │   │   SourceEventInspection | ValidationOutcomeInspection | NormalizedEventInspection
            │   │   | DetectionDefinitionInspection | DetectionResultInspection | AlertInspection
            │   └── (condition mode) ProvenanceRecord
            ├── TraceabilityProof
            └── CommandPalette                       (preserved unchanged, spec §13)
```

**[Plan] Why `InspectionLeaf` is a single mount point, not one leaf per trigger.** Spec §4.4 requires exactly one Inspection Leaf open at a time regardless of whether the Evidence Register or the Evidence Concordance triggered it. A single piece of state (`activeLeaf`, §5) feeding one component instance makes that mutual exclusivity structurally guaranteed rather than something every trigger site must remember to enforce. Spec §9 requires the leaf to render "inline, adjacent to whatever triggered it"; this is satisfied by placing the one `InspectionLeaf` mount directly after `EvidenceRegister` in document order at every breakpoint — both trigger sources (Register rows above it, Concordance rows further above) read as "the detail region right below the interactive elements," and on desktop the leaf may lay out as a column beside the Register (spec §9) without needing a second, independently positioned leaf instance.

**[Plan] Why `EvidenceRegisterEntry` and `ConcordanceConditionRow` are each a single generic component, not six/N specific ones.** The Register's six rows are structurally identical (accession index, name, caption, availability, current-marker) and differ only in the *data* a view-model supplies (§4) — six near-duplicate components would be pure duplication, which the task brief explicitly warns against. Concordance rows are likewise structurally identical regardless of whether they came from `requires_any` or `requires_all` (§3.3's table); the row component takes a `ConcordanceRow` view-model and does not need to know which clause produced it beyond an optional group label. Conversely, the six **artifact-specific Inspection Leaf views** are *not* collapsed into one generic component, because their content genuinely differs in kind (a JSON tree vs. an outcome-and-reason pair vs. a literal conditions object) — collapsing them would recreate exactly the "monolithic switch" anti-pattern `StageContent.tsx` already demonstrates the cost of.

**Component specifications:**

| Component | Responsibility | Props | Domain data consumed | Local interaction state | Accessibility responsibility | Generic or feature-specific |
| --- | --- | --- | --- | --- | --- | --- |
| `AlertInvestigationPage` | Route entry; resolves `alertId`/`demo` params, owns the query, branches route-level vs. success rendering. | none (route component) | `useAlertInvestigation` result | none (delegates to context) | Skip link, `aria-live` loading region (unchanged from today) | Feature-specific |
| `InvestigationStateProvider` / `useInvestigationState` | Owns the three-field interaction state (§5) and its action functions. | `children` | — | `currentArtifact`, `activeLeaf`, `paletteOpen` (§5) | — (state only; consumers own their own DOM roles) | Feature-specific |
| `DocketHeader` | Renders the CLAIM's identity strip. | `viewModel: DocketHeaderViewModel` | `alertId`, `detectionResult.matchReason?.{scenario,definitionName}`, `detectionDefinition.revision?`, `traceability` | none | `<header>` landmark; traceability indicator is a plain text+glyph pair, never color-only | Feature-specific |
| `InvestigationFinding` | Renders the CLAIM in prose. | `viewModel: FindingViewModel` | `alert.summary`, `detectionResult.matchReason?.{scenario,definitionName}` | none | Plain, always-visible text; no interactive role needed | Feature-specific |
| `EvidenceConcordance` | Renders the DOCUMENTED CONDITION → OBSERVED FACT rows and the summary line. | `viewModel: ConcordanceViewModel`, `onSelectCondition(id)` | `detectionDefinition`, `detectionResult` (via `lib/concordance.ts`) | none (selection lifted to context) | `<section>` with a labelled heading; delegates row semantics to `ConcordanceConditionRow` | Feature-specific |
| `ConcordanceConditionRow` | One satisfied-condition row; selectable. | `row: ConcordanceRow`, `selected: boolean`, `onSelect(id)` | none directly — pure view-model in, no contract types | none | Rendered as a native `<button>` so click/Enter/Space and focus are free; `aria-pressed`/`aria-current` for selected state | Generic (no knowledge of scenarios or artifacts beyond the view-model shape) |
| `ProvenanceRecord` | Renders one of the three provenance states for a selected condition. | `state: ProvenanceState` | none directly — consumes the already-computed view-model | none | Content-only differences carry the Verified/Partial/Unavailable distinction (spec §3.5's ban on badge/icon differentiation) | Feature-specific |
| `EvidenceRegister` | Renders the fixed six-row ledger. | `entries: RegisterEntryViewModel[]`, `current: EvidenceArtifactId`, `onSelect(id)` | `lib/evidenceRegister.ts` output | none (selection lifted to context) | `<section>`/list semantics; delegates to `EvidenceRegisterEntry` | Feature-specific |
| `EvidenceRegisterEntry` | One register row. | `entry: RegisterEntryViewModel`, `current: boolean`, `onSelect(id)` | none directly | none | Native `<button>`; `aria-current="true"` when current; unavailable state stated in visible text, not only `aria-disabled` (row remains selectable per spec §3.3/§4.2) | Generic |
| `InspectionLeaf` | Dispatches to the correct content for `activeLeaf`; renders nothing when `activeLeaf` is `null`. | `activeLeaf: ActiveLeaf \| null`, `currentArtifact: EvidenceArtifactId`, `data: AlertInvestigationResponse` | Delegates per-artifact resolution to `lib/artifacts.ts`; per-condition resolution to `lib/provenance.ts` | none | `<section aria-live="polite">` so leaf-content changes are announced; single focus-management point on open (moves focus into the leaf) | Feature-specific |
| `SourceEventInspection` | Full raw event via `JsonTree`, behind an on-demand toggle; compact summary otherwise. | `view: SourceEventArtifactView` | `sourceEvent` | `expanded: boolean` (local — mirrors today's `LineageInteraction` pattern, §5) | Toggle is a real `<button aria-expanded>` | Feature-specific |
| `ValidationOutcomeInspection` | Outcome + optional reason, all four outcome values handled generically. | `view: ValidationOutcomeArtifactView` | `validationOutcome` | none | — | Feature-specific |
| `NormalizedEventInspection` | Compact summary + on-demand full field list including whichever scenario block is present. | `view: NormalizedEventArtifactView` | `normalizedEvent` | `expanded: boolean` (local) | Toggle is a real `<button aria-expanded>` | Feature-specific |
| `DetectionDefinitionInspection` | Name, description, full literal `conditions` object, pinned-revision label. | `view: DetectionDefinitionArtifactView` | `detectionDefinition` | none | — | Feature-specific |
| `DetectionResultInspection` | Literal `matchReason` payload. | `view: DetectionResultArtifactView` | `detectionResult` | none | — | Feature-specific |
| `AlertInspection` | Full `alert.summary` fields as a literal artifact view. | `view: AlertArtifactView` | `alert` | none | — | Feature-specific |
| `TraceabilityProof` | Terminal, non-interactive verification statement. | `viewModel: TraceabilityViewModel` | `traceability` (via `lib/traceability.ts`) | none | `<section role="status">` (result is meaningful, not decorative) | Feature-specific |
| `JsonTree` | Recursive raw/generic JSON renderer with per-path link/highlight hooks. | unchanged | — | — | unchanged | Generic (already reused for two different artifact kinds — source event and, per §15 item 6 of the spec, the raw `requestObject` view) |
| `CommandPalette` | Filterable command list in a Radix Dialog. | unchanged | — | unchanged | unchanged (Radix-provided) | Generic |
| `StatusBadge` (rewritten) | Small glyph+label pairing for a narrow, real vocabulary of binary states. | `tone`, `children` | — | — | Never color-only | Generic |

## 4. Domain and view-model layer

**[Plan] Principle carried through every function below:** contract interpretation (branching on `available`, matching characteristic IDs, computing provenance) happens exactly once, in `lib/`, and never inside a `.tsx` file's JSX. Every presentational component in §3 receives an already-resolved, already-typed view-model or a discriminated union it can switch on without re-deriving anything from `AlertInvestigationResponse`.

**Reused unchanged:**

- `lib/lineage.ts` — `buildLineageLinks(normalized, raw): LineageLink[]`, `resolveHighlights(...)`. No change (§2). This is the authority `deriveProvenanceState` (below) consults for Verified-vs-Partial.
- `lib/alertSource.ts`, `hooks/useAlertInvestigation.ts` — unchanged (§2).

**Renamed / generalized (replace `deriveStageStatus.ts`, extract from `stageMapping.ts`):**

- `lib/artifacts.ts` — `EVIDENCE_ARTIFACT_LABELS: Record<EvidenceArtifactId, string>` (moved verbatim from `stageMapping.ts`), plus six small `resolve*View` functions, one per artifact, each returning a discriminated union:

  ```ts
  type SourceEventArtifactView =
    | { available: true; rawEvent: RawAuditEvent }
    | { available: false };
  // ...one such type per artifact, named to match contract.ts's own field names
  ```

  These are the only functions permitted to read an artifact's `available` flag; every consumer downstream receives the already-branched union.

- `lib/evidenceRegister.ts` — `buildEvidenceRegisterEntries(data: AlertInvestigationResponse, current: EvidenceArtifactId): RegisterEntryViewModel[]`. Replaces `deriveStageStatus.ts`'s per-artifact boolean checks, re-keyed to the six real artifacts (not eight stages) and to plain `available`/caption fields (not `StatusTone`).

**Logic currently trapped inside legacy presentation components, now extracted to pure functions:**

- `lib/finding.ts` — `buildFinding(data: AlertInvestigationResponse): FindingViewModel`. Extracts the field-selection logic currently inline in `InvestigationHero.tsx`'s JSX (`summary?.subject.username ?? "—"` and similar ternaries) into one pure function.
- `lib/concordance.ts` — `buildEvidenceConcordance(detectionDefinition, detectionResult): ConcordanceViewModel`, implementing spec §3.3's table exactly (operation row always when available; outcome row iff `requires_outcome` declared; `requires_any`/`requires_all` each rendered as satisfied-only rows, kept as two separately labeled groups when both are declared; zero rows when neither declared). This logic exists nowhere today — `StageContent.tsx`'s `DetectionStage` renders both satisfied *and* unsatisfied characteristics, which spec §3.3 explicitly now forbids (§2's inventory).
- `lib/traceability.ts` — `buildTraceabilityProof(traceability: TraceabilityResult): TraceabilityViewModel`. Extracts `ProofChain.tsx`'s `FAILED_LINK_EXPLANATION` map (preserved verbatim, spec §7) and its `intact`-vs-broken statement logic, decoupled from the SVG chain rendering that discards it.
- `lib/format.ts` — `truncateMiddle(value, head, tail): string`, extracted verbatim from `InvestigationHero.tsx`.

**New pure functions/typed view models required, with no direct legacy predecessor:**

- `lib/provenance.ts` — the Verified/Partial/Unavailable engine (spec §3.5):

  ```ts
  type ProvenanceState =
    | { kind: "verified"; rawPath: string; rawValue: string; behavior: string;
        normalizedPath: string; normalizedValue: string; condition: string }
    | { kind: "partial"; normalizedPath: string; normalizedValue: string;
        condition: string; limitation: string }
    | { kind: "unavailable"; missingArtifact: "source-event" | "normalized-event" };

  function deriveProvenanceState(
    characteristicId: string,
    data: AlertInvestigationResponse,
  ): ProvenanceState
  ```

  Algorithm: (1) if `sourceEvent.available` or `normalizedEvent.available` is `false` → `unavailable`; (2) else, look up the characteristic's normalized field path (below) and check whether `buildLineageLinks` (unchanged, `lineage.ts`) produced a link for that path — a link found → `verified` (its `rawPath`/`rawValue`/`label` populate the record); no link found → `partial`. **[Finding]** Because `lineage.ts` today only ever produces links for fields it can genuinely verify, this check requires no change to `lineage.ts` itself — the absence of a link *is* the definition of Partial provenance.

- `lib/characteristicFields.ts` — the one small, honest piece of per-scenario knowledge this system needs: a literal map from each real, already-approved characteristic `id` to its real, already-typed normalized field path, drawn directly from the three committed detection definitions (`definitions/scenario-{1,2,3}.yaml`, read during this planning pass) and `contract.ts`'s `NormalizedEvent` shape — never invented:

  ```ts
  const CHARACTERISTIC_NORMALIZED_PATH: Record<string, string> = {
    stdin_streaming: "exec.stdin",
    tty_allocation: "exec.tty",
    privileged_container: "podCreation.privileged",
    host_network: "podCreation.hostNetwork",
    host_pid: "podCreation.hostPID",
    host_ipc: "podCreation.hostIPC",
    host_path_volume: "podCreation.hostPathVolume",
    role_ref_cluster_admin: "clusterRoleBinding.roleRef",
  };
  ```

  This table is data, not logic — adding a future characteristic is a one-line addition, not a structural change (§6, §13).

- `lib/docketHeader.ts` — `buildDocketHeader(data): DocketHeaderViewModel`, a small composition of already-available fields plus the truncation helper.

## 5. Interaction and state model

**[Plan] Replacement state shape** (`lib/InvestigationStateContext.tsx`, replacing `InvestigationUIContext.tsx`):

```ts
interface InvestigationState {
  currentArtifact: EvidenceArtifactId;   // Register's "current" marker; default "detection-result" (spec §3.4)
  activeLeaf: ActiveLeaf | null;          // null = zero leaves open (spec §3.5, §4.1)
  paletteOpen: boolean;
}

type ActiveLeaf =
  | { kind: "artifact" }                              // content derived from currentArtifact
  | { kind: "condition"; characteristicId: string };   // content derived via lib/provenance.ts
```

`activeLeaf` deliberately does not duplicate an artifact ID for the `"artifact"` case — it always reads `currentArtifact`, avoiding two sources of truth that could desync. This is the entire interaction-state surface: three fields, four action functions (`selectRegisterEntry`, `selectConcordanceRow`, `closeLeaf`, `setPaletteOpen`).

**Behavior mapping to spec §4:**

- **Selected evidence artifact** — `currentArtifact`, set by `selectRegisterEntry(id)`, which also sets `activeLeaf = { kind: "artifact" }` (spec §4.2, points 1–4). Default on load: `"detection-result"`, `activeLeaf: null` (spec §4.1 — Detection Result is current, but its leaf is not auto-opened; the Evidence Concordance, not an Inspection Leaf, is what satisfies "visible on load").
- **Selected concordance condition** — `selectConcordanceRow(characteristicId)` sets `activeLeaf = { kind: "condition", characteristicId }` and explicitly does **not** touch `currentArtifact` (spec §4.3, point 3).
- **Active source or normalized record inspection** — deliberately **not** lifted into `InvestigationState`. It is local `useState<boolean>` inside `SourceEventInspection`/`NormalizedEventInspection` (§3), mirroring today's `LineageInteraction.tsx` `expandedRecord` pattern exactly: it only ever matters while that specific leaf is mounted, and naturally resets when `activeLeaf` changes and the component unmounts. Lifting it to context would be state the rest of the tree never needs to read.
- **Command-palette actions** (spec §4.6, §10) — one jump command per Register entry (calls `selectRegisterEntry`), one jump-to-Traceability-Proof (scroll only, no state change — Traceability Proof is not selectable, spec §3.6), one jump-to-Finding (scroll only), one clear-active-leaf (`closeLeaf()`). No cross-alert command exists (spec §15 item 1 — no list to jump within).
- **Keyboard navigation / focus movement** — Register rows and Concordance rows are real `<button>` elements, so Tab order and Enter/Space activation are native, requiring no custom key handling (spec §10). Escape closes the topmost open surface: the palette if open (Radix's own handling), else `closeLeaf()` if a leaf is open. `InspectionLeaf` moves focus to itself when `activeLeaf` transitions from `null`/other to a new value, so keyboard users land where new content appeared.
- **Behavior when an artifact becomes unavailable** — no state-layer change at all. `selectRegisterEntry` runs identically regardless of availability; the artifact-specific view component (§3) receives the `available: false` variant of its view-model from `lib/artifacts.ts` and renders the FR-035 statement. Availability is a rendering concern, never an interaction-state concern (§4, principle).
- **Behavior when traceability is broken** — no state-layer implication whatsoever. `TraceabilityProof` is stateless and non-interactive (spec §3.6, §4.5); it renders directly from `buildTraceabilityProof(data.traceability)`.

**Is Context still justified?** Yes — **[Plan]** evaluated explicitly, not assumed. `currentArtifact`/`activeLeaf`/`paletteOpen` are each read and/or written by multiple sibling subtrees that are not in a direct parent-child relationship (`EvidenceRegister`, `EvidenceConcordance`, `InspectionLeaf`, `useInvestigationCommands`, and the page's own keyboard-shortcut effect all need at least one of the three fields). Passing these as props alone would require threading five-to-eight individual props and callbacks through `AlertInvestigationPageInner` into four-plus separate subtrees — mechanically possible, but strictly worse than the existing, already-proven-sufficient Context (the original UX-spec authors reached the same conclusion for the legacy version of this exact state: "the one genuine cross-component sharing need... context is sufficient for it"). No external state library (Zustand or otherwise) is introduced; the state is smaller than today's, not larger.

## 6. Scenario-aware behavior

No component branches on scenario identity anywhere (spec §5). The table below documents how the generic Concordance/provenance mechanism instantiates for each scenario, using the real, currently committed detection definitions (`definitions/scenario-{1,2,3}.yaml`, read during this planning pass — not invented) and confirms **[Finding]** the exact declared-condition shape spec §15 item 7 had left as an open assumption for scenario 3.

| | Scenario 1 — interactive `pods/exec` | Scenario 2 — high-risk Pod creation | Scenario 3 — cluster-admin `ClusterRoleBinding` |
| --- | --- | --- | --- |
| **Declared conditions** (`definitions/scenario-N.yaml`) | `operation: {resource: pods, subresource: exec}`; `requires_any: [stdin_streaming, tty_allocation]`; no `requires_outcome` | `operation: {resource: pods, verb: create}`; `requires_outcome: success`; `requires_any:` 5 characteristics (`privileged_container`, `host_network`, `host_pid`, `host_ipc`, `host_path_volume`) | `operation: {resource: clusterrolebindings, verb: create}`; `requires_outcome: success`; `requires_all:` **exactly one** characteristic (`role_ref_cluster_admin`) — **[Finding]**, resolving the spec's §15 item 7 open assumption |
| **Concordance table row** applied (§3.3) | "`requires_any` only" | "`requires_any` + `requires_outcome`" | "`requires_all` + `requires_outcome`", with a single-row group |
| **Satisfied-condition rendering** | 0–2 rows, from `matchReason.satisfiedCharacteristics` | 0–5 rows (a real committed backend fixture, `internal/validation/testdata/scenario2-valid.json`, satisfies all five simultaneously) | Exactly 0 or 1 row (an all-or-nothing `requires_all` group of size one) |
| **Outcome-condition handling** | Not shown as a Concordance row (none declared); still stated in the Finding (spec §3.2, always) | Shown as a Concordance row; satisfied iff `alert.summary.outcome` indicates success | Shown as a Concordance row; satisfied iff `alert.summary.outcome` indicates success |
| **Normalized facts used** | `normalizedEvent.event.exec.{stdin, tty}` | `normalizedEvent.event.podCreation.{privileged, hostNetwork, hostPID, hostIPC, hostPathVolume}` | `normalizedEvent.event.clusterRoleBinding.{bindingName, roleRef, subjects}` (identity fields — `roleRef` is what the one declared characteristic concerns) |
| **Expected provenance state** (both artifacts available) | **Verified** — `lineage.ts` already maps both fields to `requestURI` query parameters | **Partial** — no `lineage.ts` entry exists; `requestObject` is `unknown`-typed (§4, §15 item 6 of the spec) | **Partial**, for the same reason, when the one characteristic is satisfied |
| **Current raw-source limitation** | None | `RawAuditEvent.requestObject` typed `unknown` — the full object is inspectable via `JsonTree` inside the leaf, but no verified per-field path is claimed | Same |

**[Finding, action required]** `src/fixtures/alert-investigation/v1.ts` today contains **only** scenario-1 fixtures (all three variants — intact, partial, broken-traceability — reuse `scenario1RawEvent`/`scenario1Definition`). New fixtures for scenario 2 and scenario 3 must be authored before this plan's scenario coverage can be verified (§11, step 0). Real raw-event content for both already exists in the backend's own committed testdata (`internal/validation/testdata/scenario2-valid.json`, `scenario3-valid.json`) and was read during this planning pass to confirm exact field shapes; the same provenance discipline `v1.ts`'s existing header comment already documents (byte-identical raw event, hand-derived normalized event per the documented normalization rules, illustrative-not-authoritative revision hash) applies to the new fixtures.

## 7. Artifact inspection mapping

Traceability is confirmed, once more, not a seventh artifact (spec §2, §3.4) — it is covered separately in §5's `TraceabilityProof`, not in this table.

| Artifact | Evidence Register summary | Inspection Leaf content | Existing rendering logic reused | Unavailable-state behavior | Links to related artifacts |
| --- | --- | --- | --- | --- | --- |
| `sourceEvent` | Operation verb + truncated `auditID` | Full raw JSON via `JsonTree`, collapsible | `JsonTree.tsx` (unchanged); `truncateMiddle` (extracted, §4) | Named "source event unavailable" statement (FR-035) | Destination of every Verified/Partial `ProvenanceRecord`; opened directly when a Concordance row is selected with Verified/Partial provenance |
| `validationOutcome` | The outcome value (`valid`/`invalid`/`incomplete`/`unsupported`) | Outcome + `reason` when present; all four values handled generically, `valid` legitimately reasonless | New (small; no legacy predecessor beyond `StageContent.tsx`'s `ValidationStage`, used as reference only, §2) | Named "validation outcome unavailable" statement | None — this artifact stands alone in the six-artifact set |
| `normalizedEvent` | Operation verb + target resource | Compact summary (`subject`, `operation`, `target`, `outcome`, `requestTime`) + on-demand full field list including whichever scenario block is present | `LineageInteraction.tsx`'s field-list/toggle pattern (extracted, §2); `JsonTree` for `requestObject` display within scenario 2/3's provenance context | Named "normalized event unavailable" statement; provenance records for this alert become Unavailable (§4) | Destination of every `ProvenanceRecord`'s normalized side; source of every Concordance row's observed fact |
| `detectionDefinition` | Truncated pinned revision + scenario id | Name, description, full literal `conditions` object (operation/outcome/any/all, whichever declared), pinned-revision label | New (`lib/concordance.ts` supplies the same underlying data the leaf renders literally, §4) | Named "detection definition unavailable" statement; Concordance renders with no declared-condition text available | The only place an unsatisfied declared characteristic is inspectable (spec §3.3, §6 item 4) — cross-link this fact in the leaf's own copy |
| `detectionResult` | Matched scenario id | Literal `matchReason` payload (`scenario`, `definitionName`, `definitionRevision`, full `satisfiedCharacteristics`) — **default-selected artifact (spec §3.4)**, leaf itself still not auto-opened (§5) | New (thin; the interpreted view is `lib/concordance.ts`, this leaf is the raw artifact) | Named "detection result unavailable" statement; Concordance itself becomes empty/unavailable (both derive from this artifact) | The Evidence Concordance is the *interpreted* view of this same artifact — leaf copy should say so explicitly, avoiding the impression of two unrelated data sources |
| `alert` | `#{alertId}` | Full `alert.summary` fields, literal | New (thin; `lib/finding.ts` is the prose view of the same data) | Named "alert unavailable" statement; Finding (spec §3.2) also renders its own explicit "alert summary unavailable" statement independently | The Finding is the *prose* view of this same artifact |

## 8. Legacy retirement plan

**No deletion is performed in this planning task.** This is the exact sequence a later implementation task must follow, ordered so nothing reusable is lost and nothing is deleted before its replacement is verified (§11 gives the full step-by-step build; this section is the retirement-specific subsequence within it).

1. **Extract before touching anything else:** `ProofChain.tsx`'s `FAILED_LINK_EXPLANATION` → `lib/traceability.ts`; `InvestigationHero.tsx`'s `truncateMiddle` → `lib/format.ts`; `stageMapping.ts`'s `EVIDENCE_ARTIFACT_LABELS` → `lib/artifacts.ts`; `LineageInteraction.tsx`'s `ProvenanceRecord` rendering and record-toggle mechanism → the new `ProvenanceRecord` component and the new Source/Normalized Event Inspection Leaves.
2. **Build the full new tree (§3) unwired** — legacy components keep rendering the live page throughout this step; nothing is retired yet.
3. **Swap `AlertInvestigationPage.tsx`'s composition in one step** (§11 step 4) — from this point, `ConduitField`, `ConduitMobileStrip`, `EvidenceRibbon`, `ProofChain`, `artifactIcons`, `LineageInteraction` (including `DecoderStrip`), `StageContent`, `deriveStageStatus.ts`, `stageMapping.ts`, and `InvestigationUIContext.tsx` become unreferenced by the shipped page, but are **not yet deleted**.
4. **Verify the new page fully** (§11 step 5) against every scenario and failure state before deleting anything.
5. **Delete the now-verified-dead files**, in this order, each independently confirmed unreferenced first: `components/ConduitField.tsx`+`.module.css`, `components/ConduitMobileStrip.tsx`+`.module.css`, `components/EvidenceRibbon.tsx`+`.module.css`, `components/ProofChain.tsx`+`.module.css`, `components/artifactIcons.tsx`, `components/LineageInteraction.tsx`+`.module.css`+`.test.tsx`, `components/StageContent.tsx`+`.module.css`, `lib/deriveStageStatus.ts`+`.test.ts`, `lib/stageMapping.ts`, `lib/InvestigationUIContext.tsx`.
6. **Sweep styling and vocabulary:** replace `styles/tokens.css` and `styles/global.css` content per §2/§9; remove the `@fontsource/fraunces` dependency from `package.json` once no CSS imports it (verify with a repository-wide search first, since `global.css` is its only current importer).
7. **Vocabulary sweep**, across whatever files remain, for every term in spec §14 plus the two additional terms this plan's inspection found: **[Finding]** `"Evidence Theater"` (currently `AlertInvestigationPage.tsx`'s `wordmarkSub` text in the non-success branch) and the `signal`/`branch`/`severed` `StatusTone` literal type values (spec §14 names the *concept*; this plan confirms the exact TypeScript identifiers carrying it: `StatusBadge.tsx`'s `StatusTone` union and `StatusBadge.module.css`'s `data-tone` selectors).
8. **Update or replace tests** per §10, only after steps 1–7 land — a test rewritten against a component that doesn't exist yet is not a usable checkpoint.

No legacy visual system remains reachable from the shipped page once step 5 completes; step 6–8 remove the remaining vocabulary and styling residue that step 5 alone would not catch (dead CSS variables in a token file that no component references are invisible at runtime but still violate spec §14's instruction to retire the vocabulary, not merely stop rendering it).

## 9. Styling strategy

Grayscale structural styling only (§1). Everything in the "do not define" list in the task brief — final visual polish, decorative animation, gradients, glow, serif typography, cyber-themed ornament, cards, pills, seals, stage rails — is out of scope here and already excluded by spec §11; this section defines the functional structure those constraints still leave room for.

- **Page width and content rhythm.** A single content column, `max-width` bounded for readability (a bounded measure, not full-viewport-width prose — exact value deferred, but structurally a `max-width` + horizontal auto-margins, not a page-width flex/grid split). Vertical rhythm reuses `tokens.css`'s retained spacing scale (`--cnsdp-space-*`) uniformly across the six IA elements, replacing today's per-component ad hoc spacing.
- **Responsive breakpoints.** One collapse point, matching spec §9: below it, the Evidence Register is single-column and the Inspection Leaf renders inline beneath the selected row; at or above it, the Register may show more caption detail and the Leaf may render as a side column. No second, finer-grained breakpoint is introduced — the UX spec defines exactly one.
- **Typography roles.** Two roles, generic stacks in this pass (`font-family: sans-serif` for prose/labels, `font-family: monospace` for technical/verbatim values — audit IDs, revision hashes, dot-paths, accession indices, raw/normalized values inside a `ProvenanceRecord`) — never serif, anywhere (spec §11). Role assignment is structural (which CSS class or `data-*` attribute is applied) and is part of this pass; the specific named typefaces are not (§1).
- **Ruled-record geometry.** The Evidence Register and Evidence Concordance are ledgers: hairline `border-bottom` rules between rows (reusing `tokens.css`'s `--cnsdp-border-subtle`/`-default`, repointed if their current values are tied to the retiring palette), fixed-width accession-index columns, no card borders, no shadow-based elevation. This is the direct implementation of spec §11's "hairline rule dividers, not shadows or elevation."
- **Focus and selected states.** One `:focus-visible` treatment reused from `global.css` (already a single, consistent mechanism — only its color reference changes, spec §11 permits this as a standard accessibility affordance, not a decorative violation). "Current" (Register) and "selected" (Concordance) states use one flat neutral background/border-weight shift — the single placeholder accent this pass is allowed (§1) — never the retired `signal`/`branch`/`severed` triad.
- **Long value/path behavior.** Revision hashes, audit IDs, and `requestURI` values use `overflow-wrap: anywhere` (or equivalent) within their bounded container rather than forcing page-level scroll; `JsonTree` content and the raw `requestObject` view scroll within their own bounded region exactly as today (spec §9's existing, correct behavior — no change needed there).
- **Mobile ordering.** Single reading order at every width — Docket Header → Finding → Evidence Concordance → Evidence Register → Traceability Proof, with the Inspection Leaf inline (spec §9) — achieved by a single-column flex/grid layout with no `order`-property reshuffling; the DOM order and the visual order are identical at every breakpoint, which is also what makes the keyboard Tab order correct for free.
- **Prevention of page-level horizontal overflow.** `max-width: 100%` on the page's outer container and on every technical-value block; only `JsonTree` and the raw-`requestObject` inspector get their own `overflow-x: auto`, scoped to that element, never the page body (spec §9's existing constraint, reused).

## 10. Test migration plan

| Current test | New coverage | Disposition |
| --- | --- | --- |
| `lib/alertSource.test.ts` | Unchanged | Preserve unchanged (§2). |
| `lib/deriveStageStatus.test.ts` | `lib/evidenceRegister.test.ts` (same fixture-driven assertions — all-available / partial-availability / broken-traceability-does-not-affect-availability — re-keyed to `EvidenceArtifactId`) | Rewrite target, new file. |
| `lib/lineage.test.ts` | Unchanged, **plus** new cases feeding `lib/provenance.test.ts` (below) | Preserve unchanged; extend elsewhere. |
| **New:** `lib/provenance.test.ts` | Verified (scenario-1 fixture), Partial (scenario-2/3 fixtures, once authored — §11 step 0), Unavailable (source unavailable; normalized unavailable) — four cases minimum | New. |
| **New:** `lib/concordance.test.ts` | All seven declared-condition combinations from spec §3.3's table, using real shapes from `definitions/scenario-{1,2,3}.yaml` plus hand-built edge cases for the two combinations no current scenario exercises (`requires_all` alone; `requires_all` + `requires_any` together) | New. |
| **New:** `lib/traceability.test.ts` | `intact: true`; each of the three `failedLink` values | New — extracted logic, not previously unit-tested in isolation from `ProofChain.tsx`. |
| `components/LineageInteraction.test.tsx` | Split across `components/ProvenanceRecord.test.tsx` (Verified/Partial/Unavailable rendering) and the new `SourceEventInspection.test.tsx`/`NormalizedEventInspection.test.tsx` (record-toggle behavior) | Replace (§2). |
| `components/CommandPalette.test.tsx` | Unchanged | Preserve unchanged. |
| **New:** `hooks/useInvestigationCommands.test.ts` | Command list matches Register entries + Traceability/Finding jumps + clear-leaf; no stage-focus command exists | New (no predecessor existed). |
| `AlertInvestigationPage.test.tsx` | Same seven-state intent (loading, intact/6-of-6, partial-availability, broken-traceability × each named `failedLink`, not-found, unauthorized, unavailable-with-retry) rewritten against new copy/selectors; **expanded** to also cover scenario 2 and scenario 3 fixtures directly (currently the file only ever exercises scenario 1) | Rewrite, with net-new scenario coverage. |
| `e2e/flagship.spec.ts` | Rewritten and expanded (below) | Rewrite. |

**Full acceptance-surface checklist for the rewritten Playwright suite** (spec §12; desktop **and** mobile viewport for each, per spec §9):

- Scenario 1, 2, and 3 — full dossier renders, Concordance rows match §6's table for each.
- Partial artifact availability (existing fixture 2 — detection definition unavailable) — verify Register + leaf render the FR-035 statement, and that the *other* five artifacts remain fully inspectable.
- Broken traceability, for **each** of the three `failedLink` values individually (today only `raw_event_sha256` has a fixture) — two new fixtures needed alongside the scenario-2/3 ones (§11 step 0).
- Unauthorized, not-found, retrieval-failure (with working Retry) — existing coverage, rewritten selectors.
- Keyboard-only walkthrough: Tab through Register and Concordance, Enter/Space to activate, Escape to close a leaf then the palette, `⌘K`/`Ctrl+K` to open the palette and run a jump command.
- Zero console errors/warnings across the full interaction flow (existing assertion, reused as-is — it does not depend on the visual system).

**Legacy selectors and screenshot assumptions to remove:**

- `findAlertNumber`'s `"Alert no. 0001 · scenario-1 · rev. …"` citation-line text matcher — the Docket Header's copy is being redesigned (§3.1) and will not read this way.
- The `"N / 6 present"` Evidence Ribbon count matcher — replaced by whatever literal text the new `EvidenceRegister` uses to convey the same fact (still real, still six, just not "the exhibit procession").
- The `toHaveCSS("opacity", "1", { timeout: 2000 })` waits keyed to `EvidenceRibbon`'s per-tile staggered entrance animation — the grayscale build has no such stagger (§1's deferred-motion list); remove the wait entirely rather than re-target it.
- The `review-artifacts/*.png` full-page screenshot captures (`01-wide-desktop-flagship.png`, `02-field-lineage-focused.png`, `03-narrow-mobile.png`) — these are visual-review artifacts of the *rejected* direction; new screenshot baselines, if wanted, belong to the later visual-authorship pass (§1), not this structural one.
- Any `data-role="conduit-field"`/`"hero"`/`"evidence-ribbon"`/`"proof-chain"` attribute selectors (used today by the `[data-spotlight]` dimming CSS, §2) — these `data-role` values disappear along with the components they marked.

## 11. Ordered implementation sequence

Each step is independently reviewable, leaves the repository runnable and testable, and never exposes a page mixing legacy and new visual systems.

**Step 0 — Author missing scenario fixtures.**
- Created: none (data only).
- Modified: `src/fixtures/alert-investigation/v1.ts` (append scenario-2 and scenario-3 fixture variants, plus two additional broken-traceability variants covering `failedLink: "alert"` and `failedLink: "source_key"`, following the file's existing provenance-documentation header pattern and citing `internal/validation/testdata/scenario2-valid.json`/`scenario3-valid.json` as source).
- Not yet deleted: nothing.
- Tests required before continuing: `lib/alertSource.test.ts` still passes unmodified (new fixture IDs are additive); a quick new assertion that `fixturesById` resolves the new IDs.
- Review checkpoint: fixture content reviewed against the real backend testdata cited above for byte-shape fidelity — no invented field appears.

**Step 1 — Extract the pure domain/view-model layer.**
- Created: `lib/finding.ts`, `lib/concordance.ts`, `lib/provenance.ts`, `lib/characteristicFields.ts`, `lib/evidenceRegister.ts`, `lib/traceability.ts`, `lib/artifacts.ts`, `lib/format.ts`, `lib/docketHeader.ts`, and one `.test.ts` per new file.
- Modified: none (the legacy page keeps running on `deriveStageStatus.ts`/`stageMapping.ts` throughout this step; the new files are additive and unreferenced by the live page).
- Not yet deleted: `lib/deriveStageStatus.ts`, `lib/stageMapping.ts` (still live).
- Tests required before continuing: full new unit-test suite for every function above (including all seven Concordance combinations and all three provenance states) passing; existing suite unaffected.
- Review checkpoint: confirm no `.tsx` file was touched — this step is pure-function-only, independently reviewable without any UI risk.

**Step 2 — Build the new presentational component tree, unwired.**
- Created: every component listed in §3 (`DocketHeader`, `InvestigationFinding`, `EvidenceConcordance`, `ConcordanceConditionRow`, `ProvenanceRecord`, `EvidenceRegister`, `EvidenceRegisterEntry`, `InspectionLeaf`, the six artifact-specific views under `components/artifacts/`, `TraceabilityProof`), each with its `.module.css` and a component-level test, rendered directly against the fixtures from Step 0 in isolation.
- Modified: none.
- Not yet deleted: all legacy components remain, and remain what the live route renders.
- Tests required before continuing: every new component test passing in isolation.
- Review checkpoint: each component reviewed against its §3 specification (props, a11y responsibility) independent of how the page will eventually assemble them.

**Step 3 — Build the new interaction-state layer.**
- Created: `lib/InvestigationStateContext.tsx`.
- Modified: `hooks/useInvestigationCommands.ts` (rewritten command list), `src/test/renderWithProviders.tsx` (wrap the new provider instead of the old one).
- Not yet deleted: `lib/InvestigationUIContext.tsx` (still what the live page uses).
- Tests required before continuing: `useInvestigationCommands.test.ts` (new) passing; `renderWithProviders`-dependent component tests from Step 2 continue to pass against the new provider.
- Review checkpoint: confirm the state shape matches §5 exactly (three fields, four actions) before it is wired into the page.

**Step 4 — The atomic shell swap.**
- Created: none.
- Modified: `AlertInvestigationPage.tsx` (success-branch composition replaced with the §3 tree), `AlertInvestigationPage.module.css` (rewritten per §9), `app/RootErrorBoundary.tsx`/`.module.css` (vocabulary + token fix, §2), `components/StatusBadge/*` (tone vocabulary + pill removal, §2), `components/CommandPalette/CommandPalette.module.css` (token repoint, §2), `components/StateScreens.tsx`/`.module.css` (font/token fix, §2).
- Not yet deleted: `ConduitField`, `ConduitMobileStrip`, `EvidenceRibbon`, `ProofChain`, `artifactIcons`, `LineageInteraction`, `StageContent`, `lib/deriveStageStatus.ts`, `lib/stageMapping.ts`, `lib/InvestigationUIContext.tsx` — all now unreferenced but still present on disk (§8, retirement step 3).
- Tests required before continuing: `AlertInvestigationPage.test.tsx` rewritten (§10) and passing for all seven states plus scenario 2/3; full existing suite green; `npm run typecheck`/`lint`/`build` clean.
- Review checkpoint: **this is the one step where the shipped page's behavior changes.** It must land as one reviewable unit — never split across multiple partially-migrated commits — so there is never a point where the running page mixes legacy and new visual systems (the task's binding constraint).

**Step 5 — Full functional verification of the new shell.**
- Created/modified: none (verification only) — plus any fixes surfaced by verification, scoped narrowly to what failed.
- Tests required: `npm run typecheck`, `npm run lint`, `npm run test`, `npm run build`, a manual keyboard-only walkthrough of all three scenarios and every failure state, zero console errors confirmed manually against the dev build (ahead of Step 7's automated check).
- Review checkpoint: every criterion in spec §12 verified against the running grayscale build before any deletion happens.

**Step 6 — Retire the now-dead legacy files and sweep vocabulary/styling.**
- Deleted: the files listed in §8 steps 3–5 (retirement sequence), plus the `@fontsource/fraunces` dependency from `package.json` once confirmed unimported.
- Modified: `styles/tokens.css`, `styles/global.css` (§2, §9); `AlertInvestigationPage.tsx`'s `"Evidence Theater"` string (§8).
- Not yet deleted: nothing remains to delete after this step for the vocabulary/component retirement; `docs/frontend/product-experience-brief.md` and the static design reference remain untouched (documentation retirement is out of scope for a code-implementation task).
- Tests required before continuing: full suite re-run green after deletion (catches any straggling import).
- Review checkpoint: repository-wide search for every term in spec §14 plus this plan's two additional findings (§8) returns zero hits outside historical documentation.

**Step 7 — Rewrite the Playwright suite and close out test migration.**
- Modified: `e2e/flagship.spec.ts` per §10's full checklist (scenario 2/3, partial availability, all three `failedLink` values, keyboard/palette, zero console errors, desktop and mobile).
- Tests required: `npm run e2e` green.
- Review checkpoint: `AlertInvestigationPage.test.tsx` and `flagship.spec.ts` together demonstrably cover every item in spec §12.

**Step 8 — visual-authorship pass — explicitly out of scope for this plan** (spec §16). Named here only so it is not silently skipped or silently started early: typeface selection, the single accent hue, spacing-rhythm refinement, and any motion beyond functional necessity begin only after Step 7 is reviewed and approved.

## 12. Risk controls and rollback

- **Contract fidelity.** `lib/artifacts.ts`, `lib/finding.ts`, `lib/concordance.ts`, and `lib/provenance.ts` are the *only* places permitted to branch on `AlertInvestigationResponse` shape (§4); every one of them is unit-tested directly against `contract.ts`'s types and the real fixtures, not against invented shapes.
- **Avoiding invented telemetry.** New fixtures (Step 0) are sourced from the backend's own committed testdata and detection definitions, following `v1.ts`'s existing provenance-documentation discipline verbatim — no hand-imagined field value is introduced.
- **Protecting current lineage behavior.** `lineage.ts` is explicitly unmodified (§2, §4); its existing test suite (`lineage.test.ts`) must continue passing, unedited, through every step of §11 as a regression guard.
- **Preventing scenario-1-specific architecture.** `lib/concordance.ts` and `lib/provenance.ts` are unit-tested against scenario 2's and scenario 3's real declared-condition shapes (§6) from the moment they are written (Step 1), not retrofitted after a scenario-1-only implementation is already built.
- **Maintaining accessibility.** Native `<button>` semantics for every selectable row (§3, §5) mean focus order and activation are free, not re-implemented; the single `:focus-visible` mechanism, the `aria-live` regions, and Radix's own dialog accessibility are all explicitly preserved or extended, never dropped (§2, §9).
- **Handling the fully untracked `frontend/` directory.** **[Finding]** The entire `frontend/` directory has been untracked in git for this whole engagement (confirmed by every `git status` run in this and prior sessions this branch). This means `git diff`/`git status` provide **no** safety net for Step 6's deletions — there is no committed prior state to recover from with a simple `git checkout`. Before Step 6 (destructive legacy-file deletion) proceeds, the user should either explicitly commit the pre-deletion state as a recoverable checkpoint (an action this document does not perform, per its own constraints) or explicitly accept that recovery would depend on editor/OS-level undo history alone. This is called out again in §13 as the one genuine process gate.
- **Avoiding accidental backend changes.** Every file touched across §2, §8, and §11 is under `frontend/`; §6/§11's reference reads of `definitions/*.yaml` and `internal/*/testdata/*.json` are read-only research, never edits, exactly as performed during this planning pass.
- **Rollback.** Because Step 4 (the atomic swap) is the only step that changes shipped behavior, and Steps 0–3 are purely additive, rollback before Step 4 is simply "stop, delete the new unreferenced files" with zero effect on the live page. Rollback after Step 4 but before Step 6 is a single revert of Step 4's commit, since nothing has been deleted yet. Rollback after Step 6 is where the recoverable checkpoint from the bullet above becomes necessary.

## 13. Definition of implementation readiness

**Steps 0–5 of §11 can begin immediately upon this plan's approval.** They depend on no unresolved product, UX, or architecture decision:

- The UX specification (all 16 sections) is approved and internally consistent with this plan's design.
- Every declared-condition shape needed for Steps 0–1 (§6) has been directly confirmed by reading the real, committed `definitions/*.yaml` files during this planning pass — including scenario 3's, which the UX spec itself had left as an open assumption (§15 item 7) and this plan now resolves.
- The one remaining UX-spec open item not resolved here — whether a future, documented/typed `requestObject` shape will ever let scenario 2/3 provenance move from Partial to Verified (spec §15 item 6) — is not a blocker: it is a backend/architecture question entirely outside this plan's scope, and the Partial-provenance behavior this plan implements is already the correct, complete v0.1 behavior regardless of how that question is eventually resolved.

**One genuine blocker exists, and it gates only Step 6, not the start of implementation:** because `frontend/` is entirely untracked in git, destructive legacy-file deletion (§11 Step 6) should not proceed without either (a) the user explicitly committing a recoverable checkpoint first, or (b) the user explicitly waiving that safeguard. This document does not commit on the user's behalf. No other step in §11 requires anything beyond ordinary code review.

No other blocker was found. In particular: no backend change is required or proposed anywhere in this plan; no alert index, search, detection catalog, severity, confidence, case-management, assignment, or disposition concept is introduced anywhere in §3–§10; no new telemetry family is proposed (§6's extensibility is structural, not a claim that a fourth scenario exists today); the static flagship prototype is not used as a DOM/CSS source anywhere in §9 or §11; and every file marked "preserve" in §2 is preserved for the specific, stated reason in that row, not by default.
