# Cloud-Native Security Telemetry and Detection Platform — Audit and Accountability Design (Proposed)

| Field | Value |
| --- | --- |
| Document | CNSDP Audit and Accountability Design |
| Version | 0.1 |
| Status | **Proposed.** Describes a future target design. Not approved, not implemented, not current shipped behavior. |
| Phase | Proposed — Security Foundation design phase. Not yet an approved project phase. |
| Identifier | Not assigned. Outside the closed PC-015 namespace. Referenced by path only: `docs/security/audit-and-accountability-design.md`. |
| Authoritative sources | `identity-and-session-architecture.md` and `authorization-model.md` (this same design pass), `../non-functional-requirements.md` (PD-06, NFR-006, NFR-015, NFR-019, NFR-021, NFR-022, NFR-032, NFR-035, NFR-036), `internal/diagnostics/diagnostics.go`, `internal/db/db.go`, `internal/traceability` (as an architectural precedent), `docker-compose.yml` — inspected directly during this design pass. |
| Relationship to baseline | **Proposed only.** Adds a new, persisted security-audit capability alongside the already-approved diagnostics behavior (NFR-019, NFR-021, NFR-022) — it does not replace, contradict, or reinterpret that approved behavior. See "Relationship to existing diagnostics," below. |

> **This entire document describes a proposal, not current behavior.**

## Relationship to existing diagnostics

This repository already has an approved, implemented diagnostics capability
(`internal/diagnostics`, module 9 per `../architecture.md` §2): `LogAccessDenied`
emits a structured `slog` warning (method, path, and outcome family) for
every denied-access attempt, satisfying NFR-019 ("security-relevant events...
observable in its diagnostic information") and NFR-021/NFR-022 (structured,
distinctly-reported outcome families). This is confirmed by direct inspection
during this design pass, not assumed.

That existing mechanism is **ephemeral, process-local structured logging** —
it has no persisted table, no append-only guarantee, no tamper-evidence, and
no defined retention beyond wherever the operator sends process logs. This
document proposes a **distinct, additional** capability: a persisted,
append-only, tamper-evident **security audit trail**, purpose-built for
accountability rather than operational troubleshooting. The two serve
different purposes and are not proposed as a replacement for one another:

| | Existing diagnostics (`internal/diagnostics`, approved) | Proposed security audit trail (this document) |
| --- | --- | --- |
| Purpose | Operational visibility, troubleshooting | Accountability record for security-relevant actions |
| Persistence | Ephemeral (process logs) | Persisted (PostgreSQL, append-only) |
| Guarantees | Structured, correlatable (NFR-021) | Structured, correlatable, **and** tamper-evident, **and** access-controlled itself |
| Retention | Operator-defined, out of scope for NFR-019/021 | A defined retention model (see below) |
| Scope | Every request's basic outcome | A defined catalog of security-relevant actions (below) |

A future implementation should share the same `request_id` between the two
(see "Correlation with application logs and telemetry," below) rather than
building a second, disconnected identifier space.

## Event catalog

Every event category below is required. Categories marked ⚠ describe events
for actions that are themselves ⚠ Proposed capabilities per
`authorization-model.md`'s governance note — the audit *event type* is
specified now so the schema is ready, but the event cannot actually occur
until its underlying capability is separately approved and built.

| Category | Events |
| --- | --- |
| Session lifecycle | Login success, login failure, logout, session creation, session renewal/rotation, session revocation, session expiration |
| Authorization | Rejected authorization attempts (both object-level and action-level denials, per `authorization-model.md`) |
| Identity and access administration | Role or permission changes |
| Alert workflow ⚠ | Alert acknowledgement, alert assignment, alert status changes, analyst notes added |
| Detection content ⚠ | Detection definition creation/modification, detection enable/disable |
| Data source configuration ⚠ | Data-source configuration changes |
| Sensitive data access | Sensitive raw-event access, provenance access where sensitive, evidence export |
| Administrative | Administrative configuration changes |
| Machine identity | Service identity use (e.g. each authenticated telemetry submission's identity, distinct from the submission's own product-level record) |
| Secrets | Secret or credential lifecycle operations (issuance, rotation, revocation of service credentials and, if `identity-and-session-architecture.md` Option B is ever adopted, provider refresh tokens) |

## Required fields

