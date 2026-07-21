# Spike 2 Findings — PostgreSQL durable worker

Status: complete, all scenarios passed. All data below comes from a real
PostgreSQL 16 instance (Docker, since removed) driven by real compiled Go
binaries performing real process crashes (`os.Exit`) — not simulated or
hand-reasoned. The full crash-point matrix and a baseline were executed;
raw pass/fail output is reproduced below.

## Question being tested

Does a transactional, database-backed status-transition pattern — durable
admission before processing, in-process claim-and-advance, restart-safe
recovery — reliably leave every submission in a determinable state after an
abrupt crash, and reliably avoid duplicate downstream artifacts when a stage
is retried after a partial failure? Per the approved framing: **transactional,
idempotent, and restart-safe** — not "exactly-once."

## Minimal implementation boundary

Two disposable Go binaries, isolated from any product source tree, sharing
one module (`cnsdp/spikes/pgworker`):

- `worker/` — the entire "worker": a single-threaded, single-consumer loop
  (no `SKIP LOCKED`, no goroutines, no concurrency) that repeatedly claims
  the oldest non-terminal submission and advances it exactly one stage,
  with an optional deterministic crash-injection point.
- `driver/` — the test orchestrator: seeds submissions, invokes the worker
  binary as a subprocess with specific crash-injection flags, inspects
  Postgres directly to assert pre- and post-crash state, then invokes the
  worker again with no crash flags to resume, and asserts final state.

No production pipeline logic (real validation, normalization, or detection
semantics) — every artifact table holds stub content sufficient only to
exercise the transactional/idempotency pattern.

## Exact schema

```sql
CREATE TABLE submissions (
    id SERIAL PRIMARY KEY,
    status TEXT NOT NULL DEFAULT 'admitted',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE detection_definitions (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    revision TEXT NOT NULL
);

-- One validation outcome per submission.
CREATE TABLE validation_outcomes (
    id SERIAL PRIMARY KEY,
    submission_id INTEGER NOT NULL REFERENCES submissions(id),
    outcome TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (submission_id)
);

-- One normalized event per submission.
CREATE TABLE normalized_events (
    id SERIAL PRIMARY KEY,
    submission_id INTEGER NOT NULL REFERENCES submissions(id),
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (submission_id)
);

-- One detection result per (submission, detection-definition revision).
CREATE TABLE detection_results (
    id SERIAL PRIMARY KEY,
    submission_id INTEGER NOT NULL REFERENCES submissions(id),
    detection_definition_id INTEGER NOT NULL REFERENCES detection_definitions(id),
    matched BOOLEAN NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (submission_id, detection_definition_id)
);

-- One alert per matching detection result.
CREATE TABLE alerts (
    id SERIAL PRIMARY KEY,
    detection_result_id INTEGER NOT NULL REFERENCES detection_results(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (detection_result_id)
);

-- One traceability link per (source, target, relation).
CREATE TABLE traceability_links (
    id SERIAL PRIMARY KEY,
    source_ref TEXT NOT NULL,
    target_ref TEXT NOT NULL,
    relation TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (source_ref, target_ref, relation)
);
```

The five uniqueness constraints deliberately match the approved refinement —
not a blanket `UNIQUE(submission_id)` on every table:

| Artifact | Constraint |
|---|---|
| validation outcome | `UNIQUE(submission_id)` — one per submission |
| normalized event | `UNIQUE(submission_id)` — one per submission |
| detection result | `UNIQUE(submission_id, detection_definition_id)` — one per submission **and** definition |
| alert | `UNIQUE(detection_result_id)` — one per matching detection result, not one per submission |
| traceability link | `UNIQUE(source_ref, target_ref, relation)` — unique per (source, target, relation) |

Two stub detection definitions are seeded so `detection_results` genuinely
exercises a multi-row-per-submission constraint (32 rows across 16
submissions in the final run, never more than 2 per submission).

## State transitions and transaction boundaries

`submissions.status`: `admitted → validated → normalized → evaluated →
alerted → evidenced`.

Each transition is one function (`advance` in `worker/stages.go`) that, for
a given stage, opens **one** transaction containing both the artifact
insert(s) (idempotent via `ON CONFLICT ... DO NOTHING`) and the guarded
status update (`UPDATE submissions SET status=$new WHERE id=$id AND
status=$expectedPrior`), then commits. No split artifact/status commits
exist anywhere in this implementation — that was a hard constraint, not
just a preference, and the code has no code path that could commit one
without the other.

