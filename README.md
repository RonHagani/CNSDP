# Cloud-Native Security Telemetry and Detection Platform (CNSDP)

CNSDP is a portfolio-grade, production-oriented platform demonstrating a
complete, explainable security detection workflow for Kubernetes API server
audit telemetry: intake and admission → validation → normalization →
detection → alert generation → evidence-based investigation. Every alert is
traceable back to the raw event that produced it. The goal is depth and
correctness in that one workflow, not alert volume or integration breadth.

## The problem it addresses

Security teams operating Kubernetes clusters need to turn raw, high-volume
API server audit events into a small number of alerts they can trust and
investigate. That takes more than pattern matching: telemetry must be
validated and classified rather than silently dropped, each detection must
state exactly which documented condition matched and why, and every alert
must be traceable back to the source event and detection-definition revision
that produced it. CNSDP implements that discipline for one telemetry source
family and three detection scenarios.

## What this is, and isn't

This is a **production-oriented portfolio project**: a single Go deployable
with a real PostgreSQL-backed durable worker, structured logging,
authenticated endpoints, and an end-to-end integration-tested walking
skeleton — built to demonstrate sound architecture and engineering practice,
not to compete on feature breadth. **It is not production-ready.**

It is **not** a complete SIEM, SOC platform, or commercial product. There is
no alert triage/case-management workflow, no multi-tenant support, no
in-product detection authoring, and no broad telemetry-source coverage.
Authentication today is a single shared bearer token: there is no user or
session model and no RBAC. A proposed identity, session, authorization, and
audit-logging design exists under [`docs/security/`](docs/security/) and
[ADR-0005](docs/adr/0005-external-oidc-identity-and-session-architecture.md)
— it is a design proposal only and **is not implemented**. See
[`docs/scope.md`](docs/scope.md) for the full approved list of what is out
of scope.

## Implemented end-to-end flow

```
Kubernetes audit EventList
  → authenticated intake            (POST /v1/audit-events)
  → admission                       (durably recorded submission)
  → validation                      (valid / invalid / incomplete / unsupported)
  → normalization                   (canonical event representation)
  → detection evaluation            (against version-controlled definitions)
  → alert generation                (explainable, cites matched conditions)
  → evidence and traceability       (six-artifact chain, integrity-verified)
  → authenticated retrieval         (GET /v1/alerts/{id})
```

A durable, single-worker processing loop (`internal/worker`) advances every
admitted submission through this pipeline unattended, one transaction per
stage, so a process crash at any point leaves the submission in a
determinable, safely-resumable state. This flow is implemented and locally
verified end to end — see "Current limitations" below for what it does not
yet cover.

## Quick start (Docker Compose)

Requires Docker Engine with the Compose plugin. No local Go toolchain or
PostgreSQL install is needed — the build stage compiles the application
inside a container.

```powershell
Copy-Item .env.example .env
# edit .env: set API_TOKEN and POSTGRES_PASSWORD
docker compose up -d --build
docker compose ps
```

Wait until both services show `Up (healthy)`, then confirm readiness:

```sh
curl -i http://127.0.0.1:8080/readyz
```

Full setup, environment variables, and troubleshooting:
[`docs/reference-environment.md`](docs/reference-environment.md).

## Seed alerts and open the frontend

With the stack healthy:

```powershell
./scripts/dev-seed-alerts.ps1

Copy-Item frontend/.env.example frontend/.env
# edit frontend/.env: set API_PROXY_TOKEN to match .env's API_TOKEN
cd frontend
npm install
npm run dev
```

Open `http://localhost:5173/alerts` — three real, backend-composed alerts
render within a few seconds. See [`frontend/README.md`](frontend/README.md)
for frontend-specific commands, and
[`docs/reference-environment.md`](docs/reference-environment.md) for the full
walkthrough, including inspecting a single alert's evidence via `curl`.

## HTTP endpoints

