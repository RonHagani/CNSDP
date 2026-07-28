# ADR-0005: External OIDC identity, BFF-mediated sessions, and backend-enforced RBAC

| Field | Value |
| --- | --- |
| Status | **Proposed.** Not yet Accepted. Requires the change-control approval described in "Consequences — Change control and compatibility" before this ADR, or any implementation it authorizes, takes effect. This ADR does not itself amend `docs/architecture.md`, `docs/non-functional-requirements.md`, or `docs/acceptance-criteria.md`. |
| Document | Target: `docs/architecture.md` (ARCH-01) §6 and §7. §6 currently states the opposite authentication decision ("a shared bearer token / API key represents the single approved trust level... No user or role model, no RBAC"); §7 currently commits to "Docker Compose, exactly two services." Both would require explicit amendment if this ADR is accepted — see "Consequences — Change control and compatibility," below. |

Design-phase companions (not restated here): `../security/identity-and-session-architecture.md`, `../security/authorization-model.md`, `../security/audit-and-accountability-design.md`, `../security/implementation-roadmap.md`, `../security/open-decisions.md`, `../security/security-acceptance-gate.md`. This ADR is the decision record for what those documents specify in detail.

## Context

`../architecture.md` §6 records the current, approved v0.1 decision: a
single shared bearer token represents one authenticated product trust
level, with no user model and no RBAC — an explicit, deliberate NFR-012
waiver, not an oversight, and one `../architecture.md` §6 itself names as
having a documented future "upgrade trigger": *"mutual TLS and OIDC remain
documented future options if per-submitter cryptographic identity or
federated/role-based access is ever required."* `../architecture.md` §7
separately commits to a specific reference-environment topology: *"Docker
Compose, exactly two services — the application and PostgreSQL — on a
single host (NFR-033, NFR-034, AC-028)."* `../non-functional-requirements.md`
(PD-06) and `../acceptance-criteria.md` (PD-07) both independently delegate
*"trust boundaries, identity and authorization mechanisms"* to future
**threat modeling** work, by name, as a decision not yet made.

Two facts on the ground make this the point at which that delegated decision
should actually be exercised, rather than left indefinitely deferred:

1. **The current shared credential already conflates two structurally
   different trust classes.** It authenticates both the Kubernetes
   audit-webhook submitter (a machine/service actor) and, through the
   frontend's development proxy, human browser access for alert
   investigation — one token format standing in for two fundamentally
   different kinds of caller, which is itself the exact ambiguity this ADR
   proposes to resolve.
2. **The frontend already has a real, working investigation UI reaching a
   real backend** (the Alert Inventory and Alert Investigation features,
   `frontend/src/features/alert-inventory`, `frontend/src/features/alert-investigation`),
   with no path today for it to know *which analyst* is looking at an
   alert, or to differentiate what different analysts may do — a gap that
   only grows more consequential the more of this platform's UX gets built
   on top of an undifferentiated trust model.

Candidate architectures evaluated for closing this gap: (a) a browser-held
OIDC access token used as a bearer credential directly against the Go
backend; (b) browser-managed OAuth refresh tokens for silent renewal; (c) a
custom username/password system built and owned by this platform; (d) a
same-origin BFF holding a server-side session, with the browser holding only
an opaque `HttpOnly` cookie; (e) keeping the current shared bearer token and
merely renaming it "user identity" without a real identity provider behind
it.

## Decision

Adopt, as a **future target architecture**:

- Human authentication through an **external, standards-based OIDC identity
  provider** — this platform does not become an identity provider itself.
- **Authorization Code Flow with PKCE** (RFC 7636) for the OIDC exchange,
  used unconditionally regardless of client confidentiality.
- A **same-origin reverse proxy / backend-for-frontend (BFF)** as the sole
  OIDC client and the sole holder of any provider-issued token — the
  browser never receives one.
- A **hardened, `HttpOnly`, `Secure`, `SameSite=Lax` session cookie** as the
  only authentication artifact the browser ever holds — an opaque, random,
  server-validated identifier, never a JWT and never containing any claim
  data itself.
- **No browser-readable access or refresh token, ever** — not in
  `localStorage`, `sessionStorage`, `IndexedDB`, or any non-`HttpOnly`
  cookie.
- **The session crosses the BFF↔backend boundary as identity only, never as
  an authoritative role claim.** The BFF asserts a validated subject and
  session reference; the Go backend independently resolves that identity's
  current role(s) from its own authoritative role/permission data on each
  applicable request (`../security/authorization-model.md`) — the BFF is
  never a second, unsynchronized source of role truth.