```
BEGIN
  [crash point: "before" — fires here, before BEGIN even runs]
  INSERT INTO <artifact table> ... ON CONFLICT ... DO NOTHING
  UPDATE submissions SET status=$to WHERE id=$id AND status=$from
  [crash point: "mid" — fires here, transaction open, uncommitted]
COMMIT
  [crash point: "after" — fires here, transaction already durable]
```

The status-guard (`AND status=$from`) is a defensive idempotency check: if
the row were somehow not at the expected prior status, the `UPDATE` affects
zero rows and the stage function returns an error rather than silently
proceeding — never exercised as a failure path in this spike (no
concurrency exists to trigger it), but present because "idempotent
transitions where appropriate" was an explicit requirement.

## Test strategy

A driver binary ran, against one live Postgres instance:

1. **Baseline** — seed one submission, run the worker with no crash
   injection, assert it drains straight through to `evidenced` with exactly
   the expected artifact counts.
2. **Crash-point matrix** — for each of the 5 stages × 3 timings (`before`,
   `mid`, `after`) = **15 scenarios**: seed a fresh submission, run the
   worker targeting that exact (stage, timing), confirm it actually crashed
   (distinctive exit code `111`, not a normal `0` exit — this catches the
   test itself being wrong, e.g. targeting a stage/timing that never
   fires), assert the expected pre- or post-crash state directly via SQL,
   then re-run the worker with no crash flags to resume, and assert it
   reaches `evidenced` with **no duplicate rows in any artifact table**.
3. A final whole-database check across all 16 accumulated submissions.

## Observed results

All 16 scenarios passed:

```
=== Spike 2 driver: PostgreSQL durable worker crash-point matrix ===
[PASS] baseline (no crash)                      straight-through run produced exactly the expected artifact counts
[PASS] stage=validate timing=before             recovered, drained to evidenced, no duplicate artifacts
[PASS] stage=validate timing=mid                recovered, drained to evidenced, no duplicate artifacts
[PASS] stage=validate timing=after              recovered, drained to evidenced, no duplicate artifacts
[PASS] stage=normalize timing=before            recovered, drained to evidenced, no duplicate artifacts
[PASS] stage=normalize timing=mid               recovered, drained to evidenced, no duplicate artifacts
[PASS] stage=normalize timing=after             recovered, drained to evidenced, no duplicate artifacts
[PASS] stage=evaluate timing=before             recovered, drained to evidenced, no duplicate artifacts
[PASS] stage=evaluate timing=mid                recovered, drained to evidenced, no duplicate artifacts
[PASS] stage=evaluate timing=after              recovered, drained to evidenced, no duplicate artifacts
[PASS] stage=alert timing=before                recovered, drained to evidenced, no duplicate artifacts
[PASS] stage=alert timing=mid                   recovered, drained to evidenced, no duplicate artifacts
[PASS] stage=alert timing=after                 recovered, drained to evidenced, no duplicate artifacts
[PASS] stage=evidence timing=before             recovered, drained to evidenced, no duplicate artifacts
[PASS] stage=evidence timing=mid                recovered, drained to evidenced, no duplicate artifacts
[PASS] stage=evidence timing=after              recovered, drained to evidenced, no duplicate artifacts

=== Summary ===
16/16 scenarios passed
```

Whole-database check after all 16 submissions processed:

| Table | Row count | Expected |
|---|---|---|
| submissions | 16 | 16 (all reached `evidenced`) |
| validation_outcomes | 16 | 16 × 1 |
| normalized_events | 16 | 16 × 1 |
| detection_results | 32 | 16 × 2 |
| alerts | 16 | 16 × 1 |
| traceability_links | 48 | 16 × 3 |

Zero submissions left in a non-terminal state. Zero rows found violating
any table's per-submission multiplicity. Zero duplicates anywhere.

**`before` and `mid` converge to the identical observable outcome, as
expected from transactional atomicity — verified, not assumed.** A crash
"mid" a stage's transaction (after the `INSERT`/`UPDATE` execute, before
`COMMIT`) and a crash "before" that transaction even opens both leave the
row at exactly the same pre-stage state after restart: PostgreSQL rolls
back the uncommitted work when it detects the dropped connection, so there
is no third "half-applied" state to reason about. Directly observed, e.g.
for the `evaluate` stage: killing the process after both `INSERT`
statements executed but before `COMMIT` left `detection_results` at 0 rows
for that submission and `submissions.status` still at `normalized` —
identical to what `before`-timing produced. Example captured crash output:

```
SPIKE-CRASH-INJECTED: mid stage "evaluate" tx, before commit (submission 2)
```

followed, after restart, by the stage being redone from scratch and
producing exactly 2 `detection_results` rows (not 4) — the `ON CONFLICT ...
DO NOTHING` idempotent insert did nothing on the first (rolled-back, so
actually never-happened) attempt and inserted normally on the real one, as
there was nothing to conflict with.

A crash "after" commit was confirmed to leave both the artifact and the
status change durably in place, and the resumed worker correctly moved on
to the *next* stage rather than re-attempting the just-completed one (the
status column itself is what tells the worker where to resume — the
`WHERE status=$from` guard on the next stage's `UPDATE` would simply not
match if the worker mistakenly tried to redo a stage that had already
advanced past it).

## Failures encountered during construction (disclosed)

Two real bugs were caught and fixed before the matrix was trusted — noted
here because they are informative about how easy it is to get this pattern
subtly wrong, even in a small spike:

1. An early driver draft accidentally ran an extra, unintended worker
   invocation for non-`before` timings (leftover logic from an earlier
   design iteration). It happened to be harmless (the stray call crashed
   "before" a transaction, i.e. a no-op), but was fragile and would have
   masked a real bug if the "before" case had ever behaved unexpectedly.
   Rewritten so exactly one worker invocation handles every stage/timing
   combination uniformly.
2. The `traceability_links` row-count query in the driver initially
   referenced a `$2` placeholder with no `$1` used in the query text, and
   used a `LIKE`-pattern heuristic that could have cross-matched links
   belonging to a *different* submission once the database had accumulated
   rows across scenarios. Rewritten to resolve the specific submission's
   `alert`/`detection_result`/`normalized_event` ids first, then count only
   links belonging to that exact chain.

Neither bug was in the worker (the thing actually being validated) — both
were in the test driver — but both are recorded because a spike whose test
harness has undetected bugs would produce a false "PASS."

## Architectural implications

- The transactional pattern — artifact insert(s) plus the guarded status
  update in one transaction, `ON CONFLICT DO NOTHING` on every artifact
  insert — is sufficient, on real PostgreSQL, to make restart-safe,
  duplicate-free recovery a property of the design rather than something
  requiring careful runtime coordination. This directly supports adopting
  it as the processing-model ADR's basis.
- No row-locking primitive (`SELECT ... FOR UPDATE`, `SKIP LOCKED`) was
  needed anywhere, confirming the single-worker assumption: the "next
  non-terminal submission" query has no concurrent competitor for the rows
  it reads.
- The refined, artifact-specific uniqueness constraints (rather than a
  blanket `UNIQUE(submission_id)`) worked exactly as intended:
  `detection_results` correctly allowed 2 rows per submission (one per
  definition) while still preventing a duplicate for the *same*
  (submission, definition) pair on retry.
- The status-guard (`WHERE status=$from`) on every `UPDATE` is a cheap,
  real safety net worth carrying into the product design even though nothing
  in this single-worker spike could exercise it as a genuine failure path —
  it becomes load-bearing the moment anything about the concurrency model
  changes later.
- This pattern was validated only for a single in-process worker against a
  single Postgres instance. It says nothing about behavior under multiple
  concurrent workers (explicitly out of scope here) — if a future release
  ever needed that, `SKIP LOCKED` or equivalent claim-locking would need
  its own dedicated validation, not an assumption that this spike's result
  extends automatically.

## What is explicitly excluded

- `SKIP LOCKED`, multiple concurrent workers, or any external broker — not
  introduced, per the approved constraint.
- Real validation, normalization, or detection logic — every artifact
  table holds stub content only.
- The full nine-stage product pipeline — five representative stages,
  covering all five refined artifact-uniqueness types, were used instead.
- Authentication, the intake adapter (Spike 1's concern), and the dual-layer
  contract — untouched here.
- Performance/load testing — 16 submissions processed sequentially; no
  throughput claims are made or implied.
- Multi-instance or horizontal-scaling scenarios.

## Environment cleanup performed

- Postgres container stopped and removed.
- The `postgres:16-alpine` image pulled for this spike was removed
  (an unrelated `postgres:15-alpine` image, used by a different project on
  this machine, was left untouched).
- Compiled binaries (`worker.exe`, `driver.exe`) deleted; `.gitignore`
  added so future builds don't get committed accidentally.
- Go toolchain and Docker Desktop left installed/running — durable project
  dependencies under the locked architecture, not spike-specific.
