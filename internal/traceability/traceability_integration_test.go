//go:build integration

package traceability

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"testing"

	"cnsdp/internal/submission"
	"cnsdp/internal/testutil"
)

// seedChain inserts one full submission -> normalized_event ->
// detection_result -> alert chain with a correctly derived source_key and
// raw_event_sha256, exactly as the real Admit/Advance stages would have
// produced it, and returns the alert id VerifyAlert is exercised against.
func seedChain(t *testing.T, db *sql.DB, rawEvent, auditID, auditStage string) (alertID, submissionID int64) {
	t.Helper()
	ctx := context.Background()

	key := submission.SourceKey([]byte(rawEvent), auditID, auditStage)
	digest := sha256.Sum256([]byte(rawEvent))
	if err := db.QueryRowContext(ctx,
		`INSERT INTO submissions (raw_event, audit_id, audit_stage, source_key, raw_event_sha256)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		rawEvent, auditID, auditStage, key, digest[:],
	).Scan(&submissionID); err != nil {
		t.Fatalf("seed submission: %v", err)
	}

	var normalizedEventID int64
	if err := db.QueryRowContext(ctx,
		`INSERT INTO normalized_events (submission_id, content, representation_revision) VALUES ($1, '{}', 'v1') RETURNING id`,
		submissionID,
	).Scan(&normalizedEventID); err != nil {
		t.Fatalf("seed normalized event: %v", err)
	}

	var definitionID int64
	if err := db.QueryRowContext(ctx,
		`INSERT INTO detection_definitions (scenario, revision, content) VALUES ('scenario-1', $1, '{}') RETURNING id`,
		testutil.UniqueKey(t),
	).Scan(&definitionID); err != nil {
		t.Fatalf("seed detection definition: %v", err)
	}

	var detectionResultID int64
	if err := db.QueryRowContext(ctx,
		`INSERT INTO detection_results (normalized_event_id, detection_definition_id, matched, match_reason)
		 VALUES ($1, $2, true, '{"note":"ok"}') RETURNING id`,
		normalizedEventID, definitionID,
	).Scan(&detectionResultID); err != nil {
		t.Fatalf("seed detection result: %v", err)
	}

	if err := db.QueryRowContext(ctx,
		`INSERT INTO alerts (detection_result_id, summary) VALUES ($1, '{}') RETURNING id`,
		detectionResultID,
	).Scan(&alertID); err != nil {
		t.Fatalf("seed alert: %v", err)
	}
	return alertID, submissionID
}

func TestVerifyAlert_IntactChain(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	alertID, _ := seedChain(t, db, `{"a":1}`, "audit-1", "ResponseComplete")

	got, err := VerifyAlert(context.Background(), db, alertID)
	if err != nil {
		t.Fatalf("VerifyAlert: %v", err)
	}
	if !got.Intact {
		t.Errorf("Intact = false, want true; FailedLink = %q", got.FailedLink)
	}
}

// TestVerifyAlert_NonexistentAlert_FailsVisibly is the one structural
// "broken link" reachable under normal use, since the FK-enforced schema
// makes any other broken hop impossible to construct via ordinary
// operation (ARCH-01 §4).
func TestVerifyAlert_NonexistentAlert_FailsVisibly(t *testing.T) {
	db := testutil.MigratedPostgres(t)

	got, err := VerifyAlert(context.Background(), db, 999999)
	if err != nil {
		t.Fatalf("VerifyAlert: expected a business-level result, not an error: %v", err)
	}
	if got.Intact {
		t.Error("Intact = true for a nonexistent alert, want false")
	}
	if got.FailedLink != "alert" {
		t.Errorf("FailedLink = %q, want %q", got.FailedLink, "alert")
	}
}

// TestVerifyAlert_RawEventTamperedWithoutDigestUpdate proves NFR-017/
// AC-015's detection case: raw_event changed directly in SQL, out of band,
// without the corresponding raw_event_sha256 being updated to match.
func TestVerifyAlert_RawEventTamperedWithoutDigestUpdate(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	alertID, submissionID := seedChain(t, db, `{"a":1}`, "audit-1", "ResponseComplete")

	if _, err := db.ExecContext(context.Background(),
		`UPDATE submissions SET raw_event = $1 WHERE id = $2`,
		`{"a":999}`, submissionID,
	); err != nil {
		t.Fatalf("tamper raw_event: %v", err)
	}

	got, err := VerifyAlert(context.Background(), db, alertID)
	if err != nil {
		t.Fatalf("VerifyAlert: %v", err)
	}
	if got.Intact {
		t.Error("Intact = true after raw_event was tampered without updating raw_event_sha256, want false")
	}
	if got.FailedLink != "raw_event_sha256" {
		t.Errorf("FailedLink = %q, want %q", got.FailedLink, "raw_event_sha256")
	}
}

// TestVerifyAlert_AuditIdentityChangedWithoutSourceKeyUpdate proves the
// companion detection case: audit_id/audit_stage changed directly in SQL
// without updating source_key to match.
func TestVerifyAlert_AuditIdentityChangedWithoutSourceKeyUpdate(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	alertID, submissionID := seedChain(t, db, `{"a":1}`, "audit-1", "ResponseComplete")

	if _, err := db.ExecContext(context.Background(),
		`UPDATE submissions SET audit_id = 'audit-2' WHERE id = $1`,
		submissionID,
	); err != nil {
		t.Fatalf("tamper audit_id: %v", err)
	}

	got, err := VerifyAlert(context.Background(), db, alertID)
	if err != nil {
		t.Fatalf("VerifyAlert: %v", err)
	}
	if got.Intact {
		t.Error("Intact = true after audit_id was changed without updating source_key, want false")
	}
	if got.FailedLink != "source_key" {
		t.Errorf("FailedLink = %q, want %q", got.FailedLink, "source_key")
	}
}

// --- Locate (Checkpoint 10): the reverse-lookup internal/evidence.Compose
// uses to resolve the ids it needs from an alert id, without reading
// artifact content itself.

func TestLocate_ResolvesChain(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	alertID, submissionID := seedChain(t, db, `{"a":1}`, "audit-1", "ResponseComplete")

	var wantDetectionResultID int64
	if err := db.QueryRowContext(context.Background(),
		`SELECT detection_result_id FROM alerts WHERE id = $1`, alertID,
	).Scan(&wantDetectionResultID); err != nil {
		t.Fatalf("read expected detection_result_id: %v", err)
	}

	got, err := Locate(context.Background(), db, alertID)
	if err != nil {
		t.Fatalf("Locate: %v", err)
	}
	if got.DetectionResultID != wantDetectionResultID {
		t.Errorf("DetectionResultID = %d, want %d", got.DetectionResultID, wantDetectionResultID)
	}
	if got.SubmissionID != submissionID {
		t.Errorf("SubmissionID = %d, want %d", got.SubmissionID, submissionID)
	}
}

func TestLocate_NonexistentAlert(t *testing.T) {
	db := testutil.MigratedPostgres(t)

	_, err := Locate(context.Background(), db, 999999)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Locate: expected ErrNotFound, got %v", err)
	}
}
