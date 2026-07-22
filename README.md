# Cloud-Native Security Telemetry and Detection Platform (CNSDP)

CNSDP is a portfolio-grade, production-oriented platform that demonstrates a
complete, explainable security detection workflow for Kubernetes API server
audit telemetry: intake and admission → validation → normalization →
detection → alert generation → evidence-based investigation. It exists to
show that workflow working end to end, with every alert traceable back to
the raw event that produced it, rather than to maximize alert volume or
integration breadth.

## The problem it addresses

Security teams operating Kubernetes clusters need to turn raw, high-volume
API server audit events into a small number of alerts they can actually
trust and investigate. That requires more than pattern matching: telemetry
has to be validated and classified rather than silently dropped or
misread, detections have to state exactly which documented condition
matched and why, and every alert has to be traceable back to the specific
source event and detection-definition revision that produced it. CNSDP
implements that discipline — intake, validation, normalization, detection,
alerting, evidence, and traceability as first-class, independently
verifiable stages — for one telemetry source family and three detection
scenarios.

## What this is, and isn't

This is a **production-oriented portfolio project**: a single Go
deployable with a real PostgreSQL-backed durable worker, structured
logging, authenticated endpoints, and an end-to-end integration-tested
walking skeleton — built to demonstrate sound architecture and engineering
practice, not to compete on feature breadth.

It is **not** a complete SIEM, SOC platform, or commercial product. There
is no alert triage/case-management workflow, no multi-tenant support, no
in-product detection authoring, no broad telemetry-source coverage, and no
automated response. See [`docs/scope.md`](docs/scope.md) for the full,
approved list of what is explicitly out of scope.

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
determinable, safely-resumable state.

## Architecture and module boundaries

CNSDP is a single Go deployable structured as a modular monolith — no
service decomposition, no message broker (see
[ADR-0001](docs/adr/0001-modular-monolith-in-go.md)). Each workflow stage
is an internal Go package that owns its own database table(s) and exposes
only a defined interface to the rest of the system:

| Module | Package | Responsibility |
| --- | --- | --- |
| Telemetry admission | `internal/intake`, `internal/submission` | Authenticates and durably admits each `EventList` item as a submission |
| Validation and classification | `internal/validation` | Four-way outcome: valid, invalid, incomplete, unsupported |
| Normalization | `internal/normalization` | Transforms a valid submission's raw event into one canonical event |
| Detection evaluation | `internal/detection` | Loads version-controlled detection definitions and evaluates normalized events against them |
| Alert generation | `internal/alerting` | Produces exactly one explainable alert per matching detection result |
| Evidence inventory | `internal/evidence` | Composes the six-artifact evidence set for a given alert |
| Traceability | `internal/traceability` | Verifies the alert-to-source-event chain and identifies any broken link |
| Retrieval and investigation | `internal/retrieval` | Authenticated read path from alert back to source event |
| Operational diagnostics | `internal/diagnostics` | Readiness endpoint and shared denied-access logging |

Supporting, cross-cutting packages: `internal/auth` (shared bearer-token
verification), `internal/db` (connection pool and migrations),
`internal/worker` (the continuous processing loop that dispatches each
submission to its owning stage module), and `internal/testutil`
(shared ephemeral-PostgreSQL test bootstrap).

PostgreSQL is the platform's sole persistence store
([ADR-0002](docs/adr/0002-postgresql-persistence-and-durable-worker.md)).
Intake is a Kubernetes audit-webhook-compatible HTTP endpoint
([ADR-0003](docs/adr/0003-kubernetes-audit-webhook-dual-layer-intake.md)).
Detection definitions are version-controlled, immutable, revision-identified
YAML files loaded at startup
([ADR-0004](docs/adr/0004-version-controlled-detection-definitions.md)).

## HTTP endpoints

| Endpoint | Auth | Purpose |
| --- | --- | --- |
| `POST /v1/audit-events` | Bearer token | Submits a Kubernetes audit-webhook `EventList` for admission |
| `GET /v1/alerts/{id}` | Bearer token | Retrieves an alert together with its full evidence and traceability inventory |
| `GET /readyz` | None | Reports readiness: database connectivity and detection-definition load status |

## Prerequisites

- **Docker Compose path (recommended):** Docker Engine with the Compose
  plugin. No local Go toolchain or PostgreSQL install is required — the
  build stage compiles the application inside a container.
- **Local Go development path:** Go 1.26.5 (see `go.mod`) and Docker (used
  by the integration test suite to run real, ephemeral PostgreSQL
  instances via testcontainers).

## Local setup with Docker Compose

From the repository root:

```sh
cp .env.example .env
# edit .env and set API_TOKEN and POSTGRES_PASSWORD
```

Clean deployment:

```sh
docker compose down -v --remove-orphans
docker compose build --no-cache
docker compose up -d
```

Check both containers report healthy:

```sh
docker compose ps
```

Full setup, readiness semantics, the end-to-end Scenario 1 demonstration,
and troubleshooting are documented in
[`docs/reference-environment.md`](docs/reference-environment.md).

## Environment variables

Set in `.env` (never committed — see `.env.example` and `.gitignore`):

