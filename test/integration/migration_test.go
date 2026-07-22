//go:build integration

package integration

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"testing"

	"cnsdp/internal/db"
	"cnsdp/internal/testutil"
)

func TestMigrations_ApplyExpectedSchema(t *testing.T) {
	conn := testutil.MigratedPostgres(t)
	ctx := context.Background()

	wantTables := []string{
		"submissions",
		"validation_outcomes",
		"normalized_events",
		"detection_definitions",
		"detection_results",
		"alerts",
	}
	for _, table := range wantTables {
		var exists bool
		err := conn.QueryRowContext(ctx,
			`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema='public' AND table_name=$1)`,
			table).Scan(&exists)
		if err != nil {
			t.Fatalf("check table %s: %v", table, err)
		}
		if !exists {
			t.Errorf("expected table %s to exist after migration", table)
		}
	}

	// traceability_links was deliberately removed: the chain is fully
	// derivable through detection_results -> normalized_events ->
	// submissions, so a separate persisted copy would itself be a
	// redundant, independently-mutable relationship. Locking this in as a
	// regression check, not just an assumption.
	var tracTableExists bool
	if err := conn.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema='public' AND table_name='traceability_links')`,
	).Scan(&tracTableExists); err != nil {
		t.Fatalf("check traceability_links absence: %v", err)
	}
	if tracTableExists {
		t.Error("traceability_links must not exist: the chain is derived via foreign keys, not a redundant table")
	}

	// detection_results must not carry a submission_id column: submission
	// is derived solely through normalized_event_id.
	var detectionResultsHasSubmissionID bool
	if err := conn.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name='detection_results' AND column_name='submission_id')`,
	).Scan(&detectionResultsHasSubmissionID); err != nil {
		t.Fatalf("check detection_results columns: %v", err)
	}
	if detectionResultsHasSubmissionID {
		t.Error("detection_results.submission_id must not exist: it duplicated normalized_events.submission_id and could disagree with it")
	}

	// raw_event must be BYTEA (exact source bytes), not JSONB
	// (semantically-canonicalizing) -- migration 0002.
	var rawEventType string
	if err := conn.QueryRowContext(ctx,
		`SELECT data_type FROM information_schema.columns WHERE table_schema='public' AND table_name='submissions' AND column_name='raw_event'`,
	).Scan(&rawEventType); err != nil {
		t.Fatalf("check submissions.raw_event type: %v", err)
	}
	if rawEventType != "bytea" {
		t.Errorf("submissions.raw_event data_type = %q, want bytea", rawEventType)
	}

	// source_key must exist, be NOT NULL, and be UNIQUE -- migration 0002.
	var sourceKeyNullable string
	if err := conn.QueryRowContext(ctx,
		`SELECT is_nullable FROM information_schema.columns WHERE table_schema='public' AND table_name='submissions' AND column_name='source_key'`,
	).Scan(&sourceKeyNullable); err != nil {
		t.Fatalf("check submissions.source_key existence/nullability: %v", err)
	}
	if sourceKeyNullable != "NO" {
		t.Errorf("submissions.source_key is_nullable = %q, want NO", sourceKeyNullable)
	}

	// raw_event_sha256 must exist, be NOT NULL, and be BYTEA -- migration 0003.
	var digestNullable, digestType string
	if err := conn.QueryRowContext(ctx,
		`SELECT is_nullable, data_type FROM information_schema.columns WHERE table_schema='public' AND table_name='submissions' AND column_name='raw_event_sha256'`,
	).Scan(&digestNullable, &digestType); err != nil {
		t.Fatalf("check submissions.raw_event_sha256 existence/nullability/type: %v", err)
	}
	if digestNullable != "NO" {
		t.Errorf("submissions.raw_event_sha256 is_nullable = %q, want NO", digestNullable)
	}
	if digestType != "bytea" {
		t.Errorf("submissions.raw_event_sha256 data_type = %q, want bytea", digestType)
	}

	definitionID := insertDefinition(t, conn, "scenario-1", "rev-schema-check")
	submissionID, _, _, _ := seedChain(t, conn, definitionID)
	var existingKey string
	if err := conn.QueryRowContext(ctx, `SELECT source_key FROM submissions WHERE id = $1`, submissionID).Scan(&existingKey); err != nil {
		t.Fatalf("read seeded source_key: %v", err)
	}
	dupDigest := sha256.Sum256([]byte("{}"))
	if _, err := conn.ExecContext(ctx,
		`INSERT INTO submissions (raw_event, audit_id, audit_stage, source_key, raw_event_sha256) VALUES ('{}', 'b', 'ResponseComplete', $1, $2)`,
		existingKey, dupDigest[:],
	); err == nil {
		t.Error("expected a duplicate submissions.source_key to be rejected by the UNIQUE constraint")
	}
}

