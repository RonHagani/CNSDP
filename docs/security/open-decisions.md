# Cloud-Native Security Telemetry and Detection Platform — Security Foundation Open Decisions Register (Proposed)

| Field | Value |
| --- | --- |
| Document | CNSDP Security Foundation Open Decisions Register |
| Version | 0.1 |
| Status | **Proposed.** A register of unresolved decisions blocking or shaping future implementation. This document resolves nothing itself. |
| Phase | Proposed — Security Foundation design phase. Not yet an approved project phase. |
| Identifier | Not assigned. Outside the closed PC-015 namespace. Referenced by path only: `docs/security/open-decisions.md`. |
| Authoritative sources | `identity-and-session-architecture.md`, `authorization-model.md`, `audit-and-accountability-design.md`, `implementation-roadmap.md`, `../adr/0005-external-oidc-identity-and-session-architecture.md` (this design phase). |

> **None of the 18 decisions below are settled.** Each carries a
> recommendation where this design phase has enough information to offer
> one, but a recommendation is not a decision — every item requires an
> explicit owner's sign-off before the implementation pass that depends on
> it may proceed. Where no clear owner exists in this repository's current
> governance structure, that absence is stated explicitly rather than
> assigned to an invented role.

## Priority summary

| # | Decision | Priority | Status | Blocks |
| --- | --- | --- | --- | --- |
| 1 | Local-development identity provider | High | Open, recommended | Pass 3 |
| 15 | Provider-token storage and encryption | High | Open, recommended (Option A) | Pass 3 |
| 6 | `SameSite=Lax` versus `SameSite=Strict` | High | Effectively constrained, confirm only | Pass 5 |
| 3 | PostgreSQL session storage versus a dedicated session store | Medium-High | Open, recommended | Pass 4 |
| 4 | Idle timeout | Medium-High | Open, no value fixed | Pass 5 |
| 5 | Absolute timeout | Medium-High | Open, no value fixed | Pass 5 |
| 7 | CSRF token strategy | Medium | Open, recommended | Pass 6 |
| 8 | Role granularity | Medium | Open, recommended (no new roles) | Pass 9 |
| 16 | Session revocation strategy (platform-wide emergency scope) | Medium | Partially resolved, one open extension | Pass 18 |
| 2 | Production identity-provider assumptions | Medium | Open, no vendor selected | Pass 16–17 |
| 10 | Emergency administrator access | Medium-High (security-critical) | Open, unresolved | Pass 18 |
| 14 | Step-up authentication for sensitive actions | Medium | Open, gap in B/C/D | Pass 18 and evidence-export build |
| 17 | Service credential rotation | Medium | Open, recommended pattern | Pass 19 |
| 11 | Audit retention | Low-Medium | Open, default recommended | Pass 12 |
| 12 | Sensitive raw-payload access (field-level masking) | Medium (data-sensitivity risk), Low (implementation urgency) | Open, needs its own analysis | Before raw-payload masking is ever built |
| 13 | Evidence export policy (mechanism, not permission) | Low (capability not yet approved) | Open, deferred | Before export capability is approved at all |
| 9 | Future tenant isolation | Low, deliberately deferred | Not blocking | None currently |
| 18 | Concurrent sessions per human identity | Medium-High | Open, unresolved | Pass 5, Pass 18 |

## 1. Local-development identity provider

- **Status:** Open, recommended.
- **Recommendation:** Run a real, lightweight OIDC-compliant provider (e.g.
  Dex or Keycloak) as an additional `docker-compose.yml` service, per
  `identity-and-session-architecture.md`'s "Local development versus
  production."
- **Rationale:** Keeps local development exercising the actual Authorization
  Code Flow with PKCE end to end, preserving dev/production parity and
  avoiding the well-documented risk of a "temporary" auth bypass surviving
  into a deployed environment.
- **Security impact:** Low either way if implemented correctly; meaningfully
  higher risk if a code-level bypass is chosen instead and later forgotten.
- **Operational impact:** Adds one more container to the local reference
  environment, alongside the existing `app`/`postgres` pair
  (`docker-compose.yml`).
- **Decision owner:** Whoever implements `implementation-roadmap.md` Pass 2/3.
- **Deadline / dependency:** Before Pass 3 begins.

## 2. Production identity-provider assumptions

