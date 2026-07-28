# Cloud-Native Security Telemetry and Detection Platform — Security-Control Implementation Roadmap (Proposed)

| Field | Value |
| --- | --- |
| Document | CNSDP Security-Control Implementation Roadmap |
| Version | 0.1 |
| Status | **Proposed.** Sequences future implementation work for a design that is itself not yet approved. Not implemented, not current shipped behavior. |
| Phase | Proposed — Security Foundation design phase. Not yet an approved project phase. |
| Identifier | Not assigned. Outside the closed PC-015 namespace. Referenced by path only: `docs/security/implementation-roadmap.md`. |
| Authoritative sources | `identity-and-session-architecture.md`, `authorization-model.md`, `audit-and-accountability-design.md` (this design phase), `../adr/0005-external-oidc-identity-and-session-architecture.md`, `../architecture.md`, `internal/auth/auth.go`, `internal/retrieval/*`, `frontend/vite.config.ts`, `docker-compose.yml` — all inspected directly during this design phase. |
| Relationship to baseline | **Proposed only.** No pass in this roadmap may begin until Pass 1 (architecture and ADR acceptance) itself is approved through this repository's change-control process. This document sequences work; it does not authorize any of it. |

> **This entire document describes a proposal, not current behavior.** No
> code, test, dependency, migration, or configuration change described below
> has been made.

## How to read this roadmap

Twenty passes, each independently reviewable and independently revertible.
Per the explicit instruction governing this roadmap, **OIDC integration,
session persistence, RBAC, audit logging, and the administrator UI are never
combined into one pass** — they are Passes 3, 4, 8–11, 12, and 18
respectively, each landing and being verified on its own before the next
begins. Unless a pass states otherwise, its prerequisite is "every earlier
pass in this list has landed" — the list is written in dependency order, not
merely topic order. A few passes (13, 14) are noted as substantially
independent of the identity work and could be pulled earlier by an
implementer if useful; they are ordered here for narrative continuity, not
because they are blocked on everything before them.

Every pass below is additive or reversible relative to the platform's
current, actually-shipped behavior (`internal/auth.Bearer`'s single shared
bearer token) until Pass 20 formally closes the Security Acceptance Gate
(`security-acceptance-gate.md`). No pass silently removes the current bearer
token's function before its replacement is verified — see each pass's
"Migration / compatibility considerations."

## Pass 1 — Architecture and ADR acceptance