// TestMigration0002_DownPreservesJSONSemanticsAndUpRestoresBytea drives
// migration 0002's down/up reversibility directly, rather than only
// asserting the end-state schema: it seeds a row with deliberately
// non-canonically-formatted JSON, steps down two versions (across 0003 --
// which depends on raw_event already being BYTEA -- and then 0002
// itself), confirms the column reverted to JSONB with the same JSON
// content (not the same bytes -- JSONB canonicalizes on storage, so
// byte-for-byte preservation across this lossy rollback is not claimed),
// steps back up two versions, and confirms raw_event is BYTEA again and
// source_key and raw_event_sha256 both still exist, are NOT NULL, and
// source_key is still UNIQUE.
func TestMigration0002_DownPreservesJSONSemanticsAndUpRestoresBytea(t *testing.T) {
	conn := testutil.MigratedPostgres(t)
	ctx := context.Background()

	const original = `{"b": 2, "a": 1}` // unsorted keys, internal spacing
	digest := sha256.Sum256([]byte(original))
	var id int64
	if err := conn.QueryRowContext(ctx,
		`INSERT INTO submissions (raw_event, audit_id, audit_stage, source_key, raw_event_sha256) VALUES ($1, 'a', 'ResponseComplete', $2, $3) RETURNING id`,
		[]byte(original), testutil.UniqueKey(t), digest[:],
	).Scan(&id); err != nil {
		t.Fatalf("seed row: %v", err)
	}

	m, err := db.NewMigrator(conn)
	if err != nil {
		t.Fatalf("construct migrator: %v", err)
	}

	if err := m.Steps(-2); err != nil {
		t.Fatalf("migrate down across 0003 and 0002: %v", err)
	}

	dataType := columnDataType(t, conn, "submissions", "raw_event")
	if dataType != "jsonb" {
		t.Errorf("raw_event data_type after down = %q, want jsonb", dataType)
	}

	var storedAfterDown string
	if err := conn.QueryRowContext(ctx,
		`SELECT raw_event::text FROM submissions WHERE id = $1`, id,
	).Scan(&storedAfterDown); err != nil {
		t.Fatalf("read raw_event after down: %v", err)
	}
	assertSameJSONContent(t, "after down", storedAfterDown, original)

	if err := m.Steps(2); err != nil {
		t.Fatalf("migrate up across 0002 and 0003 again: %v", err)
	}

	dataType = columnDataType(t, conn, "submissions", "raw_event")
	if dataType != "bytea" {
		t.Errorf("raw_event data_type after re-up = %q, want bytea", dataType)
	}

	var storedAfterUp []byte
	if err := conn.QueryRowContext(ctx,
		`SELECT raw_event FROM submissions WHERE id = $1`, id,
	).Scan(&storedAfterUp); err != nil {
		t.Fatalf("read raw_event after re-up: %v", err)
	}
	assertSameJSONContent(t, "after re-up", string(storedAfterUp), original)

	var sourceKeyNullable string
	if err := conn.QueryRowContext(ctx,
		`SELECT is_nullable FROM information_schema.columns WHERE table_schema='public' AND table_name='submissions' AND column_name='source_key'`,
	).Scan(&sourceKeyNullable); err != nil {
		t.Fatalf("check source_key existence/nullability after re-migration: %v", err)
	}
	if sourceKeyNullable != "NO" {
		t.Errorf("source_key is_nullable after re-migration = %q, want NO", sourceKeyNullable)
	}

	var digestNullable string
	if err := conn.QueryRowContext(ctx,
		`SELECT is_nullable FROM information_schema.columns WHERE table_schema='public' AND table_name='submissions' AND column_name='raw_event_sha256'`,
	).Scan(&digestNullable); err != nil {
		t.Fatalf("check raw_event_sha256 existence/nullability after re-migration: %v", err)
	}
	if digestNullable != "NO" {
		t.Errorf("raw_event_sha256 is_nullable after re-migration = %q, want NO", digestNullable)
	}

	var existingKey string
	if err := conn.QueryRowContext(ctx, `SELECT source_key FROM submissions WHERE id = $1`, id).Scan(&existingKey); err != nil {
		t.Fatalf("read existing source_key: %v", err)
	}
	dupDigest := sha256.Sum256([]byte("{}"))
	if _, err := conn.ExecContext(ctx,
		`INSERT INTO submissions (raw_event, audit_id, audit_stage, source_key, raw_event_sha256) VALUES ('{}', 'b', 'ResponseComplete', $1, $2)`,
		existingKey, dupDigest[:],
	); err == nil {
		t.Error("expected a duplicate source_key to still be rejected by the UNIQUE constraint after the down/up round trip")
	}
}