- **Backend-enforced RBAC** at the Go backend's authorization boundary,
  evaluated on every request before any data access, per
  `../security/authorization-model.md` — never a frontend-only control, and
  never a BFF-only control for response-shaping decisions such as
  raw-payload visibility (`../security/authorization-model.md`'s
  endpoint-to-permission mapping).
- **Structurally separate machine/service identities** from human sessions,
  with the current single shared `API_TOKEN` **gradually retired**, migrated
  per-integration rather than cut over instantaneously
  (`../security/implementation-roadmap.md` Pass 19).
- **No custom username/password authentication system** is built at this
  stage — see "Alternatives considered" for why, and the one condition under
  which this could change.

## Consequences

### Security consequences

- Removes the single largest credential-exposure surface this platform's
  own recent corrective work already identified and fixed once at smaller
  scale (the bearer-token-in-the-browser problem) — this ADR generalizes
  that fix from "one shared dev token" to "any human credential, ever."
- Introduces a **new** CSRF exposure the current header-based bearer model
  does not have (an ambient cookie is auto-attached; a header is not) —
  explicitly mitigated by `../security/identity-and-session-architecture.md`'s
  CSRF section and `../security/implementation-roadmap.md` Pass 6. This is a
  genuine trade, not a free improvement, and is recorded here rather than
  left implicit.
- Materially increases the amount of session-lifecycle logic this platform
  must get right (rotation, idle/absolute timeout, revocation) compared to
  today's stateless, always-valid-until-rotated shared token — this is the
  cost of real per-user accountability and revocability, which the current
  model cannot provide at all.
- Creates a real audit trail of *who* did *what* (`../security/audit-and-accountability-design.md`),
  which today's shared-token model structurally cannot, since "who" is
  undifferentiated by construction today.

### Operational consequences

- Adds a new operational dependency: an external OIDC identity provider
  must be available (or a local-development equivalent run) for the
  platform to be usable by a human at all — an availability dependency this
  platform does not have today. `../security/identity-and-session-architecture.md`'s
  "IdP outage behavior" section specifically bounds this: already-authenticated
  sessions remain valid during an IdP outage, only new logins are affected.
- Adds at least one new deployed service (the BFF/reverse-proxy boundary,
  `../security/implementation-roadmap.md` Pass 2), and a local-development
  identity provider (`../security/open-decisions.md`, decision 1) would add
  another to the reference environment — today's reference environment
  (`docker-compose.yml`) has exactly two services, and `../architecture.md`
  §7 commits to exactly that count; this proposal increases it, a genuine
  increase in operational surface this ADR does not minimize or hide (see
  "Change control and compatibility," below, for the required §7
  amendment).
- Requires production TLS and a documented ingress boundary
  (`../security/implementation-roadmap.md` Pass 16), which this platform's
  current reference environment does not provide or require today
  (`../architecture.md` §6's transit-protection statement is conditional on
  a boundary existing, and none does today for the single-host reference
  environment).
- Adds new I/O (a BFF hop, a session-table lookup, an authorization-decision
  lookup) to the request path of `GET /v1/alerts` and `GET /v1/alerts/{id}`,
  the two endpoints `../non-functional-requirements.md` NFR-002 and
  `../acceptance-criteria.md` AC-021 already hold to a 5-second-or-less
  retrieval target. None of the added I/O is expected to approach that
  target, but this ADR does not itself measure that — re-verification is a
  required part of this proposal's own implementation and gate process, not
  an optional follow-up (`../security/identity-and-session-architecture.md`,
  "Retrieval-latency impact on NFR-002 / AC-021";
  `../security/security-acceptance-gate.md`, gate item P10).

### Migration consequences

- The current shared `API_TOKEN` is **not removed** by adopting this ADR.
  `../security/implementation-roadmap.md` Pass 19 requires a gradual,
  per-integration migration with both old and new credentials accepted
  during a transition window — an instantaneous cutover risking real
  telemetry-delivery breakage is explicitly rejected as an implementation
  approach.
- The existing frontend's real-backend integration (`frontend/src/lib/api/*`,
  `frontend/vite.config.ts`'s dev proxy) is the human-facing surface this
  ADR's session model eventually replaces for browser access — `../security/implementation-roadmap.md`
  Pass 10 is the specific pass where that cutover for human traffic occurs,
  and it explicitly requires documenting whether the bearer-token path
  continues serving machine traffic in parallel during the transition.
