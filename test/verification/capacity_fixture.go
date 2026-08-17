package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	cnsdpdb "cnsdp/internal/db"
)

// capacityTablespace is the name of the size-bounded tablespace created on
// the fixture Postgres's tmpfs mount (docker-compose.verify.yml,
// postgres-capacity-fixture). It exists only on that disposable instance
// -- never on the primary postgres this run's other phases use.
const capacityTablespace = "verify_capacity"

// TablespaceRelation is one relation's actual, observed tablespace
// placement -- never assumed. relkind follows pg_class.relkind: "r"
// (ordinary table), "i" (index), "t" (TOAST table).
type TablespaceRelation struct {
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	Tablespace string `json:"tablespace"`
}

// TablespacePlacementReport is the harness's own machine-checked proof
// that every product-owned relation actually landed in the constrained
// tablespace before the exhaustion attempt begins -- AC-023's storage
// exhaustion clause is only trustworthy if the write path it exercises
// genuinely goes through the capped tablespace, not pg_default by
// accident.
type TablespacePlacementReport struct {
	Relations     []TablespaceRelation `json:"relations"`
	AllInCapacity bool                 `json:"allInCapacity"`
	Problems      []string             `json:"problems,omitempty"`
}

// SetupCapacityTablespace connects to the fixture Postgres with the
// privileged "cnsdp" role (the same role the postgres:16-alpine image's
// own POSTGRES_USER bootstraps as a superuser -- required for CREATE
// TABLESPACE), creates the size-bounded tablespace on its tmpfs mount,
// bulk-moves every relation the "cnsdp" role owns into it (tables AND
// indexes -- two separate bulk statements, since PostgreSQL's ALTER TABLE
// ALL IN TABLESPACE never moves indexes), and verifies actual placement
// by querying catalog metadata rather than assuming the move succeeded.
//
// This only ever runs against a freshly migrated, still-empty fixture
// database (main.go's phase ordering: the fixture app's own startup
// applies migrations and loads the tiny, static detection-definitions set
// before this is called, and no admission traffic has been sent yet) --
// so this is never a bulk copy of a large, already-populated dataset; see
// docs/architecture.md §7 for why that distinction matters.
func SetupCapacityTablespace(ctx context.Context, fixturePgHostPort, adminPassword string) (*TablespacePlacementReport, error) {
	dsn := fmt.Sprintf("postgres://cnsdp:%s@127.0.0.1:%s/cnsdp?sslmode=disable", adminPassword, fixturePgHostPort)
	conn, err := cnsdpdb.Connect(ctx, dsn)
	if err != nil {
		return nil, wrapEnvErr("connect to capacity fixture (privileged)", err)
	}
	defer conn.Close()

	ddl := []string{
		fmt.Sprintf(`CREATE TABLESPACE %s LOCATION '/tsfixture'`, capacityTablespace),
		fmt.Sprintf(`ALTER TABLE ALL IN TABLESPACE pg_default OWNED BY cnsdp SET TABLESPACE %s`, capacityTablespace),
		fmt.Sprintf(`ALTER INDEX ALL IN TABLESPACE pg_default OWNED BY cnsdp SET TABLESPACE %s`, capacityTablespace),
	}
	for i, stmt := range ddl {
		if _, err := conn.ExecContext(ctx, stmt); err != nil {
			return nil, wrapEnvErr("capacity fixture tablespace setup", fmt.Errorf("statement %d: %w", i, err))
		}
	}

	return verifyTablespacePlacement(ctx, conn)
}

