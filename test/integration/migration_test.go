//go:build integration

package integration

import (
	"context"
	"database/sql"
	"testing"

	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"cnsdp/internal/db"
)

func startPostgres(t *testing.T) string {
	t.Helper()
	ctx := context.Background()

	container, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("cnsdp_test"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("test"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Logf("terminate postgres container: %v", err)
		}
	})

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("resolve connection string: %v", err)
	}
	return dsn
}

func migratedConn(t *testing.T) *sql.DB {
	t.Helper()
	dsn := startPostgres(t)
	ctx := context.Background()

	conn, err := db.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	if err := db.RunMigrations(conn); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	return conn
}

func TestMigrations_ApplyExpectedSchema(t *testing.T) {
	conn := migratedConn(t)
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
}

func TestMigrations_StatusCheckConstraintRejectsInvalidValue(t *testing.T) {
	conn := migratedConn(t)
	ctx := context.Background()

	_, err := conn.ExecContext(ctx,
		`INSERT INTO submissions (status, raw_event, audit_id, audit_stage) VALUES ($1, '{}', 'a', 'ResponseComplete')`,
		"not-a-real-status")
	if err == nil {
		t.Fatal("expected the status CHECK constraint to reject an invalid value, got no error")
	}

	_, err = conn.ExecContext(ctx,
		`INSERT INTO submissions (status, raw_event, audit_id, audit_stage) VALUES ($1, '{}', 'a', 'ResponseComplete')`,
		"admitted")
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

	if err := conn.QueryRowContext(ctx,
		`INSERT INTO submissions (raw_event, audit_id, audit_stage) VALUES ('{}', 'a', 'ResponseComplete') RETURNING id`,
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
		`INSERT INTO detection_definitions (scenario, revision, conditions) VALUES ($1, $2, '{}') RETURNING id`,
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
	conn := migratedConn(t)
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
	conn := migratedConn(t)

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
	conn := migratedConn(t)
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
	conn := migratedConn(t)
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