- **Goal:** Move `../adr/0005-external-oidc-identity-and-session-architecture.md`
  and its prerequisite amendments to `../architecture.md` **§6 and §7**,
  `../non-functional-requirements.md` (NFR-012, NFR-033, NFR-034), and
  `../acceptance-criteria.md` (AC-024, AC-028) from Proposed to explicitly
  Accepted, through this repository's own change-control review — the gate
  every later pass depends on. §7 (the reference environment's "exactly
  two services" commitment) is included because Pass 2's BFF, and any
  local-development identity provider (`open-decisions.md` decision 1),
  each add a service beyond that committed count.
- **Prerequisites:** None. `identity-and-session-architecture.md`,
  `authorization-model.md`, `audit-and-accountability-design.md`, and
  ADR-0005 already exist as Proposed drafts.
- **Layers / files likely affected:** `docs/adr/0005-*.md` (Status flip),
  `docs/architecture.md` §6 and §7, `docs/non-functional-requirements.md`
  (NFR-012 amendment or superseding entry; NFR-033/NFR-034 revalidation
  against the new topology), `docs/acceptance-criteria.md` (AC-024
  amendment, new criteria for session/CSRF/authorization behavior; AC-028
  revalidation against the new topology).
- **Security acceptance criteria:** Not applicable to code — this pass's own
  "acceptance criterion" is a recorded, explicit approval decision (e.g. a
  merged PR with reviewer sign-off, or an equivalent recorded decision),
  since nothing here is testable software.
- **Required unit tests:** None.
- **Required integration tests:** None.
- **Required e2e / abuse-case tests:** None.
- **Explicit non-goals:** No code, dependency, or infrastructure change of
  any kind. No behavior change to the running platform.
- **Migration / compatibility considerations:** None — the currently
  deployed bearer-token authentication is entirely unaffected until Pass 2
  onward begins landing code.
- **Rollback considerations:** Trivial — revert the documentation commit.
  Zero runtime impact, since nothing has been deployed yet.
- **Recommended atomic commit boundary:** One commit/PR containing the ADR
  status flip together with every baseline-document amendment it requires,
  reviewed and merged as a single indivisible governance decision — a
  half-approved architecture (e.g. ADR accepted but NFR-012 left
  contradicting it) must not be allowed to land.

## Pass 2 — Production reverse-proxy or BFF boundary

- **Goal:** Stand up the same-origin reverse-proxy/BFF process as network
  topology only — proves TLS-termination placement and same-origin `/api/*`
  routing to the existing Go backend, with **no identity logic yet**.
- **Prerequisites:** Pass 1 accepted.
- **Layers / files likely affected:** A new BFF/proxy service and its
  configuration (concrete technology per `open-decisions.md`), a new
  `docker-compose.yml` service for local dev-parity, `docs/reference-environment.md`
  updates describing how to run it. `frontend/vite.config.ts`'s existing dev
  proxy is unaffected — it remains local developer tooling, documented as
  such in `identity-and-session-architecture.md`'s "Local development versus
  production."
- **Security acceptance criteria:** The BFF forwards `/api/*` to the Go
  backend and strips any inbound `Authorization`/identity-shaped header
  before forwarding — the exact pattern already proven in this codebase by
  `createApiProxy`'s `proxyReq.removeHeader("authorization")` — applied here
  to the new production boundary rather than only the dev proxy. The Go
  backend remains unreachable from any origin other than this one proxy hop.
- **Required unit tests:** Header-stripping/forwarding unit tests for the
  new proxy layer, mirroring `frontend/src/test/devProxyAuth.test.ts`'s
  approach for whichever stack is chosen.
- **Required integration tests:** A real request through the BFF reaches the
  real backend and returns the expected response.
- **Required e2e / abuse-case tests:** A forged inbound identity/auth header
  sent to the BFF is confirmed stripped before the backend ever sees it.
- **Explicit non-goals:** No OIDC, no sessions, no cookies, no authorization
  logic. This pass proves the pipe works, nothing more.
- **Migration / compatibility considerations:** Purely additive — the
  existing bearer-token flow can continue to operate through this new BFF
  unchanged in this pass (the BFF is a transparent forwarder so far).
- **Rollback considerations:** Stop/remove the new BFF service; traffic
  reverts to whatever path preceded it. No data or session state exists yet
  to reconcile.
- **Recommended atomic commit boundary:** One PR: the BFF service, its
  config, and its header-stripping tests — no application logic.

## Pass 3 — External OIDC integration

- **Goal:** The BFF becomes a real OIDC client: Authorization Code Flow with
  PKCE, `state`/`nonce` generation and validation, redirect-URI validation,
  and server-to-server code exchange, ending in an **ephemeral, in-memory**
  authenticated context — deliberately not yet backed by durable storage, so
  this pass can be reviewed and verified purely on OIDC mechanics, isolated
  from persistence concerns (Pass 4).
- **Prerequisites:** Pass 2.
- **Layers / files likely affected:** New BFF-side OIDC client module
  (login-initiation route, callback route), configuration for the IdP's
  issuer/client ID/client secret (see Pass 17 for how the secret itself is
  eventually managed; this pass may use a simple environment variable,
  matching this repository's existing `API_TOKEN`-style local-dev pattern,
  explicitly flagged as an interim measure).
- **Security acceptance criteria:** PKCE `S256` challenge/verifier used
  unconditionally; `state` is single-use and expires quickly; `nonce` is
  validated against the ID token; redirect URI is an exact-match,
  environment-fixed value never derived from request headers; a
  local-development OIDC provider (per `open-decisions.md`) or a real
  provider is used — never a hardcoded bypass.
- **Required unit tests:** PKCE challenge/verifier generation and matching;
  `state`/`nonce` generation, single-use consumption, and expiry;
  redirect-URI validation against the configured allowlist (exactly one
  value, not a list, per `identity-and-session-architecture.md`).
- **Required integration tests:** A full Authorization Code Flow with PKCE
  exchange against a real (local, containerized) OIDC provider, including ID
  token signature and `nonce` validation.
- **Required e2e / abuse-case tests:** A forged/mismatched `state` on
  callback is rejected; a forged/replayed `nonce` is rejected; a callback
  presented to a non-registered redirect URI is rejected.
- **Explicit non-goals:** No PostgreSQL session table yet (Pass 4). No
  cookie hardening details beyond what is needed to correlate the
  in-progress login attempt (Pass 5 finalizes cookie configuration). No
  RBAC, no audit logging wiring yet (Pass 12 wires audit events into this
  code retroactively).
- **Migration / compatibility considerations:** Additive: this new login
  path exists alongside the current bearer-token flow, which remains the
  platform's only *enforced* authentication until a later pass switches the
  backend over (see Pass 7's migration note).
- **Rollback considerations:** Disable/remove the login and callback routes;
  no durable state exists yet to reconcile, since this pass is deliberately
  ephemeral.
- **Recommended atomic commit boundary:** One PR: OIDC client, login/callback
  routes, PKCE/state/nonce logic, and their unit/integration tests — no
  persistence, no RBAC.

## Pass 4 — Server-side session persistence

- **Goal:** Replace Pass 3's ephemeral, in-memory session with a durable
  PostgreSQL-backed one: the session table, hashed session-identifier
  lookup, and the `identity-and-session-architecture.md` schema
  (`session_id_hash`, `user_subject`, `issued_at`, `last_seen_at`,
  `idle_expires_at`, `absolute_expires_at`, `revoked_at`, `ip_at_issue`,
  `user_agent_at_issue`, `rotation_count`).
- **Prerequisites:** Pass 3.
- **Layers / files likely affected:** A new migration under `migrations/`
  (following this repository's existing numbered-migration convention,
  e.g. `internal/db/migrate.go`'s pattern and `migrations/000X_*.up/down.sql`),
  a new session-storage module (own table, own module boundary, per
  `../architecture.md` §2's "no module reads or writes another module's
  table directly" discipline), BFF-side wiring to read/write it.
- **Security acceptance criteria:** Only a SHA-256 hash of the session
  identifier is ever stored (mirroring `internal/auth.Bearer`'s existing
  hash-before-compare pattern) — never the raw value; the raw value exists
  only in the cookie and in-flight in the BFF process.
- **Required unit tests:** Session-hash computation and lookup;
  create/read/expire-check logic against a fake or ephemeral store.
- **Required integration tests:** A real PostgreSQL-backed session survives
  a BFF process restart; an expired/revoked row is correctly rejected on
  lookup, using this repository's existing ephemeral-Postgres-via-testcontainers
  integration-test pattern (`internal/testutil.MigratedPostgres`).
- **Required e2e / abuse-case tests:** A session row manually marked
  `revoked_at` is rejected on the very next request (no caching window).
- **Explicit non-goals:** Idle/absolute timeout *enforcement* and rotation
  policy are specified by `identity-and-session-architecture.md` but their
  operational wiring is Pass 5's job — this pass only needs the schema and
  basic create/read/expire-check to exist and be correct.
- **Migration / compatibility considerations:** New table, additive
  migration; no existing table is altered. The bearer-token flow is
  unaffected.
- **Rollback considerations:** A down-migration drops the new table; no
  existing data is at risk since nothing else references it yet.
- **Recommended atomic commit boundary:** One PR: the migration, the session
  module, and its tests — no cookie-hardening or lifecycle-policy logic yet.

## Pass 5 — Secure cookie and session lifecycle controls

- **Goal:** Finalize the cookie configuration (`HttpOnly`, `Secure`,
  `SameSite=Lax`, `Path`, expiration mirroring `absolute_expires_at`) and
  implement the full session lifecycle: rotation on login/privilege
  change/periodic interval, idle-timeout and absolute-timeout enforcement on
  every request, and an explicit revocation path.
- **Prerequisites:** Pass 4.
- **Layers / files likely affected:** BFF cookie-issuing code, session
  lookup/rotation logic (extends Pass 4's module), a logout route.
- **Security acceptance criteria:** Every cookie attribute matches
  `identity-and-session-architecture.md`'s table exactly; idle and absolute
  timeouts are enforced by re-checking the database row on every request,
  never inferred from the cookie's presence alone; rotation issues a
  genuinely new session identifier and invalidates the old one rather than
  extending it in place.
- **Required unit tests:** Cookie-attribute construction; idle/absolute
  timeout boundary conditions (just-inside vs. just-outside the window);
  rotation logic (old row invalidated, new row created, both linked for
  audit purposes per Pass 12).
- **Required integration tests:** A session past its idle timeout is
  rejected; a session past its absolute timeout is rejected even with
  continuous activity; a rotated session's old identifier is rejected after
  rotation.
- **Required e2e / abuse-case tests:** A browser replaying a
  pre-rotation cookie value after rotation is rejected; a session fixation
  attempt (an attacker-supplied pre-auth session identifier surviving into
  an authenticated session) fails, since login always issues a fresh
  identifier per `identity-and-session-architecture.md`.
- **Explicit non-goals:** CSRF protection (Pass 6). Federated/IdP logout is
  explicitly optional per the design document and may be deferred beyond
  this pass.
- **Migration / compatibility considerations:** No schema change beyond Pass
  4's table (timeout/rotation are policy, not schema). Bearer-token flow
  still unaffected.
- **Rollback considerations:** Revert lifecycle-policy code; the underlying
  session table from Pass 4 is unaffected and can remain.
- **Recommended atomic commit boundary:** One PR: cookie hardening,
  timeout/rotation enforcement, logout route, and their tests.

## Pass 6 — CSRF protection

- **Goal:** Implement the synchronizer-token CSRF defense
  `identity-and-session-architecture.md` requires, closing the exposure the
  new ambient session cookie introduces (which the current bearer-token
  model does not have).
- **Prerequisites:** Pass 5 (a real session cookie must exist to protect).
- **Layers / files likely affected:** A CSRF-token issuance endpoint or
  companion mechanism at the BFF, a CSRF-validation check applied to every
  non-`GET` route at the BFF layer, frontend API client changes to attach
  the token header (`frontend/src/lib/api/client.ts`'s eventual
  session-era equivalent).
- **Security acceptance criteria:** Every state-changing request lacking a
  valid, session-bound CSRF token is rejected before reaching the backend;
  the check is enforced at the BFF, not only assumed from `SameSite=Lax`.
- **Required unit tests:** Token issuance and validation logic, including
  rejection of a missing, mismatched, or session-mismatched token.
- **Required integration tests:** A real cross-origin-simulated request
  without the CSRF header is rejected end to end; a same-origin request with
  a valid token succeeds.
- **Required e2e / abuse-case tests:** A simulated forged cross-site form
  submission (relying on the ambient cookie alone, no CSRF header) is
  rejected.
- **Explicit non-goals:** No change to the read-only `GET` endpoints, which
  already correctly perform no mutation (confirmed by direct inspection of
  `internal/intake` and `internal/retrieval` during the design phase) and so
  are not CSRF-relevant.
- **Migration / compatibility considerations:** Purely additive to the new
  session flow; the bearer-token flow (still header-based, still
  CSRF-immune by construction) is unaffected and needs no CSRF token.
- **Rollback considerations:** Remove the CSRF check; since it is a pure
  request-rejection gate with no persisted state of its own beyond the
  token, rollback has no data-cleanup step.
- **Recommended atomic commit boundary:** One PR: CSRF issuance, validation,
  frontend wiring, and tests.

## Pass 7 — Backend authentication context

- **Goal:** Teach the Go backend to receive and trust a BFF-asserted
  identity (subject, session reference) as a request-scoped context value —
  distinct from, and initially additive alongside, today's single
  `auth.Bearer` boolean check.
- **Prerequisites:** Pass 2 (BFF exists) and Pass 3 (an identity is
  establishable) at minimum; can be developed and unit-tested against a
  stubbed identity header before Passes 4–6 fully land, but is not
  meaningfully end-to-end testable until they do.
- **Layers / files likely affected:** A new `internal/authcontext` (or
  similarly named) module, following the existing module-boundary
  discipline; `cmd/platform/main.go`'s handler wiring; `internal/auth/auth.go`
  is extended, not replaced, in this pass (see migration note).
- **Security acceptance criteria:** The backend trusts an
  identity-asserting header or mechanism **only** when it demonstrably
  arrived through the one trusted BFF hop (per
  `identity-and-session-architecture.md`'s trusted-proxy rule) — formalized
  fully in Pass 15, but this pass must not introduce a blindly-trusted
  header as an interim shortcut.
- **Required unit tests:** Context construction from a valid asserted
  identity; rejection of a request with no identity asserted at all (falls
  back to "unauthenticated," never a default identity).
- **Required integration tests:** A request through the full BFF→backend
  path carries the correct identity into the backend's request context.
- **Required e2e / abuse-case tests:** A request sent directly to the
  backend, bypassing the BFF (simulating an attacker who reaches the backend
  port directly), is rejected — the backend must not accept a
  self-asserted identity from an untrusted origin.
- **Explicit non-goals:** No authorization *decision* yet (Pass 8) — this
  pass only establishes **who**, not **what they may do**.
- **Migration / compatibility considerations:** `internal/auth.Bearer` and
  the existing `API_TOKEN` check remain fully functional and are not removed
  in this pass — the new identity-context mechanism is additive, serving
  only the new session-based requests, so the machine/service-credential
  path (Pass 19) is unaffected until it is deliberately migrated.
- **Rollback considerations:** Remove the new context-extraction middleware;
  existing bearer-token-gated handlers are entirely unaffected since they
  do not consume it.
- **Recommended atomic commit boundary:** One PR: the identity-context
  module and its tests, with no handler behavior change yet.

## Pass 8 — Backend authorization middleware

- **Goal:** Introduce the generic `(identity, role, resource, action) ->
  allow/deny` decision point as reusable middleware, wired in front of
  handlers — but with role data not yet differentiated (Pass 9), so this
  pass's effective behavior for any authenticated session is "full access,"
  a deliberate, temporary equivalent of today's single-trust-level model,
  proving the middleware's mechanics before real differentiation lands.
- **Prerequisites:** Pass 7.
- **Layers / files likely affected:** A new `internal/authorization` module
  implementing the decision function against `authorization-model.md`'s
  matrix shape (even though most entries are placeholder-allow at this
  stage); wiring into `cmd/platform/main.go`.
- **Security acceptance criteria:** Deny-by-default is structurally true
  even at this stage — an action with no matching rule at all (not merely
  "no role assigned yet") is denied, never silently allowed; the middleware
  is the first thing each wired handler executes, before any database
  access, mirroring the existing `internal/retrieval` handlers' pattern of
  checking authentication before all other work.
- **Required unit tests:** Deny-by-default for an unrecognized
  resource/action pair; allow for the placeholder full-access role.
- **Required integration tests:** A request through the full path is denied
  when the middleware is deliberately misconfigured to have no matching
  rule (proving fail-closed, not fail-open).
- **Required e2e / abuse-case tests:** None beyond the above at this stage —
  meaningful abuse-case coverage (privilege escalation, IDOR) requires real
  role differentiation from Pass 9 onward.
- **Explicit non-goals:** No real role differentiation yet (Pass 9). No
  endpoint-specific wiring yet beyond a proof-of-concept route (Pass 10
  applies it to the real alert endpoints).
- **Migration / compatibility considerations:** Additive; existing
  bearer-token-gated handlers are unaffected until Pass 10 deliberately
  migrates them.
- **Rollback considerations:** Remove the middleware; no handler currently
  depends on it exclusively until Pass 10.
- **Recommended atomic commit boundary:** One PR: the authorization module
  and its tests, wired to nothing but a proof-of-concept route.

## Pass 9 — Role and permission persistence

- **Goal:** Implement the real role model from `authorization-model.md`: a
  roles/permissions table (or equivalent), role assignment to identities,
  and Pass 8's middleware now resolving genuinely differentiated roles
  instead of the placeholder full-access stand-in.
- **Prerequisites:** Pass 8.
- **Layers / files likely affected:** A new migration for the roles/
  permission-assignment table(s); an admin-facing assignment mechanism
  (minimal — full administrator UX is Pass 18); the Pass 8 middleware's role
  resolution logic.
- **Security acceptance criteria:** The six roles from
  `authorization-model.md` ("Role evaluation") exist as data, each mapped to
  exactly the permission matrix's stated grants; no role receives a
  permission the matrix does not list for it.
- **Required unit tests:** Role-to-permission resolution for every role in
  the matrix; an identity with no assigned role resolves to zero
  permissions (deny-by-default extended to "no role" as a case, not just
  "no rule").
- **Required integration tests:** A real database-backed role assignment
  correctly gates a proof-of-concept protected action.
- **Required e2e / abuse-case tests:** An identity assigned Viewer cannot
  perform any action the matrix marks denied for Viewer, verified against a
  real request path, not only a unit-level check.
- **Explicit non-goals:** No alert-specific or raw-payload-specific wiring
  yet (Passes 10–11). No audit logging of role changes yet (Pass 12, though
  this pass should leave an obvious integration point for it).
- **Migration / compatibility considerations:** New table(s), additive
  migration. Every existing identity created by Pass 3–7's OIDC flow needs a
  default role assignment strategy (e.g. "new identities default to Viewer,
  an administrator must explicitly elevate") — decided and documented in
  this pass, not left implicit.
- **Rollback considerations:** Down-migration drops the new table(s); Pass
  8's middleware reverts to its placeholder full-access behavior in the
  interim. **This is time-ordering-dependent, not unconditionally safe:**
  before any later pass depends on real role differentiation, this
  fallback is a reasonable, low-stakes reversion to a temporary baseline,
  subject to that baseline's own documented limitations (Pass 8 itself).
  **Once Pass 10 (alert-resource authorization), Pass 11 (raw-payload
  authorization), or any other permission boundary has shipped and is
  relied upon, rolling Pass 9 back is a privilege-escalation exposure, not
  a safe fallback** — it grants every authenticated session, including
  Viewer, whatever Pass 10/11 and later passes exist specifically to
  restrict (raw payload, export, administrative-adjacent actions). Once
  that point is reached, do not roll back Pass 9 in isolation as an
  incident response; use a forward fix, a feature-disable path scoped to
  the specific defect, or a coordinated rollback of every pass that
  depends on Pass 9's role data. Operators must not treat "revert to
  permissive" as a safe default once later passes are live.
- **Recommended atomic commit boundary:** One PR: role schema, assignment
  logic, and tests.

## Pass 10 — Alert-resource authorization

- **Goal:** Apply Pass 8/9's real authorization middleware to this
  platform's actual existing protected endpoints — `GET /v1/alerts` and
  `GET /v1/alerts/{id}` — replacing `internal/auth.Bearer`'s binary check
  for these two routes with the new role-based "View alert inventory" /
  "View alert investigation details" permission checks.
- **Prerequisites:** Pass 9.
- **Layers / files likely affected:** `internal/retrieval/list.go`,
  `internal/retrieval/retrieval.go` (handler wiring change), `cmd/platform/main.go`.
- **Security acceptance criteria:** Both endpoints reject a request whose
  resolved role lacks the corresponding permission, using the object-level
  (`404`) versus action-level (`403`) distinction `authorization-model.md`
  specifies; the existing 401-for-no-session behavior is preserved for
  entirely unauthenticated requests.
- **Required unit tests:** Handler-level tests for allow/deny per role,
  extending `internal/retrieval`'s existing test files rather than
  duplicating their structure.
- **Required integration tests:** A real database-backed request from each
  of the six roles against both endpoints, confirmed against the matrix.
- **Required e2e / abuse-case tests:** A Viewer-role session cannot reach
  data gated to Analyst+; a session for one identity cannot be reused to
  imply access it was never granted (regression check against the
  authorization change, not a new session-mechanics test — those belong to
  Pass 5).
- **Explicit non-goals:** Raw-payload/provenance sub-view gating within
  `GET /v1/alerts/{id}`'s response is Pass 11's job, not this pass's — this
  pass gates at the endpoint level only.
- **Migration / compatibility considerations:** This is the **first pass
  that changes behavior for real, currently-reachable endpoints.** The
  bearer-token path must be explicitly decided here: either (a) continue to
  work in parallel for machine identities only (see Pass 19), with human
  browser access now required to go through the session path, or (b) both
  paths accepted temporarily behind a feature flag during rollout. Document
  the choice; do not silently break the existing frontend's real-backend
  integration mid-rollout.
- **Rollback considerations:** Revert the handler wiring to
  `internal/auth.Bearer`; no data migration is implicated, since roles and
  sessions are additive tables that can remain unused if rolled back.
- **Recommended atomic commit boundary:** One PR per endpoint is acceptable
  given the small blast radius (two files), or one PR for both together
  given how small each change is — either is reasonable; do not bundle this
  with Pass 11's finer-grained change.

## Pass 11 — Sensitive raw-payload authorization

- **Goal:** Implement the finer-grained, **response-shaping** authorization
  `authorization-model.md` requires for raw event payload and provenance
  detail within `GET /v1/alerts/{id}` — a materially different, higher-risk
  implementation than Pass 10's coarse endpoint gate, kept as its own pass
  deliberately.
- **Prerequisites:** Pass 10.
- **Layers / files likely affected:** `internal/retrieval/retrieval.go`'s
  response-composition logic (`toResponse`), `internal/evidence` (composition
  boundary) — the raw-event and provenance fields must be conditionally
  populated based on the resolved role, not merely present-or-401 for the
  whole response.
- **Security acceptance criteria:** A session with alert-view but not
  raw-payload-view permission receives a response with the raw-event field
  explicitly marked unavailable-by-permission (never silently empty in a
  way indistinguishable from a genuine data gap — this reuses this
  platform's own existing, already-approved discipline, FR-035, that a
  limitation must be visible, never a silent omission, applied here to an
  authorization-driven gap rather than a data-availability gap) — the field
  is omitted from the payload, never included and merely hidden by the
  frontend.
- **Required unit tests:** Response composition for each role against a
  fixture alert, confirming exactly which fields are present.
- **Required integration tests:** A real request from a role without
  raw-payload permission never receives the raw bytes on the wire — verified
  by inspecting the actual HTTP response body, not just a code-level flag.
- **Required e2e / abuse-case tests:** A network-level capture of the
  response for a denied role confirms the raw payload is absent, not merely
  suppressed by frontend rendering.
- **Explicit non-goals:** Export-specific gating (a separate permission
  from viewing per `authorization-model.md`) is not in scope for this pass
  if no export endpoint exists yet — see `authorization-model.md`'s
  endpoint-to-permission mapping for which endpoints are ⚠ Proposed and not
  yet built.
- **Migration / compatibility considerations:** Changes the actual response
  shape for `GET /v1/alerts/{id}` for lower-privileged roles — any existing
  frontend consumer must handle the field's absence gracefully (extending
  the same `available: boolean` pattern this response already uses for
  data-availability gaps, per `internal/retrieval/retrieval.go`'s existing
  `sourceEventField`/etc. types).
- **Rollback considerations:** Revert to always including the raw-event
  field for any authenticated session (Pass 10's coarser gate still
  applies); no data loss, since this is a response-shaping change only.
- **Recommended atomic commit boundary:** One PR: response-composition
  changes plus their tests, kept separate from Pass 10's endpoint-level
  change even though both touch the same handler.

## Pass 12 — Audit logging

- **Goal:** Implement `audit-and-accountability-design.md`'s append-only
  audit trail — schema, hash chain, database role separation — and wire
  audit-emission calls into every code path built by Passes 3–11
  retroactively (login, session lifecycle, authorization denials, role
  changes).
- **Prerequisites:** Passes 3–11 (there must be real events to audit).
- **Layers / files likely affected:** A new migration for the audit table;
  a new `internal/audit` module (own table, own module boundary); emission
  call sites added to the OIDC login/callback code (Pass 3), session
  lifecycle code (Pass 5), authorization middleware (Pass 8), and role
  management (Pass 9) — each a small, additive change to existing new code,
  not a rewrite.
- **Security acceptance criteria:** Every event category in
  `audit-and-accountability-design.md`'s catalog that is reachable by code
  landed so far actually emits a row; the append-only enforcement (no
  update/delete code path, a database role without `UPDATE`/`DELETE`/
  `TRUNCATE` grants) is verified, not merely asserted; the hash chain
  verifies across a representative sample of rows.
- **Required unit tests:** Audit-row construction for each event type;
  hash-chain computation and verification, including a deliberately
  tampered row failing verification.
- **Required integration tests:** A real database-backed write using the
  restricted audit-writer role succeeds for `INSERT` and fails for an
  attempted `UPDATE`/`DELETE` (proving the database-level enforcement, not
  only the application-level one).
- **Required e2e / abuse-case tests:** A failed-login attempt is audited
  with a closed-vocabulary `reason_code` and never the attempted credential
  value, verified by direct inspection of the persisted row.
- **Explicit non-goals:** No audit-log *viewing* UI/endpoint yet — that is
  part of Pass 18 (administrator workflows), gated by its own "view audit
  logs" permission.
- **Migration / compatibility considerations:** New table, additive
  migration, no existing behavior changed by its mere existence — only the
  small instrumentation additions to Passes 3–11's own code paths.
- **Rollback considerations:** Down-migration drops the audit table; the
  instrumentation call sites can be reverted independently without
  affecting the underlying login/session/authorization behavior they were
  observing.
- **Recommended atomic commit boundary:** One PR for the audit module and
  schema itself; one or more small follow-up PRs wiring emission into each
  earlier pass's code, so a reviewer can verify each call site against its
  own pass's behavior rather than reviewing one enormous cross-cutting diff.

## Pass 13 — Rate limiting

- **Goal:** Protect the login endpoint against brute-force and
  credential-stuffing, and the general API surface against abusive request
  volume — substantially independent of the identity work and could be
  pulled earlier by an implementer if useful.
- **Prerequisites:** Pass 2 (a BFF boundary to apply the limiting at) is the
  only hard prerequisite; login-specific limiting additionally benefits from
  Pass 3 existing so there is a real login endpoint to protect.
- **Layers / files likely affected:** BFF-layer rate-limiting middleware;
  possibly a shared store (e.g. the same PostgreSQL instance, or an
  in-memory/token-bucket approach for a single-instance deployment,
  consistent with this platform's existing "no premature scale" discipline,
  `PC-P-006`).
- **Security acceptance criteria:** Repeated failed login attempts from one
  source are throttled with a visible, safe response (never revealing
  whether the throttling is username-specific, which would itself leak
  account-existence information); general API rate limiting does not starve
  legitimate sustained use within this platform's already-approved capacity
  envelope (NFR-003, 10 submissions/sec — the human investigation API has no
  equivalent approved numeric target yet and should not silently invent
  one here beyond what protects against abuse).
- **Required unit tests:** Limiter logic (window, threshold, reset).
- **Required integration tests:** Repeated requests past the threshold are
  rejected; requests resume succeeding after the window resets.
- **Required e2e / abuse-case tests:** A simulated credential-stuffing burst
  against the login endpoint is throttled without denying legitimate,
  normally-paced logins.
- **Explicit non-goals:** No numeric rate-limit value is fixed by this
  roadmap itself — the concrete threshold is an implementation-time decision
  informed by real usage, not invented here.
- **Migration / compatibility considerations:** Additive; applies to new
  session-based routes primarily, and optionally to the existing
  bearer-token intake endpoint as a general hardening measure independent
  of this identity work.
- **Rollback considerations:** Remove/disable the limiter; no persisted
  state beyond transient counters.
- **Recommended atomic commit boundary:** One PR: limiter middleware,
  applied to the login route at minimum, with tests.

## Pass 14 — Security headers

- **Goal:** Apply standard hardening HTTP response headers (Content-Security-Policy,
  `X-Content-Type-Options`, `Referrer-Policy`, `Permissions-Policy`, and
  `Strict-Transport-Security` once Pass 16's TLS boundary exists) at the BFF
  layer — also substantially independent of the identity work.
- **Prerequisites:** Pass 2. `Strict-Transport-Security` specifically
  depends on Pass 16.
- **Layers / files likely affected:** BFF-layer response-header middleware.
- **Security acceptance criteria:** Headers present on every response;
  `Content-Security-Policy` is restrictive (no wildcard script/style
  sources) and does not silently break the existing frontend — verified
  against the real built application, not assumed compatible.
- **Required unit tests:** Header presence and value assertions.
- **Required integration tests:** A real response from the BFF carries the
  expected headers.
- **Required e2e / abuse-case tests:** A CSP violation report (or console
  error, in a headless-browser e2e check) confirms the policy actually
  blocks an injected inline script in a deliberate test page, proving the
  policy is enforced, not merely present as an unenforced header value.
- **Explicit non-goals:** No change to application logic; this pass is
  header configuration only.
- **Migration / compatibility considerations:** A misconfigured CSP can
  break legitimate frontend functionality (inline styles, third-party
  fonts, etc.) — must be verified against the real built `frontend/dist`
  output, not a synthetic page, before rollout.
- **Rollback considerations:** Loosen or remove the header middleware;
  purely a response-header change with no persisted state.
- **Recommended atomic commit boundary:** One PR: header middleware and its
  tests, verified against a real frontend build.

## Pass 15 — Trusted-proxy configuration

- **Goal:** Formalize exactly which forwarded headers
  (`X-Forwarded-For`, `X-Forwarded-Proto`, the BFF-asserted-identity header
  from Pass 7) the Go backend trusts, and from where — hardening what Pass 2
  set up loosely into an explicit, verified boundary.
- **Prerequisites:** Passes 2 and 7.
- **Layers / files likely affected:** The Pass 7 identity-context module's
  trust logic; backend startup configuration naming the exact trusted hop
  (private network segment, Unix socket, or shared-secret header the proxy
  alone injects).
- **Security acceptance criteria:** A request that could plausibly have
  arrived directly from the internet (i.e. did not demonstrably pass through
  the one trusted hop) never has its forwarded/identity headers trusted,
  regardless of their content.
- **Required unit tests:** Trust-decision logic for both the trusted-hop and
  untrusted-origin cases.
- **Required integration tests:** A request simulating direct backend
  access (bypassing the BFF) with a forged identity/forwarded header is
  rejected or treated as untrusted.
- **Required e2e / abuse-case tests:** The forged-header scenario is
  exercised through the real deployed topology, not only a unit-level
  simulation — extending the same principle this platform already proved
  for the dev proxy's `Authorization`-header stripping
  (`frontend/e2e/real-backend.spec.ts`'s forged-header test) to the
  production identity-header boundary.
- **Explicit non-goals:** No change to what the BFF itself asserts — this
  pass only hardens what the *backend* is willing to believe.
- **Migration / compatibility considerations:** None beyond Pass 7's own.
- **Rollback considerations:** Loosen the trust check back toward Pass 7's
  initial state; no persisted state involved.
- **Recommended atomic commit boundary:** One PR: trust-boundary
  configuration and its tests.

## Pass 16 — TLS and production ingress

- **Goal:** Stand up the actual TLS-terminating boundary in front of (or as
  part of) the BFF — the point at which this platform can honestly be
  called production-deployed rather than "works over plaintext localhost."
- **Prerequisites:** Pass 2 (something must exist for TLS to terminate in
  front of).
- **Layers / files likely affected:** Deployment/ingress configuration
  (concrete mechanism per `open-decisions.md` — a managed load balancer, a
  reverse-proxy TLS terminator, or equivalent); `docs/reference-environment.md`
  or a new production-deployment document describing it; certificate
  provisioning/renewal process.
- **Security acceptance criteria:** All production traffic is TLS-only;
  plaintext HTTP is either redirected to HTTPS or refused outright; the
  `Secure` cookie attribute (Pass 5) is now meaningfully enforced rather
  than a no-op over plaintext.
- **Required unit tests:** Not generally applicable to infrastructure
  configuration; covered instead by integration/e2e checks below.
- **Required integration tests:** A plaintext request to the production
  boundary is redirected or refused; a TLS request succeeds with a valid
  certificate chain.
- **Required e2e / abuse-case tests:** A downgrade attempt (forcing
  plaintext) is rejected; certificate validity is checked as part of routine
  deployment verification.
- **Explicit non-goals:** No application-code change in this pass.
- **Migration / compatibility considerations:** Local development remains
  the one documented exception to mandatory TLS, per
  `identity-and-session-architecture.md`'s "Local development versus
  production" — this pass must not silently force TLS onto the local
  dev-parity path in a way that breaks the existing documented dev workflow.
- **Rollback considerations:** Infrastructure-level; revert to the prior
  ingress configuration per the deployment platform's own rollback
  mechanism.
- **Recommended atomic commit boundary:** One PR/change for the ingress
  configuration and its documentation, reviewed by whoever owns production
  deployment.

## Pass 17 — Secret management

- **Goal:** Move production secrets (OIDC client secret, database
  credentials, service credentials, and — only if `identity-and-session-architecture.md`
  Option B is ever adopted — the provider-refresh-token encryption key) from
  the current `.env`-file local-dev pattern to a real production
  secret-management mechanism.
- **Prerequisites:** Passes 3, 4, and 16 (there must be secrets worth
  managing and a production boundary to deliver them to).
- **Layers / files likely affected:** Deployment configuration; no
  application-code change beyond how configuration values are read
  (already environment-variable-based, per `cmd/platform/main.go`'s existing
  pattern, so the application-facing interface may not need to change at
  all — only *where* those environment values come from in production).
- **Security acceptance criteria:** No production secret is ever committed
  to the repository, bundled into the frontend build, or written to a log —
  directly extending this platform's already-proven discipline
  (`frontend/src/test/noLeakedSecretsInBuild.test.ts`,
  `noClientCredentials.test.ts`) to the new secret set.
- **Required unit tests:** Not generally applicable; covered by the
  build/log-scanning checks below.
- **Required integration tests:** A build/deploy performed with the new
  secret-management mechanism succeeds without any secret value appearing
  in build artifacts.
- **Required e2e / abuse-case tests:** A scan of the deployed application's
  logs, diagnostics, and audit trail (Pass 12) confirms no secret value
  appears anywhere, extending this platform's existing sentinel-based
  build-leak-scan technique to the production secret set.
- **Explicit non-goals:** No change to local development's existing
  `.env`-based workflow (`docs/reference-environment.md`), which remains
  documented as local-only and explicitly not a production mechanism.
- **Migration / compatibility considerations:** A cutover step — secrets
  move from wherever they were provisioned during Passes 3–16's development
  to the production mechanism; must be sequenced so no secret is ever
  briefly absent during the cutover (a deployment-runbook concern, not a
  code concern).
- **Rollback considerations:** Revert to the prior secret-delivery
  mechanism per the deployment platform's own process; no application code
  is at risk since the interface (environment variables) does not change.
- **Recommended atomic commit boundary:** One change for the
  secret-management integration itself, reviewed by whoever owns production
  deployment, separate from any application-code PR.

## Pass 18 — Administrator workflows

- **Goal:** Build the actual UI/API for Platform-Administrator-only
  capabilities: user and role management, session revocation, audit-log
  viewing — deliberately last among the "core" passes, since it is the
  highest-privilege surface and should only be built once identity,
  authorization, and audit logging beneath it are solid and verified.
- **Prerequisites:** Passes 9 (roles exist) and 12 (audit logging exists,
  so administrator actions are themselves auditable from day one — this
  pass must never ship an unaudited administrative capability).
- **Layers / files likely affected:** New backend endpoints (per
  `authorization-model.md`'s endpoint-to-permission mapping's ⚠ Proposed
  administrative rows), a new frontend administrative surface (out of scope
  for the existing Alert Investigation feature's own implementation-plan
  documents — a separate frontend feature).
- **Security acceptance criteria:** Every administrative action is gated by
  the Platform-Administrator permission via the same Pass 8 middleware used
  everywhere else (no bespoke admin-only authorization path); every
  administrative action is audited (Pass 12) with correct
  `previous_value`/`new_value` metadata; role changes trigger the session
  rotation `identity-and-session-architecture.md` requires for the affected
  identity.
- **Required unit tests:** Handler-level tests for every new administrative
  endpoint, allow/deny per role.
- **Required integration tests:** A real role change through this UI/API
  takes effect and is reflected in the affected identity's next
  authorization decision.
- **Required e2e / abuse-case tests:** A non-administrator session cannot
  reach any administrative endpoint, verified end to end; a role change
  performed through this workflow correctly triggers session rotation for
  the affected user (a vertical-privilege-escalation regression check).
- **Explicit non-goals:** This pass does not itself decide emergency/
  break-glass administrator access procedures — see `open-decisions.md`,
  "Emergency administrator access."
- **Migration / compatibility considerations:** The very first Platform
  Administrator identity must be bootstrapped somehow (no administrator
  exists yet to grant the role) — this pass must document and implement
  that bootstrap path explicitly (e.g. a one-time migration-driven seed, or
  a startup-configuration-driven initial administrator), not leave it as an
  unstated assumption.
- **Rollback considerations:** Remove the administrative endpoints/UI; roles
  and audit data from earlier passes are unaffected.
- **Recommended atomic commit boundary:** Multiple small PRs, one per
  administrative capability (user management, role management, session
  revocation, audit-log viewing), rather than one large administrative-UI
  PR — each is independently reviewable and independently gated by its own
  permission.

## Pass 19 — Service identities

- **Goal:** Formalize per-integration machine credentials, replacing today's
  single shared `API_TOKEN` used by the Kubernetes audit-webhook submitter —
  the "gradual retirement of the current shared service credential" ADR-0005
  names.
- **Prerequisites:** Pass 8 **and Pass 9** — the authorization middleware
  must exist *and* already resolve real, differentiated roles/permissions
  (Pass 9), not Pass 8's placeholder full-access behavior. A service
  identity scoped to "submit telemetry only" is not actually narrowly
  scoped if the middleware in front of it still grants full access to
  every authenticated caller; Pass 9's real permission data is what makes
  this pass's own acceptance criteria (below) achievable.
- **Layers / files likely affected:** `internal/auth/auth.go` (extended to
  support multiple, distinctly-scoped service credentials rather than one
  global token), `cmd/platform/main.go`, `docker-compose.yml`/`.env.example`
  documentation for the new credential-provisioning process,
  `internal/intake` (the submission endpoint's credential check).
- **Security acceptance criteria:** Each service identity is scoped to
  exactly the permissions `authorization-model.md`'s "Machine identity
  authorization" specifies (the audit-webhook submitter gets exactly
  "submit telemetry," nothing else); the old single shared `API_TOKEN` is
  retired only after every real integration has migrated to its own
  credential — never removed while still relied upon.
- **Required unit tests:** Per-credential permission scoping; rejection of a
  service credential attempting an action outside its scope.
- **Required integration tests:** The real intake endpoint continues to
  function correctly for a migrated service credential.
- **Required e2e / abuse-case tests:** A service credential scoped to
  submission-only cannot read alert data, verified against the real
  endpoint, not only a unit-level policy check.
- **Explicit non-goals:** No change to the intake wire format or submission
  semantics (ADR-0003) — this pass changes only how the submitter
  authenticates, not what it submits.
- **Migration / compatibility considerations:** This is the pass with the
  most direct operational risk to the platform's existing, working
  Kubernetes-audit-event ingestion path — requires a documented, gradual
  cutover (both old and new credentials accepted during a transition
  window, per ADR-0005's "gradual retirement," never an instantaneous
  cutover that could silently break real telemetry delivery) and explicit
  verification against a real submitter before the old token is disabled.
- **Rollback considerations:** Re-enable the old shared `API_TOKEN` path
  (kept dormant, not deleted, until this pass is fully verified in
  production) if the new per-credential scheme has an issue.
- **Recommended atomic commit boundary:** One PR for the multi-credential
  mechanism itself; a separate, later change (not a code PR — an
  operational cutover step) to actually retire the old shared token once
  every integration has migrated.

## Pass 20 — Security and abuse-case testing

- **Goal:** Execute `security-acceptance-gate.md`'s full checklist end to
  end as a dedicated hardening/verification pass — the final gate before
  this platform can be described as having real user authentication and
  authorization, not merely as having landed the code for it.
- **Prerequisites:** All of Passes 1–19.
- **Layers / files likely affected:** A dedicated abuse-case test suite
  (backend integration tests and frontend/BFF e2e tests) exercising IDOR,
  horizontal and vertical privilege escalation, forged-header, revoked-
  session, expired-session, and CSRF-bypass scenarios explicitly, as named
  in `security-acceptance-gate.md`; a retrieval-latency measurement run
  against `GET /v1/alerts` and `GET /v1/alerts/{id}` re-verifying NFR-002/
  AC-021 under the new request path (`identity-and-session-architecture.md`,
  "Retrieval-latency impact on NFR-002 / AC-021"; `security-acceptance-gate.md`
  gate item P10).
- **Security acceptance criteria:** Every item in
  `security-acceptance-gate.md`'s Design, Implementation, and
  Production-Readiness tiers is checked and passes, including the
  NFR-002/AC-021 retrieval-latency re-verification — no claim of continued
  compliance with either requirement is made before that measurement
  exists.
- **Required unit tests:** Already required cumulatively by Passes 1–19;
  this pass does not add new unit-level surface, it verifies the existing
  suite's completeness against the gate's checklist.
- **Required integration tests:** As above, verified for completeness
  against the gate.
- **Required e2e / abuse-case tests:** The specific named abuse cases in
  the gate document — this pass's primary deliverable.
- **Explicit non-goals:** No new product feature; this pass adds no
  capability, only verification.
- **Migration / compatibility considerations:** None — a verification pass
  changes no production behavior by itself (findings from it may spawn
  follow-up fix passes, which are their own small, atomic changes, not
  folded into this one).
- **Rollback considerations:** Not applicable — this pass is tests, not
  behavior.
- **Recommended atomic commit boundary:** One PR per abuse-case category
  (IDOR, escalation, forged-header, session-lifecycle, CSRF), so a failing
  category can be fixed and re-reviewed independently of the others,
  followed by a final, explicit sign-off recording the gate as satisfied.
