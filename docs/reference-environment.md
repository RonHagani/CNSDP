# Docker Compose Reference Environment

This document is the setup procedure for the one documented reference
environment ARCH-01 §7 names: Docker Compose, exactly two services — the
application and PostgreSQL — on a single host (NFR-033). Following it
alone, with no other knowledge of the codebase, is sufficient to deploy
the platform and reproduce the Scenario 1 walking-skeleton demonstration
(NFR-028, AC-028).

It documents operational setup only; it does not redefine or restate any
approved product, requirements, or architecture decision.

## Quick start (PowerShell)

The fastest path from a clean checkout to real alerts rendered in the
frontend at `/alerts`. Every command below is exact PowerShell, run from
the repository root unless noted otherwise. Each step links to the fuller
section below it for detail and troubleshooting.

1. Copy both `.env.example` files and fill in matching values (see
   "Environment variables" below and `frontend/.env.example`):

   ```powershell
   Copy-Item .env.example .env
   Copy-Item frontend/.env.example frontend/.env
   ```

   Edit `.env`: set `API_TOKEN` (any long random value) and
   `POSTGRES_PASSWORD`. Edit `frontend/.env`: set `API_PROXY_TOKEN` to the
   *same* value as `.env`'s `API_TOKEN`. If you changed `APP_PORT` away
   from its `8080` default, also set `frontend/.env`'s `API_PROXY_TARGET`
   to `http://127.0.0.1:<APP_PORT>`.

2. Start PostgreSQL and the backend ("Starting the stack" below):

   ```powershell
   docker compose up -d --build
   docker compose ps
   ```

   Wait until both services show `Up (healthy)`.

3. Load development alerts ("Loading development alerts" below):

   ```powershell
   ./scripts/dev-seed-alerts.ps1
   ```

4. Start the frontend, in a second terminal ("Frontend development"
   below):

   ```powershell
   cd frontend
   npm install
   npm run dev
   ```

   Open the printed local URL (`http://localhost:5173/alerts`) — three
   real, backend-composed alerts render within a few seconds.