| Endpoint | Auth | Purpose |
| --- | --- | --- |
| `POST /v1/audit-events` | Bearer token | Submits a Kubernetes audit-webhook `EventList` for admission |
| `GET /v1/alerts` | Bearer token | Lists persisted alerts as ordered summaries for the frontend alert-inventory view |
| `GET /v1/alerts/{id}` | Bearer token | Retrieves an alert together with its full evidence and traceability inventory |
| `GET /v1/detections` | Bearer token | Lists the currently active detection definitions and their documented conditions |
| `GET /v1/submissions` | Bearer token | Keyset-paginated, outcome-filterable review of every admitted submission and its validation outcome, including ones that never produce an alert |
| `GET /v1/data-sources` | Bearer token | Reports a retrospective count and last-event time for the platform's one ingestion channel |
| `GET /readyz` | None | Reports readiness: database connectivity and detection-definition load status |

## Testing

```sh
go build ./...
go vet ./...
go test -count=1 ./...
go test -tags=integration -p 1 -count=1 ./...
```

Integration tests require Docker — they run against real, ephemeral
PostgreSQL instances via testcontainers. Frontend checks (`typecheck`,
`lint`, `test`, `build`, `e2e`) are documented in
[`frontend/README.md`](frontend/README.md).

## Documentation

| Document | Contents |
| --- | --- |
| [`docs/product.md`](docs/product.md) | Product charter: purpose, goals, principles, constraints |
| [`docs/architecture.md`](docs/architecture.md) | Approved Phase 1 architecture baseline, data flow, walking-skeleton definition |
| [`docs/adr/`](docs/adr/) | Architecture Decision Records — ADR-0001–0004 (Accepted), ADR-0005 (Proposed) |
| [`docs/reference-environment.md`](docs/reference-environment.md) | Docker Compose setup, environment variables, end-to-end demonstration, troubleshooting |
| [`docs/security/`](docs/security/) | Proposed identity, session, authorization, and audit-logging design — **not implemented**; see `threat-model.md` and `open-decisions.md` |
| [`docs/functional-requirements.md`](docs/functional-requirements.md), [`docs/non-functional-requirements.md`](docs/non-functional-requirements.md) | Functional and non-functional requirements |
| [`docs/acceptance-criteria.md`](docs/acceptance-criteria.md) | Acceptance criteria |
| [`docs/use-cases.md`](docs/use-cases.md), [`docs/personas.md`](docs/personas.md) | Use cases and the personas they serve |
| [`docs/scope.md`](docs/scope.md), [`docs/glossary.md`](docs/glossary.md) | Approved scope and non-goals, and terminology glossary |

## Repository structure

```
cmd/platform/       Application entry point (HTTP server + worker loop wiring)
internal/           Workflow modules and supporting packages — see docs/architecture.md
migrations/         Versioned SQL schema migrations, embedded into the binary
definitions/        Version-controlled YAML detection definitions, embedded into the binary
docs/               Product, architecture, security, and ADR documentation
frontend/           Alert-investigation UI (Vite/React) — see frontend/README.md
scripts/            Local development tooling (e.g. dev-seed-alerts.ps1)
spikes/             Pre-architecture research spikes — historical, not part of the build
```

## Current limitations

The Phase 1 walking-skeleton flow above is implemented and locally verified
end to end. A real submission passes through the running worker loop to a
retrievable, traceability-intact alert (`docs/architecture.md` §8), and the
build, `vet`, unit-test, and real-PostgreSQL integration-test suites all
pass. The following are deliberately deferred, not defects:

- No CI pipeline.
- No performance/capacity testing against the approved throughput and
  latency envelope.
- No restart/recovery demonstration at the application level — the
  underlying durable-worker crash-recovery pattern was validated separately
  (`spikes/02-postgres-durable-worker/FINDINGS.md`).
- Identity, session, and RBAC capabilities are not yet implemented; see
  "What this is, and isn't" above.

See `docs/architecture.md` §9 for the full list of deferred implementation
choices and open assumptions.