| Field | Type | Required | Description | Sensitivity note |
| --- | --- | --- | --- | --- |
| `audit_event_id` | UUID | Always | Unique identifier for this audit row. | — |
| `occurred_at` | Timestamp, server-generated | Always | When the event occurred, per the audit-writing server's own clock. Never client-supplied. | — |
| `actor_id` | Nullable identifier | When resolved | The authenticated identity that performed the action — a session's `user_subject` or a service identity's own identifier. Null for events with no resolved identity yet (e.g. a failed login before any identity is established). | Not itself sensitive, but the join key to a real person — access-controlled per "Access controls for audit records," below. |
| `attempted_identifier` | String, nullable | For failed-login-shaped events only | The identifier the caller attempted to authenticate as (e.g. an email supplied to the IdP), recorded distinctly from `actor_id` since no verified identity exists yet. | Never a credential value — see "Explicitly prohibited fields." |
| `actor_type` | Enum: `human` \| `service` \| `system` | Always | Distinguishes a human session, a machine/service credential, and the platform's own internal actions (e.g. automated session expiration), per `identity-and-session-architecture.md`'s human/machine separation. | — |
| `effective_roles` | Snapshot (array/JSON) | For human actor events | The role(s) the actor held **at the time of the action**, copied at write time — never a live re-lookup against the current role table, since roles can change after the fact and the audit record must reflect what was actually true when the action occurred. | — |
| `session_id_ref` | Hash reference, nullable | For session-bound events | A reference to the session, hashed the same way `identity-and-session-architecture.md` hashes session identifiers for storage — never the raw session identifier. | Never the raw session id. |
| `request_id` | String | Always | Correlates to the same request-scoped identifier the existing structured diagnostics (NFR-021) already use. | — |
| `action` | Enum, fixed set matching the event catalog | Always | What happened, from a closed vocabulary — never a free-text description that could vary between call sites. | — |
| `resource_type` | String | When applicable | The type of resource acted on (e.g. `alert`, `session`, `detection_definition`, `user`). | — |
| `resource_id` | String, nullable | When applicable | The specific resource's identifier. | — |
| `result` | Enum: `success` \| `denied` \| `error` | Always | The outcome, using the same three-family discipline this repository already applies elsewhere (NFR-022) — never conflated. | — |
| `reason_code` | Enum, fixed set | Always for `denied`/`error` | A stable, closed-vocabulary reason (e.g. `invalid_credentials`, `session_expired`, `insufficient_permission`) — never a raw error string or exception message, which could leak internal detail. | See "How rejected actions are audited safely," below. |
| `source_ip` | String | Always | The client IP address, derived per the trust rule below. | Access-controlled; potentially personal data depending on deployment jurisdiction. |
| `trusted_proxy_derivation` | Enum: `trusted_proxy_header` \| `direct_socket_peer` | Always alongside `source_ip` | Explicitly records **how** `source_ip` was derived, so a value taken from a forwarded header (only trusted when it demonstrably came through the one known proxy hop, per `identity-and-session-architecture.md`'s "Trusted proxy and forwarded-header handling") can never be silently indistinguishable from ground truth. | — |
| `user_agent` | String, nullable | Where meaningful | Omitted or ignored for machine-identity events where it carries no useful meaning. | — |
| `previous_value` / `new_value` | Minimal diff/summary (JSON) | For change events (role changes, detection enable/disable, config changes) | A **minimal** summary of what changed — e.g. `{"role": "Viewer -> SOC Analyst"}` — never a full object dump. These fields must never contain a raw session identifier, a session cookie value, a provider access token, a provider refresh token, a CSRF secret, or any authorization credential, regardless of which change event produced them — see "Explicitly prohibited fields," which applies to these fields exactly as it does to every other field in this table. They may contain safe identifiers (e.g. a role name, a detection-definition name), redacted or masked values, classification labels, hashes where hashing is specifically permitted elsewhere in this document (e.g. `session_id_ref`'s hash), or references sufficient for investigation without themselves enabling credential replay. | Must not itself become a secret-disclosure vector. |
| `security_classification` | Enum: `routine` \| `sensitive` \| `high_sensitivity` | Always | Drives retention and access-control tier for the record itself (e.g. an evidence export or a role change is `high_sensitivity`; an ordinary alert view is `routine`). | — |
| `integrity_metadata` | Hash chain value | Always | This row's tamper-evidence value — see "Tamper detection," below. | — |

## Explicitly prohibited fields

The audit trail must never store:

- **Passwords.** This platform never handles a user password directly — the
  IdP does (`identity-and-session-architecture.md`) — so none should ever
  reach this system in the first place; this is stated as an explicit
  invariant regardless.
- **Access tokens.** Never persisted, per the same document's "What
  information the browser receives" and the recommended Option A (discard
  provider tokens after claim extraction).
- **Refresh tokens.** If Option B is ever adopted, the encrypted token lives
  in the session store described there — never in the audit trail, even
  encrypted.
- **Complete session secrets.** Only the hashed `session_id_ref`, never the
  raw session identifier.
- **Raw secrets of any kind** (service credentials, encryption keys, IdP
  client secrets).
- **Unnecessarily complete sensitive payloads.** A "sensitive raw-event
  access" audit row records *that* an access happened (actor, resource
  identifier, timestamp) — never a copy of the accessed payload itself. This
  extends the already-approved principle that diagnostics must not "copy
  sensitive source-telemetry content beyond what its diagnostic purpose
  requires" (NFR-015) to this new persisted store.

