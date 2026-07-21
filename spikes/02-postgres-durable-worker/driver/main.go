package main

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// stageDef mirrors worker/stages.go's pipeline shape (name, expected
// artifact-table row counts once fully drained) so the driver can assert
// against it without importing the worker package (kept as two separate
// binaries deliberately, matching "durable admission" and "worker" as
// distinct concerns).
type stageDef struct {
	name          string
	fromStatus    string
	toStatus      string
	artifactTable string
	preCount      int // rows expected in artifactTable for this submission BEFORE this stage runs
	postCount     int // rows expected in artifactTable for this submission AFTER this stage commits
}

var stages = []stageDef{
	{name: "validate", fromStatus: "admitted", toStatus: "validated", artifactTable: "validation_outcomes", preCount: 0, postCount: 1},
	{name: "normalize", fromStatus: "validated", toStatus: "normalized", artifactTable: "normalized_events", preCount: 0, postCount: 1},
	{name: "evaluate", fromStatus: "normalized", toStatus: "evaluated", artifactTable: "detection_results", preCount: 0, postCount: 2},
	{name: "alert", fromStatus: "evaluated", toStatus: "alerted", artifactTable: "alerts", preCount: 0, postCount: 1},
	{name: "evidence", fromStatus: "alerted", toStatus: "evidenced", artifactTable: "traceability_links", preCount: 0, postCount: 3},
}

const crashExitCode = 111

var finalCounts = map[string]int{
	"validation_outcomes": 1,
	"normalized_events":   1,
	"detection_results":   2,
	"alerts":              1,
	"traceability_links":  3,
}

type result struct {
	scenario string
	pass     bool
	detail   string
}

var results []result

func record(scenario string, pass bool, detail string) {
	results = append(results, result{scenario, pass, detail})
	status := "PASS"
	if !pass {
		status = "FAIL"
	}
	fmt.Printf("[%s] %-40s %s\n", status, scenario, detail)
}

func runWorker(args ...string) (exitCode int, stdout, stderr string) {
	cmd := exec.Command("./worker.exe", args...)
	cmd.Env = append(os.Environ(), "SPIKE2_DSN="+dsn())
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	err := cmd.Run()
	code := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		} else {
			code = -1
		}
	}
	return code, out.String(), errBuf.String()
}

func dsn() string {
	return "postgres://postgres:spike@localhost:5433/spike2?sslmode=disable"
}

func seed(db *sql.DB) int {
	code, out, errOut := runWorker("-seed")
	if code != 0 {
		log.Fatalf("seed failed: code=%d stderr=%s", code, errOut)
	}
	var id int
	fmt.Sscanf(out, "%d", &id)
	return id
}

func submissionStatus(ctx context.Context, db *sql.DB, id int) string {
	var status string
	if err := db.QueryRowContext(ctx, `SELECT status FROM submissions WHERE id=$1`, id).Scan(&status); err != nil {
		log.Fatalf("query status: %v", err)
	}
	return status
}

func tableCount(ctx context.Context, db *sql.DB, table string, id int) int {
	var n int

	if table == "alerts" {
		q := `SELECT count(*) FROM alerts a JOIN detection_results dr ON dr.id=a.detection_result_id WHERE dr.submission_id=$1`
		if err := db.QueryRowContext(ctx, q, id).Scan(&n); err != nil {
			log.Fatalf("query count %s: %v", table, err)
		}
		return n
	}

	if table == "traceability_links" {
		// Resolve this submission's specific artifact ids first, then
		// count only links belonging to its own chain — avoids any
		// cross-submission false match against links left behind by
		// earlier scenarios in the same accumulating database.
		var alertID, detResultID, normEventID sql.NullInt64
		err := db.QueryRowContext(ctx, `
			SELECT a.id, dr.id, n.id
			FROM normalized_events n
			LEFT JOIN detection_results dr ON dr.submission_id = n.submission_id AND dr.matched = true
			LEFT JOIN alerts a ON a.detection_result_id = dr.id
			WHERE n.submission_id = $1`, id).Scan(&alertID, &detResultID, &normEventID)
		if err == sql.ErrNoRows || !alertID.Valid || !detResultID.Valid || !normEventID.Valid {
			return 0
		}
		if err != nil {
			log.Fatalf("resolve chain for traceability_links count: %v", err)
		}
		sub := fmt.Sprintf("submission:%d", id)
		norm := fmt.Sprintf("normalized_event:%d", normEventID.Int64)
		det := fmt.Sprintf("detection_result:%d", detResultID.Int64)
		al := fmt.Sprintf("alert:%d", alertID.Int64)
		q := `SELECT count(*) FROM traceability_links
			WHERE (source_ref=$1 AND target_ref=$2)
			   OR (source_ref=$3 AND target_ref=$4)
			   OR (source_ref=$5 AND target_ref=$6)`
		if err := db.QueryRowContext(ctx, q, al, det, al, norm, norm, sub).Scan(&n); err != nil {
			log.Fatalf("query count %s: %v", table, err)
		}
		return n
	}

	q := fmt.Sprintf(`SELECT count(*) FROM %s WHERE submission_id=$1`, table)
	if err := db.QueryRowContext(ctx, q, id).Scan(&n); err != nil {
		log.Fatalf("query count %s: %v", table, err)
	}
	return n
}