// verifyTablespacePlacement queries pg_class/pg_tablespace directly for
// every relation reachable from the "public" schema's own tables --
// never a bare "relnamespace = pg_toast" scan, which would also match
// TOAST tables belonging to system catalogs (pg_type, pg_proc, and
// similar low-OID catalogs each have their own TOAST table in the same
// pg_toast namespace, correctly left in pg_default/pg_global -- confirmed
// empirically the first time this query was that broad: dozens of
// catalog TOAST tables/indexes were misreported as placement failures
// that were never supposed to move). Scoped explicitly to: every ordinary
// table in "public", every index on one of those tables, every one of
// those tables' TOAST table (if any), and every index on that TOAST
// table -- exactly the relation set ALTER TABLE/INDEX ... OWNED BY cnsdp
// was supposed to move, nothing else. WAL and system catalogs are
// structurally untouched by tablespace commands regardless (they are not
// ordinary relations at all), so no separate exclusion logic is needed
// for them.
func verifyTablespacePlacement(ctx context.Context, conn *sql.DB) (*TablespacePlacementReport, error) {
	rows, err := conn.QueryContext(ctx, `
		WITH product_tables AS (
			SELECT oid, reltoastrelid
			FROM pg_class
			WHERE relnamespace = 'public'::regnamespace AND relkind = 'r'
		),
		product_toast AS (
			SELECT reltoastrelid AS oid FROM product_tables WHERE reltoastrelid <> 0
		),
		relevant AS (
			SELECT oid FROM product_tables
			UNION
			SELECT oid FROM product_toast
			UNION
			SELECT indexrelid AS oid FROM pg_index WHERE indrelid IN (SELECT oid FROM product_tables)
			UNION
			SELECT indexrelid AS oid FROM pg_index WHERE indrelid IN (SELECT oid FROM product_toast)
		)
		SELECT c.relname, c.relkind, COALESCE(ts.spcname, 'pg_default') AS tablespace
		FROM pg_class c
		LEFT JOIN pg_tablespace ts ON ts.oid = c.reltablespace
		WHERE c.oid IN (SELECT oid FROM relevant)
		ORDER BY c.relkind, c.relname`)
	if err != nil {
		return nil, wrapEnvErr("verify tablespace placement", err)
	}
	defer rows.Close()

	report := &TablespacePlacementReport{}
	for rows.Next() {
		var rel TablespaceRelation
		if err := rows.Scan(&rel.Name, &rel.Kind, &rel.Tablespace); err != nil {
			return nil, wrapEnvErr("verify tablespace placement", fmt.Errorf("scan: %w", err))
		}
		report.Relations = append(report.Relations, rel)
		if rel.Tablespace != capacityTablespace {
			report.Problems = append(report.Problems, fmt.Sprintf("%s (%s) is in tablespace %q, want %q", rel.Name, kindLabel(rel.Kind), rel.Tablespace, capacityTablespace))
		}
	}
	if err := rows.Err(); err != nil {
		return nil, wrapEnvErr("verify tablespace placement", err)
	}
	if len(report.Relations) == 0 {
		report.Problems = append(report.Problems, "no relations found in public/pg_toast at all -- migrations may not have applied")
	}
	report.AllInCapacity = len(report.Problems) == 0
	return report, nil
}

func kindLabel(relkind string) string {
	switch relkind {
	case "r":
		return "table"
	case "i":
		return "index"
	case "t":
		return "toast table"
	default:
		return relkind
	}
}

// tableRowCounts snapshots the row count of every product table reachable
// through conn -- shared by the fixture database (immediately before
// driving to exhaustion, the "nothing committed yet beyond fixture setup"
// baseline, and immediately after, to prove the failing attempt left no
// partial row: AC-023's "no recorded artifact is silently lost" /
// rollback clause) and the primary database (retention_evidence.go's
// early/late baseline comparison) -- one implementation, since both are a
// bare *sql.DB and the query is identical either way.
func tableRowCounts(ctx context.Context, conn *sql.DB) (map[string]int64, error) {
	out := make(map[string]int64, len(tableOrder))
	for _, tbl := range tableOrder {
		var n int64
		if err := conn.QueryRowContext(ctx, `SELECT count(*) FROM `+tbl).Scan(&n); err != nil {
			return nil, wrapEnvErr("table row counts", fmt.Errorf("%s: %w", tbl, err))
		}
		out[tbl] = n
	}
	return out, nil
}

// tableOrder is the complete, fixed set of product-owned tables (ARCH-01
// §2) -- every relation the capacity-fixture tablespace is expected to
// hold and every table tableRowCounts/reconciliation checks span.
var tableOrder = []string{
	"submissions", "validation_outcomes", "normalized_events",
	"detection_definitions", "detection_results", "alerts",
}

// logMarker is the exact structured-log substring proving the product
// classified a failure as the distinct resource_exhausted family
// (internal/diagnostics.LogResourceExhausted, internal/worker's
// errorFamily) -- checked by direct substring match against captured
// container logs, matching the same "never text-match the underlying
// driver error, only the product's own deliberate classification"
// discipline the product code itself follows for SQLSTATE matching.
const logMarker = "error_family=resource_exhausted"