- No database migration under this ADR touches any existing table owned by
  the nine already-approved workflow modules (`../architecture.md` §2) —
  every new table this ADR's implementation requires (sessions, roles, audit
  records) is additive and owned by new, separate modules, consistent with
  this platform's existing module-boundary discipline.

### Risks

- **Implementation risk of rolling one's own session/cookie/CSRF logic
  incorrectly.** Mitigated by the explicit, detailed hardening
  specification in `../security/identity-and-session-architecture.md`
  (exact cookie attributes, rotation triggers, CSRF pattern) rather than
  leaving these as unstated implementation details, and by
  `../security/security-acceptance-gate.md`'s mandatory verification gate
  before this can be called complete.
- **Operational risk of the new BFF/proxy component itself becoming a
  vulnerability** (e.g. a forwarded-header trust bug). Mitigated by
  `../security/implementation-roadmap.md` Pass 15's explicit trusted-proxy
  hardening and by reusing this codebase's own already-proven
  "never blindly trust an inbound credential-shaped header" pattern
  (`frontend/vite.config.ts`'s `createApiProxy`).
- **Migration risk to the existing, working telemetry-ingestion path**
  during Pass 19's service-identity cutover. Mitigated by the gradual,
  dual-accept migration strategy specified there rather than an
  instantaneous cutover, and by Pass 19 depending on Pass 9 (real,
  differentiated authorization data) rather than only Pass 8 (placeholder
  full-access middleware), so a narrowly-scoped service credential is
  never issued into a system that would in practice still grant it full
  access.
- **Governance risk of this ADR being treated as already-authoritative**
  before its required approval lands. Mitigated by this ADR's own Status
  field, the repeated "Proposed" marking across every companion document,
  and Pass 1 of the implementation roadmap being an explicit, blocking
  approval gate before any other pass may begin.
- **Risk of an incomplete baseline-amendment list.** An earlier draft of
  this ADR and its companions named only `../architecture.md` §6,
  NFR-012, and AC-024 as requiring amendment, omitting §7's "exactly two
  services" commitment even though Pass 2 (BFF) and a local-development
  identity provider both add services beyond that count. Corrected in this
  revision — see "Change control and compatibility," below — and noted
  here as a reminder that the completeness of this list is itself a
  reviewable claim, not a settled fact.

### Change control and compatibility

This decision, if accepted, **directly contradicts** the current text of
`../architecture.md` §6 (authentication) and §7 (reference-environment
topology), and `../non-functional-requirements.md` NFR-012's "single
authenticated product trust level, no RBAC" statement. It does not silently
override any of them. Until the steps below are completed,
`../architecture.md` (§6 and §7), NFR-012, and AC-024 remain the governing,
approved statement of this platform's actual authentication model and
reference-environment topology, and this ADR's Status remains Proposed, not
Accepted.

Accepting this ADR requires, together, not independently:

1. This ADR's own Status field flipping to Accepted through explicit review.
2. An amendment to `../architecture.md` §6 replacing its current
   authentication statement.
3. An amendment to `../architecture.md` §7 replacing its current "exactly
   two services" commitment, since this proposal's BFF (and, for local
   development, a recommended local identity provider — `../security/open-decisions.md`
   decision 1) each add a service beyond that count.
4. An amendment or superseding entry against `../non-functional-requirements.md`
   NFR-012 (including its "Excluded and deferred candidates" item 5),
   NFR-033, and NFR-034; and against `../acceptance-criteria.md` AC-024 and
   AC-028; plus new acceptance criteria for session lifecycle, CSRF, and
   authorization behavior. NFR-033, NFR-034, and AC-028 must be
   revalidated against the new topology, not merely assumed to still hold.
5. Confirmation this does not introduce a `PC-011` non-goal or `PC-C-005`
   multi-tenancy — it does not; it adds real human identity to an otherwise
   unchanged, still single-tenant, still v0.1-scoped product.

`../security/implementation-roadmap.md` Pass 1 is this exact approval step,
sequenced as the first, blocking pass before any code is written, and its
own scope has been updated to include item 3 above.

### Verification

Adoption is verified in two stages, matching `../security/security-acceptance-gate.md`:

1. **Design acceptance** — this ADR and its companion documents reviewed
   and explicitly approved per "Change control and compatibility," above.