- **Status:** Open, no vendor selected.
- **Recommendation:** None offered — this is a deployment/vendor/budget
  decision outside this design phase's authority. Any OIDC-compliant
  provider satisfying Authorization Code Flow with PKCE and, ideally,
  RP-Initiated Logout is architecturally acceptable per ADR-0005.
- **Rationale:** Vendor selection depends on organizational context (cost,
  existing SSO investment, self-hosting appetite) this design phase has no
  visibility into.
- **Security impact:** Varies by provider's own security posture and
  operational maturity — a factor to evaluate at selection time, not
  something this document can bound in advance.
- **Operational impact:** Recurring cost and/or maintenance burden,
  materially different between a managed SaaS IdP and a self-hosted one.
- **Decision owner:** **No owner currently exists in this repository's
  documented governance structure** — this repository has no named
  infrastructure/vendor-decision stakeholder distinct from the personas in
  `../personas.md`, none of which is an operator/administrator persona
  (`../personas.md`, "Excluded stakeholders": *"Platform Operator...not a
  persona-driven product workflow in v0.1"*). This gap is stated explicitly
  rather than silently assigned.
- **Deadline / dependency:** Before Pass 16–17 (production ingress and
  secret management).

## 3. PostgreSQL session storage versus a dedicated session store

- **Status:** Open, recommended.
- **Recommendation:** PostgreSQL, reusing the existing instance.
- **Rationale:** `../adr/0002-postgresql-persistence-and-durable-worker.md`
  already decided "one PostgreSQL instance is the platform's sole
  persistence store" — introducing a second store (e.g. Redis) for sessions
  alone would need its own justification against that existing decision,
  and this platform's approved capacity envelope (NFR-003, 10
  submissions/sec) gives no indication that session-table load would
  approach a scale justifying a dedicated store.
- **Security impact:** Equivalent either way if implemented correctly
  (hashed lookup key, as specified in `identity-and-session-architecture.md`).
- **Operational impact:** PostgreSQL avoids standing up and operating a
  second data store; a dedicated store could reduce database load under
  very high session-check volume, which is not currently an identified
  concern at this platform's approved scale.
- **Decision owner:** Whoever implements Pass 4, confirmed against ADR-0002's
  existing decision before deviating from it.
- **Deadline / dependency:** Before Pass 4.

## 4. Idle timeout

- **Status:** Open, no numeric value fixed.
- **Recommendation:** A placeholder range of 15–30 minutes is reasonable for
  a SOC-analyst workflow, but the final number is explicitly not fixed by
  this design phase.
- **Rationale:** `identity-and-session-architecture.md` deliberately left
  this open, mirroring how this platform's own approved numeric targets
  (NFR-001, NFR-002, NFR-009) were each set by explicit, separate decisions
  rather than invented as a side effect of an architecture document.
- **Security impact:** Shorter bounds the exposure window of an unattended,
  unlocked workstation more tightly; too short materially harms usability.
- **Operational impact:** Too aggressive a timeout increases re-authentication
  friction during active investigation work, which can itself degrade the
  UC-003 investigation experience this platform is built around.
- **Decision owner:** A product/security stakeholder informed by real
  analyst workflow — not fixed by engineering judgment alone.
- **Deadline / dependency:** Before Pass 5.

## 5. Absolute timeout

- **Status:** Open, no numeric value fixed.
- **Recommendation:** A placeholder range of 8–12 hours (a working shift) is
  reasonable; final number not fixed here.
- **Rationale:** Same reasoning as idle timeout — bounds the maximum value
  of a stolen session identifier even against faked activity, but the exact
  number is a product decision, not an architectural one.
- **Security impact:** Shorter bounds worst-case exposure more tightly.
- **Operational impact:** Too short forces re-authentication mid-shift,
  which is a real usability cost for a role expected to work sustained
  investigation sessions.
- **Decision owner:** Same as idle timeout.
- **Deadline / dependency:** Before Pass 5.

## 6. `SameSite=Lax` versus `SameSite=Strict`

- **Status:** Effectively constrained by the OIDC flow's own mechanics —
  listed here for explicit confirmation, not because the choice is
  genuinely open.