5. When done for the session, stop without losing data ("Stopping and
   resetting" below):

   ```powershell
   docker compose stop
   ```

## Prerequisites

- Docker Engine with the Compose plugin (`docker compose version` prints
  a version).
- A checkout of this repository, at the repository root.
- No local Go toolchain, PostgreSQL install, or other dependency is
  required — the build stage compiles the application inside a
  container.
- Node.js and npm, only for the frontend (see "Frontend development"
  below) — not required to run the backend alone.

## Environment variables

Copy `.env.example` to `.env` and fill in real values before the first
`docker compose up`. `.env` is gitignored — never commit it.

**This local `.env` mechanism is for the single-host Docker Compose
reference environment only. It is not the platform's future production
secrets-supply mechanism** — ARCH-01 §9 leaves that decision explicitly
deferred to later design work.

| Variable | Required | Description |
| --- | --- | --- |
| `API_TOKEN` | Yes | The shared bearer token required at every authenticated product-exposed path (NFR-012, NFR-013): the intake and alert-retrieval endpoints. Compose fails to start with a clear error if this is unset. |
| `POSTGRES_PASSWORD` | Yes | Password for the `cnsdp` PostgreSQL user. Compose fails to start with a clear error if this is unset. **Must be URL-safe**: it is embedded directly into a `postgres://` connection URI in `docker-compose.yml`, so it must not contain characters with special meaning in a URI — `: / ? # [ ] @ %` or whitespace. Use only letters, digits, and `- _ . ~`; a password containing e.g. `@` or `/` will produce an invalid connection URI and the application will fail to start with a database-connection error, not a clear validation message. |
| `HTTP_ADDR` | No (default `:8080`) | Address the application's HTTP server binds to inside its container. |
| `MAX_BODY_BYTES` | No (default `10485760`, i.e. 10 MiB) | Maximum accepted intake request body size — transport hardening, not the NFR-003 capacity envelope. |
| `WORKER_POLL_INTERVAL` | No (default `1s`) | How often the worker polls for new work when idle. |
| `APP_PORT` | No (default `8080`) | Host-side port the application's HTTP endpoint is published on — `docker-compose.yml` maps `127.0.0.1:${APP_PORT}:8080`. This changes only where the container is reachable *from the host*; it does not change `HTTP_ADDR` or the container's own internal listen address, which stays `:8080` regardless. Override it if `8080` is already in use on your machine (see "Readiness verification" below). |

`DATABASE_URL` is not itself a variable you set: `docker-compose.yml`
assembles it from `POSTGRES_PASSWORD` and the fixed in-network hostname
of the `postgres` service.

## Starting the stack

### Routine start (preserves existing data)

The normal day-to-day command, from the repository root with `.env`
filled in — safe to run repeatedly, including right after
`docker compose stop`:

```powershell
docker compose up -d --build
docker compose ps
```

`postgres` becomes healthy once `pg_isready` succeeds against it; `app`
does not start until `postgres` reports healthy
(`depends_on: condition: service_healthy`), so there is no
connect-before-ready race to handle. If a named `postgres-data` volume
already exists from a previous session, its data is preserved and reused
— this command never deletes it.

Expected: both `postgres` and `app` show `Up (healthy)`. If `app` is
still `starting`, wait a few seconds and re-check — the application must
connect to PostgreSQL, apply migrations, and load the detection
definitions before it starts serving.

### First-time or full-reset deployment (destroys existing data)

Use this only when you deliberately want a genuinely clean database — a
first-time deployment, or recovering from corrupted local state. This is
the one and only command in this document that deletes data; see
"Stopping and resetting" below for when to reach for it.

```powershell
docker compose down -v --remove-orphans
docker compose build --no-cache
docker compose up -d
```

`down -v --remove-orphans` guarantees a clean instance (no leftover
volume or container from a prior run) — the "clean instance of exactly
one documented reference environment" NFR-033 requires.

## Readiness verification

```sh
curl -i http://127.0.0.1:8080/readyz
```

This and every other host-facing URL in this document is really
`http://127.0.0.1:${APP_PORT}/...` — `8080` above is `APP_PORT`'s
default. The application always listens on `:8080` *inside* its
container regardless of `APP_PORT`; only the host-side published port
changes.

If `8080` is already in use by something else on this machine, set
`APP_PORT` to a free port instead — either add `APP_PORT=8081` to
`.env`, or run `APP_PORT=8081 docker compose up -d` for a single
invocation — then substitute that port for `8080` in every command in
this document, including the ones below.

Expected response: `HTTP/1.1 200 OK` with body

```json
{"status":"ready","checks":{"database":"ok","detection_definitions":"ok"}}
```

If either check has not yet passed, this returns `503 Service
Unavailable` with `{"status":"not_ready","failed_check":"database"}` or
`{"status":"not_ready","failed_check":"detection_definitions"}` instead
— never a silent hang or an unrelated error (NFR-020, AC-026).

## Loading development alerts

`scripts/dev-seed-alerts.ps1` submits the same real, committed fixtures
used by this repository's own tests
(`internal/intake/testdata/scenario-1-eventlist.json`,
`internal/validation/testdata/scenario{2,3}-valid.json`) through the real
ingestion API (`POST /v1/audit-events`) — never a frontend fixture, and
never anything invented for this script. It is development tooling only:
it is never run automatically, and it only ever talks to whatever
`-BaseUrl` you point it at (default: this local Compose stack).

```powershell
./scripts/dev-seed-alerts.ps1
```

It reads `API_TOKEN` and `APP_PORT` from the repository root `.env` by
default (never printing the token) — pass `-ApiToken` / `-BaseUrl`
explicitly to target something else.

Running it again is safe and does not create duplicate alerts: submission
admission is keyed by a deterministic `source_key` derived from each
event's `auditID`/`auditStage`
(`internal/submission/submission.go`, `migrations/0002_submission_source_key.up.sql`),
so resubmitting identical fixture content resolves to the same existing
rows. To start over with genuinely empty alert data instead, use the
full-reset deployment above, then re-run the seed script.

After seeding, `/v1/alerts` (and the frontend's `/alerts` page) shows
three alerts within a few seconds — the same three scenarios exercised by
`frontend/e2e/real-backend.spec.ts`.

## Frontend development

The frontend (`frontend/`) is a separate Vite/React app that talks to
this backend only through its own dev/preview server's same-origin `/api`
proxy — see `frontend/vite.config.ts` and `frontend/.env.example` for how
that proxy attaches the bearer credential server-side, never in the
browser.

```powershell
Copy-Item frontend/.env.example frontend/.env
# edit frontend/.env: set API_PROXY_TOKEN to match this backend's API_TOKEN
cd frontend
npm install
npm run dev
```

Open `http://localhost:5173/alerts`. This requires the backend already
running and reachable (see "Starting the stack" above) — `npm run dev`
alone starts only the frontend, and its proxy refuses to start at all if
`API_PROXY_TOKEN` is unset (a clear startup error, not a silent failure).

`frontend/README.md` documents the frontend's own checks (`npm run
typecheck`, `lint`, `test`, `build`) and `npm run e2e`, which runs against
a production preview build. `frontend/e2e/real-backend.spec.ts`
specifically requires this real backend (with development alerts loaded,
above) already running; every other e2e spec mocks the API and needs no
backend at all.

## Scenario 1 end-to-end demonstration

This reproduces the same walking-skeleton proof exercised by
`cmd/platform`'s integration test (Checkpoint 11), against the running
Compose stack instead of `go test`.

1. Submit the real, Spike-1-captured Scenario 1 fixture (a `pods/exec`
   request with TTY allocation) already committed at
   `internal/intake/testdata/scenario-1-eventlist.json`:

   ```sh
   curl -i -X POST http://127.0.0.1:8080/v1/audit-events \
     -H "Authorization: Bearer $API_TOKEN" \
     -H "Content-Type: application/json" \
     --data @internal/intake/testdata/scenario-1-eventlist.json
   ```

   Expected response: `HTTP/1.1 200 OK` with body

   ```json
   {"results":[{"index":0,"id":1}]}
   ```

   (The `id` is the admitted submission's id; it will differ on a
   non-fresh database.)

2. Wait a few seconds for the continuous worker loop to drive the
   submission through `validated → normalized → evaluated → alerted →
   evidenced` — well within the polling interval and far under the
   60-second NFR-001 target.

3. Retrieve the resulting alert. The submission id from step 1 is not
   the alert id — find the alert by polling `/v1/alerts/{id}` starting
   from `1` (on a fresh database, the first alert is `1`):

   ```sh
   curl -i http://127.0.0.1:8080/v1/alerts/1 \
     -H "Authorization: Bearer $API_TOKEN"
   ```

   Expected response: `HTTP/1.1 200 OK` with a body whose shape includes
   (abbreviated):

   ```json
   {
     "alertId": 1,
     "sourceEvent": {"available": true, "rawEvent": { "...": "..." }},
     "validationOutcome": {"available": true, "outcome": "valid"},
     "normalizedEvent": {"available": true, "event": { "...": "..." }},
     "detectionDefinition": {"available": true, "revision": "...", "definition": {"scenario": "scenario-1", "...": "..."}},
     "detectionResult": {
       "available": true,
       "matchReason": {
         "scenario": "scenario-1",
         "definitionName": "Interactive exec request",
         "satisfiedCharacteristics": [
           {"id": "stdin_streaming", "description": "..."},
           {"id": "tty_allocation", "description": "..."}
         ]
       }
     },
     "alert": {"available": true, "summary": { "...": "..." }},
     "traceability": {"intact": true}
   }
   ```

   The key checks: `detectionResult.matchReason.scenario` is
   `"scenario-1"`, both `stdin_streaming` and `tty_allocation` appear in
   `satisfiedCharacteristics` (the fixture requests both), and
   `traceability.intact` is `true`.

## Stopping and resetting

**Routine stop (preserves data).** Use this at the end of a normal
session — the recorded database survives (NFR-010), and "Routine start"
above brings it straight back:

```powershell
docker compose stop
```

**Explicit reset (destroys data).** Use this only when you deliberately
want to discard all local development data — e.g. between independent
demonstration runs, or to recover from corrupted local state. This is the
one command in this document that deletes the database volume; never run
it as part of routine shutdown:

```powershell
docker compose down -v --remove-orphans
```

`-v` removes the named PostgreSQL volume, so the next `docker compose up`
starts from a genuinely clean database and development alerts must be
reloaded (see "Loading development alerts" above).

## Troubleshooting

**`app` exits immediately after `postgres` becomes healthy, logs
`DATABASE_URL is required` or `API_TOKEN is required`.**
`.env` was not created from `.env.example`, or is missing one of the two
required variables. Compose should have already refused to start with a
`variable is not set` error before reaching this point — if you see this
instead, confirm you are running `docker compose` from the directory
containing `.env` (Compose only auto-loads a `.env` file in the current
working directory).

**`docker compose up` fails immediately with `POSTGRES_PASSWORD must be
set` or `API_TOKEN must be set`.**
Working as documented (see "Environment variables" above) — `.env` is
missing, not copied from `.env.example`, or one of the two values is
still empty.

**`app` container logs a database connection error mentioning an
unexpected host, scheme, or "invalid port" / "invalid URI".**
`POSTGRES_PASSWORD` contains a character with special meaning in a URI
(commonly `@`, `/`, `#`, or `%`). Choose a password using only letters,
digits, and `- _ . ~` (see "Environment variables" above), then
`docker compose down -v --remove-orphans` and redeploy.

**`postgres` never reports healthy; `app` stays `Created` and never
starts.**
Check `docker compose logs postgres`. The most common local cause is a
port conflict or insufficient disk space for the named volume; this
environment does not expose PostgreSQL's port to the host, so a
host-side port conflict on 5432 is not the cause.

**`curl` to `/readyz` or `/v1/audit-events` fails to connect at all.**
Confirm `app` reports `Up (healthy)` in `docker compose ps` first — the
host port mapping (`127.0.0.1:${APP_PORT}`, default `8080`) only starts
accepting connections once the container itself is running, and the
container's own `GET /readyz` healthcheck must pass before Compose
reports it healthy.

**`docker compose up` fails with `port is already allocated`.**
Something else on the host is already using the app's published port
(default `8080`). Set `APP_PORT` to a free port instead of editing
`docker-compose.yml` — add `APP_PORT=8081` to `.env`, or run
`APP_PORT=8081 docker compose up -d` for one invocation — then
substitute that port for `8080` in every command in this document. The
container's own port (`8080`) and `HTTP_ADDR` are unaffected either way.

**Intake or retrieval returns `401 Unauthorized`.**
The `Authorization: Bearer <token>` header does not match `API_TOKEN` in
`.env` exactly. Confirm the value you exported into your shell (`$API_TOKEN`
in the commands above) matches `.env`.
