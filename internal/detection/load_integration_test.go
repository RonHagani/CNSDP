//go:build integration

package detection

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"testing/fstest"

	"cnsdp/definitions"
	"cnsdp/internal/testutil"
)

func countDefinitions(t *testing.T, db *sql.DB) int {
	t.Helper()
	var n int
	if err := db.QueryRowContext(context.Background(),
		`SELECT count(*) FROM detection_definitions`).Scan(&n); err != nil {
		t.Fatalf("count detection_definitions: %v", err)
	}
	return n
}

func validYAML(scenario string) []byte {
	return []byte(`
scenario: ` + scenario + `
name: Test
description: Test description.
conditions:
  operation:
    resource: pods
  requires_any:
    - id: a
      description: A
`)
}

// --- Clean load ---

func TestLoadFS_CleanDatabaseReceivesExactlyThreeDefinitions(t *testing.T) {
	db := testutil.MigratedPostgres(t)

	if err := LoadFS(context.Background(), db, definitions.FS); err != nil {
		t.Fatalf("load: %v", err)
	}

	if n := countDefinitions(t, db); n != 3 {
		t.Errorf("row count = %d, want 3", n)
	}

	rows, err := db.QueryContext(context.Background(), `SELECT scenario FROM detection_definitions ORDER BY scenario`)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got = append(got, s)
	}
	want := []string{"scenario-1", "scenario-2", "scenario-3"}
	if len(got) != len(want) {
		t.Fatalf("scenarios = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("scenarios = %v, want %v", got, want)
			break
		}
	}
}

func TestLoadFS_PreservesScenario2Characteristics(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	if err := LoadFS(context.Background(), db, definitions.FS); err != nil {
		t.Fatalf("load: %v", err)
	}

	var content []byte
	if err := db.QueryRowContext(context.Background(),
		`SELECT content FROM detection_definitions WHERE scenario = 'scenario-2'`,
	).Scan(&content); err != nil {
		t.Fatalf("select scenario-2: %v", err)
	}

	var def Definition
	if err := json.Unmarshal(content, &def); err != nil {
		t.Fatalf("unmarshal content: %v", err)
	}

	want := []string{"privileged_container", "host_network", "host_pid", "host_ipc", "host_path_volume"}
	gotIDs := make(map[string]bool)
	for _, c := range def.Conditions.RequiresAny {
		gotIDs[c.ID] = true
	}
	for _, id := range want {
		if !gotIDs[id] {
			t.Errorf("scenario-2 content missing characteristic %q", id)
		}
	}
	if def.Conditions.RequiresOutcome != "success" {
		t.Errorf("scenario-2 requires_outcome = %q, want %q", def.Conditions.RequiresOutcome, "success")
	}
}

// TestLoadFS_StoredColumnsMatchReconstructedContent protects the
// duplicated scenario representation (the scenario column vs. the
// scenario field inside content) from unnoticed drift, and proves the
// re-parse-then-rehash verification path described in RevisionID's doc
// comment actually works.
func TestLoadFS_StoredColumnsMatchReconstructedContent(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	if err := LoadFS(context.Background(), db, definitions.FS); err != nil {
		t.Fatalf("load: %v", err)
	}

	rows, err := db.QueryContext(context.Background(), `SELECT scenario, revision, content FROM detection_definitions`)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()

	checked := 0
	for rows.Next() {
		var scenario, revision string
		var content []byte
		if err := rows.Scan(&scenario, &revision, &content); err != nil {
			t.Fatalf("scan: %v", err)
		}

		var reconstructed Definition
		if err := json.Unmarshal(content, &reconstructed); err != nil {
			t.Fatalf("unmarshal content for %s: %v", scenario, err)
		}

		if reconstructed.Scenario != scenario {
			t.Errorf("stored scenario column %q does not match reconstructed Definition.Scenario %q", scenario, reconstructed.Scenario)
		}

		recanonical, err := reconstructed.Canonical()
		if err != nil {
			t.Fatalf("re-canonicalize %s: %v", scenario, err)
		}
		recomputed := RevisionID(recanonical)
		if recomputed != revision {
			t.Errorf("%s: recomputed revision %q does not match stored revision %q", scenario, recomputed, revision)
		}
		checked++
	}
	if checked != 3 {
		t.Fatalf("checked %d rows, want 3", checked)
	}
}