- **Recommendation:** `Lax`.
- **Rationale:** `Strict` would prevent the session cookie from being sent
  on the IdP's own redirect-back navigation to the callback route (a
  top-level cross-site navigation in the `SameSite` sense), breaking the
  `state`-correlation step of the login flow itself —
  `identity-and-session-architecture.md`'s cookie-configuration table
  explains this in full. `Lax` still blocks the cookie on the realistic CSRF
  vector (cross-site `POST`/`fetch`/image-triggered requests).
- **Security impact:** Marginally more permissive than `Strict`, fully
  compensated by the explicit CSRF-token check (`implementation-roadmap.md`
  Pass 6), which is the primary control regardless of `SameSite` value.
- **Operational impact:** None.
- **Decision owner:** Implementer of Pass 5 — low discretion, since the
  technical constraint above effectively pre-decides this.
- **Deadline / dependency:** Before Pass 5.

## 7. CSRF token strategy

- **Status:** Open, recommended.
- **Recommendation:** Synchronizer-token pattern (a dedicated, session-scoped
  token-issuance endpoint) as the primary mechanism.
- **Rationale:** Does not depend on cookie-read timing or client-side
  cookie-parsing behavior the way a pure double-submit-cookie pattern does;
  simpler to reason about and to test.
- **Security impact:** Both the synchronizer-token and double-submit-cookie
  patterns are acceptable if implemented correctly; this is a
  correctness/maintainability preference, not a materially different
  security posture.
- **Operational impact:** Minor implementation-complexity difference only.
- **Decision owner:** Implementer of Pass 6.
- **Deadline / dependency:** Before Pass 6.

## 8. Role granularity

- **Status:** Open, recommended.
- **Recommendation:** Start with, and do not exceed, the six roles
  `authorization-model.md` evaluates and justifies. Do not add roles beyond
  that set without a documented, specific need, per that document's "do not
  add cosmetic roles" discipline.