// TestMigration0003_DownDropsColumnAndUpRestoresBackfilled drives
// migration 0003's own down/up reversibility: it seeds a row (present
// before the round trip, so the up-migration's backfill path is
// exercised on re-apply, not just its bare ALTER TABLE ADD COLUMN),
// steps down one version (undoing 0003 only -- 0002 remains applied, so
// raw_event stays BYTEA throughout), confirms raw_event_sha256 is gone,
// steps back up, and confirms the column is restored, NOT NULL, and
// correctly backfilled to sha256(raw_event) for the pre-existing row.
func TestMigration0003_DownDropsColumnAndUpRestoresBackfilled(t *testing.T) {
	conn := testutil.MigratedPostgres(t)
	ctx := context.Background()

	const rawEvent = `{"c":3}`
	digest := sha256.Sum256([]byte(rawEvent))
	var id int64
	if err := conn.QueryRowContext(ctx,
		`INSERT INTO submissions (raw_event, audit_id, audit_stage, source_key, raw_event_sha256) VALUES ($1, 'a', 'ResponseComplete', $2, $3) RETURNING id`,
		[]byte(rawEvent), testutil.UniqueKey(t), digest[:],
	).Scan(&id); err != nil {
		t.Fatalf("seed row: %v", err)
	}

	m, err := db.NewMigrator(conn)
	if err != nil {
		t.Fatalf("construct migrator: %v", err)
	}

	if err := m.Steps(-1); err != nil {
		t.Fatalf("migrate down across 0003: %v", err)
	}

	var columnExists bool
	if err := conn.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name='submissions' AND column_name='raw_event_sha256')`,
	).Scan(&columnExists); err != nil {
		t.Fatalf("check raw_event_sha256 absence after down: %v", err)
	}
	if columnExists {
		t.Error("raw_event_sha256 still exists after migrating down across 0003")
	}

	dataType := columnDataType(t, conn, "submissions", "raw_event")
	if dataType != "bytea" {
		t.Errorf("raw_event data_type after down across 0003 only = %q, want unchanged bytea (0002 remains applied)", dataType)
	}

	if err := m.Steps(1); err != nil {
		t.Fatalf("migrate up across 0003 again: %v", err)
	}

	var digestNullable, digestType string
	if err := conn.QueryRowContext(ctx,
		`SELECT is_nullable, data_type FROM information_schema.columns WHERE table_schema='public' AND table_name='submissions' AND column_name='raw_event_sha256'`,
	).Scan(&digestNullable, &digestType); err != nil {
		t.Fatalf("check raw_event_sha256 existence/nullability/type after re-up: %v", err)
	}
	if digestNullable != "NO" {
		t.Errorf("raw_event_sha256 is_nullable after re-up = %q, want NO", digestNullable)
	}
	if digestType != "bytea" {
		t.Errorf("raw_event_sha256 data_type after re-up = %q, want bytea", digestType)
	}

	var backfilled []byte
	if err := conn.QueryRowContext(ctx,
		`SELECT raw_event_sha256 FROM submissions WHERE id = $1`, id,
	).Scan(&backfilled); err != nil {
		t.Fatalf("read raw_event_sha256 after re-up: %v", err)
	}
	if !bytes.Equal(backfilled, digest[:]) {
		t.Errorf("raw_event_sha256 after re-up backfill = %x, want %x (sha256 of the pre-existing raw_event)", backfilled, digest)
	}
}

// TestMigrations_RawEventSHA256CheckConstraint proves the digest-shape
// enforcement at the database boundary (migration 0003): a 32-byte digest
// is accepted, and both a too-short and a too-long digest are rejected by
// the octet_length CHECK constraint, independent of any application code
// path.
func TestMigrations_RawEventSHA256CheckConstraint(t *testing.T) {
	conn := testutil.MigratedPostgres(t)
	ctx := context.Background()

	if _, err := conn.ExecContext(ctx,
		`INSERT INTO submissions (raw_event, audit_id, audit_stage, source_key, raw_event_sha256) VALUES ('{}', 'a', 'ResponseComplete', $1, $2)`,
		testutil.UniqueKey(t), make([]byte, 32),
	); err != nil {
		t.Fatalf("expected a 32-byte digest to be accepted, got: %v", err)
	}

	if _, err := conn.ExecContext(ctx,
		`INSERT INTO submissions (raw_event, audit_id, audit_stage, source_key, raw_event_sha256) VALUES ('{}', 'a', 'ResponseComplete', $1, $2)`,
		testutil.UniqueKey(t), make([]byte, 31),
	); err == nil {
		t.Error("expected a 31-byte digest to be rejected by the octet_length CHECK constraint")
	}

	if _, err := conn.ExecContext(ctx,
		`INSERT INTO submissions (raw_event, audit_id, audit_stage, source_key, raw_event_sha256) VALUES ('{}', 'a', 'ResponseComplete', $1, $2)`,
		testutil.UniqueKey(t), make([]byte, 33),
	); err == nil {
		t.Error("expected a 33-byte digest to be rejected by the octet_length CHECK constraint")
	}
}

func columnDataType(t *testing.T, conn *sql.DB, table, column string) string {
	t.Helper()
	var dataType string
	if err := conn.QueryRowContext(context.Background(),
		`SELECT data_type FROM information_schema.columns WHERE table_schema='public' AND table_name=$1 AND column_name=$2`,
		table, column,
	).Scan(&dataType); err != nil {
		t.Fatalf("check %s.%s type: %v", table, column, err)
	}
	return dataType
}

// assertSameJSONContent asserts got is valid JSON with the same key/value
// content as want -- structural/semantic equivalence only, never
// byte-for-byte, since JSONB canonicalizes whitespace and key order.
func assertSameJSONContent(t *testing.T, label, got, want string) {
	t.Helper()
	var gotVal, wantVal map[string]any
	if err := json.Unmarshal([]byte(got), &gotVal); err != nil {
		t.Fatalf("%s: raw_event is not valid JSON: %v (%s)", label, err, got)
	}
	if err := json.Unmarshal([]byte(want), &wantVal); err != nil {
		t.Fatalf("%s: test's own expected JSON is invalid: %v", label, err)
	}
	if len(gotVal) != len(wantVal) {
		t.Fatalf("%s: raw_event = %v, want the same content as %v", label, gotVal, wantVal)
	}
	for k, wv := range wantVal {
		if gv, ok := gotVal[k]; !ok || gv != wv {
			t.Errorf("%s: raw_event[%q] = %v, want %v", label, k, gv, wv)
		}
	}
}

func TestMigrations_StatusCheckConstraintRejectsInvalidValue(t *testing.T) {
	conn := testutil.MigratedPostgres(t)
	ctx := context.Background()
	digest := sha256.Sum256([]byte("{}"))

	_, err := conn.ExecContext(ctx,
		`INSERT INTO submissions (status, raw_event, audit_id, audit_stage, source_key, raw_event_sha256) VALUES ($1, '{}', 'a', 'ResponseComplete', $2, $3)`,
		"not-a-real-status", testutil.UniqueKey(t), digest[:])
	if err == nil {
		t.Fatal("expected the status CHECK constraint to reject an invalid value, got no error")
	}

	_, err = conn.ExecContext(ctx,
		`INSERT INTO submissions (status, raw_event, audit_id, audit_stage, source_key, raw_event_sha256) VALUES ($1, '{}', 'a', 'ResponseComplete', $2, $3)`,
		"admitted", testutil.UniqueKey(t), digest[:])
	if err != nil {
		t.Fatalf("expected a valid status value to be accepted, got: %v", err)
	}
}

// seedChain inserts one full submission -> normalized_event -> detection_result
// -> alert chain and returns each artifact's id, for tests that need a
// complete, realistic chain to work with.
func seedChain(t *testing.T, conn *sql.DB, definitionID int64) (submissionID, normalizedEventID, detectionResultID, alertID int64) {
	t.Helper()
	ctx := context.Background()
	digest := sha256.Sum256([]byte("{}"))

	if err := conn.QueryRowContext(ctx,
		`INSERT INTO submissions (raw_event, audit_id, audit_stage, source_key, raw_event_sha256) VALUES ('{}', 'a', 'ResponseComplete', $1, $2) RETURNING id`,
		testutil.UniqueKey(t), digest[:],
	).Scan(&submissionID); err != nil {
		t.Fatalf("insert submission: %v", err)
	}
	if err := conn.QueryRowContext(ctx,
		`INSERT INTO normalized_events (submission_id, content, representation_revision) VALUES ($1, '{}', 'v0') RETURNING id`,
		submissionID).Scan(&normalizedEventID); err != nil {
		t.Fatalf("insert normalized event: %v", err)
	}
	if err := conn.QueryRowContext(ctx,
		`INSERT INTO detection_results (normalized_event_id, detection_definition_id, matched, match_reason)
		 VALUES ($1, $2, true, '{"note":"ok"}') RETURNING id`,
		normalizedEventID, definitionID).Scan(&detectionResultID); err != nil {
		t.Fatalf("insert detection result: %v", err)
	}
	if err := conn.QueryRowContext(ctx,
		`INSERT INTO alerts (detection_result_id, summary) VALUES ($1, '{}') RETURNING id`,
		detectionResultID).Scan(&alertID); err != nil {
		t.Fatalf("insert alert: %v", err)
	}
	return
}

func insertDefinition(t *testing.T, conn *sql.DB, scenario, revision string) int64 {
	t.Helper()
	ctx := context.Background()
	var id int64
	if err := conn.QueryRowContext(ctx,
		`INSERT INTO detection_definitions (scenario, revision, content) VALUES ($1, $2, '{}') RETURNING id`,
		scenario, revision).Scan(&id); err != nil {
		t.Fatalf("insert detection definition: %v", err)
	}
	return id
}

// resolveSubmissionForAlert performs the canonical chain-resolution join
// (alert -> detection_result -> normalized_event -> submission) that a
// future chain verifier would use.
func resolveSubmissionForAlert(t *testing.T, conn *sql.DB, alertID int64) int64 {
	t.Helper()
	ctx := context.Background()
	var submissionID int64
	err := conn.QueryRowContext(ctx, `
		SELECT s.id
		FROM alerts a
		JOIN detection_results dr ON dr.id = a.detection_result_id
		JOIN normalized_events ne ON ne.id = dr.normalized_event_id
		JOIN submissions s ON s.id = ne.submission_id
		WHERE a.id = $1`, alertID).Scan(&submissionID)
	if err != nil {
		t.Fatalf("resolve chain for alert %d: %v", alertID, err)
	}
	return submissionID
}

// TestForeignKeys_PreventDeletionOfReferencedArtifact proves that Postgres
// itself refuses to delete any artifact still referenced by a later stage
// of an existing chain, at every link: submission <- normalized_event,
// normalized_event <- detection_result, detection_result <- alert. This is
// what makes an already-established alert-to-source chain durable against
// an out-of-band deletion, not just against a mismatched insert.
func TestForeignKeys_PreventDeletionOfReferencedArtifact(t *testing.T) {
	conn := testutil.MigratedPostgres(t)
	ctx := context.Background()

	definitionID := insertDefinition(t, conn, "scenario-1", "rev1")
	submissionID, normalizedEventID, detectionResultID, alertID := seedChain(t, conn, definitionID)
	_ = alertID

	t.Run("submission referenced by normalized_event", func(t *testing.T) {
		_, err := conn.ExecContext(ctx, `DELETE FROM submissions WHERE id = $1`, submissionID)
		if err == nil {
			t.Fatal("expected deleting a submission still referenced by a normalized_event to be rejected")
		}
	})

	t.Run("normalized_event referenced by detection_result", func(t *testing.T) {
		_, err := conn.ExecContext(ctx, `DELETE FROM normalized_events WHERE id = $1`, normalizedEventID)
		if err == nil {
			t.Fatal("expected deleting a normalized_event still referenced by a detection_result to be rejected")
		}
	})

	t.Run("detection_result referenced by alert", func(t *testing.T) {
		_, err := conn.ExecContext(ctx, `DELETE FROM detection_results WHERE id = $1`, detectionResultID)
		if err == nil {
			t.Fatal("expected deleting a detection_result still referenced by an alert to be rejected")
		}
	})

	// The chain must still resolve intact after every rejected deletion attempt.
	if got := resolveSubmissionForAlert(t, conn, alertID); got != submissionID {
		t.Errorf("chain resolved to submission %d after rejected deletions, want %d", got, submissionID)
	}
}

// TestChainResolution_NeverCrossesBetweenSubmissions is the requested proof
// that a cross-submission mismatch cannot occur. detection_results no
// longer has a submission_id column to desync (see the schema comment and
// TestMigrations_ApplyExpectedSchema), so there is nothing to "attempt" and
// nothing for Postgres to reject at insert time — the invalid state has no
// representation. What this test instead proves is the positive property
// that removing the column was meant to guarantee: with two independent
// chains present in the same database at once, each alert's resolved
// chain always, unambiguously, resolves back to its own submission and
// never to the other one.
func TestChainResolution_NeverCrossesBetweenSubmissions(t *testing.T) {
	conn := testutil.MigratedPostgres(t)

	definitionID := insertDefinition(t, conn, "scenario-1", "rev1")

	submissionA, _, _, alertA := seedChain(t, conn, definitionID)
	submissionB, _, _, alertB := seedChain(t, conn, definitionID)

	if submissionA == submissionB {
		t.Fatalf("test setup error: expected two distinct submissions, got the same id twice (%d)", submissionA)
	}

	if got := resolveSubmissionForAlert(t, conn, alertA); got != submissionA {
		t.Errorf("alert A resolved to submission %d, want its own submission %d", got, submissionA)
	}
	if got := resolveSubmissionForAlert(t, conn, alertB); got != submissionB {
		t.Errorf("alert B resolved to submission %d, want its own submission %d", got, submissionB)
	}
}

// TestDetectionResults_UniqueConstraintRejectsDuplicateEvaluation proves the
// constraint that makes idempotent, conflict-safe re-insertion possible
// (ADR-0002): a second detection_results row for the same normalized event
// and detection definition must be rejected by Postgres.
func TestDetectionResults_UniqueConstraintRejectsDuplicateEvaluation(t *testing.T) {
	conn := testutil.MigratedPostgres(t)
	ctx := context.Background()

	definitionID := insertDefinition(t, conn, "scenario-1", "rev1")
	_, normalizedEventID, _, _ := seedChain(t, conn, definitionID)

	_, err := conn.ExecContext(ctx,
		`INSERT INTO detection_results (normalized_event_id, detection_definition_id, matched, match_reason)
		 VALUES ($1, $2, true, '{"note":"duplicate"}')`,
		normalizedEventID, definitionID)
	if err == nil {
		t.Fatal("expected a second detection_result for the same (normalized_event, definition) pair to be rejected")
	}
}

func TestMigrations_MatchReasonCheckConstraint(t *testing.T) {
	conn := testutil.MigratedPostgres(t)
	ctx := context.Background()

	definitionID := insertDefinition(t, conn, "scenario-1", "rev1")
	_, normalizedEventID, _, _ := seedChain(t, conn, definitionID)

	definitionID2 := insertDefinition(t, conn, "scenario-2", "rev1")

	// matched = true with a NULL match_reason must be rejected.
	_, err := conn.ExecContext(ctx,
		`INSERT INTO detection_results (normalized_event_id, detection_definition_id, matched, match_reason)
		 VALUES ($1, $2, true, NULL)`,
		normalizedEventID, definitionID2)
	if err == nil {
		t.Fatal("expected matched=true with NULL match_reason to be rejected")
	}

	// matched = false with a non-NULL match_reason must be rejected.
	_, err = conn.ExecContext(ctx,
		`INSERT INTO detection_results (normalized_event_id, detection_definition_id, matched, match_reason)
		 VALUES ($1, $2, false, '{"note":"should not be allowed"}')`,
		normalizedEventID, definitionID2)
	if err == nil {
		t.Fatal("expected matched=false with a non-NULL match_reason to be rejected")
	}

	// matched = false with no match_reason must succeed.
	_, err = conn.ExecContext(ctx,
		`INSERT INTO detection_results (normalized_event_id, detection_definition_id, matched, match_reason)
		 VALUES ($1, $2, false, NULL)`,
		normalizedEventID, definitionID2)
	if err != nil {
		t.Fatalf("expected matched=false with no match_reason to be accepted, got: %v", err)
	}
}