// pgDiskFullMarker is the literal PostgreSQL server-log phrase for the
// real ENOSPC condition (confirmed verbatim by the AC-023 empirical
// spike, 2026-08-15, against this exact postgres:16-alpine image) --
// independent, lower-level corroboration that the underlying database
// event genuinely happened, not merely that the product's classifier ran.
const pgDiskFullMarker = "No space left on device"

// ExhaustionOutcome is the complete evidence trail for AC-023's
// resource-exhaustion clause: what was driven, how the condition was
// actually observed, and the before/after state needed to verify no
// partial artifact and no silent loss.
type ExhaustionOutcome struct {
	AttemptsSent         int              `json:"attemptsSent"`
	AttemptsAdmitted     int              `json:"attemptsAdmitted"`
	Exhausted            bool             `json:"exhausted"`
	ObservedVia          string           `json:"observedVia"` // "http_503" | "app_log" | "postgres_log" | "" (not observed)
	AppLogHasMarker      bool             `json:"appLogHasMarker"`
	PostgresLogHasENOSPC bool             `json:"postgresLogHasEnospc"`
	RowCountsBefore      map[string]int64 `json:"rowCountsBefore"`
	RowCountsAfter       map[string]int64 `json:"rowCountsAfter"`
	PlatformUsableAfter  bool             `json:"platformUsableAfter"`
	PlatformUsableDetail string           `json:"platformUsableDetail"`
}

// DriveToExhaustion submits real filler-shaped submissions through the
// disposable fixture app's real POST /v1/audit-events endpoint --
// deliberately paced under the product's approved 10/s admission envelope
// and retrying on 429 (the same backpressure-honoring pattern
// recovery.go's seedOneAdmission already uses), never a synthetic SQL
// fault injection -- until the resource-exhaustion condition is genuinely
// observed, via whichever signal surfaces first: a direct HTTP 503 from
// intake's own synchronous submissions insert, or the distinct
// resource_exhausted classification appearing in the fixture app's own
// logs (which also covers the case where the *worker*, not intake, is
// the one that hits the capped tablespace first on an already-admitted
// submission's later stage -- a real possibility given all six product
// tables share the one constrained tablespace).
func DriveToExhaustion(ctx context.Context, api *APIClient, fixtureConn *sql.DB, compose *Compose, maxAttempts int, budget time.Duration) (*ExhaustionOutcome, error) {
	out := &ExhaustionOutcome{}

	before, err := tableRowCounts(ctx, fixtureConn)
	if err != nil {
		return nil, err
	}
	out.RowCountsBefore = before

	const pace = 150 * time.Millisecond // ~6.7/s, safely under the 10/s envelope
	deadline := time.Now().Add(budget)

	for i := 0; i < maxAttempts && time.Now().Before(deadline); i++ {
		start := time.Now()
		item, _ := NewFillerEvent(1_000_000 + i) // offset well clear of any other sequence this run used
		_, status, retryAfter, postErr := api.PostAuditEvent(ctx, item)
		out.AttemptsSent++

		if postErr == nil {
			out.AttemptsAdmitted++
		} else if status == 429 {
			wait := retryAfter
			if wait <= 0 {
				wait = 250 * time.Millisecond
			}
			sleepUntil(ctx, time.Now().Add(wait))
			continue
		} else if status == 503 {
			out.Exhausted = true
			out.ObservedVia = "http_503"
		}

		// Poll logs after every attempt regardless of HTTP outcome: the
		// worker processes admitted submissions asynchronously, so the
		// exhaustion condition can surface there even while intake keeps
		// returning 200 for new admissions.
		if appLogs, logErr := compose.Logs(ctx, "app-capacity-fixture"); logErr == nil && strings.Contains(appLogs, logMarker) {
			out.AppLogHasMarker = true
			out.Exhausted = true
			if out.ObservedVia == "" {
				out.ObservedVia = "app_log"
			}
		}

		if out.Exhausted {
			break
		}

		if elapsed := time.Since(start); elapsed < pace {
			sleepUntil(ctx, time.Now().Add(pace-elapsed))
		}
	}

	// Final, unconditional log checks -- covers the case the loop above
	// exited on the very attempt that triggered an async worker-side
	// failure whose log line hadn't been flushed/captured yet.
	if appLogs, logErr := compose.Logs(ctx, "app-capacity-fixture"); logErr == nil {
		if strings.Contains(appLogs, logMarker) {
			out.AppLogHasMarker = true
			out.Exhausted = true
			if out.ObservedVia == "" {
				out.ObservedVia = "app_log"
			}
		}
	}
	if pgLogs, logErr := compose.Logs(ctx, "postgres-capacity-fixture"); logErr == nil {
		if strings.Contains(pgLogs, pgDiskFullMarker) {
			out.PostgresLogHasENOSPC = true
			out.Exhausted = true
			if out.ObservedVia == "" {
				out.ObservedVia = "postgres_log"
			}
		}
	}

	after, err := tableRowCounts(ctx, fixtureConn)
	if err != nil {
		return nil, err
	}
	out.RowCountsAfter = after

	var one int
	usableErr := fixtureConn.QueryRowContext(ctx, `SELECT 1`).Scan(&one)
	var inRecovery bool
	recErr := fixtureConn.QueryRowContext(ctx, `SELECT pg_is_in_recovery()`).Scan(&inRecovery)
	out.PlatformUsableAfter = usableErr == nil && one == 1 && recErr == nil && !inRecovery
	if !out.PlatformUsableAfter {
		out.PlatformUsableDetail = fmt.Sprintf("select-1 err=%v pg_is_in_recovery err=%v value=%v", usableErr, recErr, inRecovery)
	} else {
		out.PlatformUsableDetail = "SELECT 1 succeeded and pg_is_in_recovery() = false after the exhaustion attempt"
	}

	return out, nil
}