- **Rationale:** Several of the six roles are themselves contingent on
  separate, not-yet-approved product capabilities (see
  `authorization-model.md`'s governance note) — expanding role count before
  even the base six are validated in practice would compound an already
  partly-speculative model.
- **Security impact:** Over-granular roles increase management and
  audit-review complexity without a proportional security benefit;
  under-granular roles risk violating least privilege.
- **Operational impact:** Role count directly affects the complexity of the
  administrator role-management UI (`implementation-roadmap.md` Pass 18).
- **Decision owner:** Joint product and security sign-off before Pass 9.
- **Deadline / dependency:** Before Pass 9.

## 9. Future tenant isolation

- **Status:** Not blocking; deliberately deferred, not decided.
- **Recommendation:** Do not build any tenant concept now. Keep every
  authorization decision parameterized by a specific resource identity (as
  `authorization-model.md` already does), so a `tenant_id` dimension could
  be added later without a redesign, per that document's "Possible future
  tenant isolation."
- **Rationale:** `PC-C-005` and `../scope.md` (PD-04) currently exclude
  multi-tenancy from v0.1; nothing in this design phase proposes changing
  that.
- **Security impact:** Not applicable while no tenant concept exists.
- **Operational impact:** Not applicable now.
- **Decision owner:** A future, separate product-scope decision (a PD-04
  amendment) — explicitly not this design phase's to make.
- **Deadline / dependency:** None currently; revisit only if PD-04 scope is
  ever amended to include multi-tenancy.

## 10. Emergency administrator access ("break-glass")

- **Status:** Open, unresolved. Not addressed by any of documents B/C/D in
  detail.
- **Recommendation:** A documented, itself-heavily-audited break-glass
  procedure — for example, a distinct emergency credential held outside the
  normal role-assignment path (e.g. in an offline or separately secured
  location), usable only when normal administrator access is unavailable,
  whose use immediately triggers a high-severity audit event and a
  mandatory post-use review. The exact mechanism is not decided here.
- **Rationale:** Every real system with an administrator role needs a
  recovery path for "all administrators are locked out" — leaving this
  entirely unspecified risks either no recovery path (an availability
  failure) or an ad hoc, unaudited one improvised under pressure (a security
  failure).
- **Security impact:** High if done poorly — a break-glass path is
  structurally a backdoor and must be tightly scoped, rarely usable, and
  heavily audited, or it becomes the weakest link in the entire model.
- **Operational impact:** Rarely exercised in practice, which itself creates
  risk (an untested recovery path may not work when actually needed) —
  periodic testing should be part of whatever procedure is adopted.
- **Decision owner:** A security/platform-operations stakeholder — distinct
  from, and in addition to, the ordinary Platform Administrator role design.
- **Deadline / dependency:** Before Pass 18 ships. Related to, but distinct
  from, Pass 18's own "bootstrap the first administrator" problem — bootstrap
  is "how the first admin is created," break-glass is "how access is
  regained if every admin is already locked out."

## 11. Audit retention

- **Status:** Open, default recommended.
- **Recommendation:** Deployment-lifetime retention by default (no automatic
  deletion), per `audit-and-accountability-design.md`.
- **Rationale:** Mirrors this platform's own already-approved pattern for
  NFR-032 (the minimum evidence set); no compliance driver requiring a
  different number exists in any approved document reviewed during this
  design phase, and `../non-functional-requirements.md`'s own
  excluded-candidate list already notes "no charter basis" for compliance
  frameworks today.
- **Security impact:** Longer retention preserves more forensic value; it
  also means more accumulated per-actor history is stored over time, which
  is itself a sensitivity consideration for whoever can read the audit
  trail (already gated to Platform Administrator only, per
  `authorization-model.md`).
- **Operational impact:** Storage grows over the deployment's lifetime,
  governed by this platform's existing NFR-035/NFR-036 bounded-resource
  pattern rather than a new mechanism.
- **Decision owner:** A product/compliance stakeholder, if one is ever
  identified — the default above can ship without further decision unless
  explicitly overridden.
- **Deadline / dependency:** Before Pass 12; the default is usable without
  blocking that pass.

## 12. Sensitive raw-payload access (field-level masking)

- **Status:** Open, needs its own dedicated analysis beyond this design
  phase.
- **Recommendation:** `authorization-model.md` already gates *who* may view
  a raw payload at all (Analyst-tier and above), but the deeper question —
  whether specific fields *within* a raw Kubernetes audit event (e.g.
  environment-variable values that could contain secrets) should ever be
  masked from any role — is explicitly **not resolved** by documents B, C,
  or D. `../non-functional-requirements.md` (PD-06) itself delegates
  "privacy, masking, and sensitive-data handling for audit subjects and
  request content carried in source telemetry" to threat modeling — the
  still-pending Document A (Threat Model) from this same design phase is
  the more appropriate place to analyze this fully, not a late addition
  bolted onto this register.
- **Rationale:** This tension is real and unresolved: `FR-032` already
  approves a requirement that source-event evidence "faithfully represent
  the submission content as received" — any masking scheme must be
  reconciled with that existing, approved evidentiary-fidelity guarantee,
  not simply layered on top of it without review.
- **Security impact:** Real — Kubernetes audit events can carry sensitive
  request/response content.
- **Operational impact:** A masking scheme adds meaningful complexity to
  the evidence-composition path (`internal/evidence`) and must be designed
  carefully enough not to silently violate FR-032's fidelity guarantee.
- **Decision owner:** Joint security and product sign-off, likely requiring
  its own requirements-level discussion (potentially a PD-05/PD-06
  amendment), not a decision this document can make unilaterally.
- **Deadline / dependency:** Should be resolved before any field-level
  masking is built, and ideally before the deferred Threat Model document is
  completed, since it depends on that document's fuller analysis.

## 13. Evidence export policy (mechanism, not permission)

- **Status:** Open, deferred — lower urgency since the underlying capability
  itself is not yet an approved product feature.
- **Recommendation:** `authorization-model.md` already specifies the
  *permission* structure (export is separately gated from view, audited
  per `audit-and-accountability-design.md`) — this decision concerns the
  *mechanism*: export format, whether exports are watermarked or bound to a
  chain-of-custody record, whether download links expire, and similar
  operational details, none of which are decided here.
- **Rationale:** Export is the highest-leverage data-exfiltration point in
  the entire model — once evidence leaves the platform's controlled
  surface, none of this platform's own access controls apply to the
  exported copy any longer.
- **Security impact:** High leverage if the mechanism is designed
  carelessly (e.g. a permanent, unwatermarked, unexpiring download link).
- **Operational impact:** Depends entirely on the downstream use case (legal
  hold, escalation to another team, external reporting) this design phase
  has no visibility into.
- **Decision owner:** Joint product and security sign-off.
- **Deadline / dependency:** Before the export endpoint is built at all —
  which itself first requires the underlying capability to be approved as
  product scope, per `authorization-model.md`'s governance note.

## 14. Step-up authentication for sensitive actions

- **Status:** Open — a genuine gap not addressed by documents B, C, or D.
- **Recommendation:** Consider requiring fresh re-authentication (or an
  equivalent step-up signal) immediately before high-sensitivity actions —
  evidence export, role/permission changes, administrative configuration
  changes — rather than relying solely on an already-established session's
  ambient privilege for these specific actions.
- **Rationale:** A common, well-understood pattern for bounding the blast
  radius of a hijacked-but-not-yet-detected session: even if a session
  identifier is stolen, the highest-value actions still require a fresh
  proof of presence.
- **Security impact:** Meaningfully reduces the value of a stolen session
  for an attacker, at the cost of some friction for legitimate
  high-privilege users.
- **Operational impact:** Low if scoped narrowly to genuinely rare,
  high-stakes actions — most ordinary investigation work is unaffected.
- **Decision owner:** A security stakeholder, in coordination with whoever
  implements Pass 18.
- **Deadline / dependency:** Before Pass 18 (administrator workflows) and
  before any evidence-export capability is built.

## 15. Provider-token storage and encryption

- **Status:** Open, recommended (Option A).
- **Recommendation:** Discard the OIDC provider's access and refresh tokens
  entirely after identity-claim extraction (Option A in
  `identity-and-session-architecture.md`), unless a specific, separately
  justified need for silent IdP-session-length renewal emerges (Option B).
- **Rationale:** Minimizes the amount of long-lived provider-issued secret
  material this platform must protect; Option B's encryption-at-rest
  requirement is only a live concern if Option B is actually adopted.
- **Security impact:** Option A has meaningfully smaller stored-secret
  surface than Option B.
- **Operational impact:** Option A means a fully expired local session
  always requires a full IdP round-trip login (no silent renewal past the
  local session's own absolute timeout) — an accepted trade-off under the
  recommendation.
- **Decision owner:** Implementer of Pass 3, confirmed by a security
  stakeholder before deviating toward Option B.
- **Deadline / dependency:** Before Pass 3.
- **If Option B is ever proposed:** this design phase deliberately does not
  fully design Option B, since it remains non-recommended — but adoption
  must not proceed on encryption-at-rest alone
  (`identity-and-session-architecture.md`, "Whether stored provider tokens
  require encryption at rest"). Before Option B could be approved, its own
  proposal must define, at minimum: encryption-key ownership (who/what
  holds and controls the key, distinct from database-credential holders);
  key rotation cadence and mechanism; key versioning (so a key rotation
  does not orphan tokens encrypted under a prior key); the re-encryption
  or migration procedure when a key is rotated; key revocation; a
  compromise-recovery procedure if the key itself is suspected exposed;
  and how key access and use are themselves auditable
  (`audit-and-accountability-design.md`). None of these are specified here
  precisely because Option A remains the recommended default.

## 16. Session revocation strategy (platform-wide emergency scope)

- **Status:** Partially resolved — `identity-and-session-architecture.md`
  already specifies per-session and per-user revocation, effective
  immediately on the next request (no caching window, since every request
  re-checks the database row). **Open extension:** whether a distinct
  "revoke every session platform-wide" emergency capability should exist,
  separate from ordinary per-user revocation.
- **Recommendation:** Yes — a platform-wide emergency revocation capability,
  available to Platform Administrators (or as part of the break-glass
  procedure, decision 10), for scenarios like a suspected IdP compromise
  where every existing session's trustworthiness is simultaneously in
  question.
- **Rationale:** Per-user revocation alone does not cover an
  IdP-compromise-class incident, where the platform cannot know in advance
  which specific sessions are affected.
- **Security impact:** Important incident-response capability; absence of
  it would leave no fast, complete response to a suspected
  identity-provider-level compromise.
- **Operational impact:** Must be implemented carefully to avoid
  false-positive mass logout during routine operations — this is a rare,
  high-impact action, not a routine one.
- **Decision owner:** A security stakeholder.
- **Deadline / dependency:** Before Pass 18.

## 17. Service credential rotation

- **Status:** Open, recommended pattern.
- **Recommendation:** Define a rotation cadence and procedure for the
  per-integration machine credentials `implementation-roadmap.md` Pass 19
  introduces — periodic scheduled rotation, plus immediate rotation on
  suspected compromise, analogous to the human "Compromised-session
  response" `identity-and-session-architecture.md` already specifies.
- **Rationale:** Extends this platform's existing least-privilege principle
  (NFR-014) to machine credentials specifically — a credential that is
  never rotated is a common real-world compromise vector regardless of how
  narrowly it is scoped.
- **Security impact:** Meaningful — bounds the value of a leaked,
  undetected machine credential over time.
- **Operational impact:** Rotation must be sequenced carefully against Pass
  19's own gradual-migration requirement, so a rotation event never
  silently breaks the live Kubernetes-audit-event ingestion path.
- **Decision owner:** A platform-operations/security stakeholder.
- **Deadline / dependency:** Before Pass 19 ships, and as an ongoing
  operational practice thereafter — not a one-time decision.

## 18. Concurrent sessions per human identity

- **Status:** Open, unresolved. Not addressed by `identity-and-session-architecture.md`
  in detail — surfaced by this design phase's own security architecture
  review, since that document's "Compromised-session response" section
  ("revoke... every session belonging to a given user identity") presumes
  more than one concurrent session per identity is possible without ever
  deciding whether that should actually be allowed.
- **Alternatives:**
  1. **Single active session** — a new login invalidates any other session
     already held by that identity. Simplest to reason about; strongest
     bound on total exposure; may be a poor fit if analysts legitimately
     use more than one device or browser during a shift.
  2. **Bounded multiple sessions** — up to a fixed, small number of
     concurrent sessions per identity (e.g. 2–3), with the oldest evicted
     when the bound is exceeded.
  3. **Unrestricted concurrent sessions** — no cap; every login creates an
     additional valid session alongside any existing ones.
- **Recommendation:** No recommendation is offered as a settled choice.
  Option 2 (bounded multiple sessions) is noted as a plausible middle
  ground worth evaluating first — it accommodates realistic multi-device
  analyst use without the unbounded exposure of option 3 — but this is an
  illustrative starting point for the decision workshop, not a proposed
  answer.
- **Interaction with other lifecycle behavior, which this decision must
  resolve together, not separately:**
  - **Logout-all** — does "log out" end only the current session, or every
    session for that identity? `identity-and-session-architecture.md`'s
    "Logout and IdP logout behavior" currently describes only local logout
    of the current session plus optional federated IdP logout; it does not
    address a user-initiated "sign out everywhere."
  - **Credential/session compromise** — the "Compromised-session response"
    section already assumes multi-session revocation is possible
    (administrator-initiated); this decision determines whether that is the
    *only* way multiple sessions ever arise, or whether ordinary login
    behavior also produces them.
  - **Revocation** — per-session and per-user revocation are already
    specified (`identity-and-session-architecture.md`); this decision
    determines how many sessions a "per-user" revocation typically has to
    reach in practice.
  - **Device visibility** — if concurrent sessions are allowed, should an
    analyst be able to see which devices/sessions are currently active for
    their own identity (a common pattern for options 2 and 3)? Not
    specified by any document in this design phase.
  - **Account recovery** — if every session for a compromised identity must
    be revoked and the identity itself is suspect, does the platform need
    its own recovery step beyond "the analyst re-authenticates against the
    IdP," or does external IdP-level account recovery fully cover this
    case? Not analyzed here.
- **Security impact:** Unrestricted concurrent sessions maximize the
  exposure of a compromised credential (every login the attacker performs
  survives alongside the legitimate user's own session, unnoticed, unless
  device visibility is also built). A single-session model minimizes that
  exposure but may push analysts toward workarounds (e.g. shared
  credentials) if it doesn't fit real usage patterns.
- **Operational impact:** Directly shapes Pass 5's rotation/revocation
  implementation and Pass 18's administrator session-management surface
  (`implementation-roadmap.md`) — building Pass 5 without this decision
  risks having to rework session-table semantics once it is made.
- **Decision owner:** A product/security stakeholder informed by real
  analyst multi-device usage patterns — not decided by engineering judgment
  alone.
- **Deadline / dependency:** Before Pass 5 (session lifecycle controls) and
  Pass 18 (administrator workflows, if device-visibility/logout-all
  capability is wanted there).
