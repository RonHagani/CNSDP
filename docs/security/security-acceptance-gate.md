# Cloud-Native Security Telemetry and Detection Platform — Security Acceptance Gate (Proposed)

| Field | Value |
| --- | --- |
| Document | CNSDP Security Acceptance Gate |
| Version | 0.1 |
| Status | **Proposed.** Defines a future mandatory gate for a design that is itself not yet approved. Nothing below has been executed or verified against running code. |
| Phase | Proposed — Security Foundation design phase. Not yet an approved project phase. |
| Identifier | Not assigned. Outside the closed PC-015 namespace. Referenced by path only: `docs/security/security-acceptance-gate.md`. |
| Authoritative sources | `identity-and-session-architecture.md`, `authorization-model.md`, `audit-and-accountability-design.md`, `implementation-roadmap.md`, `open-decisions.md`, `../adr/0005-external-oidc-identity-and-session-architecture.md` (this design phase). |

> **This document defines a future gate. It does not itself certify
> anything.** Until every item below is checked against real, running code
> and explicitly signed off, **this platform must not be described as
> having real user authentication or authorization** — only the single
> shared bearer token `../architecture.md` §6 already documents.

## Purpose

This is the mandatory checklist the platform must satisfy before any claim
of "real user authentication and authorization" is made — internally, in
documentation, or externally. It is organized into three tiers, each with
its own distinct pass/fail meaning. **Passing an earlier tier is a
prerequisite for attempting the next; none may be skipped or assumed.**

## Tier 1 — Design acceptance

Verifies the *proposal itself* is sound and formally approved — no code is
evaluated at this tier. This tier corresponds to
`implementation-roadmap.md` Pass 1 and must be satisfied before Pass 2 (or
any later pass) may begin.