// RunStorageExhaustionPhase is AC-023's final, destructive phase: bring
// up the profile-gated capacity-fixture services (never started before
// this point -- every other phase's evidence, including retention
// evidence, is already captured against the untouched primary
// postgres/app pair by the time this runs), set up and verify the
// constrained tablespace on the freshly migrated, still-empty fixture
// database, drive it to genuine exhaustion, and tear the fixture back
// down explicitly.
//
// Only a genuine environment failure (compose up, port discovery,
// readiness, or the tablespace DDL itself failing) is returned as a Go
// error, aborting the run -- matching every other phase file's design
// rule. A tablespace-placement mismatch is NOT treated as an environment
// failure: it is itself a reportable AC-023 finding (the
// tablespace_placement_verified clause), so driving traffic is skipped
// (a mis-placed tablespace would make "exhaustion never observed"
// meaningless) but the phase still returns normally with a zero-value
// ExhaustionOutcome, letting the report explain exactly why.
func RunStorageExhaustionPhase(ctx context.Context, compose *Compose, params Params) (*TablespacePlacementReport, *ExhaustionOutcome, error) {
	if err := compose.UpCapacityFixture(ctx); err != nil {
		return nil, nil, err
	}
	defer func() {
		downCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		_ = compose.DownCapacityFixture(downCtx) // best-effort; the final whole-project compose down cleans up regardless
	}()

	pgPort, err := compose.Port(ctx, "postgres-capacity-fixture", 5432)
	if err != nil {
		return nil, nil, err
	}
	appPort, err := compose.Port(ctx, "app-capacity-fixture", 8080)
	if err != nil {
		return nil, nil, err
	}

	fixtureAPI := NewAPIClient("http://127.0.0.1:"+appPort, compose.APIToken())
	if !waitReadyBool(ctx, fixtureAPI, 90*time.Second) {
		return nil, nil, envErrorf("storage exhaustion phase", "capacity-fixture app did not become ready within 90s")
	}

	placement, err := SetupCapacityTablespace(ctx, pgPort, compose.PostgresPassword())
	if err != nil {
		return nil, nil, err
	}
	if !placement.AllInCapacity {
		return placement, &ExhaustionOutcome{}, nil
	}

	fixtureConn, err := cnsdpdb.Connect(ctx, fmt.Sprintf("postgres://cnsdp:%s@127.0.0.1:%s/cnsdp?sslmode=disable", compose.PostgresPassword(), pgPort))
	if err != nil {
		return nil, nil, wrapEnvErr("connect to capacity fixture for exhaustion drive", err)
	}
	defer fixtureConn.Close()

	outcome, err := DriveToExhaustion(ctx, fixtureAPI, fixtureConn, compose, params.CapacityFixtureMaxAttempts, params.CapacityFixtureDriveBudget)
	if err != nil {
		return nil, nil, err
	}
	return placement, outcome, nil
}