func runScenario(ctx context.Context, db *sql.DB, stageIdx int, timing string) {
	st := stages[stageIdx]
	scenario := fmt.Sprintf("stage=%s timing=%s", st.name, timing)

	id := seed(db)

	// One invocation from a fresh ("admitted") submission is enough for
	// every stage/timing combination: the worker loop processes prior
	// stages in order with no crash (the crash spec only matches this
	// stage's name), then crashes exactly at the requested point once it
	// reaches the targeted stage.
	code, _, errOut := runWorker(
		fmt.Sprintf("-crash-submission=%d", id),
		fmt.Sprintf("-crash-stage=%s", st.name),
		fmt.Sprintf("-crash-timing=%s", timing),
	)
	if code != crashExitCode {
		record(scenario, false, fmt.Sprintf("expected crash exit %d, got %d, stderr=%s", crashExitCode, code, errOut))
		return
	}

	if timing == "before" || timing == "mid" {
		assertPreState(ctx, db, id, stageIdx, scenario)
	} else { // "after"
		time.Sleep(200 * time.Millisecond) // let Postgres detect the dropped connection before we touch this row again
		status := submissionStatus(ctx, db, id)
		if status != st.toStatus {
			record(scenario, false, fmt.Sprintf("expected status %q after commit, got %q", st.toStatus, status))
			return
		}
		got := tableCount(ctx, db, st.artifactTable, id)
		if got != st.postCount {
			record(scenario, false, fmt.Sprintf("expected %d row(s) in %s after commit, got %d", st.postCount, st.artifactTable, got))
			return
		}
	}

	resumeAndAssertFinal(ctx, db, id, scenario)
}

func assertPreState(ctx context.Context, db *sql.DB, id int, stageIdx int, scenario string) {
	time.Sleep(200 * time.Millisecond) // let Postgres roll back / detect the dropped connection
	st := stages[stageIdx]
	status := submissionStatus(ctx, db, id)
	if status != st.fromStatus {
		record(scenario, false, fmt.Sprintf("expected status still %q (rolled back), got %q", st.fromStatus, status))
		return
	}
	got := tableCount(ctx, db, st.artifactTable, id)
	if got != st.preCount {
		record(scenario, false, fmt.Sprintf("expected %d row(s) in %s (rolled back), got %d", st.preCount, st.artifactTable, got))
		return
	}
}

func resumeAndAssertFinal(ctx context.Context, db *sql.DB, id int, scenario string) {
	code, _, errOut := runWorker() // no crash flags: drain to completion
	if code != 0 {
		record(scenario, false, fmt.Sprintf("resume run failed: code=%d stderr=%s", code, errOut))
		return
	}
	status := submissionStatus(ctx, db, id)
	if status != "evidenced" {
		record(scenario, false, fmt.Sprintf("expected final status 'evidenced', got %q", status))
		return
	}
	for table, want := range finalCounts {
		got := tableCount(ctx, db, table, id)
		if got != want {
			record(scenario, false, fmt.Sprintf("expected %d row(s) in %s after resume, got %d (possible duplicate)", want, table, got))
			return
		}
	}
	record(scenario, true, "recovered, drained to evidenced, no duplicate artifacts")
}

func runBaseline(ctx context.Context, db *sql.DB) {
	id := seed(db)
	code, _, errOut := runWorker()
	if code != 0 {
		record("baseline (no crash)", false, fmt.Sprintf("code=%d stderr=%s", code, errOut))
		return
	}
	status := submissionStatus(ctx, db, id)
	if status != "evidenced" {
		record("baseline (no crash)", false, fmt.Sprintf("expected evidenced, got %q", status))
		return
	}
	for table, want := range finalCounts {
		got := tableCount(ctx, db, table, id)
		if got != want {
			record("baseline (no crash)", false, fmt.Sprintf("table %s: expected %d got %d", table, want, got))
			return
		}
	}
	record("baseline (no crash)", true, "straight-through run produced exactly the expected artifact counts")
}

func main() {
	ctx := context.Background()
	db, err := sql.Open("pgx", dsn())
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	fmt.Println("=== Spike 2 driver: PostgreSQL durable worker crash-point matrix ===")

	runBaseline(ctx, db)

	for i, st := range stages {
		for _, timing := range []string{"before", "mid", "after"} {
			runScenario(ctx, db, i, timing)
		}
		_ = st
	}

	fmt.Println("\n=== Summary ===")
	passCount := 0
	for _, r := range results {
		if r.pass {
			passCount++
		}
	}
	fmt.Printf("%d/%d scenarios passed\n", passCount, len(results))
	if passCount != len(results) {
		os.Exit(1)
	}
}
