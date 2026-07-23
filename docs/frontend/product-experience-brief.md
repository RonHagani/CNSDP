# Security Investigation Experience — Product Experience Brief

| Field | Value |
| --- | --- |
| Document | Security Investigation Experience — Product Experience Brief |
| Status | **Superseded** — see notice below. Retained for historical reference only; must not guide implementation. |
| Phase | Phase 1.5 — Security Investigation Experience |
| Identifier | Not assigned. This document is deliberately outside the closed PC-015 identifier namespace (PC-###, PD-###, PER-###, UC-###, FR-###, NFR-###, AC-### are not reopened or extended by this document) and is referenced by path only: `docs/frontend/product-experience-brief.md`. |
| Relationship to baseline | Extends, and must not contradict, the approved Phase 0 product baseline (`../product.md` and its companions) and the approved Phase 1 architecture baseline (`../architecture.md`, ARCH-01). Defines no new product scope, functional requirement, non-functional requirement, or acceptance criterion. Where this document proposes a backend capability, that capability requires its own separate architecture and implementation approval before any code is written (see §11). |

> **SUPERSEDED.** This document's visual and interaction direction — "The Signal Path" creative direction (§4), the flagship screen specification (§5), the information architecture (§6, as it pertains to the Alert Investigation route), the design system (§7), and the motion system (§8) — is superseded by [`docs/frontend/alert-investigation-ux-spec.md`](./alert-investigation-ux-spec.md). This document is retained only as historical design exploration and **must not guide implementation**. Its backend-capability audit (§2) and backend-capability plan (§11) remain accurate reference material and are restated, for self-containment, in the superseding specification's §2 and §15. No content below this notice has been rewritten or removed.

## Purpose and scope

Phase 1.5 designs and builds the platform's primary product experience: a
forensic command center in which a security analyst can see and investigate
how a Kubernetes audit event moves through intake, validation, normalization,
detection, alerting, evidence assembly, traceability verification, and
investigation. It is not a demonstration wrapper around the API — it is
intended to become the project's primary portfolio artifact, and must read as
a credible, premium, specialist security product.

This document defines: the current and planned backend read surface (§2, §11);
the precise mapping between the platform's real architecture and this
experience's visual language (§3), which is the terminology correction this
document exists in part to enforce; the chosen creative direction (§4); the
flagship Alert Investigation screen (§5); the information architecture (§6);
the design system (§7); the motion system (§8); the technology stack (§9); the
delivery strategy and fixture policy (§10); the backend-capability
classification (§11); the quality gate (§12); and open questions (§13).

This document does not write frontend code, install dependencies, modify
backend code, define an API specification, or modify any existing product or
architecture document.

## 1. Product ambition — non-negotiable

These constraints exist so that a later implementation session cannot quietly
replace this concept with a conventional dashboard. They are binding on every
subsequent Phase 1.5 decision, not aspirational framing.

- This is not a simple demonstration frontend. It is the project's primary
  portfolio experience and will be judged as such.
- It must feel like a credible, premium, specialist security product — the
  kind a detection engineering or incident-response team would actually want
  to work in — not a generic AI-generated cybersecurity dashboard.
- The flagship Alert Investigation screen (§5) must be visually distinctive
  enough, on its own, to establish the identity of the entire product. Every
  later screen inherits its design system (§7–§8); none may fall back to a
  sidebar-plus-KPI-cards-plus-line-chart layout, default component-library
  styling, or decorative charts not grounded in real data.
- Every major visualization must represent real security, evidence,
  traceability, lineage, or workflow meaning. No invented telemetry,
  statistics, activity feeds, or unsupported claims, at any point, including
  temporary fixture-driven milestones (§10.1).
- A later screen that reintroduces a sidebar-plus-card layout, adopts default
  shadcn/Tailwind aesthetics, or adds a chart not backed by a real field from
  §2 or §11 is a deviation from this brief, not a reasonable implementation
  shortcut, and must be treated as one.

## 2. Backend-capability audit — current state

Verified directly against `cmd/platform/main.go` and the handler
implementations; no endpoint exists beyond these three.

| Endpoint | Auth | Real response |
| --- | --- | --- |
| `POST /v1/audit-events` | Bearer | `{"results":[{"index","id","error"}]}` — admission outcome only. Never carries a validation outcome or processing status. |
| `GET /v1/alerts/{id}` | Bearer | The full six-artifact evidence inventory for one alert, plus the traceability verification result. See below. |
| `GET /readyz` | None | `{"status","checks":{"database","detection_definitions"}}` on success; `503` with `{"status":"not_ready","failed_check":...}` otherwise. |

`GET /v1/alerts/{id}` (`../../internal/retrieval/retrieval.go`) is the
platform's one genuinely rich read surface. Its real response shape, cited
here for reference — not proposed, already implemented:

```
{
  "alertId": <int64>,
  "sourceEvent":         { "available": bool, "rawEvent": <raw k8s audit JSON> },
  "validationOutcome":   { "available": bool, "outcome": string, "reason": string },
  "normalizedEvent":     { "available": bool, "event": { subject, operation, target, outcome, requestTime, exec?, podCreation?, clusterRoleBinding? } },
  "detectionDefinition": { "available": bool, "revision": string, "definition": { scenario, name, description, conditions } },
  "detectionResult":     { "available": bool, "matchReason": { scenario, definitionName, definitionRevision, satisfiedCharacteristics[] } },
  "alert":               { "available": bool, "summary": { matchReason, subject, operation, target, outcome, requestTime } },
  "traceability":        { "intact": bool, "failedLink": string }
}
```

Every top-level field carries its own `available` flag (`FR-035`): a missing
artifact is a visible, distinct state, never a blank or a fabricated
placeholder. This document treats that contract as binding on the frontend —
every one of the seven `available` flags must be independently rendered.

**Capabilities this alone supports today:** a complete Alert Investigation
experience — source event, validation outcome, normalized event (including
whichever one of the three scenario-specific characteristic blocks applies),
detection definition and its documented conditions, match reason, alert
summary, and traceability integrity — for any alert that already exists, with
zero fabrication.

**Capabilities this alone cannot support**, confirmed by exhaustive route
search (no other handler is mounted anywhere in the repository):

1. No way to discover which alert IDs exist (no list/index).
2. No way to retrieve the outcome of a submission that was rejected,
   flagged incomplete, classified unsupported, or validly processed but did
   not match any scenario — `POST /v1/audit-events` returns only an admission
   result, never a validation outcome, and no submission-lookup endpoint
   exists.
3. No standalone way to review the three detection definitions independent of
   an alert that happens to reference one.
4. No per-stage timestamps — the real six-value workflow-state model (§3.2)
   exists in the database and worker but is never returned in any response.
5. No aggregate counts of any kind beyond `/readyz`'s two booleans.
6. No listing, search, or streaming capability — consistent with, not a gap
   against, `../scope.md`'s explicit SIEM-style-search exclusion (PC-011).

These six gaps are carried forward into the classified capability plan in
§11, not resolved here.

## 3. Architecture and terminology mapping

The platform's real architecture contains three distinct enumerations that
must never be conflated with one another or with this document's visual
design. This section is the correction this document exists partly to
enforce, and every later section's stage references resolve against it.

### 3.1 The eight pipeline-relevant architecture modules (`../architecture.md` §2)

The approved architecture defines nine internal modules; the ninth
(operational diagnostics) is operational infrastructure, not part of the
security-event pipeline narrative, and is surfaced separately by the System
Status screen (§6), not as a pipeline stage. The other eight are the real
backbone of this experience, in the same order the product README states the
implemented flow:

1. Telemetry admission (**intake**) — `../../internal/intake`,
   `../../internal/submission`
2. Validation and classification (**validation**) —
   `../../internal/validation`
3. Normalization (**normalization**) — `../../internal/normalization`
4. Detection evaluation (**detection evaluation**) —
   `../../internal/detection`
5. Alert generation (**alerting**) — `../../internal/alerting`
6. Evidence inventory (**evidence**) — `../../internal/evidence`
7. Traceability (**traceability**) — `../../internal/traceability`
8. Retrieval and investigation (**investigation**) —
   `../../internal/retrieval`

### 3.2 The six persisted workflow states (`../architecture.md` §3)

A submission's `status` column (`../../internal/submission/submission.go`)
takes exactly six values, confirmed by Spike 2 and stated in ARCH-01 §3 as
the "confirmed status vocabulary":

```
admitted → validated → normalized → evaluated → alerted → evidenced
```

This is a real, exact model — it is accurate to describe it as a six-state
machine, because the repository defines exactly that. What is **not**
accurate is treating this six-value enumeration as interchangeable with the
eight pipeline modules above or the six evidence artifacts below. In
particular: **traceability** and **investigation** are not workflow states a
submission passes through — no submission row is ever "at traceability" or
"at investigation." Traceability is a verification operation, and
investigation is the authenticated read path; both operate on a submission
that has already reached `evidenced`.

### 3.3 The six evidence artifacts (`../scope.md` scope decision 8; `../functional-requirements.md` FR-031)

The approved minimum evidence set is exactly six persisted artifacts:

1. the source submission (raw event)
2. the validation outcome
3. the normalized event
4. the detection definition and its documented conditions
5. the detection result, including its recorded match reason
6. the generated alert

Traceability verification — confirming these six connect correctly and
remain unmodified — describes the integrity of this set. **It is explicitly
not a seventh artifact** (stated three times across `../scope.md`,
`../functional-requirements.md`, and `../glossary.md`; this document repeats
it because it is exactly the kind of distinction a later session could
silently blur).

### 3.4 The Signal Path's visual grouping, and its mapping back to 3.1–3.3

The Signal Path (§4) renders **eight visual stages**, one per pipeline-
relevant module from §3.1 — not six, and not a re-skin of the workflow-state
model from §3.2. Eight was chosen deliberately over a superficially simpler
six, because collapsing to six is exactly the move that previously caused the
artifact/state/module conflation this section corrects. The mapping every
later section relies on:

| Visual stage | Module (§3.1) | Workflow-state relevance (§3.2) | Evidence artifact(s) surfaced (§3.3) | Nature |
| --- | --- | --- | --- | --- |
| Intake | 1 | enters `admitted` | 1 — source submission | persisted, write-path |
| Validation | 2 | `admitted → validated` | 2 — validation outcome | persisted, write-path |
| Normalization | 3 | `validated → normalized` (valid only, FR-014) | 3 — normalized event | persisted, write-path |
| Detection evaluation | 4 | `normalized → evaluated` | 4 — detection definition; 5 — detection result / match reason | persisted, write-path |
| Alerting | 5 | `evaluated → alerted` | 6 — generated alert | persisted, write-path; matching results only (FR-027) |
| Evidence | 6 | `alerted → evidenced` | verifies artifacts 1–6 are complete as a set | persisted gate — `evidence.Advance` will not allow the `evidenced` transition unless `Inventory.Complete()` holds |
| Traceability | 7 | not a workflow state | produces `{intact, failedLink}` — not an artifact (§3.3) | **recomputed live** — run once during the Evidence write-path gate, and again independently on every retrieval (`traceability.VerifyAlert`), never a stored flag read back unchanged |
| Investigation | 8 | not a workflow state | presents all of the above | the authenticated read path itself — this stage *is* the Alert Investigation screen |

Any future document or implementation session that shortens this to "the
six-stage pipeline" without reproducing this table is misdescribing the
architecture and must be corrected against this section.

## 4. Creative direction — The Signal Path

Direction is decided, not under evaluation. This section documents the
chosen direction, its provenance, and — as important — what was deliberately
excluded, so a later session cannot reintroduce it by accident.

**Thesis.** The product visualizes telemetry the way a security or controls
engineer reads a signal/circuit schematic: an event physically moves,
left to right, through the platform's own real processing stages (§3.4). The
schematic is not illustrative decoration bolted onto a dashboard — it *is*
the navigation model and the primary way meaning is communicated.

**Tone.** Precise, industrial-technical, engineering-drawing credibility.
Not hacker-green, not neon, not glassmorphic softness — the tone of a real
instrument panel or protective-relay schematic, built to be trusted under
pressure.

**Composition.** A fixed horizontal stage rail carrying the eight stages
from §3.4, always visible. A detail region below responds to whichever
stage is selected. There is no separate sidebar and no card grid.

**Navigation model.** The stage rail *is* the primary navigation for a given
alert. A command palette (incorporated from The Interrogation Room, see
below) is the secondary, keyboard-first way to move between alerts and
routes — not a competing navigation paradigm, but the fast path into the same
schematic.

**Typography direction.** A technical, monospace-leaning grotesk for
structural chrome — stage labels, IDs, revision hashes, connector labels,
timestamps (tabular figures) — paired with one legible humanist sans for
prose content (descriptions, rationale text, match-reason descriptions). Two
families, deployed by role, never mixed within one content type.

**Color and contrast philosophy.** A deep graphite/blue-black schematic
background. Exactly three semantic colors, each meaning exactly one thing,
never decorative: a cool cyan/blue "signal" line for an intact, normal
connector or active stage; amber at a genuine classification branch point
(e.g., a non-valid validation outcome); red reserved solely for a severed
connector (`traceability.intact === false`) or a required-but-unavailable
evidence artifact. No other UI state borrows these colors.

**Density strategy.** Schematic-dense: precise small labels, tick marks, and
connector annotations, the way a real engineering drawing is dense — not
padded card whitespace pretending to be minimalism.

**Graph and evidence visualization approach.** The eight-stage rail is a
literal signal-path diagram, not a generic force-directed graph: fixed
topology, real connector semantics. A solid connector means the underlying
foreign-key chain resolved and traceability verified intact; a severed
connector, rendered with a fault glyph, means `traceability.failedLink`
named that specific link (`alert`, `source_key`, or `raw_event_sha256` —
never a generic "error"). The compact evidence graph inside the Investigation
stage renders the real chain
(`alert → detection_result → normalized_event → submission`) using the real
IDs returned by the traceability module, not illustrative placeholders.

**Motion language.** See §8. Motion exists only to show a real transition:
a stage becoming active, a connector resolving intact or severed, a field's
lineage being traced.

**Flagship interaction.** Raw-to-normalized field lineage (incorporated from
The Ledger, see below): selecting a normalized field highlights its origin
directly inside the raw source-event viewer, and vice versa — because the
platform's core value proposition is that every normalized fact traces to a
real raw fact, and the interaction should make that literal, not describe it
in a caption.

**Alert Investigation, in one sentence.** Inspecting the schematic at the
point the analyst cares about — the Investigation stage — with every
upstream stage one click or keystroke away, never buried in a separate page.

**What makes it immediately recognizable as CNSDP.** No other security
product renders its own real state machine as a literal, inspectable
schematic; the identity comes from architectural honesty, not decoration.

**Risks and weaknesses.** An honest timeline across the eight stages needs
real per-stage timestamps, which do not exist yet (§2, §11) — until that
backend addition is approved and built, any stage-to-stage timing must be
either order-based (a stepper, not a clock) or, in Scenario Lab only, a
client-observed wall-clock measurement the frontend itself took (§5, §10).
The schematic tone also risks reading as cold for evidentiary storytelling;
mitigated by keeping the schematic strictly for pipeline flow and rendering
evidence content itself in a separate, high-legibility inspector, never
inside the schematic's own connector labels.

**Provenance — what was incorporated, and why it stays coherent.**

- From **The Ledger**: precise raw-to-normalized field-lineage highlighting
  only. Not adopted: The Ledger's document/scroll composition, its exhibit
  numbering metaphor, or its serif/mono typographic pairing — those would
  compete with the schematic as a second navigation model and must not
  reappear in any later screen.
- From **The Interrogation Room**: the command palette, keyboard-first
  navigation, and focused-inspector interaction pattern only. Not adopted:
  its multi-pane resizable debugger composition, its terminal-only monospace
  typography for prose content, or its rejection of a persistent structural
  chrome — the Signal Path keeps a persistent stage rail, which The
  Interrogation Room deliberately had none of.

The result is one coherent direction — a schematic with a precise inspection
interaction and a fast keyboard-driven way to move between alerts — not a
blend of three aesthetics. Any later addition of exhibit-style scrolling
document layouts or unstructured multi-pane terminal composition is a
regression against this section.

## 5. Flagship screen specification — Alert Investigation

**Governing principle.** Every element on this screen maps to a named field
in §2's response shape or a named concept in §3.4's table. Nothing here is
descriptive filler.

**Information hierarchy, most to least primary:**

1. **Stage rail** (fixed, horizontal, always visible) — the eight stages
   from §3.4, each showing this alert's real structural state: which
   evidence artifacts are `available` at or before that stage, per §2's
   `available` flags. No simulated timing is shown on the rail until §11's
   timestamp capability is separately approved and built; the rail is
   order-based, not clock-based, at first release.
2. **Alert identity header** — matched scenario and definition name, with
   emphasis earned only from real content (the scenario identity and the
   satisfied-characteristics list) — never an invented severity score. The
   backend has no severity field; none is fabricated here.
3. **Per-artifact detail panes**, one per evidence artifact from §3.3, each
   independently rendering its own `available: false` state distinctly
   (never a shared blank or generic spinner):
   - **Source event** — the raw audit JSON, collapsible and searchable.
   - **Validation outcome** — the outcome and, when non-valid, its stated
     reason.
   - **Normalized event** — subject, operation, target, outcome, request
     time, and whichever single scenario-characteristics block
     (`exec` / `podCreation` / `clusterRoleBinding`) is populated. Every
     field here is a lineage-highlight source or target back to the raw
     event pane (§4's flagship interaction).
   - **Detection definition and revision** — scenario, name, description,
     full conditions (operation match, `requires_outcome`, `requires_any`,
     `requires_all`), and the pinned revision hash, labeled explicitly as
     pinned per `NFR-025`: a later definition edit does not change what this
     alert resolves to.
   - **Detection result and match reason** — the satisfied characteristics
     against the definition's declared set, shown as an honest checklist,
     never a numeric confidence score not present in the data.
   - **Alert summary.**
4. **Evidence inventory** — the six-artifact checklist rendered as a literal
   UI element, one row per artifact from §3.3, each showing its `available`
   state. This is not decorative summary; it is `FR-031`/`FR-035` made
   visible.
5. **Traceability panel** — `intact` / `failedLink` rendered as the
   schematic's own connector state (§4), the single highest-trust signal on
   the screen. Framed explicitly as a live-verified result (§3.4), not a
   cached flag.
6. **Interactive evidence graph** — the compact real-ID chain
   (`alert → detection_result → normalized_event → submission`) from
   `traceability.Locate`, distinct from, and smaller than, the top stage
   rail.

**Interactions:**

- Raw-to-normalized field lineage (click/focus a field on either side; its
  counterpart highlights and the connection animates, §8).
- Stage-rail navigation (click or arrow-key a stage to scroll/filter the
  detail region to it).
- Command palette (`⌘K` / `Ctrl+K`) to jump between alerts known to the
  current session (seeded by Scenario Lab submissions until §11's alert-list
  capability exists) and between routes.
- Distinct, explicit rendering for every unavailable, partial, or broken
  state — never silently omitted, per `FR-035`.

**Broken or unavailable evidence states.** Every one of the seven
`available` flags in §2's response, plus `traceability.intact === false`
with each of its three named `failedLink` values, has its own distinct
rendering. There is no shared generic "error" state for this screen.

**Keyboard-first investigation.** The full artifact set, the traceability
result, and stage navigation must be reachable and inspectable without a
mouse: command palette to arrive, arrow/`j`/`k`-style keys to move between
stages and panes, a dedicated key to enter/exit field-lineage mode, `Escape`
to close any open inspector.

**Responsive behavior.** Below tablet width, the stage rail collapses to a
compact vertical stepper preserving the same eight labels and states; the
active detail pane becomes the primary view; a bottom sheet replaces any
side-anchored inspector. No stage or evidence-availability information is
dropped at any breakpoint — only its layout changes.

## 6. Information architecture

Only routes with a real, present-or-planned backend grounding are included,
per the rule that a route must have a real product purpose and can
eventually be grounded in real data — not a permanently fabricated one.

| Route | Status | Backend basis |
| --- | --- | --- |
| **Alert Investigation** | Build first (flagship) | Fully supported today — `GET /v1/alerts/{id}` (§2) |
| **Scenario Lab** | Build after flagship gate | Fully supported today for alerting scenarios — `POST /v1/audit-events` + polled `GET /v1/alerts/{id}` (§2). Honest handling of non-alerting outcomes depends on §11's "non-matching and non-valid submission retrieval" capability; until approved and built, Scenario Lab must render a labeled "not yet retrievable" state for those cases rather than inventing one. |
| **System Status** | Build after flagship gate | Fully supported today — `GET /readyz` (§2) |
| **Detections catalog** | Build after flagship gate, interim form | Interim: the three committed, version-controlled YAML definitions (`../../definitions/`) mirrored statically into the frontend, explicitly labeled as sourced from those files. Full form depends on §11's "detection-definition catalog retrieval" capability. |
| **Alerts index** | Deferred | Requires §11's "alert list/index" capability; not buildable with real data before it is approved and built |
| **Overview** | Deferred, not currently planned | No aggregate data source exists or is proposed in §11; a KPI-style overview would require its own separately justified capability under `PC-C-003`, not assumed here |
| **Live Pipeline** | Not a separate route | Folded into Scenario Lab — a persistent "live feed" independent of a session-initiated submission would require the list/streaming capability this document does not propose (§11's "optional future") |

## 7. Design system

- **Typography.** Two families by role only (§4): technical monospace-
  leaning grotesk for structural/schematic chrome; one humanist sans for
  prose. No third family.
- **Spacing and layout grid.** A schematic module grid drives the stage
  rail's fixed connector spacing; a separate, denser field-table grid governs
  content inside inspector panes. The two grids are visually distinct so a
  reader always knows whether they are looking at pipeline structure or
  artifact content.
- **Panel and surface behavior.** Flat, bordered "instrument panel" surfaces.
  No glassmorphism, no soft drop-shadow elevation. Depth is communicated by
  border weight and connector-line geometry, with exactly one elevation step
  reserved for the active/focused inspector.
- **Borders and depth.** Connector lines, not shadows, are the primary depth
  and relationship cue throughout.
- **Status language.** Exactly the platform's own real vocabulary —
  `valid` / `invalid` / `incomplete` / `unsupported`; `matched` / `non-match`;
  `intact` / broken (named by `failedLink`) — never invented synonyms like
  generic "critical/warning/info" severities that do not map to a real
  backend concept (`NFR-031`: explanations must be expressed in the
  platform's own documented terms).
- **Semantic colors.** Exactly three, each meaning exactly one thing (§4):
  cyan/blue = intact/normal; amber = a genuine classification branch point;
  red = severed connector or unavailable required artifact. No other UI
  state uses these hues.
- **Graph nodes and edges.** The fixed eight-node schematic (§3.4, §4) and
  the compact real-ID evidence chain (§5) — never a generic node-link layout
  algorithm applied to arbitrary data.
- **Timelines.** Order-based stepper across the eight stages at first
  release; upgraded to real elapsed-time rendering only once §11's
  processing-stage timing capability is approved and built.
- **Code and JSON presentation.** A purpose-built recursive tree renderer for
  the raw event and normalized event, chosen specifically because it must
  support per-field lineage-highlight hooks (§4, §5) that a generic JSON
  viewer library does not expose.
- **Focus and keyboard states.** High-contrast, always-visible focus
  treatment tuned against the dark schematic background, on every
  interactive stage node, field, and command-palette entry.
- **Loading, empty, partial, error, unauthorized, and unavailable states.**
  Each of the seven `available` flags (§2) and the traceability
  `intact`/`failedLink` state gets its own distinct rendering — never a
  single shared placeholder standing in for "something is missing."
- **Responsive breakpoints.** One collapse point for the stage rail
  (horizontal rail → vertical stepper) and one for the inspector
  (side-anchored → bottom sheet), per §5.
- **Reduced-motion accessibility.** Every animation named in §8 has a
  reduced-motion fallback that is an instant state change carrying the exact
  same information — no animation is the sole carrier of any fact.

## 8. Motion and signature moments

Motion is included only where it improves understanding of a real state
change; everything else is excluded on principle, not by oversight.

**Included, and why each is meaningful:**

1. **Stage activation** — the rail visibly indicates which stage a selected
   alert's data corresponds to; communicates position in the real pipeline.
2. **Connector resolution** — a connector animates to its resolved state
   (intact or severed) when traceability verification completes; communicates
   a real, just-computed trust result, not a decorative reveal.
3. **Field-lineage highlight travel** — selecting a normalized field draws a
   visible connection to its raw-event origin (§4's flagship interaction);
   communicates real data lineage.
4. **Stage-rail scrub** — moving between stages animates the detail region's
   transition; communicates pipeline order.
5. **Severed-connector fault transition** — when `traceability.intact` is
   false, the specific named link (`alert` / `source_key` /
   `raw_event_sha256`) visibly severs with a fault glyph; communicates
   exactly which relationship failed, not a generic error state.
6. **Command-palette open/jump** — a crossfade/collapse on palette
   invocation and navigation; communicates a completed navigation action.

**Explicitly excluded:** ambient background animation, hover bounce/scale
effects, particle or glow effects, animated counters for numbers that are not
independently verifiable, and any transition whose sole purpose is visual
interest rather than communicating one of the six moments above.

## 9. Technology evaluation

- **React + TypeScript + Vite.** A client-rendered SPA against a JSON API;
  no SSR/server-component requirement exists, so no meta-framework
  (e.g. Next.js) is justified — consistent with the backend's own
  no-premature-scale discipline (`PC-P-006`).
- **Routing.** React Router in data mode; the route count in §6 does not
  justify more.
- **Server-state management.** TanStack Query — fits the polling pattern
  Scenario Lab genuinely needs (poll `GET /v1/alerts/{id}` to resolution)
  and caches alert responses without hand-rolled cache logic.
- **Local interaction state.** Component state/context by default; a small
  external store (e.g. Zustand) only if command-palette state or
  field-lineage-highlight state genuinely needs to be shared beyond what
  context comfortably supports — not adopted preemptively.
- **Graph visualization.** A purpose-built SVG renderer for the fixed
  eight-node schematic and the compact evidence chain, not a generic
  force-directed graph library — the topology is fixed and known, and a
  generic library would default toward exactly the "random chart" look this
  brief prohibits (§1).
- **Code/JSON viewing.** A small custom recursive JSON tree component, not a
  full code editor (Monaco/CodeMirror) — this is read-only inspection
  (detection definitions are read-only by product scope, `PD-04` scope
  decision 3), and a full editor is unjustified weight; a custom renderer is
  also what makes per-field lineage highlighting (§4, §5) implementable at
  all.
- **Motion.** Framer Motion — declarative API matching §8's six named
  moments, with built-in reduced-motion support (§7).
- **Testing.** Vitest + React Testing Library for unit/component coverage;
  Playwright for the flagship screen's interaction and visual acceptance
  gate (§10), matching the seriousness the backend already applies to its
  own integration-test discipline.
- **Accessibility primitives.** Radix UI's unstyled behavior primitives for
  the command palette and focus/dialog management — correct ARIA and
  keyboard behavior without adopting Radix's or shadcn's default styled
  visual layer, which this brief explicitly rejects (§1).
- **Styling approach.** Deliberately left to the implementation phase, with
  one binding constraint stated now: if a utility framework such as Tailwind
  is used at all, it must run on a fully custom design-token theme (§7),
  never default spacing/color scales — the default look is exactly what §1
  prohibits.

## 10. Delivery strategy

1. **Experience brief and visual system.** This document, plus the concrete
   design-token and motion-primitive kit it implies (§7, §8) — no code.
2. **High-fidelity Alert Investigation flagship, built against a versioned
   fixture.** The fixture must be byte-shape-identical to the real
   `GET /v1/alerts/{id}` response cited in §2 — ideally captured directly
   from a real reference-environment run of the documented Scenario 1
   walking-skeleton demonstration (`../reference-environment.md`), not
   hand-imagined. It is version-labeled (date and the backend commit or
   contract version it mirrors), visibly marked as fixture data in any
   dev/demo tooling, and never presented as live in a portfolio artifact
   without that label. See §10.1.
3. **Visual review and refinement gate.** Full-page and key-interaction
   screenshots, light and dark, across the breakpoints defined in §7,
   reviewed against the full checklist in §12 before any further screen is
   attempted.
4. **Real backend connection for the flagship screen.** The fixture from
   step 2 is replaced by the live `GET /v1/alerts/{id}` call against a real
   alert produced by the running reference environment. Because the fixture
   was contract-identical, this must require no UI redesign — if it does,
   the fixture was wrong, not the UI.
5. **Scenario Lab and System Status.** Both build entirely on capabilities
   already current today (§2, §11) — no new backend work is a prerequisite
   for this step.
6. **Separately approved backend read extensions.** The capabilities listed
   as "required for later Phase 1.5 screens" in §11 go through their own
   architecture and implementation approval, exactly as this repository's
   change-control discipline requires, before any corresponding UI work
   begins.
7. **Alerts index, the full Detections catalog, and broader navigation.**
   Built only once the relevant §11 extensions are approved and implemented.
8. **Final consistency, responsiveness, accessibility, and motion review.**
   A whole-product pass re-checking every screen built since step 2 against
   the same design system (§7) and the same checklist (§12) the flagship
   established — the mechanism that prevents later screens from drifting
   toward generic patterns.

### 10.1 Fixture policy

Fixtures are a temporary design and testing tool, never a permanent data
source for a shipped screen. A fixture used anywhere in this delivery
strategy must:

- be captured from, or be byte-shape-identical to, a real backend contract
  cited in §2 or approved under §11 — never hand-invented data;
- be versioned (a filename, date, or commit reference indicating which real
  contract shape it mirrors);
- be visibly labeled as fixture data within whatever tooling displays it;
- be replaceable by a live call with no UI redesign — a fixture that would
  require redesigning the screen to accommodate the real response was
  authored incorrectly and must be recaptured, not worked around.

## 11. Backend-capability plan

This section classifies every backend read capability this experience
touches. It designs no API and specifies no implementation; each capability
in the "required" or "optional" columns needs its own separate architecture
and implementation approval before any backend code changes.

### 11.1 Current — available today, no backend change needed

- `POST /v1/audit-events` — telemetry admission.
- `GET /v1/alerts/{id}` — the full six-artifact evidence inventory and
  traceability result for one existing alert (§2).
- `GET /readyz` — platform health.

### 11.2 Required for the flagship milestone (§10 steps 2–4)

- None beyond §11.1. The Alert Investigation flagship is fully buildable —
  fixture-first, then live — on the current `GET /v1/alerts/{id}` contract
  alone.

### 11.3 Required for later Phase 1.5 screens (§10 steps 6–7), pending separate approval

- **Alert list/index capability.** Required for the Alerts index route
  (§6). Without it, alert IDs are only discoverable through session-local
  seeding from Scenario Lab submissions.
- **Retrieval of non-matching and non-valid submissions.** Required for
  Scenario Lab (§6) to honestly show the four real data-quality outcomes —
  today, a submission classified invalid, incomplete, or unsupported, or one
  that validly processed without matching a scenario, has no read-back path
  at all.
- **Detection-definition catalog retrieval.** Required for the full
  Detections catalog route (§6) to move beyond the interim static mirror of
  the committed YAML files.
- **Processing-stage timing or transition-history data, where justified.**
  Required only if a real elapsed-time stage rail or timeline (§5, §7) is
  pursued; this is the most speculative of the four and needs its own
  explicit justification tied to a documented use case before being treated
  as committed work (see §13).

### 11.4 Optional future capabilities — not committed, would need separate justification under `PC-C-003`

- **Aggregate counts or metrics** (e.g., submissions-by-outcome,
  alerts-by-scenario) — only if a genuine, documented use case justifies
  them; this brief does not propose an Overview screen or any KPI surface
  on the strength of this option alone (§1, §6).
- **Streaming or push updates** (SSE/WebSocket) for a true live pipeline
  feed, as opposed to Scenario Lab's session-scoped polling.
- **Any evidence beyond the approved minimum evidence set** — explicitly
  out of scope by product charter, not merely deferred: `../scope.md` scope
  decision 8 defers "contextual evidence beyond the approved v0.1 minimum
  evidence set" to a future release, and this document does not propose
  reopening that boundary.

## 12. Quality gate

Every screen, at every delivery-strategy gate (§10), is checked against all
of the following before being considered done:

- **Originality.** No sidebar-plus-KPI-cards-plus-line-chart layout; no
  default shadcn/Tailwind aesthetic; no three concepts that are merely color
  variations of one layout.
- **Information hierarchy.** The eight-stage rail and the six evidence
  artifacts are always the most prominent elements on any investigation-
  related screen.
- **Authenticity of displayed data.** Every number, label, and status traces
  to a named field in §2 or an approved §11 capability. Zero fabricated
  statistics, fake activity, or invented severity.
- **Forensic usability.** A reviewer can move from alert to source event to
  match reason to traceability result without leaving the screen.
- **Data density.** Dense technical content is legible, not padded into
  empty card whitespace.
- **Interaction quality.** Field-lineage highlighting, stage-rail
  navigation, and the command palette all function against real state, not
  placeholder data.
- **Motion purpose.** Every animation maps to one of §8's six named moments;
  nothing decorative survives review.
- **Accessibility.** Full keyboard-reachable investigation path (§5), visible
  focus states (§7), and reduced-motion parity (§7, §8).
- **Responsive quality.** The stage rail and inspector both degrade per §5's
  and §7's defined breakpoints, with no information silently dropped.
- **Consistency.** Later screens reuse the flagship's design-token and
  motion-primitive kit rather than introducing new visual language (§1,
  §10 step 8).
- **Loading/error/empty states.** Every `available` flag and every
  `failedLink` value has its own distinct, named rendering (§5, §7).
- **Absence of generic dashboard patterns.** No KPI cards, no decorative
  charts, no glassmorphism, no hacker-green cliché, checked explicitly, not
  assumed.

## 13. Open questions

1. Whether "processing-stage timing or transition-history data" (§11.3) is
   sufficiently justified by a documented use case to commit to as backend
   work, versus remaining permanently order-based (§5, §7) — needs an
   explicit decision before that capability is proposed for architecture
   review.
2. How Scenario Lab should render a non-alerting outcome before §11.3's
   "non-matching and non-valid submission retrieval" capability is approved
   and built: a labeled "not yet retrievable" state for every submission, or
   restricting Scenario Lab to alerting scenarios only until that capability
   lands.
3. The exact styling implementation approach (CSS Modules, vanilla-extract,
   or a fully re-themed utility framework) — deferred to the implementation
   phase by design (§9), not resolved here.

## Traceability

This experience presents, and must remain faithful to, product behavior
already approved elsewhere and defines no new product behavior itself:

- **Use cases served:** `UC-002` (verify a detection match and evaluate
  alert explainability) and `UC-003` (investigate an alert using its
  explanation and supporting evidence) — `../use-cases.md`.
- **Product goals served:** `PC-G-005` (explainable alert generation),
  `PC-G-006` (evidence-based investigation), `PC-G-007` (end-to-end
  traceability) — `../product.md`.
- **Requirements the presented data must remain faithful to:** `FR-029`
  (alert explainability content), `FR-031`/`FR-035` (evidence inventory and
  visible absence), `FR-033`/`FR-034` (traceability links and navigation),
  `NFR-025` (definition-revision pinning), `NFR-031` (self-contained
  understandability in the product's own terms) — `../functional-
  requirements.md`, `../non-functional-requirements.md`.
- **Scope boundary this document does not reopen:** the approved minimum
  evidence set (`../scope.md` scope decision 8) and the PC-011 non-goals
  (no SIEM-style search, no case management) remain binding on every route
  in §6.