2. **Implementation and production-readiness acceptance** — the full
   `../security/security-acceptance-gate.md` checklist (no browser-readable
   credential, backend authorization on every protected endpoint,
   deny-by-default, session-fixation/rotation/revocation/timeout tests,
   CSRF tests, IDOR and horizontal/vertical privilege-escalation tests,
   forged-header tests, revoked/expired-session tests, secret-leak
   scanning, NFR-002/AC-021 retrieval-latency re-verification, and the rest
   of that document's checklist) executed and passing, as
   `../security/implementation-roadmap.md` Pass 20 specifies.

No verification step in either stage has been performed by this ADR itself.

## Alternatives considered

- **Access token stored in `localStorage` (or any JS-readable storage).**
  Rejected. Any XSS vulnerability anywhere in the frontend becomes a
  full account-takeover primitive, since the token is directly readable and
  exfiltratable by injected script. `HttpOnly` cookies close exactly this
  path — the browser's JavaScript layer never sees the credential at all.
- **Browser-managed refresh tokens.** Rejected for the same reason as
  above, with higher stakes: a stolen refresh token typically has a longer
  lifetime and broader blast radius than a stolen access token. This
  platform's design keeps all provider tokens server-side in the BFF
  (`../security/identity-and-session-architecture.md`, "Where authorization
  codes and provider tokens are handled").
- **Direct SPA-to-backend OIDC bearer-token architecture** (the SPA obtains
  an access token itself, e.g. via the implicit or a public-client
  authorization-code flow, and attaches it directly to backend requests).
  Rejected. This is architecturally equivalent to the localStorage/token
  problem even if the token is held only in memory, because it requires the
  token to exist in the browser's JavaScript execution context at all,
  where it remains reachable to a sufficiently capable XSS payload for the
  duration of the page's life. The BFF pattern removes the token from the
  browser's trust boundary entirely, not merely from persistent storage.
- **Custom username/password authentication system built by this
  platform.** Rejected for the initial target architecture. This platform
  has no charter basis for owning credential storage, password-reset flows,
  multi-factor enrollment, or breach-monitoring — all real, ongoing
  security obligations an identity provider already solves, and none of
  which this project's scope (`PC-011`, `PC-C-001`) justifies rebuilding.
  Per the explicit instruction governing this decision, a custom system
  would only be justified by a *documented repository requirement that
  makes an external IdP impossible* — no such requirement exists in any
  approved document reviewed during this design phase (`docs/product.md`,
  `docs/scope.md`, `docs/non-functional-requirements.md`,
  `docs/acceptance-criteria.md`).
- **Frontend-only authorization** (hiding UI affordances by role without a
  corresponding backend check). Rejected outright — this is not a security
  control at all, only a UX convenience, restated explicitly in
  `../security/authorization-model.md`, "Frontend visibility as convenience
  only."
- **Keeping the shared bearer token as human identity** (treating
  possession of the one shared `API_TOKEN` as if it were a per-user
  identity). Rejected. It provides no way to know *which* human is acting,
  no way to revoke one analyst's access without revoking everyone's, and no
  way to differentiate authorization at all — precisely the gap this ADR
  exists to close.
- **Embedding an identity provider directly into application domain
  logic** (e.g. a bespoke auth module living inside the same modules that
  own alert/evidence/detection data, rather than as its own bounded
  concern). Rejected as an architectural anti-pattern for this codebase
  specifically: `../architecture.md` §2's entire modular-monolith design
  rests on "no module reads or writes another module's table directly" and
  explicit per-module ownership boundaries (ADR-0001). Identity and session
  concerns get their own module boundary (`../security/implementation-roadmap.md`
  Passes 4, 7–9) for exactly the same change-isolation reason (NFR-023) the
  other nine modules already have one.

## References

`docs/architecture.md` §2, §6, §7 (ARCH-01); `docs/non-functional-requirements.md`
NFR-012, NFR-013, NFR-014, NFR-015, NFR-023, NFR-033, NFR-034 (PD-06);
`docs/acceptance-criteria.md` AC-024, AC-025, AC-028 (PD-07); `docs/product.md`
PC-011, PC-C-001, PC-C-005 (PD-01); `docs/adr/0001-modular-monolith-in-go.md`;
`docs/adr/0003-kubernetes-audit-webhook-dual-layer-intake.md`;
`docs/adr/0004-version-controlled-detection-definitions.md`;
`internal/auth/auth.go`; `internal/retrieval/*`; `frontend/vite.config.ts`;
`frontend/src/test/noClientCredentials.test.ts`;
`frontend/src/test/noLeakedSecretsInBuild.test.ts`;
`docs/security/identity-and-session-architecture.md`;
`docs/security/authorization-model.md`;
`docs/security/audit-and-accountability-design.md`;
`docs/security/implementation-roadmap.md`; `docs/security/open-decisions.md`;
`docs/security/security-acceptance-gate.md`.