// --- Idempotency ---

func TestLoadFS_IdempotentAcrossRepeatedCalls(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	ctx := context.Background()

	if err := LoadFS(ctx, db, definitions.FS); err != nil {
		t.Fatalf("first load: %v", err)
	}
	firstIDs := definitionIDs(t, db)

	if err := LoadFS(ctx, db, definitions.FS); err != nil {
		t.Fatalf("second load: %v", err)
	}
	secondIDs := definitionIDs(t, db)

	if n := countDefinitions(t, db); n != 3 {
		t.Errorf("row count after second load = %d, want 3", n)
	}
	if len(firstIDs) != len(secondIDs) {
		t.Fatalf("row id sets differ in size: %v vs %v", firstIDs, secondIDs)
	}
	for id := range firstIDs {
		if !secondIDs[id] {
			t.Errorf("row id %d present after first load is missing after second load (not a true no-op)", id)
		}
	}
}

func definitionIDs(t *testing.T, db *sql.DB) map[int64]bool {
	t.Helper()
	rows, err := db.QueryContext(context.Background(), `SELECT id FROM detection_definitions`)
	if err != nil {
		t.Fatalf("query ids: %v", err)
	}
	defer rows.Close()
	out := make(map[int64]bool)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan id: %v", err)
		}
		out[id] = true
	}
	return out
}

func TestLoadFS_ChangedContentAddsNewRevisionWithoutTouchingOld(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	ctx := context.Background()

	// Simulate a previous run that stored an older revision of scenario-1.
	oldContent := []byte(`{"scenario":"scenario-1","name":"Old Name","description":"old","conditions":{"operation":{"resource":"pods"},"requires_any":[{"id":"a","description":"A"}]}}`)
	const oldRevision = "old-simulated-revision"
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin seed tx: %v", err)
	}
	defer tx.Rollback()
	if err := insertDefinition(ctx, tx, "scenario-1", oldRevision, oldContent); err != nil {
		t.Fatalf("seed old revision: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit seed tx: %v", err)
	}

	if err := LoadFS(ctx, db, definitions.FS); err != nil {
		t.Fatalf("load: %v", err)
	}

	// scenario-1 must now have two rows: the untouched simulated old
	// revision, and the real current one -- never overwritten.
	var scenario1Count int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM detection_definitions WHERE scenario = 'scenario-1'`).Scan(&scenario1Count); err != nil {
		t.Fatalf("count scenario-1: %v", err)
	}
	if scenario1Count != 2 {
		t.Errorf("scenario-1 row count = %d, want 2 (old simulated + new real)", scenario1Count)
	}

	var storedOldContent []byte
	if err := db.QueryRowContext(ctx,
		`SELECT content FROM detection_definitions WHERE scenario = 'scenario-1' AND revision = $1`, oldRevision,
	).Scan(&storedOldContent); err != nil {
		t.Fatalf("the simulated old revision must remain untouched: %v", err)
	}
}

// --- Cross-file validation ---

func TestLoadFS_RejectsDuplicateScenarioAcrossFiles(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	fsys := fstest.MapFS{
		"a.yaml": {Data: validYAML("scenario-1")},
		"b.yaml": {Data: validYAML("scenario-1")}, // duplicate
		"c.yaml": {Data: validYAML("scenario-2")},
	}

	if err := LoadFS(context.Background(), db, fsys); err == nil {
		t.Fatal("expected an error for a duplicate scenario across files")
	}
	if n := countDefinitions(t, db); n != 0 {
		t.Errorf("row count = %d, want 0 (nothing should be written)", n)
	}
}

func TestLoadFS_RejectsMissingScenario(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	fsys := fstest.MapFS{
		"a.yaml": {Data: validYAML("scenario-1")},
		"b.yaml": {Data: validYAML("scenario-2")},
		// scenario-3 missing
	}

	if err := LoadFS(context.Background(), db, fsys); err == nil {
		t.Fatal("expected an error for a missing required scenario")
	}
	if n := countDefinitions(t, db); n != 0 {
		t.Errorf("row count = %d, want 0 (nothing should be written)", n)
	}
}

// --- Transaction proof, Category A: pre-write validation gate ---
//
// This proves that nothing is written when a file fails to parse or
// validate. It is NOT evidence of transaction rollback: no transaction is
// ever opened for this failure path, since the failure occurs during
// in-memory validation before BeginTx is called.
func TestLoadFS_PreWriteValidationGate_NothingWrittenOnInvalidFile(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	fsys := fstest.MapFS{
		"a.yaml": {Data: validYAML("scenario-1")},
		"b.yaml": {Data: validYAML("scenario-2")},
		"c.yaml": {Data: []byte(`