This prohibition list applies without exception to `previous_value` and
`new_value` (see "Required fields," above) — a change-event diff is not a
carve-out from these rules. In particular, `previous_value`/`new_value`
must never contain a raw session identifier, a session cookie, a provider
access or refresh token, a CSRF secret, or any authorization credential,
even when the change being recorded is itself session- or
credential-related (e.g. a forced session revocation, a service-credential
rotation).

## Append-only enforcement

Two independent layers, matching this repository's existing preference for
structural guarantees over policy conventions (the same reasoning
`../adr/0004-version-controlled-detection-definitions.md` gives for why
detection definitions have no in-product edit path: *"Structurally excludes
in-product authoring by construction... without relying on a policy
convention that could be bypassed later"*):

1. **Application-level:** the audit-writer module exposes only an insert
   operation — no update or delete code path exists in it at all, mirroring
   ADR-0004's own pattern for a different immutability guarantee.
2. **Database-level (defense-in-depth):** a dedicated, least-privilege
   PostgreSQL role used only by the audit-writer, granted `INSERT` and
   `SELECT` on the audit table and explicitly **not** granted `UPDATE`,
   `DELETE`, or `TRUNCATE`. Even a fully compromised application process
   using that role cannot alter history at the SQL layer. Any future
   legitimate need to redact a record (e.g. a legal hold) would require a
   separate, out-of-band procedure under different credentials — itself an
   audited action.

## Database permissions

Today's reference environment (`docker-compose.yml`, `internal/db/db.go`)
uses a single PostgreSQL user (`cnsdp`) for the entire application — a
reasonable simplification for a single-host reference environment, confirmed
by direct inspection, not a role-separation scheme. This proposal recommends
a production role split, extending the already-approved least-privilege
principle (NFR-014) concretely to the audit store:

- One application role for ordinary module read/write, scoped per this
  repository's existing module-boundary discipline (`../architecture.md`
  §2).
- One separate, narrower audit-writer role, as described above.
- One separate migration/administration role, used only at deploy/migration
  time, never held by the request-serving process.

## Retention

This document does not invent a specific numeric retention period. Following
this repository's own existing pattern for NFR-032 (deployment-lifetime
retention for the minimum evidence set, with "no automatic deletion,
archival, or retention-policy management... required in v0.1"), the
recommended default is **deployment-lifetime retention** for audit records —
they are not automatically deleted — unless and until a specific,
documented driver (e.g. an eventual compliance requirement) justifies a
different number. `../non-functional-requirements.md`'s own excluded-candidate
list already notes "Compliance frameworks and certifications" have "no
charter basis" today, so no compliance-driven retention number is invented
here either.

## Archival

No automatic archival or deletion machinery is proposed, for the same reason
retention is left at deployment-lifetime by default: NFR-032 already
establishes this pattern for the existing minimum evidence set, and this
document extends it rather than inventing a second, inconsistent lifecycle
model. If storage growth for the audit trail specifically becomes a concern,
it should be governed by the same already-approved bounded-resource-
consumption and visible-resource-exhaustion-behavior requirements (NFR-035,
NFR-036) this platform already applies elsewhere, not a new mechanism
invented for this store alone.

## Tamper detection

**Recommended mechanism: an append-only hash chain.** Each audit row's
`integrity_metadata` is computed from a hash of that row's own content
concatenated with the previous row's `integrity_metadata` value. Altering or
deleting any historical row breaks the chain verifiably from that point
forward; a periodic verifier job can walk the chain and detect the break.

This is not a new idea introduced for this document alone — it is a direct
structural application of a pattern this same codebase **already implements
and has already proven** for a different integrity guarantee: the
traceability-chain verifier (`internal/traceability`, satisfying NFR-029 and
AC-016) already exists to "fail visibly and identify the specific affected
link" when a chain of linked records is broken, rather than silently
representing a broken chain as intact. This document proposes reusing that
exact architectural pattern — a verifiable chain that fails loudly and
specifically — for audit-record integrity, not inventing a new one.

A periodic external anchoring step (publishing a rolling digest of the chain
to a separate, harder-to-tamper store) is noted as a possible future
hardening measure, not required for an initial implementation.

## Clock and timestamp expectations

