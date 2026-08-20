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

## Recovery from an abrupt interruption

The documented recovery procedure exercised and measured by
`test/verification`'s AC-019 verification (recovery-time objective,
NFR-009): after an abrupt, non-graceful interruption (a crashed host, a
`docker kill`, a process termination outside Compose's own control), bring
the application back with:

```sh
docker compose start app
```

then confirm readiness the same way as initial startup (see "Readiness
verification" above). This is an explicit step, not merely a wait: the
base `docker-compose.yml`'s `restart: unless-stopped` policy is declared
and remains unchanged, but empirical testing (`test/verification`'s own
first runs against this exact environment) found it did not reliably
restart the container on its own after an abrupt `SIGKILL` — `docker
inspect`'s `RestartCount` stayed at `0` for over ten minutes in that
condition. Issuing `docker compose start app` explicitly is the
realistic action an operator or a supervising process takes, and is fast:
the same verification harness measured full recovery (interruption to
`/readyz` returning `200`, including migrations and detection-definition
reload) at under 2 seconds once the explicit start was issued, well within
the approved 15-minute objective.

Persistent state is unaffected either way — every submission, validation
outcome, normalized event, detection result, and alert already committed
to PostgreSQL survives an application-container interruption untouched;
only the application process itself needs restarting.

## Behavior at persistent-storage exhaustion

`docker-compose.yml`'s `postgres-data` volume carries no size limit of its
own — its usable capacity is whatever the host's Docker installation and
underlying filesystem actually provide. This environment does not document
or configure a storage-capacity number (ARCH-01 §7), and none is set here:
sizing a production or reference deployment's storage is an operational
decision for whoever provisions the underlying disk/volume, not something
this repository prescribes.

If that environment-supplied capacity is ever exhausted, the platform does
not silently continue or corrupt state (NFR-036): the in-flight write fails
and rolls back cleanly, every previously committed submission, validation
outcome, normalized event, detection result, and alert remains intact and
retrievable exactly as before, and the condition is visible in the
application's structured logs as a distinct `resource_exhausted` outcome,
not an undifferentiated error. No action beyond restoring available capacity
to the volume/disk is required or defined by the product itself.

`test/verification`'s AC-023 phase reproduces this exact condition on
demand, against a disposable, isolated fixture — never against this
reference environment's own `postgres-data` volume — to verify the behavior
above holds. See `test/verification`'s own source for how that fixture is
constructed and sized; its capacity is a deliberately small test value
chosen to reproduce the condition quickly, not a suggested size for this
volume.

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

## AWS CloudTrail live delivery (bridge)

Everything above proves the CloudTrail intake (`POST /v1/cloudtrail-events`)
against fixture or recorded records — the endpoint itself needs no bridge
at all. This section is optional and covers proving that same, unchanged
endpoint against a **real** AWS account's own CloudTrail activity, via the
external delivery bridge approved by ADR-0006
(`docs/adr/0006-aws-cloudtrail-delivery-and-intake-topology.md`):
CloudTrail Trail → EventBridge → SQS → `cmd/cloudtrail-bridge` →
`POST /v1/cloudtrail-events`.

**This is not part of the platform's own two-service Compose deployment**
(ARCH-01 §7, NFR-033/034) — the bridge is a separate command you run
yourself, against your own personal-sandbox AWS account, entirely outside
`docker-compose.yml`.

### Prerequisites

- The CNSDP reference environment already running (see "Quick start"
  above), with `POST /v1/cloudtrail-events` reachable and authenticated.
- Go 1.26.5+ (to `go run`/build the bridge).
- AWS CLI v2, authenticated to a **personal sandbox account** — never a
  production or shared account (see "Cost and safety notes" below).
- A stable IAM principal (role or user ARN) you control, to authorize the
  bridge's dedicated IAM role.

### AWS login, profile, and role setup

1. Authenticate with your own AWS identity, e.g. AWS SSO:

   ```powershell
   aws configure sso
   aws sso login --profile <your-sso-profile>
   ```

2. Determine the **stable** IAM ARN behind that identity — not the
   ephemeral session ARN `aws sts get-caller-identity` reports while
   logged in (shape `arn:aws:sts::<account-id>:assumed-role/...`, which
   changes on every login and cannot be used as a durable IAM trust
   principal). For an SSO permission set:

   ```powershell
   aws iam list-roles --query "Roles[?contains(RoleName,'<permission-set-name>')].Arn" --output text
   ```

3. Run the provisioning script (below) with that stable ARN as
   `-OperatorPrincipalArn`.

4. After it prints `cnsdp-cloudtrail-bridge-role`'s ARN, add a profile to
   `~/.aws/config` that assumes it:

   ```ini
   [profile cnsdp-cloudtrail-bridge]
   role_arn = arn:aws:iam::<account-id>:role/cnsdp-cloudtrail-bridge-role
   source_profile = <your-sso-profile>
   ```

No static AWS access key is created or used anywhere in this default
path — the bridge calls only the AWS SDK's standard credential chain and
never reads, holds, or logs an access key or session token itself (see
`cmd/cloudtrail-bridge/.env.example`). A static-key fallback is
deliberately out of scope for this slice.

### Provisioning command

```powershell
./scripts/provision-cloudtrail-bridge.ps1 -OperatorPrincipalArn arn:aws:iam::<account-id>:role/<your-role-name>
```

Safe to re-run — every resource is created only if missing, and every
policy/attribute is re-applied idempotently. Creates: an S3 bucket (+
public-access block + the CloudTrail-required bucket policy), a
single-region `us-east-1` trail (write-only management events, global
service events included), an EventBridge rule, an SQS main queue + DLQ,
and `cnsdp-cloudtrail-bridge-role`. Prints the non-secret values needed
below. It never prints or creates `CNSDP_API_TOKEN` — get that from this
repository's own root `.env`.

### Bridge configuration

```powershell
Copy-Item cmd/cloudtrail-bridge/.env.example cmd/cloudtrail-bridge/.env
```

Fill in (or export in your shell instead of writing to the file):
`AWS_PROFILE` (the profile from step 4 above), `AWS_REGION=us-east-1`,
`SQS_QUEUE_URL` (the provisioning script's output), `CNSDP_ENDPOINT`
(your running CNSDP's `/v1/cloudtrail-events` URL), and
`CNSDP_API_TOKEN` (copied from the platform's own `.env` — never a newly
invented value).

### Starting CNSDP

Already covered above ("Starting the stack") — confirm
`docker compose ps` shows both services `Up (healthy)` before starting
the bridge.

### Starting the bridge

```powershell
$env:AWS_REGION = "us-east-1"
$env:AWS_PROFILE = "cnsdp-cloudtrail-bridge"
$env:SQS_QUEUE_URL = "<from provisioning output>"
$env:CNSDP_ENDPOINT = "http://127.0.0.1:8080/v1/cloudtrail-events"
$env:CNSDP_API_TOKEN = "<API_TOKEN from .env>"
go run ./cmd/cloudtrail-bridge
```

Expected startup log: a `cloudtrail-bridge starting` line naming the
region, queue URL, resolved queue ARN, and endpoint, followed by a
long-poll loop with no further output until a message arrives. Leave it
running in this terminal for the demo below. Stop it with Ctrl+C
(SIGINT) — it shuts down gracefully, leaving any in-flight message
untouched (safely redeliverable later).

### Scenario 5 (CreateAccessKey) live demo

Scenario 5 is the safest live trigger: purely additive (issues one new
credential on a disposable, policy-less user), weakens no existing
security control, and is completely and trivially reversible.

1. Create a disposable IAM user:

   ```powershell
   aws iam create-user --user-name cnsdp-demo-user
   ```

2. Trigger the scenario:

   ```powershell
   aws iam create-access-key --user-name cnsdp-demo-user
   ```

3. Watch the bridge's terminal for an `admitted` log line, then check
   CNSDP:

   ```powershell
   curl -s http://127.0.0.1:8080/v1/alerts -H "Authorization: Bearer $env:API_TOKEN"
   ```

   Expect one new alert whose `detectionName` is "New IAM access key
   created". Fetch it in full and verify the evidence:

   ```powershell
   curl -s http://127.0.0.1:8080/v1/alerts/<id> -H "Authorization: Bearer $env:API_TOKEN"
   ```

   Check: `detectionResult.matchReason.scenario` is `"scenario-5"`,
   `satisfiedCharacteristics` includes `access_key_created`,
   `sourceEvent.available` is `true` (the real CloudTrail record),
   and `traceability.intact` is `true`.

   CloudTrail itself never includes the created key's `secretAccessKey`
   value in a `CreateAccessKey` event's `responseElements` — only the
   access key ID and status. Neither this bridge nor CNSDP performs any
   redaction of its own: the record is forwarded and persisted exactly
   as CloudTrail delivered it, so this demo is safe only because
   CloudTrail never puts secret key material in the record in the first
   place, not because of anything CNSDP does with it.

   Typical observed latency (API call to visible alert) is on the order
   of one to a few minutes in normal operation — dominated by
   CloudTrail's own delivery step, not by CNSDP or the bridge (see
   "Delivery and reliability limitations" below). To tell a delivery
   failure from a detection failure: check whether the SQS queue ever
   received a message at all (AWS Console or `aws sqs
   get-queue-attributes --attribute-names ApproximateNumberOfMessages`);
   if it did, check the bridge's own log for a POST outcome; if that was
   `200`, check `GET /v1/submissions` for the admitted record's
   validation outcome.

### Immediate cleanup

Delete the access key right away — do not leave a live credential
sitting unused, even on a policy-less user:

```powershell
aws iam delete-access-key --user-name cnsdp-demo-user --access-key-id <id-from-step-2>
aws iam delete-user --user-name cnsdp-demo-user   # optional, if not repeating the demo
```

### Teardown

When done testing AWS delivery entirely:

```powershell
./scripts/teardown-cloudtrail-bridge.ps1
```

Removes every resource provisioning created, tolerating already-missing
resources, in dependency-safe order (EventBridge target before rule,
trail stopped before deleted, then queues, then the IAM role, then
bucket objects before the bucket). Never touches anything outside the
`cnsdp-cloudtrail-bridge-*` naming prefix.

### Delivery and reliability limitations

Read before relying on this path for anything beyond a demonstration:

- **CloudTrail's delivery of events to EventBridge is documented by AWS
  as best-effort, not durable.** In rare cases a real API call's event
  may simply never reach EventBridge at all — nothing on the CNSDP or
  bridge side can detect that gap; there is no sequence number or count
  exposed anywhere in this pipeline that would reveal a missing event.
- **SQS's own durability and retry guarantees begin only once an event
  has reached the queue.** From there, standard SQS at-least-once
  delivery applies, and the bridge's own retry/backoff/DLQ behavior
  (documented in `cmd/cloudtrail-bridge`'s own source comments) governs
  delivery to CNSDP.
- **The SQS DLQ is a durability and inspection boundary, not a
  malformed-event quarantine.** The main queue's redrive policy moves a
  message to the DLQ once it has been received `maxReceiveCount=50`
  times without being deleted — that can happen to a perfectly valid
  CloudTrail event, not just a bad one, if CNSDP or the network path to
  it is down or rejecting requests for long enough. 50 was chosen to
  deliberately tolerate a substantially longer transient outage than
  this project's original value of 5. It does not make delivery
  lossless: after any sustained outage, check the DLQ's
  `ApproximateNumberOfMessagesVisible` (`aws sqs get-queue-attributes
  --queue-url <dlq-url> --attribute-names
  ApproximateNumberOfMessagesVisible`) and, if it is nonzero, inspect
  and explicitly redrive or otherwise reprocess those messages by hand —
  nothing in this slice does so automatically.
- **CNSDP is idempotent for redelivery, keyed by the CloudTrail record's
  own `eventID`.** A record delivered more than once — SQS redelivery,
  or a bridge restart reprocessing an undeleted message — is safely
  deduplicated into exactly one submission, never a duplicate alert.
- **This demo proves a real, successful delivery path end to end** — a
  genuine AWS API call, through to a CNSDP alert with intact evidence
  and traceability. **It does not prove exactly-once or lossless
  delivery**, a formal latency SLA (AWS publishes none for the
  CloudTrail→EventBridge hop), or behavior under sustained volume or an
  extended AWS-side outage.

### Cost and safety notes

- Expected incremental cost on a personal sandbox account: near zero.
  The first copy of management events delivered to one trail carries no
  CloudTrail service charge (only negligible S3 storage for the log
  files, which this slice never reads); AWS-service-sourced EventBridge
  events are not billed as custom events; this demo's SQS volume is well
  within the standing free tier.
- What could unexpectedly cost more: configuring the trail for
  read+write instead of write-only management events, a multi-region
  trail, or data-event logging — the provisioning script does none of
  these.
- Recommended safeguards: an AWS Budget alert at a low threshold; a
  personal sandbox account only, never production or shared; never
  root-account credentials for any part of this.

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