| Variable | Required | Default | Description |
| --- | --- | --- | --- |
| `API_TOKEN` | Yes | — | Shared bearer token required on every authenticated endpoint |
| `POSTGRES_PASSWORD` | Yes | — | Password for the `cnsdp` PostgreSQL user; must be URL-safe (letters, digits, `- _ . ~` only) |
| `APP_PORT` | No | `8080` | Host-side port the application's HTTP endpoint is published on |
| `HTTP_ADDR` | No | `:8080` | Address the application's HTTP server binds to inside its container |
| `MAX_BODY_BYTES` | No | `10485760` (10 MiB) | Maximum accepted intake request body size |
| `WORKER_POLL_INTERVAL` | No | `1s` | How often the worker polls for new work when idle |

`DATABASE_URL` is not set directly when using Docker Compose — it is
assembled from `POSTGRES_PASSWORD` and the in-network `postgres` hostname
in `docker-compose.yml`. Running the binary directly (outside Compose)
requires `DATABASE_URL` and `API_TOKEN` to be set explicitly; see
`cmd/platform/main.go`.

## Developer commands

Build:

```sh
go build ./...
```

Format check:

```sh
gofmt -l cmd internal migrations definitions test
```

Vet:

```sh
go vet ./...
```

Unit tests:

```sh
go test -count=1 ./...
```

Integration tests (require Docker; spin up real, ephemeral PostgreSQL
instances via testcontainers):

```sh
go test -tags=integration -p 1 -count=1 ./...
```

## Exercising the running stack

With the Compose stack up and `$API_TOKEN` exported to match `.env`:

Readiness check:

```sh
curl -i http://127.0.0.1:8080/readyz
```

Submit the committed Scenario 1 fixture (a `pods/exec` request with TTY
allocation):

```sh
curl -i -X POST http://127.0.0.1:8080/v1/audit-events \
  -H "Authorization: Bearer $API_TOKEN" \
  -H "Content-Type: application/json" \
  --data @internal/intake/testdata/scenario-1-eventlist.json
```

Retrieve the resulting alert (on a fresh database, the first alert is
`1`):

```sh
curl -i http://127.0.0.1:8080/v1/alerts/1 \
  -H "Authorization: Bearer $API_TOKEN"
```

Cleanup:

```sh
docker compose down -v --remove-orphans
```

The full walkthrough, including expected response bodies, is in
[`docs/reference-environment.md`](docs/reference-environment.md).

## Documentation

| Document | Contents |
| --- | --- |
| [`docs/product.md`](docs/product.md) | Product charter: purpose, goals, principles, constraints |
| [`docs/architecture.md`](docs/architecture.md) | Phase 1 architecture baseline, data flow, walking-skeleton definition |
| [`docs/adr/`](docs/adr/) | Architecture Decision Records (language/deployment style, persistence, intake, detection definitions) |
| [`docs/reference-environment.md`](docs/reference-environment.md) | Docker Compose setup, environment variables, end-to-end demonstration, troubleshooting |
| [`docs/functional-requirements.md`](docs/functional-requirements.md) | Functional requirements (FR-001–FR-035) |
| [`docs/non-functional-requirements.md`](docs/non-functional-requirements.md) | Non-functional requirements (NFR-001–NFR-036) |
| [`docs/acceptance-criteria.md`](docs/acceptance-criteria.md) | Acceptance criteria (AC-001–AC-030) |
| [`docs/use-cases.md`](docs/use-cases.md), [`docs/personas.md`](docs/personas.md), [`docs/scope.md`](docs/scope.md), [`docs/glossary.md`](docs/glossary.md) | Supporting product definition documents |

## Repository structure

```
cmd/platform/       Application entry point (HTTP server + worker loop wiring)
internal/           The nine workflow modules plus supporting packages (see table above)
migrations/         Versioned SQL schema migrations, embedded into the binary
definitions/        Version-controlled YAML detection definitions, embedded into the binary
test/integration/   Cross-package integration tests (e.g. migration application)
docs/               Product, architecture, and ADR documentation
docs/adr/           Architecture Decision Records
spikes/             Pre-architecture research spikes (Kubernetes audit intake, PostgreSQL durable worker); historical, not part of the build
Dockerfile          Multi-stage build for the reference-environment container image
docker-compose.yml  Two-service (app + PostgreSQL) local reference environment
.env.example        Template for the local .env file consumed by docker-compose.yml
```

## Status and deferred work

Phase 1's walking-skeleton implementation (`docs/architecture.md` §8) is
**complete and locally verified**: the full flow above runs unattended
end to end, proven by an integration test that drives a real submission
through the running worker loop to a retrievable, traceability-intact
alert, backed by build, `vet`, unit-test, and real-PostgreSQL
integration-test passes.

Consistent with the approved architecture's explicit walking-skeleton
boundary, the following are deliberately **deferred, not defects**:

- Restart/recovery demonstration at the application level (the underlying
  durable-worker crash-recovery pattern was already validated by a
  dedicated spike — see `spikes/02-postgres-durable-worker/FINDINGS.md`).
- Performance and capacity testing against the approved throughput and
  latency envelope (NFR-001–NFR-003).
- Full admission-control enforcement beyond baseline request-size limits.
- Continuous integration tooling.
- Further hardening (e.g. the production secrets-supply mechanism, mTLS/OIDC
  upgrade paths) — explicitly left open in `docs/architecture.md` §6 and §9.