scenario: scenario-3
description: Missing name field.
conditions:
  operation:
    resource: clusterrolebindings
  requires_all:
    - id: x
      description: y
`)}, // invalid: name is required
	}

	if err := LoadFS(context.Background(), db, fsys); err == nil {
		t.Fatal("expected an error for the invalid third file")
	}
	if n := countDefinitions(t, db); n != 0 {
		t.Errorf("row count = %d, want 0 (pre-write validation must prevent any write)", n)
	}
}

// --- Transaction proof, Category B: genuine PostgreSQL rollback ---
//
// This is the primary proof of database rollback: a real insert succeeds
// inside an open transaction, a second, deliberately invalid statement is
// then rejected by PostgreSQL itself (a NOT NULL violation -- not a
// duplicate (scenario, revision), which ON CONFLICT DO NOTHING would
// silently absorb rather than reject), and rolling back must undo the
// earlier successful insert too.
func TestLoadFS_RealPostgresFailureRollsBackTransaction(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	ctx := context.Background()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback() // safety net; the explicit Rollback below may make this a no-op

	if err := insertDefinition(ctx, tx, "scenario-1", "rev-test", []byte(`{"scenario":"scenario-1"}`)); err != nil {
		t.Fatalf("expected the first insert to succeed: %v", err)
	}

	// Deliberately invalid, issued directly: content violates the NOT
	// NULL constraint, a genuine PostgreSQL-level rejection (23502).
	_, err = tx.ExecContext(ctx,
		`INSERT INTO detection_definitions (scenario, revision, content) VALUES ($1, $2, NULL)`,
		"scenario-2", "rev-test-2")
	if err == nil {
		t.Fatal("expected PostgreSQL to reject a NULL content value")
	}

	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	if n := countDefinitions(t, db); n != 0 {
		t.Errorf("row count after rollback = %d, want 0 (the earlier successful insert must also be undone)", n)
	}
}

// TestLoadFS_ContextCancellationAlsoAborts is a secondary test of a
// different, client-side failure mode. It supplements, but does not
// replace, TestLoadFS_RealPostgresFailureRollsBackTransaction as the
// primary evidence of rollback.
func TestLoadFS_ContextCancellationAlsoAborts(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	ctx := context.Background()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback() // safety net; the explicit Rollback below may make this a no-op

	if err := insertDefinition(ctx, tx, "scenario-1", "rev-cancel-test", []byte(`{"scenario":"scenario-1"}`)); err != nil {
		t.Fatalf("expected the first insert to succeed: %v", err)
	}

	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := insertDefinition(cancelledCtx, tx, "scenario-2", "rev-cancel-test-2", []byte(`{"scenario":"scenario-2"}`)); err == nil {
		t.Fatal("expected the second insert to fail against a cancelled context")
	}

	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	if n := countDefinitions(t, db); n != 0 {
		t.Errorf("row count after rollback = %d, want 0", n)
	}
}
