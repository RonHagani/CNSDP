# Cloud-Native Security Telemetry and Detection Platform — Alert Investigation UX Implementation Plan

| Field | Value |
| --- | --- |
| Document | CNSDP Alert Investigation UX Implementation Plan |
| Version | 0.2 |
| Status | Approved — Phase 1.5 implementation baseline |
| Phase | Phase 1.5 — Security Investigation Experience |
| Identifier | Not assigned. Outside the closed PC-015 namespace. Referenced by path only: `docs/frontend/alert-investigation-implementation-plan.md`. |
| Authoritative sources | `../product.md`, `../personas.md`, `../use-cases.md`, `../scope.md`, `../functional-requirements.md`, `../non-functional-requirements.md`, `../acceptance-criteria.md`, `../architecture.md`, `../reference-environment.md`, and — for this document's UX authority — `alert-investigation-ux-spec.md` v0.2 in full. `product-experience-brief.md` and this file's own v0.1 predecessor are superseded historical direction, cited below only to identify what must not be reintroduced. |
| Relationship to baseline | This document proposes no new product scope, requirement, or acceptance criterion, and no backend change. It is an implementation plan only: it translates `alert-investigation-ux-spec.md` v0.2 into a concrete, phased build plan against the current codebase as it actually exists today (confirmed by direct inspection during this planning pass, cited as **[Finding]** below). |

**Scope discipline, restated as binding constraints on this plan:** no backend change; no alert index, search, or detection-definition catalog; no severity, confidence, case status, assignment, or disposition; no new telemetry source family; no dedicated mobile/phone investigation experience (UX spec §17); nothing already built is preserved merely because it exists — every retained file below is retained for a stated, specific reason. **This plan performs no deletion, staging, commit, or push.** It defines the sequence a later implementation task must follow.

## 1. What changed since the prior implementation plan

**[Finding]** The prior implementation plan (v0.1) targeted the "Causal Evidence Dossier" direction approved by UX spec v0.1. That plan was substantially executed: the live code under `frontend/src/features/alert-investigation/` today is a six-element investigation screen with a sound, presentation-neutral domain/view-model layer underneath it — but its presentation layer was subsequently restyled, without a corresponding approved specification, into what its own source comments call the "Forensic Case Folio" (`InvestigationDossier.tsx`: *"Composition Reset"*; `components/dossier/dossier.module.css`: folio/custody/evidentiary-clause vocabulary). This restyling is live on disk today and is not the approved direction.

**[Finding]** Separately, a first-generation cluster of components from the *original* "Signal Path" direction (`product-experience-brief.md`, already superseded before v0.1 of the UX spec was approved) remains present on disk and **entirely unreferenced by the live page** — confirmed by a repository-wide import search during this planning pass (§4). This cluster was never cleaned up by the v0.1 plan's own retirement step.

**This plan therefore has two independent cleanup tracks, not one:**

1. **Track A — already-dead code** (the Signal Path cluster): safe to remove immediately, independent of any new build work, because it is provably unreferenced today.
2. **Track B — live-but-superseded code** (the Forensic Case Folio presentation layer): must be replaced by the new Dark Evidence Map presentation, and only removed after its replacement is built and verified — never deleted first.

The sound domain/view-model layer beneath both is unaffected by either track and is preserved through this plan almost entirely unchanged (§3).

## 2. What the new implementation must accomplish

At the end of this plan's execution:

- The rendered screen is the Dark Evidence Map: a permanent alert identity header, the six evidence artifacts in their distinct causal-role shapes, a permanent Traceability Rail, and the condition-selection/provenance-tracing interaction — exactly as `alert-investigation-ux-spec.md` v0.2 §3–§16 specifies.
- All three supported scenarios (interactive `pods/exec`; high-risk Pod creation; cluster-admin `ClusterRoleBinding` creation) render correctly through the same generic mechanisms, with no scenario-specific branch in any component (spec §7, §8, §19 item 10).
- All six evidence artifacts render both their normal and unavailable states correctly and independently (spec §15).
- All three provenance states — Verified, Partial, Unavailable — render correctly, structurally distinct from one another (spec §13).
- Both traceability states render correctly, including each of the three named `failedLink` values individually localized (spec §11).
- Declared-but-unsatisfied characteristics render in their own distinct, non-matched-evidence state (spec §8, §12) — a real gap in the current live implementation, which discards them entirely (§3).
- The full map is keyboard-operable with visible focus, per spec §16.
- The four route-level states (loading, not-found, unauthorized, unavailable/retrieval-failure) render distinctly, reusing the existing, sound fetch/error taxonomy unchanged.
- The screen is fully functional and legible at 1600px, 1440px, 1280px, and 1024px, with a deliberate, readable, non-catastrophic fallback below 1024px (spec §17) — no dedicated mobile composition is built.
- Neither the Forensic Case Folio presentation (Track B) nor the Signal Path cluster (Track A) remains reachable from the shipped page.

