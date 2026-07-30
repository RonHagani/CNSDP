# Cloud-Native Security Telemetry and Detection Platform — Security Foundation Open Decisions Register (Proposed)

| Field | Value |
| --- | --- |
| Document | CNSDP Security Foundation Open Decisions Register |
| Version | 0.1 |
| Status | **Proposed.** A register of unresolved decisions blocking or shaping future implementation. This document resolves nothing itself. |
| Phase | Proposed — Security Foundation design phase. Not yet an approved project phase. |
| Identifier | Not assigned. Outside the closed PC-015 namespace. Referenced by path only: `docs/security/open-decisions.md`. |
| Authoritative sources | `identity-and-session-architecture.md`, `authorization-model.md`, `audit-and-accountability-design.md`, `implementation-roadmap.md`, `../adr/0005-external-oidc-identity-and-session-architecture.md` (this design phase). |

> **Four of the 18 decisions below are now resolved; 14 remain open.**
> Decisions 3, 4, 5, and 18 were resolved on 2026-07-29 by this
> repository's Project Owner and Security Design Authority, Ron Hagani,
> following Design Workshop 1, and are recorded as **Resolved** in both the
> priority summary and their own numbered sections below. Each still-open
> item carries a recommendation where this design phase has enough
> information to offer one, but a recommendation is not a decision — every
> open item still requires an explicit owner's sign-off before the
> implementation pass that depends on it may proceed. Where no clear owner
> exists in this repository's current governance structure, that absence is
> stated explicitly rather than assigned to an invented role.

## Priority summary

| # | Decision | Priority | Status | Blocks |
| --- | --- | --- | --- | --- |
| 1 | Local-development identity provider | High | Open, recommended | Pass 3 |
| 15 | Provider-token storage and encryption | High | Open, recommended (Option A) | Pass 3 |
| 6 | `SameSite=Lax` versus `SameSite=Strict` | High | Effectively constrained, confirm only | Pass 5 |
| 3 | PostgreSQL session storage versus a dedicated session store | Medium-High | **Resolved** — PostgreSQL, no dedicated store | Pass 4 |
| 4 | Idle timeout | Medium-High | **Resolved** — 20 minutes | Pass 5 |
| 5 | Absolute timeout | Medium-High | **Resolved** — 8 hours | Pass 5 |
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
| 18 | Concurrent sessions per human identity | Medium-High | **Resolved** — max 3, explicit revocation at limit | Pass 5, Pass 18 |

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

- **Status:** **Resolved (2026-07-29, Ron Hagani — Project Owner and
  Security Design Authority).**
- **Decision:** Use the existing PostgreSQL instance as the session store.
  Do not introduce Redis or another persistence service at this stage. Any
  later proposal for a dedicated session store requires a new architecture
  decision supported by measured scale or latency evidence, not a
  preference alone.
