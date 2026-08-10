# Cloud-Native Security Telemetry and Detection Platform — Authorization Model and Permission Matrix (Proposed)

| Field | Value |
| --- | --- |
| Document | CNSDP Authorization Model and Permission Matrix |
| Version | 0.1 |
| Status | **Proposed.** Describes a future target authorization model. Not approved, not implemented, not current shipped behavior. |
| Phase | Proposed — Security Foundation design phase. Not yet an approved project phase. |
| Identifier | Not assigned. Outside the closed PC-015 namespace. Referenced by path only: `docs/security/authorization-model.md`. |
| Authoritative sources | `identity-and-session-architecture.md` (this same design pass — authorization here presumes that document's session model), `../personas.md` (PD-02), `../scope.md` (PD-04), `../functional-requirements.md` (PD-05), `../non-functional-requirements.md` (PD-06, NFR-012), `../acceptance-criteria.md` (PD-07, AC-024), `../adr/0004-version-controlled-detection-definitions.md`, `internal/retrieval/*`, `internal/auth/auth.go` — inspected directly during this design pass. |
| Relationship to baseline | **Proposed only.** Does not supersede NFR-012's current "single authenticated product trust level, no RBAC" statement — see `identity-and-session-architecture.md`, "Relationship to the approved baseline," for the change-control steps adoption requires. |

> **This entire document describes a proposal, not current behavior.**

## A governance note this document must not obscure

Several actions this document was asked to model — alert acknowledgement,
status, and assignment; analyst notes; in-product management of detection
definitions and data sources; user/role management; audit-log viewing;
evidence export — **do not exist as approved product capabilities today**,
independent of who would be allowed to perform them:

- Alert disposition, status, and assignment management are explicitly
  **excluded** from v0.1 by `../scope.md` (PD-04) exclusion 2, itself
  grounded in the `PC-011` case-management non-goal. `../functional-requirements.md`'s
  excluded-candidate list item 6 restates the same exclusion.
- In-product detection-definition authoring and lifecycle management are
  explicitly excluded by PD-04 scope decision 3, and structurally
  impossible today by design: `../adr/0004-version-controlled-detection-definitions.md`
  states plainly, *"No code path exists to edit a definition through the
  product itself."*
- A second, configurable telemetry data source is not current scope: PD-04
  selects exactly one source family, and "additional telemetry source
  families" is listed under "Deferred to later releases" item 1 — there is
  no plural "data sources" concept in the approved product today.
- There is no user or account model of any kind today (NFR-012), so "manage
  users and roles" and "view audit logs" describe capabilities with no
  existing product surface to attach a permission check to.

This document nonetheless models permissions for all of these, **because an
authorization model is more useful if it is designed for where the product is
credibly headed, not only for what exists today** — but every such row in the
matrix below is marked **⚠ Proposed capability**, meaning: adopting this
authorization model does not itself create the capability. The capability's
own product-scope approval (a PD-04 amendment, and corresponding FR/AC
entries) is a **separate prerequisite decision**, not a side effect of
approving this document. Rows without that marker describe permissions over
capabilities that already exist in the approved product today.

## Role evaluation

Six roles were evaluated, per the explicit instruction not to add cosmetic
ones. Each is justified individually below; none is included merely because
it was suggested — each is checked against either an existing approved
persona (`../personas.md`) or a concrete, distinct operational need.

### Viewer (read-only)

**Included.** Maps to no single existing persona but serves a real,
distinct need: broad read visibility with zero mutation capability —
appropriate for onboarding a new analyst before granting them action
capability, for a read-only dashboard consumer, or for a stakeholder who
needs to observe the platform's evidence-based investigation workflow
without being able to alter anything. Least-privilege floor above "no
access at all."

### SOC Analyst

**Included.** Maps directly to **PER-001, Security Analyst** — the
already-approved primary persona and `UC-003`'s primary actor. This is the
platform's core operational role: investigates alerts, inspects evidence,
and (once the underlying capability is itself approved — see the governance
note above) acknowledges and annotates alerts.

### Senior Analyst / Incident Responder

**Included, but contingent.** This role's distinguishing permissions
(changing alert status, assignment) are exactly the ⚠ Proposed-capability
actions the governance note above flags — they do not exist in the approved
product today. This document includes the role anyway, narrowly justified as
the natural place to attach that differentiation **if and when** disposition
and assignment workflows are separately approved: it distinguishes "can
acknowledge and annotate" (ordinary Analyst) from "can also change
disposition and reassign" (Senior/IR), which is a real, common SOC
tiering pattern, not a cosmetic addition. Until that separate approval
exists, this role has no permissions beyond SOC Analyst in practice.

### Detection Engineer

**Included.** Maps directly to **PER-002, Detection Engineer** — the
already-approved persona for reviewing detection definitions, documented
conditions, and match reasons (`UC-002`). Its "manage detection
definitions" / "enable or disable detections" permissions are ⚠ Proposed
capabilities per the governance note: today, detection definitions are
immutable, version-controlled YAML deployed through ordinary code review and
redeploy (ADR-0004), with **no in-product edit path by design**. This
document's default permissions for Detection Engineer therefore grant only
the review capability that already exists today (FR-022), with the
management/enable-disable permissions marked contingent on a **future
ADR-0004 amendment or supersession** — not something this document, or any
authorization-layer change alone, can create.

### Platform Administrator

**Included.** Necessary for any real deployment with actual user identity:
user and role management, system configuration, and audit-log access all
require some role capable of them, and none of the existing personas
(PER-001–PER-003) is defined as an operator/administrator persona
(`../personas.md`, "Excluded stakeholders": *"Platform Operator — operability
of the platform is an engineering requirement... not a persona-driven
product workflow in v0.1"*). This document treats Platform Administrator as
an operational/engineering role introduced by this proposal itself, not a
retrofit of an existing product persona — consistent with `../personas.md`'s
own explicit exclusion.

### Machine / service identity

**Included.** Already exists today in nascent, undifferentiated form: the
single shared `API_TOKEN` used by the Kubernetes audit-webhook submitter.
This role formalizes it as its own authorization class, structurally
distinct from every human role — see `identity-and-session-architecture.md`,
"Separation between human sessions and machine/service credentials."

### Roles explicitly not added

- **No per-tenant or per-customer role.** Multi-tenancy is excluded from
  v0.1 by `PC-C-005`; see "Possible future tenant isolation," below.
- **No separate "Guest" role.** Viewer already serves the broadest
  legitimate read-only need; a narrower "Guest" would be a distinction
  without an operational difference today.
- **No dedicated "Auditor" role**, even though "view audit logs" is a
  genuinely sensitive, distinct permission. A segregation-of-duties case for
  a role that can read audit logs but hold no other permission (so the
  person reviewing administrator activity is not the administrator who could
  also act) is real and worth a future proposal — but it was not among the
  six roles this document was asked to evaluate, and adding a seventh role
  the request did not name would itself be the kind of unjustified addition
  "do not add cosmetic roles" warns against. Recorded here as a flagged
  future consideration, not added now.

## Permission matrix

Legend: **✓** allowed · **—** denied (deny-by-default; absence of ✓ always
means denied, never "unspecified") · **⚠** the row's underlying capability is
not yet an approved product feature — see "A governance note," above; the ✓
marks the *proposed* role assignment for if/when it is approved, not a grant
against something that exists today.

| Resource / action | Viewer | SOC Analyst | Senior Analyst / IR | Detection Engineer | Platform Admin | Machine / service |
| --- | --- | --- | --- | --- | --- | --- |
| View alert inventory | ✓ | ✓ | ✓ | ✓ | ✓ | — |
| View alert investigation details | ✓ | ✓ | ✓ | ✓ | ✓ | — |
| View raw event payload | — | ✓ | ✓ | ✓ | ✓ | — |
| View provenance / traceability details | ✓ | ✓ | ✓ | ✓ | ✓ | — |
| Acknowledge an alert ⚠ | — | ✓ | ✓ | — | ✓ | — |
| Change alert status ⚠ | — | — | ✓ | — | ✓ | — |
| Assign an alert ⚠ | — | — | ✓ | — | ✓ | — |
| Add analyst notes ⚠ | — | ✓ | ✓ | — | ✓ | — |
| Manage detection definitions ⚠ | — | — | — | ✓ | ✓ | — |
| Enable or disable detections ⚠ | — | — | — | ✓ | ✓ | — |
| Manage data sources ⚠ | — | — | — | — | ✓ | — |
| View system health | ✓ | ✓ | ✓ | ✓ | ✓ | — |
| Manage users and roles ⚠ | — | — | — | — | ✓ | — |
| View audit logs ⚠ | — | — | — | — | ✓ | — |
| Export sensitive evidence ⚠ | — | — | ✓ | — | ✓ | — |
| Change administrative configuration ⚠ | — | — | — | — | ✓ | — |
| Submit telemetry (existing `POST /v1/audit-events`) | — | — | — | — | — | ✓ |

Notes on specific rows:

- **View raw event payload** is deliberately denied to Viewer — see
  "Treatment of sensitive raw payloads and exports," below, for the
  rationale (potentially sensitive request/response content, e.g.
  environment-variable values, means a broad, unauthenticated-adjacent
  read-only role must not see it merely because it can see alert
  summaries). Viewer retains "View alert inventory," "View alert
  investigation details," and "View provenance / traceability details" —
  only the raw payload specifically is Analyst-tier and above.
- **View system health** is listed for completeness against the requested
  action set. The existing `GET /readyz` (`internal/diagnostics`) is
  deliberately **unauthenticated today**, by design (`internal/diagnostics/diagnostics.go`:
  *"Unauthenticated by design: NFR-012/AC-024 bind access to product data and
  functions... none of which readiness exposes"*) — this row describes a
  future, richer operational-health view that may carry more than the
  current minimal readiness signal, not a proposal to add authentication to
  today's `/readyz`.
- **Export sensitive evidence** is deliberately not granted to plain SOC
  Analyst even though "view raw event payload" is — see "Treatment of
  sensitive raw payloads and exports," below, for why viewing and exporting
  are treated as separately gated actions.
- **Machine / service identity** is granted exactly one permission —
  submission — never any read or administrative permission. See "Machine
  identity authorization," below.

## Deny-by-default

The absence of an explicit ✓ in the matrix above always means denied — there
is no "unspecified, therefore allowed" state. Any new action or resource
added to this model in the future defaults to no access for every role until
an explicit grant is added to the matrix; the matrix, not the code, is the
source of truth for what a role may do, and the code must fail closed for
anything the matrix does not enumerate.

## Least privilege

Each role's permission set above is the minimum needed to satisfy the
persona or operational need that justified its inclusion (see "Role
evaluation") — no role is granted a permission "for convenience" without a
stated reason tied to that role's actual responsibility.

## Backend enforcement

Every permission check in this model **must** be enforced at the Go
backend's authorization boundary, before any data is read or written —
never only in the frontend. This directly extends this repository's
existing, already-correct pattern: today's single boolean gate
(`internal/auth.Bearer`, called at the top of every handler in
`internal/intake` and `internal/retrieval` before any other work happens)
becomes a structured `(identity, role, resource, action) -> allow/deny`
decision, evaluated at the same point in the same request lifecycle — the
first thing a handler does, before touching the database. This repository's
own prior security review already confirmed the current implementation never
trusts a client-supplied authorization signal; this model's job is to extend
that same guarantee across a richer permission surface, not weaken it.

## Frontend visibility as convenience only

The React frontend may hide or disable UI affordances for actions the
current session's role cannot perform — this is good UX, and nothing more.
It is never a security control. A forged, replayed, or hand-crafted request
attempting a denied action must be rejected by the backend identically
whether or not the frontend would have shown the corresponding button. This
generalizes the same principle already implicit in this platform's existing
architecture — the backend, not any client, is the trust boundary — from "is
this bearer token valid" to "is this specific session, with this specific
role, allowed to perform this specific action on this specific resource."

## Authentication versus authorization

Authentication (`identity-and-session-architecture.md`) establishes **who**
is making a request — a verified human identity via OIDC, or a verified
machine identity via a service credential. Authorization (this document)
determines **what** that verified identity may then do. A valid session or
service credential is necessary but never sufficient on its own — this
restates, for the proposed future model, exactly the finding already on
record from this project's own prior security review of the current
implementation: *"Do not claim that authentication equals authorization."*
Today's platform has authentication (`NFR-012`) and, by NFR-012's own
explicit text, deliberately no differentiated authorization; this document
proposes what the latter would look like if and when it is approved.

## Role permissions versus resource ownership

Today's data model has no ownership concept at all — no alert is "owned" by
a particular analyst, and nothing is scoped per user. This model's baseline
is therefore purely role-based (RBAC), not ownership-based. **If** a future
"assigned analyst" capability is separately approved (see the governance
note), an ownership-flavored rule could be layered on top of the role
matrix — for example, "the analyst an alert is assigned to may always view
and annotate it regardless of role tier" — as an addition to, never a
replacement for, the underlying role check. This is recorded here as a
future extension point; it is not built or specified further by this
document.

## Possible future tenant isolation

`PC-C-005` and `../scope.md` (PD-04) currently exclude multi-tenancy from
v0.1, and this document introduces no tenant concept. The model is
deliberately shaped so a future tenant boundary could be added without a
redesign: every authorization decision here already takes a specific
resource identity as input, so adding "and that resource's `tenant_id` must
match the requesting session's `tenant_id`" would be an additive check
layered beside the existing role check, not a restructuring of the role
model itself. No tenant scoping is proposed, designed, or implied to be
imminent by this note — it exists only to confirm this model does not
foreclose that possibility, consistent with `PC-G-010` (responsible
extensibility).

## Machine identity authorization

A service credential is granted its own narrow, explicit permission set —
never the permission surface of any human role, and never a superset "just
in case." The existing Kubernetes audit-webhook submitter's machine identity
is granted exactly "submit telemetry" and nothing else — not alert read
access, not administrative access. A future automated integration (for
example, an approved external export mechanism, if one is ever proposed)
would receive its own distinct, narrowly-scoped machine identity — service
credentials are never shared across integrations, and never reused as a
stand-in for a human role.

## Vertical privilege escalation controls

- Role and permission changes require Platform Administrator authorization
  and are themselves audited (`audit-and-accountability-design.md`).
- No self-service role elevation exists in this model — an identity can
  never grant itself a higher role.
- A session's effective role is re-resolved from the authoritative store at
  least on every rotation event (`identity-and-session-architecture.md`),
  not cached indefinitely in a way that could outlive a demotion or
  revocation applied mid-session.

## Horizontal object-access controls

Because no per-object ownership exists today (see "Role permissions versus
resource ownership"), horizontal escalation — one analyst improperly
accessing another analyst's private data — is not yet a distinct risk at the
alert level: every alert is visible to any role holding alert-read
permission, by current product design, and `PC-011` already excludes
case-management/assignment concepts that would otherwise create
per-analyst-private data. This changes the moment any future "private note"
or "assignment" capability ships (per the governance note): a note authored
by one analyst must not be editable or deletable by another without an
explicit permission, which requires an `author_id`-based ownership check at
implementation time, not merely a role check. Recorded here as a forward
requirement for that future capability's own design, not built now.

## Treatment of sensitive raw payloads and exports

Kubernetes audit events can carry sensitive request/response content
(potentially including values from request bodies, such as environment
variables set on a Pod) — already flagged as a data-sensitivity question
delegated to threat modeling by `../non-functional-requirements.md` (PD-06):
*"privacy, masking, and sensitive-data handling for audit subjects and
request content carried in source telemetry."* This model treats two
actions as elevated-sensitivity, above ordinary alert viewing:

- **Viewing** a raw event payload requires at least an Analyst-tier role
  (not Viewer) — an unauthenticated-adjacent read-only role should not
  automatically see potentially sensitive raw request content just because
  it can see alert summaries.
- **Exporting** evidence is a **separate, more restrictive permission** from
  viewing it, granted only to Senior Analyst/IR and Platform Administrator
  in the default matrix above. Export changes the data's risk profile —
  content leaves the platform's controlled investigation surface entirely —
  so the ability to view something in the UI must not automatically imply
  the ability to remove it from the platform. Every export is an audited
  action (`audit-and-accountability-design.md`).

## Authorization failure behavior and safe error responses

A denied request returns a response with no internal detail — no role name,
no policy internals, no stack trace — extending this repository's own
already-proven pattern (`internal/retrieval`'s handlers already return a
bare `401` with an empty body on authentication failure). Two distinct
denial shapes are recommended:

- **Object-level denial**, where confirming a resource even exists would
  itself leak information to a party who should not know that (e.g. "alert
  47 exists but you may not view it" is itself a disclosure) — respond
  `404`, identical to the resource genuinely not existing, to prevent
  enumeration.
- **Action-level denial**, on a resource the requester can otherwise see but
  may not act on (e.g. a Viewer attempting to acknowledge an alert they can
  already view) — respond `403`, since the resource's existence is not in
  question and a generic "forbidden" response leaks nothing further.

Every denial, of either shape, is itself an audited event
(`audit-and-accountability-design.md`, "rejected authorization attempts").

## Endpoint-to-permission mapping

| Endpoint | Status | Required permission |
| --- | --- | --- |
| `POST /v1/audit-events` | Exists today | Submit telemetry (machine/service identity only) |
| `GET /v1/alerts` | Exists today | View alert inventory |
| `GET /v1/alerts/{id}` | Exists today | View alert investigation details (composes raw payload, provenance/traceability). The Go backend — never the browser, frontend, or BFF — authorizes each sub-view individually against the matrix above and composes or omits the corresponding response section accordingly (see Pass 11, `implementation-roadmap.md`); the frontend only renders whatever the authorized backend response actually contains. |
| `GET /readyz` | Exists today, intentionally unauthenticated | None (see "Notes on specific rows," above) |
| `GET /v1/data-sources` | Exists today | View data sources summary — read-only, bearer-token authenticated (same mechanism as `GET /v1/alerts`); a retrospective ingestion-channel summary (count and latest-timestamp facts only), not a management or configuration capability. Distinct from `POST /v1/data-sources`, below, which is a separate, still-proposed capability. |
| `POST /v1/alerts/{id}/acknowledge` ⚠ | Proposed, not implemented | Acknowledge an alert |
| `PATCH /v1/alerts/{id}/status` ⚠ | Proposed, not implemented | Change alert status |
| `POST /v1/alerts/{id}/assignment` ⚠ | Proposed, not implemented | Assign an alert |
| `POST /v1/alerts/{id}/notes` ⚠ | Proposed, not implemented | Add analyst notes |
| `GET /v1/alerts/{id}/export` ⚠ | Proposed, not implemented | Export sensitive evidence |
| `PUT /v1/detections/{id}` ⚠ | Proposed, not implemented; requires an ADR-0004 amendment before it can exist at all | Manage detection definitions |
| `PATCH /v1/detections/{id}/enabled` ⚠ | Proposed, not implemented; same ADR-0004 dependency | Enable or disable detections |
| `POST /v1/data-sources` ⚠ | Proposed, not implemented; requires a PD-04 scope amendment before it can exist at all | Manage data sources |
| `GET /v1/admin/users` ⚠ | Proposed, not implemented | Manage users and roles |
| `GET /v1/admin/audit-logs` ⚠ | Proposed, not implemented | View audit logs |
| `PATCH /v1/admin/config` ⚠ | Proposed, not implemented | Change administrative configuration |

Every endpoint above — existing or proposed — is expected to perform exactly
one authorization check as the first action in its handler, following the
existing `internal/retrieval` handlers' pattern of checking authentication
before any other work: resolve the caller's identity from the
BFF-asserted session reference (or service credential), independently
resolve that identity's current role from the backend's own authoritative
role/permission data (`identity-and-session-architecture.md`, "Component
roles" — the session carries identity only, never an authoritative role
claim), look up the required permission for the endpoint and method from
this matrix, and reject before touching the database if the check fails.
No endpoint is expected to perform its own bespoke authorization logic
outside this shared model.