| # | Requirement | Verification |
| --- | --- | --- |
| D1 | `../adr/0005-external-oidc-identity-and-session-architecture.md` Status flipped from Proposed to Accepted through explicit review. | Recorded approval (e.g. merged PR with reviewer sign-off). |
| D2 | `../architecture.md` §6 **and §7** amended to reflect the accepted decision, no longer contradicting it. §7 ("Docker Compose, exactly two services") requires amendment because Pass 2's BFF, and any local-development identity provider (`open-decisions.md` decision 1), each add a service beyond that committed count. | Diff review against the current approved text, for both §6 and §7. |
| D3 | `../non-functional-requirements.md` NFR-012 (and its "Excluded and deferred candidates" item 5), NFR-033, and NFR-034 amended or superseded; `../acceptance-criteria.md` AC-024 amended with new session/CSRF/authorization criteria, and AC-028 revalidated against the new topology. | Diff review; NFR-033/NFR-034/AC-028 revalidation evidence, not merely a text diff, since these are testable requirements about the reference environment. |
| D4 | Confirmation recorded that adoption introduces no `PC-011` non-goal and no `PC-C-005` multi-tenancy. | Written confirmation in the approval record. |
| D5 | `identity-and-session-architecture.md`, `authorization-model.md`, and `audit-and-accountability-design.md` reviewed and approved as the governing design (their own Status fields may then move from Proposed toward whatever this repository's convention designates for accepted design documents). | Recorded approval. |
| D6 | The Threat Model (Document A of this design phase, still pending as of this document's writing) completed and reviewed, with particular attention to decision 12 in `open-decisions.md` (sensitive raw-payload masking) and decision 13 (evidence export). | Threat model document exists and is reviewed. |
| D7 | Every "High" and "Medium-High" priority item in `open-decisions.md`'s priority summary has an explicit owner and, at minimum, a triage decision recorded (resolved, deliberately deferred with rationale, or explicitly accepted as a known gap) — not left silently unaddressed. | Review of `open-decisions.md` against its own priority table. |

## Tier 2 — Implementation acceptance

Verifies the *code* correctly implements the accepted design. Each item
names the specific, testable property and the kind of evidence required —
"tests exist" is not sufficient; the tests must pass against real,
integration-level infrastructure (this platform's existing testcontainers-
backed PostgreSQL pattern, `internal/testutil.MigratedPostgres`, or
equivalent), not only mocks.

| # | Requirement | Verification |
| --- | --- | --- |
| I1 | **No browser-readable credential.** No ID token, access token, refresh token, or session secret appears in `import.meta.env`, `localStorage`, `sessionStorage`, `IndexedDB`, or any non-`HttpOnly` cookie. | Static source scan (extending `frontend/src/test/noClientCredentials.test.ts`'s pattern) plus a real-build scan (extending `frontend/src/test/noLeakedSecretsInBuild.test.ts`'s pattern) for JWT-shaped strings, run against the actual production build output. |
| I2 | **No shared service credential acting as human identity.** The machine/service credential path (`implementation-roadmap.md` Pass 19) and the human session path are structurally distinct; no code path accepts a service credential as proof of a specific human identity. | Integration test: a service credential presented where a human session is expected is rejected; a human session cannot be used to perform a machine-only action (submission). |
| I3 | **Backend authorization on every protected endpoint.** Every endpoint in `authorization-model.md`'s endpoint-to-permission mapping that has been implemented performs its permission check as the first action in its handler. | Code review against the mapping table, plus I10 (authorization integration tests) covering every implemented endpoint. |
| I4 | **Deny-by-default behavior.** An action or resource with no matching rule is denied, never allowed by omission. | Unit test: an unrecognized `(resource, action)` pair is denied; integration test: a deliberately misconfigured rule set fails closed, per `implementation-roadmap.md` Pass 8's own acceptance criteria. |
| I5 | **Session fixation prevention.** A pre-authentication identifier is never promoted in place into an authenticated session — login always issues a genuinely new identifier. | Integration test: a session identifier obtained before login is confirmed invalid for use as, or reuse toward, the post-login session. |
| I6 | **Session rotation.** Rotation occurs on login, on privilege/role change, and periodically during a long-lived session, per `identity-and-session-architecture.md`. Test design must reflect whatever `open-decisions.md` decision 18 (concurrent sessions per human identity) resolves — rotation's interaction with a user's *other* concurrent sessions, if any are allowed, is part of this item, not a separate one. | Integration tests for each of the three rotation triggers, confirming the prior identifier is invalidated. |
| I7 | **Session revocation.** Logout and administrator-forced revocation take effect on the very next request, with no propagation delay or caching window. | Integration test: a session revoked mid-test is rejected on the immediately following request. |
| I8 | **Idle and absolute expiry.** Both are enforced by re-checking the persisted session row on every request. | Integration tests at the idle-timeout and absolute-timeout boundaries (just inside and just outside each window). |
| I9 | **CSRF protection.** Every state-changing (`POST`/`PUT`/`PATCH`/`DELETE`) request is rejected without a valid, session-bound CSRF token. | Integration test simulating a cross-site request with a valid session cookie but no/invalid CSRF token — rejected. |
| I10 | **Authorization integration tests.** Every implemented endpoint is exercised against every role in `authorization-model.md`'s matrix, confirming the response matches the matrix exactly (allow or the correct denial shape). | A dedicated authorization-matrix test suite, one case per (endpoint, role) pair for every implemented endpoint. |
| I11 | **Horizontal privilege-escalation tests.** Once any per-object ownership concept exists (e.g. analyst notes, assignment — see `authorization-model.md`, "Horizontal object-access controls"), one identity cannot access or modify another identity's private object without explicit permission. | Integration test once the relevant capability is implemented; not applicable (and explicitly not required) before such a capability exists. |
| I12 | **Vertical privilege-escalation tests.** No identity can grant itself a higher role; a role change takes effect (including forcing session rotation) rather than being bypassable via a cached or stale session. | Integration test: an attempted self-role-elevation request is rejected; a demoted identity's existing session is confirmed to lose the higher privilege promptly. |
| I13 | **IDOR tests.** Object-level denial returns `404` (never confirming existence to an unauthorized party), per `authorization-model.md`'s "Authorization failure behavior." | Integration test: requesting a resource ID the caller may not view returns a response indistinguishable from that ID not existing. |
| I14 | **Revoked-session tests.** A revoked session is rejected, and the client-visible response clears the cookie rather than leaving a stale one. | Integration/e2e test, extending the "Revoked or expired session" sequence in `identity-and-session-architecture.md`. |
| I15 | **Expired-session tests.** Both idle-expired and absolute-expired sessions are rejected identically to revoked ones. | Integration test, covering both expiry types distinctly. |
| I16 | **Forged-header tests.** A forged or attacker-supplied `Authorization`, forwarded-identity, or `X-Forwarded-For`-family header is never trusted unless it demonstrably arrived through the one trusted proxy hop. | Integration/e2e test extending this platform's own already-proven pattern (`frontend/e2e/real-backend.spec.ts`'s forged-`Authorization`-header test) to the new identity-header boundary, per `implementation-roadmap.md` Pass 15. |
| I17 | **Append-only security auditability.** The audit-writer database role cannot `UPDATE`, `DELETE`, or `TRUNCATE` the audit table; the hash chain (`audit-and-accountability-design.md`) verifies across a representative sample, and a deliberately tampered row is detected. | Integration test attempting a direct `UPDATE`/`DELETE` using the restricted role (must fail at the database layer); a chain-verification test with an injected tampered row. |
| I18 | **Secret-leak scanning.** No provider client secret, database credential, service credential, or (if ever retained) provider refresh token appears in source, logs, diagnostics, the audit trail, or build output. | Extends the existing sentinel-based build-leak-scan technique (`frontend/src/test/noLeakedSecretsInBuild.test.ts`) to the full new secret set; a log/audit-content scan for credential-shaped values. |
| I19 | **Secure-cookie verification.** The session cookie carries exactly the attributes `identity-and-session-architecture.md` specifies (`HttpOnly`, `Secure` outside local dev, `SameSite=Lax`, correct `Path`, expiration mirroring the session's absolute timeout). | Integration/e2e test inspecting the real `Set-Cookie` header on login and on rotation. |
| I20 | **Trusted-proxy tests.** Covered by I16 above; listed separately here because it is its own named requirement — confirms the backend's trust decision is correct for both the trusted-hop and untrusted-origin cases. | Same evidence as I16, explicitly including the "arrived directly, bypassing the proxy" negative case. |
| I21 | **Safe authentication and authorization error behavior.** No denial response of any kind (`401`, `403`, `404`) includes internal detail — no role name, no policy internals, no stack trace, no database error text. | Response-body inspection across every denial-path test above, confirming this platform's existing safe-failure-message discipline (`internal/retrieval`'s current handlers) is preserved for every new denial path. |

## Tier 3 — Production-readiness acceptance

Verifies the *deployment*, not just the code, is ready to carry real
traffic. This tier corresponds to `implementation-roadmap.md` Passes 16–20.

| # | Requirement | Verification |
| --- | --- | --- |
| P1 | **Production TLS boundary documented and live.** Every production request path terminates or passes through TLS; plaintext is redirected or refused. | `implementation-roadmap.md` Pass 16's own integration/e2e checks (plaintext redirect/refusal, valid certificate chain), plus a documented description of the boundary (which component terminates TLS) in the deployment documentation. |
| P2 | **Rate-limiting expectations documented and live.** Login-endpoint throttling and general API rate limiting are implemented and their thresholds documented, even though this design phase deliberately does not fix the exact numeric values (`implementation-roadmap.md` Pass 13). | Integration test confirming throttling activates past the documented threshold; documentation listing the chosen values and their rationale. |
| P3 | **No sensitive token or raw-secret logging**, verified against real production-shaped logs and the audit trail, not only source code. | A log/audit-content review of an actual staging or production-like run, not only a source-code scan (I18 covers source/build; this covers runtime output). |
| P4 | **Secret management in place**, not the local `.env`-file pattern (`implementation-roadmap.md` Pass 17). | Confirmation that production secrets are sourced from the chosen production secret-management mechanism, with no fallback to a committed or environment-inlined value. |
| P5 | **Security headers live**, including `Strict-Transport-Security` (dependent on P1), verified against the real deployed BFF, not only a local build. | Response-header inspection against the production or production-equivalent deployment (`implementation-roadmap.md` Pass 14). |
| P6 | **Failure-mode behavior verified**, per `audit-and-accountability-design.md`'s "Failure behavior when audit persistence fails": high-sensitivity actions fail closed on audit-write failure; routine actions fail open but loudly, never silently. | Induced-failure integration test against a real (or realistically simulated) audit-write failure. |
| P7 | **IdP-outage behavior verified.** Already-authenticated sessions remain usable during a simulated IdP outage; new logins fail with a safe, generic message. | Integration test simulating IdP unreachability. |
| P8 | **Emergency/break-glass and platform-wide revocation procedures documented and exercised at least once**, per `open-decisions.md` decisions 10 and 16. | A recorded dry run of the break-glass procedure and the platform-wide revocation capability, confirming both actually work and are themselves correctly audited. |
| P9 | **Service-credential migration complete or explicitly in a documented transition state**, per `implementation-roadmap.md` Pass 19 — the old shared `API_TOKEN` is either fully retired or its continued, narrowed use is explicitly documented and time-bounded. | Review of the current credential-migration state against the documented cutover plan. |
| P10 | **NFR-002/AC-021 retrieval-latency re-verification.** `GET /v1/alerts` and `GET /v1/alerts/{id}` continue to meet the existing approved 5-second-or-less retrieval target under the new request path (BFF hop, session lookup, authorization lookup), measured under the AC-021 reference dataset and load profile — not assumed compliant by inspection alone. | A real, measured latency run against both endpoints under the documented AC-021 conditions, per `identity-and-session-architecture.md`'s "Retrieval-latency impact on NFR-002 / AC-021" and `implementation-roadmap.md` Pass 20. No claim of continued NFR-002/AC-021 compliance may be made before this measurement exists. |

## What this gate does not certify

Passing all three tiers certifies that this platform's identity, session,
and authorization implementation matches its own accepted design and does
not exhibit the specific failure modes enumerated above. It does **not**
certify:

- General penetration-test coverage beyond the named abuse cases.
- Compliance with any named regulatory framework — `../non-functional-requirements.md`
  already records "no charter basis" for compliance frameworks in this
  platform's approved scope.
- Correctness of the identity provider's own security posture — that
  remains the provider's responsibility, evaluated at selection time
  (`open-decisions.md`, decision 2).
- Any capability still marked ⚠ Proposed in `authorization-model.md` that
  has not itself been separately approved as product scope — this gate
  certifies the authorization *mechanism* is sound; it does not certify
  that every action the mechanism could someday gate is itself an approved
  feature.

## Gate outcome recording

Per `implementation-roadmap.md` Pass 20, the outcome of this gate — pass,
fail with specific findings, or partial-pass with an explicitly accepted,
time-bounded exception — must be recorded explicitly, not implied by the
mere existence of passing tests. Only after an explicit, recorded pass of
all three tiers may this platform's documentation or any external-facing
material describe it as having real user authentication and authorization.