- **Required properties of the session record, confirmed by this
  decision:** browser session credentials must be opaque and generated
  using a cryptographically secure random source; only a cryptographic
  digest of the session credential is ever stored, never the bearer value
  itself (`identity-and-session-architecture.md`, "How the application
  session is identified," already specifies exactly this mechanism — this
  decision confirms it, not supersedes it). Each session record must
  support creation time, last genuine user activity, idle expiry, absolute
  expiry, revocation state, identity binding, and the minimum metadata
  needed for user-visible session management (see decision 18). Missing,
  expired, malformed, or revoked session state must fail closed
  (`identity-and-session-architecture.md`, "Fail-closed session
  validation"). Expired and revoked session records must have a documented
  cleanup strategy (`identity-and-session-architecture.md`,
  "Session-record cleanup").
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
- **Decision owner:** Ron Hagani — Project Owner and Security Design
  Authority. Implementation of Pass 4 must conform to this decision and to
  ADR-0002's existing single-persistence-store decision.
- **Deadline / dependency:** Before Pass 4. Satisfied by this resolution;
  implementation remains outstanding.

## 4. Idle timeout

- **Status:** **Resolved (2026-07-29, Ron Hagani — Project Owner and
  Security Design Authority).**
- **Decision:** Idle timeout is **20 minutes**. A warning is displayed 2
  minutes before expiration (at 18 minutes of inactivity). Only genuine
  user interaction may refresh the idle timer — background polling,
  automatic refreshes, WebSocket traffic, heartbeats, and any other
  application-generated request must never refresh it. After expiration,
  re-authentication is required.
- **Rationale:** Falls within the placeholder 15–30 minute range this
  design phase originally suggested for a SOC-analyst workflow; the
  decision owner fixed the fully-elapsed value at 20 minutes. The
  genuine-interaction-only refresh rule exists specifically to prevent the
  timer from being defeated by the application's own background activity
  (session polling, live-update traffic), which would otherwise make the
  stated bound meaningless in practice.
- **Security impact:** Bounds the exposure window of an unattended,
  unlocked workstation to 20 minutes; the 2-minute warning gives a
  legitimate user a final opportunity to confirm presence before expiration
  without weakening the bound itself.
- **Operational impact:** Consistent with ordinary SOC shift-work
  expectations; the pre-expiration warning reduces the risk of losing
  in-progress investigation work to an unexpected timeout.
- **Decision owner:** Ron Hagani — Project Owner and Security Design
  Authority.
- **Deadline / dependency:** Before Pass 5. Satisfied by this resolution;
  implementation remains outstanding.

## 5. Absolute timeout

- **Status:** **Resolved (2026-07-29, Ron Hagani — Project Owner and
  Security Design Authority).**
- **Decision:** Absolute session lifetime is **8 hours** from initial
  authentication. It is not extended by activity, session rotation,
  background requests, or token renewal of any kind — re-authentication
  against the IdP is required once the 8-hour bound is reached, regardless
  of how recently the session was used.
- **Rationale:** Falls within the placeholder 8–12 hour (single-shift)
  range this design phase originally suggested; the decision owner fixed
  the value at the shorter end of that range. This decision is
  intentionally aligned with decision 15's recommended provider-token
  handling (Option A — discard provider tokens after claim extraction):
  because Option A performs no silent, IdP-session-length renewal, a
  fixed, non-extendable 8-hour absolute bound is what actually governs how
  long a compromised session identifier remains valuable, and the two
  decisions must not be read independently of one another. Decision 15
  itself remains **Open, recommended (Option A)** — this decision does not
  resolve it, only assumes its recommended default holds.
- **Security impact:** Bounds the worst-case value of a stolen session
  identifier to 8 hours even against continuously faked activity.
- **Operational impact:** Forces re-authentication once per working shift
  at most under ordinary use; does not force re-authentication mid-shift
  for a session that started at shift start.
- **Decision owner:** Ron Hagani — Project Owner and Security Design
  Authority.
- **Deadline / dependency:** Before Pass 5. Satisfied by this resolution;
  implementation remains outstanding.

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

- **Status:** **Resolved (2026-07-29, Ron Hagani — Project Owner and
  Security Design Authority).** Resolved toward alternative 2 below
  (bounded multiple sessions), the option this design phase had flagged as
  a plausible middle ground.
- **Decision:**
  - A human identity may hold a **maximum of 3 active sessions**
    concurrently.
  - Every session has its own independently revocable server-side record
    (already specified by `identity-and-session-architecture.md`'s
    per-session schema; this decision fixes the cardinality bound on top
    of it).
  - The product must support: viewing a human identity's own active
    sessions, revoking one specific session, and logging out of all
    sessions at once.
  - **When the 3-session limit is reached, a new login must not silently
    evict an existing session.** The user must explicitly select an
    existing session to revoke before the new session is created — there
    is no automatic "oldest session evicted" behavior.
  - Password changes, user disablement, identity-compromise handling, or
    any other authorized security action may revoke all of an identity's
    sessions at once, in addition to the ordinary per-session and
    logout-all paths above.
  - Only minimal device/session metadata required for recognition and
    investigation is stored — no invasive fingerprinting. The
    already-specified `ip_at_issue` and `user_agent_at_issue` fields
    (`identity-and-session-architecture.md`, "Persistence") are the
    intended basis for this; this decision does not require collecting
    anything beyond them.
- **Alternatives considered (retained from the original analysis for
  record; not re-decided here):**
  1. **Single active session** — not chosen: stronger bound, but a poor fit
     for realistic analyst multi-device use.
  2. **Bounded multiple sessions (chosen)** — up to a fixed, small number,
     with explicit user selection required to displace one once the bound
     is reached, rather than silent eviction of the oldest.
  3. **Unrestricted concurrent sessions** — not chosen: maximizes exposure
     of a compromised credential with no bound and no forcing function for
     the legitimate user to notice.
- **Resolved interactions with other lifecycle behavior** (previously
  listed as open sub-questions this decision had to resolve together, not
  separately):
  - **Logout-all** — resolved: supported, per the Decision above.
  - **Credential/session compromise** — resolved: security-triggered global
    revocation (password change, disablement, compromise handling, or
    another authorized security action) revokes every session for the
    affected identity, per the Decision above.
  - **Revocation** — unchanged from the existing per-session/per-user
    revocation design (`identity-and-session-architecture.md`,
    "Revocation"); this decision adds self-service single-session and
    logout-all paths on top of the existing administrator-forced path, and
    fixes how many sessions a "per-user" revocation typically has to reach
    in practice (at most 3).
  - **Device visibility** — resolved: yes, the product must support
    viewing active sessions, per the Decision above.
  - **Account recovery** — **not resolved by this decision.** Whether the
    platform needs its own recovery step beyond ordinary IdP-level
    re-authentication, for the case where every session for a compromised
    identity has been revoked and the identity itself is suspect, remains
    open and is not addressed here.
- **Security impact:** A bounded 3-session cap, combined with mandatory
  explicit selection at the limit and self-service device visibility,
  meaningfully reduces — but does not eliminate — the exposure the
  previously-considered "unrestricted" alternative carried: an attacker who
  logs in while the legitimate identity already holds 3 sessions cannot add
  a fourth without forcing an explicit choice to revoke one of the existing
  three, a self-alerting event once the cap is reached. Before the cap is
  reached (the legitimate identity holds 1 or 2 sessions), an attacker with
  valid credentials can still add a session silently; this residual
  exposure is bounded by the cap and by the self-service visibility this
  decision also requires, not eliminated by it.
- **Operational impact:** Directly shapes Pass 5's session-table
  cardinality-enforcement logic and Pass 18's administrator
  session-management surface (`implementation-roadmap.md`). A self-service
  session-management capability (view / revoke-one / logout-all) is now
  required starting at Pass 5, since it is not an administrator-only
  capability — see `identity-and-session-architecture.md` and
  `implementation-roadmap.md` Pass 5 for where this is now recorded.
- **Decision owner:** Ron Hagani — Project Owner and Security Design
  Authority.
- **Deadline / dependency:** Before Pass 5 (session lifecycle controls) and
  Pass 18 (administrator workflows). Satisfied by this resolution;
  implementation remains outstanding.