`occurred_at` is always server-generated — a client-supplied timestamp is
never trusted for this field. Monotonic ordering within a single node is
sufficient; this document does not introduce a distributed clock-
synchronization requirement, consistent with `../non-functional-requirements.md`'s
own excluded-candidate list ("A numeric clock-accuracy or
timestamp-synchronization requirement — no charter basis"). An NTP-synced
host clock is assumed as ordinary operational hygiene, not a new formal
requirement this document adds.

## Access controls for audit records

Reading the audit trail is itself a sensitive, permission-gated action —
"view audit logs" in `authorization-model.md`'s matrix, granted only to
Platform Administrator by default, per least privilege. As a recommended
enhancement (not required for an initial implementation): a Platform
Administrator's own audit-log reads should themselves be logged, supporting
a future segregation-of-duties review of who has been looking at the
accountability record itself.

## Failure behavior when audit persistence fails

A genuine design tension: should the underlying action be blocked if its
audit record cannot be written? This document proposes a policy covering
all three `security_classification` values defined above — `routine`,
`sensitive`, and `high_sensitivity` — not only the two extremes. **This is a
proposed policy only; it is not implemented, and no code path currently
enforces it.**

- **`routine`** — the underlying business operation **may continue** during
  a temporary audit-persistence failure, but the failure itself must never
  be silent: it must trigger a bounded retry, mark the platform's own
  health/diagnostic state as degraded for the duration, emit a metric, and
  raise an operational alert. This extends NFR-036's "visible behavior at
  resource exhaustion" principle (the condition must be observable, never
  silent) and this platform's existing three-outcome-family discipline
  (NFR-022): "the audit trail failed to record this" must itself be a
  distinctly reported condition, never quietly absorbed into "the action
  succeeded, nothing further to report."
- **`sensitive`** — **fail closed** for every mutation, privilege change,
  session or other security-state change, evidence export, raw-payload
  disclosure, or other action that discloses protected data. An ordinary,
  read-only operation classified `sensitive` that discloses no protected
  data (for example, a read that touches sensitive-adjacent metadata
  without revealing its content) may continue, but **only** in an
  explicitly defined, alerted degraded mode equivalent to the `routine`
  behavior above — it must never silently drop the audit record as if the
  failure had not occurred. The default for any `sensitive` event whose
  read/write or disclosure character is not yet classified more precisely
  is fail-closed; a narrower fail-open carve-out must be justified
  explicitly per action, not assumed.
- **`high_sensitivity`** — **fail closed**, unconditionally. Block the
  action if its audit write fails. An unaudited high-sensitivity action is
  unacceptable for a security platform, directly extending this
  repository's own already-approved principle that the platform "shall not
  silently lose or silently corrupt any recorded product artifact"
  (NFR-006) to this new class of artifact.

**A caller must never be able to downgrade an event's own classification.**
`security_classification` is assigned by the audit-writer module itself,
per the fixed `action` enum (`audit-and-accountability-design.md`'s event
catalog), not supplied or overridable by the code path emitting the event —
a call site cannot mark a role change or an evidence export `routine` to
obtain fail-open behavior. This is an integrity property of the audit
module's own design, not a convention calling code is trusted to honor.

## Correlation with application logs and telemetry

`request_id` ties an audit row back to the same request-scoped structured
diagnostic log entry this platform's already-approved NFR-021 requires, so
an investigator can pivot from "this security-relevant action happened" to
"here is everything else recorded about that same request" without a second,
disconnected logging system to reconcile by hand.

## How rejected actions are audited safely without leaking secrets

A rejected action — a failed login, a denied authorization attempt — is
itself audited (it appears explicitly in the event catalog above), but the
audit row records only the closed-vocabulary `reason_code` (e.g.
`invalid_credentials`, `session_expired`, `insufficient_permission`), never
the attempted secret value itself. This directly extends the discipline
already proven in this repository's recent bearer-token-removal work — the
same "never let a credential-shaped value reach a persisted or logged
surface" principle verified there by a dedicated build-leakage scan
(`frontend/src/test/noLeakedSecretsInBuild.test.ts`) — applied here
specifically to failed authentication attempts: a failed login must never
result in an attempted password, token, or credential value landing in the
audit trail, even as forensic evidence of the attempt.

## Explicit non-goals of this document

- This document does not select a concrete database schema, table
  structure, or ORM — those remain implementation choices for a future
  approved ADR.
- This document does not define the session or role model itself — see
  `identity-and-session-architecture.md` and `authorization-model.md`.
- This document does not propose a SIEM integration, external log shipping,
  or a dedicated security-monitoring product — consistent with `PC-011`'s
  non-goals, this remains an in-platform accountability record, not a new
  product category.