## 3. Domain and view-model layer: what is preserved, what changes

**[Finding]** The domain/view-model layer under `frontend/src/features/alert-investigation/lib/` is sound, presentation-neutral, and was already identified as reusable regardless of visual direction by the design investigation that preceded this plan (`frontend/review-artifacts/alert-investigation-design-decision.md` §2.9, a local, gitignored review artifact — not itself authoritative, but its factual findings about this layer were independently reconfirmed by direct inspection during this planning pass). Only two small, scoped changes are required; everything else is preserved as-is.

| Module | Disposition | Reason |
| --- | --- | --- |
| `lib/alertSource.ts`, `hooks/useAlertInvestigation.ts` | **Preserve unchanged** | Fetch seam, `AlertFetchError` taxonomy, and TanStack Query wrapper are visual-direction-independent. |
| `lib/lineage.ts` | **Preserve unchanged** | `buildLineageLinks`/`resolveHighlights` already produce a link only for a genuinely verifiable raw path — the exact Verified/Partial boundary spec §13 requires, by construction. |
| `lib/provenance.ts` | **Preserve unchanged** | Already implements exactly the three-state engine spec §13 specifies, consuming `lineage.ts` unchanged. |
| `lib/traceability.ts` | **Preserve unchanged** | Already implements exactly the intact/broken model spec §11 specifies, including the real `FAILED_LINK_EXPLANATION` copy the rail reuses verbatim. |
| `lib/evidenceRegister.ts` | **Preserve unchanged** | The six-artifact availability/default-selection logic is visual-direction-independent; only its consumer changes (§5) — from a ledger list to six map positions. |
| `lib/finding.ts` | **Preserve unchanged** | The CLAIM view-model the identity header (spec §4) consumes directly. |
| `lib/artifactInspection.ts` | **Preserve unchanged** | The six typed, presentation-neutral artifact views each map object (spec §5–§10) renders directly. |
| `lib/detectionConditions.ts` | **Preserve unchanged** | `buildDeclaredConditions` already computes `satisfied: boolean` per characteristic — exactly the data spec §8's declared-vs-satisfied pin distinction needs. Nothing here needs to change; the gap is entirely in how `concordance.ts` currently *discards* this information (next row). |
| `lib/concordance.ts` | **Modify, not rewrite** | **[Finding]** `buildEvidenceConcordance` currently contains `if (!c.satisfied) continue;` (line ~106) — it silently drops every declared-but-unsatisfied `requires_any` characteristic before it ever reaches a component. Spec §8 requires these to render, in their own recessed state, on the characteristic bus. The fix is scoped: stop discarding unsatisfied characteristics; carry each row's existing `satisfied: boolean` through to the output type instead (`CharacteristicConcordanceRow` gains one field); only compute a `ProvenanceState` for satisfied rows, since provenance is only meaningful for matched evidence (spec §8, §12's "only matched evidence may be traced to source"). No other function in this file changes. |
| `lib/investigationViewModel.ts` | **Modify — rename only** | `DocketHeaderViewModel` renames to match spec §4's vocabulary (e.g. `AlertIdentityHeaderViewModel`); its fields and computation are unchanged. A cosmetic rename, not a logic change. |
| **New:** `lib/characteristicGroups.ts` | **New — data only** | The subgroup-clustering table spec §8 requires (host-namespace/access vs. privilege for scenario 2's five characteristics) does not exist today. This is a literal `Record<string, string>` mapping each real, already-declared characteristic id to a group label drawn directly from that characteristic's own `description` text in the committed detection definitions (`definitions/scenario-2.yaml`) — data, not logic, following the exact pattern `lib/provenance.ts`'s own `CHARACTERISTIC_NORMALIZED_PATH` table already establishes. Adding a future scenario's grouping is a one-line addition, never a structural change. |

**Contract fidelity rule, carried forward from the prior plan and restated as still binding:** `lib/artifacts.ts`-equivalent (`artifactInspection.ts`), `lib/finding.ts`, `lib/concordance.ts`, and `lib/provenance.ts` remain the *only* places permitted to branch on `AlertInvestigationResponse` shape. Every presentational component below receives an already-resolved, already-typed view-model and never re-derives anything from the raw response.

## 4. Track A — already-dead code (safe to remove independent of this plan)

**[Finding]** Confirmed by a repository-wide search for real `import` statements (not comments or test-only references) across `frontend/src`, the following form one self-contained, mutually-referencing cluster with **zero** live consumers — not imported by `AlertInvestigationPage.tsx`, `InvestigationDossier.tsx`, any `components/dossier/**` file, or any other file outside the cluster itself:

| File | Confirmed reference source |
| --- | --- |
| `components/ConduitField.tsx` (+ `.module.css`) | Imports `stageMapping.ts`, `deriveStageStatus.ts`, `InvestigationUIContext.tsx` — all inside the cluster. Zero external importers. |
| `components/ConduitMobileStrip.tsx` (+ `.module.css`) | Same three cluster-internal imports. Zero external importers. |
| `components/EvidenceRibbon.tsx` (+ `.module.css`) | Imports `InvestigationUIContext.tsx`, `artifactIcons.tsx` — cluster-internal. Zero external importers. |
| `components/ProofChain.tsx` (+ `.module.css`) | No cluster-internal imports; zero external importers either. |
| `components/StageContent.tsx` (+ `.module.css`) | Imports `stageMapping.ts`, `InvestigationUIContext.tsx`, `LineageInteraction.tsx` — cluster-internal. Zero external importers. |
| `components/LineageInteraction.tsx` (+ `.module.css`) | Imports `InvestigationUIContext.tsx` — cluster-internal. Its only importer is `StageContent.tsx` (cluster-internal) and its own test file (next row). |
| `components/LineageInteraction.test.tsx` | Imports `LineageInteraction.tsx` and `src/test/renderWithProviders.tsx` — both cluster-internal/cluster-only. |
| `components/artifactIcons.tsx` | Zero external importers beyond `EvidenceRibbon.tsx` (cluster-internal). |
| `components/InvestigationHero.tsx` (+ `.module.css`) | Zero importers found anywhere, including within the cluster — fully orphaned. |
| `lib/deriveStageStatus.ts` (+ `.test.ts`) | Imported only by `ConduitField.tsx`/`ConduitMobileStrip.tsx` (cluster-internal) and its own test. |
| `lib/stageMapping.ts` | Imported only by `ConduitField.tsx`/`ConduitMobileStrip.tsx`/`StageContent.tsx` (cluster-internal). |
| `lib/InvestigationUIContext.tsx` | Imported only by the cluster components above and `src/test/renderWithProviders.tsx`. |
| `src/test/renderWithProviders.tsx` | Imported only by `LineageInteraction.test.tsx` (cluster-internal). |

**This entire track is independently removable in one isolated step, with no dependency on Track B or on any new component existing yet.** It requires only: delete the thirteen files above; run the full test suite (their removal should reduce test count by exactly `LineageInteraction.test.tsx`'s cases, and no other suite should fail); run `typecheck`/`lint`/`build` to confirm no straggling reference. This step may be executed before, during, or independently of the rest of this plan — it is called out first because it is the lowest-risk, highest-confidence cleanup available, not because it must happen first.

## 5. Track B — proposed new component architecture

**[Plan] Component tree**, replacing `InvestigationDossier.tsx`'s current Folio composition:

```
AlertInvestigationPage                         (route component; unchanged name/path/logic)
└── on success:
    InvestigationMap                           (new orchestrator — replaces InvestigationDossier)
    ├── AlertIdentityHeader                     (spec §4 — replaces CaseOpening)
    ├── EvidenceCanvas                          (new — the spatial map container; owns layout only)
    │   ├── SourceSubmissionSpecimen            (spec §5)
    │   │   └── (on-demand) RawRecordViewer      (reuses JsonTree unchanged)
    │   ├── ValidationStamp                     (spec §6)
    │   ├── NormalizedEventSurface              (spec §7)
    │   ├── DetectionDefinitionFrame            (spec §8)
    │   │   └── CharacteristicPin  × N          (spec §8 — replaces ConcordanceConditionRow; satisfied and declared-only variants)
    │   ├── DetectionResultResolution           (spec §9 — new concept, no direct predecessor)
    │   ├── GeneratedAlertMarker                (spec §10)
    │   ├── TraceabilityRail                    (spec §11 — replaces TraceabilityProof)
    │   └── ProvenanceAnnotation                (spec §12–§13 — replaces ProvenanceRecord; anchored/leader-line positioned, not inline-expanding)
    └── (optional, spec §16 [Future]) CommandPalette   (reused unchanged, if retained)
```

**[Plan] Why `EvidenceCanvas` owns layout only.** Every one of the six artifact components and the rail receives already-resolved view-model data from `InvestigationMap` (which builds `investigationViewModel.ts`'s output exactly once per response, unchanged pattern from today) and knows nothing about its neighbors' positions. `EvidenceCanvas` is the one place spatial arrangement (which corner, which connector, which z-order) is decided, so that no individual artifact component needs to know the map's overall geometry — mirroring the isolated prototype's own `LAYOUT` constant and `wireUpDesktop` separation of concerns (`frontend/review-artifacts/dark-evidence-map/prototype.js`), translated into component boundaries rather than a single script.

**[Plan] Why `CharacteristicPin` is one generic component, not N specific ones.** Pins are structurally identical regardless of which characteristic, which group, or which scenario produced them, differing only in the *data* `concordance.ts` supplies (id, description, group label, satisfied flag, and — for satisfied pins only — a `ProvenanceState`). One component takes a discriminated view-model and renders either its full (satisfied) or recessed (declared-only) treatment; it does not need scenario knowledge.

**[Plan] Why `ProvenanceAnnotation` is a single mount point.** Exactly one annotation is ever open at a time (spec §12), regardless of which pin triggered it — the same mutual-exclusivity reasoning the prior plan applied to its single `InspectionLeaf` mount point applies here, structurally guaranteeing the "one leaf" rule rather than relying on every trigger site to remember it.

**Component specifications:**

| Component | Responsibility | Domain data consumed | Local state | Generic or feature-specific |
| --- | --- | --- | --- | --- |
| `InvestigationMap` | Builds the view model once; owns selection state (`selectedCharacteristicId: string \| null`) and its one action. | `buildInvestigationViewModel(data)` (unchanged) | `selectedCharacteristicId` | Feature-specific |
| `AlertIdentityHeader` | Renders spec §4's composed sentence plus metadata. | `AlertIdentityHeaderViewModel` (renamed, §3) | none | Feature-specific |
| `EvidenceCanvas` | Spatial layout and connector rendering only. | passes through to children | none | Feature-specific |
| `SourceSubmissionSpecimen` | Three-tier raw specimen (spec §5); hosts the on-demand full-record toggle. | `ArtifactInspectionModel.sourceEvent` | `rawRecordExpanded: boolean` (local, mirrors today's established toggle pattern) | Feature-specific |
| `ValidationStamp` | Compact outcome checkpoint (spec §6). | `ArtifactInspectionModel.validationOutcome` | none | Feature-specific |
| `NormalizedEventSurface` | Full field surface, generic across scenario blocks (spec §7). | `ArtifactInspectionModel.normalizedEvent` | none | Generic row renderer, feature-specific container |
| `DetectionDefinitionFrame` | Rule frame chrome; lays out `CharacteristicPin` children along the bus, applying `lib/characteristicGroups.ts` clustering. | `ArtifactInspectionModel.detectionDefinition`, `EvidenceConcordance.rows` (post-§3 modification) | none (selection lifted to `InvestigationMap`) | Feature-specific |
| `CharacteristicPin` | One pin, satisfied or declared-only. | `ConcordanceRow` (post-modification shape) | none | Generic (no scenario knowledge) |
| `DetectionResultResolution` | The convergence/tally object (spec §9). | `EvidenceConcordance.declaredCharacteristicCount`/`satisfiedCharacteristicCount` | none | Feature-specific |
| `GeneratedAlertMarker` | Terminal artifact marker (spec §10). | `ArtifactInspectionModel.alert` | none | Feature-specific |
| `TraceabilityRail` | The permanent four-segment path (spec §11). | `TraceabilityProofModel` (unchanged) | none | Feature-specific |
| `ProvenanceAnnotation` | The anchored leader-line annotation for the selected pin (spec §12–§13). | `ProvenanceState` of the selected characteristic, or the declared-only statement | none (position computed from the selected pin's rendered location) | Feature-specific |
| `JsonTree` | Unchanged recursive raw/JSON renderer. | — | — | Generic (unchanged) |

## 6. Interaction and state model

**[Plan]** `InvestigationMap` owns exactly one piece of interaction state:

```ts
const [selectedCharacteristicId, setSelectedCharacteristicId] = useState<string | null>(null);
```

- Selecting a `CharacteristicPin` (satisfied or declared-only) calls `setSelectedCharacteristicId(id)`, toggling to `null` on re-selecting the same pin (matches the isolated prototype's own toggle behavior).
- `ProvenanceAnnotation` renders only when `selectedCharacteristicId` is non-null, deriving its content from `deriveProvenanceState` (satisfied pins) or the plain declared-only statement (declared-only pins) — computed in `InvestigationMap`, passed down as an already-resolved prop, never recomputed inside a presentational component (§3's contract-fidelity rule).
- No context provider is introduced. **[Plan, evaluated explicitly]** Unlike the prior plan's `InvestigationStateContext` (never actually built — the live code today uses plain `useState` in `InvestigationDossier.tsx`), this state is smaller still: one nullable string, read by two direct children (`DetectionDefinitionFrame`'s pins and `ProvenanceAnnotation`) of the same parent. Prop-drilling two levels is simpler and more locally reviewable than introducing a provider for a single field; if a future addition (e.g., a reinstated command palette, spec §16) needs broader access, that is a separate, explicitly justified decision at that time, not assumed now.
- Keyboard behavior (spec §16): every `CharacteristicPin` and the raw-record toggle are native-focusable `<button>`-equivalent elements; Tab order follows DOM order; Enter/Space activation is free from native semantics, requiring no custom key handling beyond what the isolated prototype already demonstrated (`prototype.js`'s `attachPinHandlers`, adapted to React event handlers rather than a single delegated listener).

## 7. Scenario-aware behavior verification matrix

No component branches on scenario identity anywhere (spec §7, §19 item 10). This table confirms, from the real committed fixtures and detection definitions, exactly what each scenario's map must render — the acceptance surface for Pass 4/5 (§9).

| | Scenario 1 — `pods/exec` | Scenario 2 — high-risk Pod creation | Scenario 3 — cluster-admin `ClusterRoleBinding` |
| --- | --- | --- | --- |
| Declared characteristics | `requires_any`: `stdin_streaming`, `tty_allocation` (2) | `requires_any`: 5 (`privileged_container`, `host_network`, `host_pid`, `host_ipc`, a host-path-volume characteristic) | `requires_all`: `role_ref_cluster_admin` (1) |
| Outcome clause | None declared (FR-024) | `requires_outcome: success` | `requires_outcome: success` |
| Bus grouping (spec §8) | None (below the two-clustering-cases threshold; both characteristics render as one ungrouped set) | Host-access (4) vs. privilege (1), via `lib/characteristicGroups.ts` | None (single characteristic) |
| Satisfied count (current fixtures) | 2 of 2 | 5 of 5 | 1 of 1 |
| **Untested by current fixtures:** a declared-but-unsatisfied `requires_any` characteristic | **Gap — §8's new pin state has no fixture coverage today.** A new fixture variant (or a hand-built unit-test case, §8 of this plan) exercising a `requires_any` group where fewer than all declared options are satisfied is required before spec §19 item 4 can be verified end-to-end. | Same gap. | Not applicable — `requires_all` groups are always fully satisfied by construction for an existing alert. |
| Provenance (all satisfied characteristics) | Verified (both `requestURI`-parsed fields) | Partial (all five — `requestObject` untyped) | Partial (the one characteristic — same reason) |

## 8. Fixture and test gaps to close before full verification

**[Finding]** `frontend/src/fixtures/alert-investigation/v1.ts` today provides five fixtures: scenario-1 intact, scenario-1 partial-availability (detection definition unavailable), scenario-1 broken-traceability (`raw_event_sha256` only), scenario-2 intact, scenario-3 intact. Two real gaps exist against this plan's required verification surface (§7, §9):

1. **No fixture exercises a declared-but-unsatisfied `requires_any` characteristic.** All three scenarios' real committed fixtures happen to satisfy every declared option. A new fixture (or a scoped hand-built object for `lib/concordance.test.ts` alone, if a fully honest end-to-end fixture cannot be sourced from real backend testdata) is required to verify spec §8's declared-only pin rendering and §12's declared-only selection behavior. Any new fixture content must follow `v1.ts`'s own established provenance discipline (cited source, real backend testdata reference) — never hand-invented event content presented as if it were real.
2. **Two of the three `failedLink` values have a fixture; the third cannot.** `raw_event_sha256` (id 3) is covered, and `source_key` is the one additional broken-traceability fixture variant required to verify spec §11's per-segment localization beyond `raw_event_sha256` — additive to `v1.ts`. **[Finding, established during Pass 1's execution]** The third named value, `alert`, cannot be added as a real `v1.ts` fixture: direct inspection of the current Go backend shows `internal/traceability/traceability.go`'s `VerifyAlert` returns `FailedLink: "alert"` only when the alert's join query finds no rows — the exact condition under which `internal/evidence/evidence.go`'s `Compose` (which resolves the identical join via `traceability.Locate` before reading any artifact) already returns `ErrNotFound`, mapped by `internal/retrieval` to a 404 before any response body exists. A six-artifact-available response with `failedLink: "alert"` is therefore not producible by the current backend — a backend-reachability limitation, not grounds to drop the contract value or its frontend handling. `alert` remains a valid `FailedLink` contract value, fully supported by the frontend's rail/localization logic (spec §11); its existing synthetic, hand-built-model coverage (`lib/traceability.test.ts`; `TraceabilityRail.test.tsx`'s dedicated unit-level test) remains the correct and sufficient verification for it, and no fixture is fabricated to fill this gap.

Both gaps are additive to `v1.ts`, not a change to any existing fixture — the existing five remain valid and unmodified.

## 9. Ordered implementation sequence

Each pass is independently reviewable, leaves the repository runnable and testable, and never exposes a page mixing the new map with the live Folio presentation.

**Pass 0 — Track A cleanup (independent, may run any time).**
- Delete the thirteen files listed in §4.
- Verify: full test suite green (`npm run test`); `npm run typecheck`; `npm run lint`; `npm run build`.

**Pass 1 — Close fixture/test gaps (§8).**
- Add the declared-but-unsatisfied characteristic case and the one addable broken-traceability fixture variant (`source_key`) to `v1.ts`, following its existing documentation-header discipline. The third named `failedLink` value, `alert`, has no corresponding `v1.ts` fixture, for the backend-reachability reason §8 documents; its existing synthetic coverage is the correct verification for it.
- Verify: `lib/alertSource.test.ts` and fixture-consuming tests still pass unmodified against the additive fixtures; a new assertion resolves each new fixture id.

**Pass 2 — Domain-layer adjustments (§3).**
- Modify `lib/concordance.ts` to retain declared-but-unsatisfied rows (with `satisfied: false`, no `ProvenanceState`) instead of discarding them.
- Add `lib/characteristicGroups.ts`.
- Rename `DocketHeaderViewModel` in `investigationViewModel.ts`.
- Verify: every existing `lib/*.test.ts` still passes; new/updated assertions cover the retained declared-only rows using Pass 1's new fixture; `npm run typecheck`. No `.tsx` file is touched in this pass — it is independently reviewable without UI risk, exactly as the prior plan's equivalent step was.

**Pass 3 — Build the new presentational component tree, unwired.**
- Every component in §5's tree, each with its own test, rendered in isolation against the fixtures (existing plus Pass 1's additions).
- Legacy Folio components continue rendering the live page throughout this pass; nothing is retired yet.
- Verify: every new component test passes in isolation; each component reviewed against its §5 specification row (props, data consumed, accessibility responsibility) independent of final page assembly.

**Pass 4 — Build the interaction-state layer (§6).**
- Wire `selectedCharacteristicId` into `InvestigationMap`, connecting `DetectionDefinitionFrame`'s pins to `ProvenanceAnnotation`.
- Verify: component-level interaction tests (select a satisfied pin → correct annotation; select a declared-only pin → correct plain statement; re-select → closes) pass against Pass 3's isolated tree, still unwired from the live route.

**Pass 5 — The atomic shell swap.**
- Replace `InvestigationDossier.tsx`'s composition (or replace the file itself with `InvestigationMap.tsx`, updating `AlertInvestigationPage.tsx`'s one import) in a single reviewable change.
- Legacy Folio components (`CaseOpening`, `EvidenceConcordance`, `ConcordanceConditionRow`, `ProvenanceRecord`, `EvidenceRegister`, `EvidenceRegisterEntry`, `InspectionLeaf`, `TraceabilityProof`, `components/dossier/dossier.module.css`) become unreferenced by the shipped page at this point, but are **not yet deleted**.
- Verify: `AlertInvestigationPage.test.tsx` rewritten and passing for all route-level states plus all three scenarios; full suite green; `npm run typecheck`/`lint`/`build` clean. **This is the one pass where shipped behavior changes and must land as one reviewable unit** — never split across multiple partially-migrated commits, so the running page never mixes the Folio and Dark Evidence Map presentations.

**Pass 6 — Full functional verification (§10's gates, in full).**
- Every state in §7's matrix, plus loading/not-found/unauthorized/unavailable, at 1600px, 1440px, 1280px, and 1024px, plus a manual check of the below-1024px fallback (spec §17) for absence of horizontal overflow and absence of hidden content.
- Manual keyboard-only walkthrough of all three scenarios and every failure state.
- Verify: every criterion in spec §19 confirmed against the running build before any deletion happens.

**Pass 7 — Retire Track B (the now-verified-dead Folio files) and sweep vocabulary.**
- Delete: `components/dossier/CaseOpening.tsx`, `EvidenceConcordance.tsx`, `ConcordanceConditionRow.tsx`, `ProvenanceRecord.tsx`, `EvidenceRegister.tsx`, `EvidenceRegisterEntry.tsx`, `InspectionLeaf.tsx`, `TraceabilityProof.tsx`, `dossier.module.css`, `dossier/index.ts`, `dossier/types.ts`, and the six `dossier/artifacts/*.tsx` views (superseded by §5's new artifact components — confirm each new component's replacement is in place and verified before deleting its predecessor).
- Modify: `contract.ts` (remove the dead `SignalPathStageId` export, §3 of the UX spec); `styles/tokens.css`/`global.css` (remove the Fraunces import and any remaining Signal-Path-era token names — confirmed still present today, §11); `package.json` (fix the `"description"` field, still literally *"CNSDP Phase 1.5 — The Signal Path..."* today, §11; remove `@fontsource/fraunces` once confirmed unimported); `components/StatusBadge/StatusBadge.tsx` and `components/StateScreens.tsx` (replace the live `"signal" | "branch" | "severed" | "neutral"` `StatusTone` vocabulary with names matching the UX spec's own real vocabulary, e.g. `"intact" | "warning" | "broken" | "neutral"`, §11); `app/RootErrorBoundary.tsx` (**[Finding]** its rendered eyebrow text is the literal live string `"CNSDP — Signal Path"`, line 20 — a one-line text change, distinct from the dossier tree entirely, but still a real, currently-shipped instance of the retired term).
- Verify: full suite green after deletion (catches any straggling import); repository-wide search for every retired term in UX spec §21 returns zero hits outside historical documentation and version-control history.

**Pass 8 — Rewrite the Playwright suite.**
- `e2e/flagship.spec.ts` rewritten against the new DOM, covering the full checklist in §10.
- Verify: `npm run e2e` green, desktop viewports only (1600/1440/1280/1024 — no mobile viewport per spec §17).

## 10. Verification gates

Every pass above states its own scoped verification; this section is the full gate set referenced throughout, stated once.

| Gate | Command / method | Applies to |
| --- | --- | --- |
| Typecheck | `npm run typecheck` | Every pass that touches a `.ts`/`.tsx` file |
| Lint | `npm run lint` | Every pass that touches a `.ts`/`.tsx` file |
| Unit tests | `npm run test` | Every pass |
| Production build | `npm run build` | Passes 0, 2, 5, 6, 7 (structural risk points) |
| Playwright e2e | `npm run e2e` | Pass 8, and re-run after Pass 7's deletions |
| Manual keyboard-only walkthrough | All three scenarios, every failure state, Tab/Enter/Space only, visible focus confirmed at every step | Pass 6 |
| Desktop viewport check | 1600px, 1440px, 1280px, 1024px — no horizontal scroll, no dropped content | Pass 6 |
| Below-1024px fallback check | Manual — confirm readable, non-catastrophic, no hidden artifact/traceability state (spec §17) | Pass 6 |
| Vocabulary sweep | Repository-wide text search for every term in UX spec §21 | Pass 7 |
| Zero console errors | Manual dev-build check across the full interaction flow | Pass 6 |

**State coverage required at Pass 6 (spec §19's full acceptance surface):** Scenario 1, Scenario 2, and Scenario 3 (each with a satisfied-pin selection); the declared-but-unsatisfied pin case (Pass 1's new fixture); partial artifact availability (existing fixture 2); broken traceability for **each** of the three `failedLink` values individually (Pass 1 adds the `source_key` fixture; `alert` is verified through its existing synthetic coverage, §8, since no real fixture for it exists); loading; not-found; unauthorized; unavailable-with-retry.

## 11. Cleanup inventory

Restated here as one consolidated list for the final report, cross-referencing where each item is handled above. **No item in this list is deleted, moved, renamed, staged, or committed by this planning document.**

**Safe to remove now, independent of any other work (Track A, §4 — confirmed zero live references):**

- `components/ConduitField.tsx` + `.module.css`
- `components/ConduitMobileStrip.tsx` + `.module.css`
- `components/EvidenceRibbon.tsx` + `.module.css`
- `components/ProofChain.tsx` + `.module.css`
- `components/StageContent.tsx` + `.module.css`
- `components/LineageInteraction.tsx` + `.module.css` + `.test.tsx`
- `components/artifactIcons.tsx`
- `components/InvestigationHero.tsx` + `.module.css`
- `lib/deriveStageStatus.ts` + `.test.ts`
- `lib/stageMapping.ts`
- `lib/InvestigationUIContext.tsx`
- `src/test/renderWithProviders.tsx`

**Safe to remove only after its named replacement is built and verified (Track B, §5, §9 Pass 7):**

- `components/dossier/CaseOpening.tsx` — after `AlertIdentityHeader` ships.
- `components/dossier/EvidenceConcordance.tsx`, `ConcordanceConditionRow.tsx` — after `DetectionDefinitionFrame`/`CharacteristicPin` ship.
- `components/dossier/ProvenanceRecord.tsx` — after `ProvenanceAnnotation` ships.
- `components/dossier/EvidenceRegister.tsx`, `EvidenceRegisterEntry.tsx` — after the six map-positioned artifact components ship (no direct one-to-one replacement; superseded by spec §15's model).
- `components/dossier/InspectionLeaf.tsx` — after the six artifact components' own always-visible content plus `RawRecordViewer` ship.
- `components/dossier/TraceabilityProof.tsx` — after `TraceabilityRail` ships.
- `components/dossier/dossier.module.css`, `dossier/index.ts`, `dossier/types.ts`, `dossier/artifacts/*.tsx` (6 files) — after every component that currently imports them is retired.
- `InvestigationDossier.tsx` + `.test.tsx` — after `InvestigationMap` ships and `AlertInvestigationPage.tsx` points to it.

**Requires further investigation before this plan's own execution, not before this documentation task:**

- Whether `components/CommandPalette/CommandPalette.tsx` and `hooks/useInvestigationCommands.ts` are retained (spec §16 marks a command surface [Future], not mandated). If retained, `useInvestigationCommands.ts` needs a rewritten command list against the new component tree; if not, both retire alongside Track B and `CommandPalette.module.css`'s token references (§11 finding, next item) become moot.
- `components/CommandPalette/CommandPalette.module.css` — **[Finding]** references a Signal-Path-era token (`--cnsdp-signal-surface`) for its active-item background; needs repointing regardless of the above decision, since the Radix mechanism itself is sound and likely retained even if the command list changes.
- `styles/tokens.css` / `styles/global.css` — **[Finding]** both still `@import` `@fontsource/fraunces` today, and `tokens.css` still defines Signal-Path-era custom properties beyond the ones the currently-live Folio CSS actually consumes; a full audit of which custom properties the new Dark Evidence Map components will actually need (versus which are pure holdovers) is implementation-time work, not resolved by this plan.
- Whether any additional `internal/*/testdata` or `definitions/*.yaml` backend files should be read again at implementation time to confirm Pass 1's new fixtures remain byte-faithful, since this planning pass did not re-read them (unlike the prior plan, which did) — recommended before Pass 1 begins, not assumed unnecessary.

**Must remain (confirmed genuinely reusable, not superseded by anything in this plan):**

- The entire `lib/` domain layer except the two scoped changes in §3.
- `lib/alertSource.ts`, `hooks/useAlertInvestigation.ts`.
- `components/JsonTree.tsx`.
- `AlertInvestigationPage.tsx`'s route/query/error-branching logic (only its success-branch composition changes).
- `app/router.tsx`.
- `src/fixtures/alert-investigation/v1.ts`'s five existing fixtures (extended, never replaced, §8).
- `src/types/contract.ts` (minus the one dead `SignalPathStageId` export, §9 Pass 7).

## 12. Risk controls and rollback

- **Contract fidelity.** `artifactInspection.ts`, `finding.ts`, `concordance.ts`, and `provenance.ts` remain the only modules permitted to branch on `AlertInvestigationResponse` shape (§3); every one is unit-tested directly against `contract.ts`'s types and the real fixtures.
- **Avoiding invented telemetry.** Pass 1's new fixtures must follow `v1.ts`'s own established provenance discipline — cited real backend testdata, never hand-imagined field content.
- **Protecting current lineage/provenance/traceability behavior.** `lineage.ts`, `provenance.ts`, and `traceability.ts` are explicitly unmodified (§3); their existing test suites must continue passing, unedited, through every pass as a regression guard.
- **Preventing scenario-specific architecture.** `lib/concordance.ts`'s modification (§3) and every new component in §5 are unit/component-tested against all three scenarios' real declared-condition shapes from the moment each is written, not retrofitted afterward.
- **Sequencing deletion after verification, never before.** Track A (§4) is deletable immediately because it is *already* dead — this is a factual claim, not a migration outcome. Track B (§9 Pass 7) is deletable only after Pass 6's full verification, mirroring the same discipline.
- **Never mixing visual systems in a reviewable state.** Pass 5 is the one pass where shipped behavior changes, and it must land as a single reviewable unit (§9), exactly as the prior plan required for its own equivalent step.
- **Rollback.** Passes 0–4 are independently revertible with no effect on the live page (Pass 0 removes only dead code; Passes 1–4 are additive/isolated). Rollback after Pass 5 but before Pass 7 is a single revert of Pass 5's commit, since nothing has been deleted yet. Rollback after Pass 7 depends on ordinary version-control history, since `frontend/` is a tracked directory today (confirmed by this session's `git status`), unlike the untracked state a prior planning pass once flagged as a risk — that specific risk no longer applies, but standard care around force-pushes and history rewrites still does.

## 13. Definition of implementation readiness

**Passes 0–6 can begin immediately upon this plan's approval.** They depend on no unresolved product, UX, or architecture decision:

- `alert-investigation-ux-spec.md` v0.2 is approved and internally consistent with this plan's component design.
- Every declared-condition shape needed for §7's verification matrix has been directly confirmed against the real, committed fixtures (`frontend/src/fixtures/alert-investigation/v1.ts`) during this planning pass.
- The two fixture/test gaps (§8) are known, scoped, and additive — not blockers to starting, only to completing Pass 6's full verification.

**One item gates only Pass 7, not the start of implementation:** each Track B file's replacement (§11) must be independently confirmed shipped and verified before that specific file is deleted — this is a sequencing rule within Pass 7 itself, not an external blocker.

No other blocker was found. In particular: no backend change is required or proposed anywhere in this plan; no alert index, search, detection catalog, severity, confidence, case-management, assignment, or disposition concept is introduced anywhere in §5–§10; no new telemetry family or dedicated mobile experience is proposed (spec §17); and every file classified "preserve" in §3 and §11 is preserved for the specific, stated reason in its row, not by default.
