//go:build integration

package submission

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"cnsdp/internal/testutil"
)

func seedSubmission(t *testing.T, db DB, status Status) int64 {
	t.Helper()
	var id int64
	err := db.QueryRowContext(context.Background(),
		`INSERT INTO submissions (status, raw_event, audit_id, audit_stage, source_key) VALUES ($1, '{}', 'a', 'ResponseComplete', $2) RETURNING id`,
		string(status), testutil.UniqueKey(t)).Scan(&id)
	if err != nil {
		t.Fatalf("seed submission with status %s: %v", status, err)
	}
	return id
}

func TestGet_NotFound(t *testing.T) {
	db := testutil.MigratedPostgres(t)

	_, err := Get(context.Background(), db, 999999)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestGet_ReturnsSeededSubmission(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	id := seedSubmission(t, db, StatusAdmitted)

	got, err := Get(context.Background(), db, id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != StatusAdmitted {
		t.Errorf("status = %q, want %q", got.Status, StatusAdmitted)
	}
}

func TestOldestNonTerminal_SelectsOldest(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	first := seedSubmission(t, db, StatusAdmitted)
	seedSubmission(t, db, StatusAdmitted) // a second, newer one

	got, err := OldestNonTerminal(context.Background(), db)
	if err != nil {
		t.Fatalf("oldest non-terminal: %v", err)
	}
	if got.ID != first {
		t.Errorf("selected submission %d, want the oldest (%d)", got.ID, first)
	}
}

func TestOldestNonTerminal_SkipsEvidenced(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	seedSubmission(t, db, StatusEvidenced) // terminal: must never be selected
	admitted := seedSubmission(t, db, StatusAdmitted)

	got, err := OldestNonTerminal(context.Background(), db)
	if err != nil {
		t.Fatalf("oldest non-terminal: %v", err)
	}
	if got.ID != admitted {
		t.Errorf("selected submission %d, want the non-terminal one (%d)", got.ID, admitted)
	}
}

func TestOldestNonTerminal_NoWorkWhenAllEvidenced(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	seedSubmission(t, db, StatusEvidenced)

	_, err := OldestNonTerminal(context.Background(), db)
	if !errors.Is(err, ErrNoWork) {
		t.Fatalf("expected ErrNoWork, got %v", err)
	}
}

func TestAdvance_Succeeds(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	id := seedSubmission(t, db, StatusAdmitted)

	if err := Advance(context.Background(), db, id, StatusAdmitted, StatusValidated); err != nil {
		t.Fatalf("advance: %v", err)
	}

	got, err := Get(context.Background(), db, id)
	if err != nil {
		t.Fatalf("get after advance: %v", err)
	}
	if got.Status != StatusValidated {
		t.Errorf("status = %q, want %q", got.Status, StatusValidated)
	}
}

func TestAdvance_RejectsStaleExpectedStatus(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	id := seedSubmission(t, db, StatusValidated) // already past "admitted"

	err := Advance(context.Background(), db, id, StatusAdmitted, StatusValidated)
	if !errors.Is(err, ErrStatusConflict) {
		t.Fatalf("expected ErrStatusConflict, got %v", err)
	}

	got, getErr := Get(context.Background(), db, id)
	if getErr != nil {
		t.Fatalf("get: %v", getErr)
	}
	if got.Status != StatusValidated {
		t.Errorf("status changed to %q despite a rejected transition", got.Status)
	}
}

func TestAdvance_NotFound(t *testing.T) {
	db := testutil.MigratedPostgres(t)

	err := Advance(context.Background(), db, 999999, StatusAdmitted, StatusValidated)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestAdmit_CreatesAdmittedSubmission(t *testing.T) {
	db := testutil.MigratedPostgres(t)

	id, err := Admit(context.Background(), db, []byte(`{"auditID":"a1"}`), "a1", "ResponseComplete")
	if err != nil {
		t.Fatalf("admit: %v", err)
	}

	got, err := Get(context.Background(), db, id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != StatusAdmitted {
		t.Errorf("status = %q, want %q", got.Status, StatusAdmitted)
	}
	if got.AuditID != "a1" || got.AuditStage != "ResponseComplete" {
		t.Errorf("audit_id/audit_stage = %q/%q, want a1/ResponseComplete", got.AuditID, got.AuditStage)
	}
}

func TestAdmit_AssignsDistinctIDs(t *testing.T) {
	db := testutil.MigratedPostgres(t)

	id1, err := Admit(context.Background(), db, []byte(`{}`), "a1", "ResponseComplete")
	if err != nil {
		t.Fatalf("admit 1: %v", err)
	}
	id2, err := Admit(context.Background(), db, []byte(`{}`), "a2", "ResponseComplete")
	if err != nil {
		t.Fatalf("admit 2: %v", err)
	}
	if id1 == id2 {
		t.Errorf("expected distinct ids, got %d twice", id1)
	}
}

func TestAdmit_RetryReturnsSameID(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	raw := []byte(`{"auditID":"a1","stage":"ResponseComplete"}`)

	id1, err := Admit(context.Background(), db, raw, "a1", "ResponseComplete")
	if err != nil {
		t.Fatalf("first admit: %v", err)
	}
	id2, err := Admit(context.Background(), db, raw, "a1", "ResponseComplete")
	if err != nil {
		t.Fatalf("retry admit: %v", err)
	}
	if id1 != id2 {
		t.Errorf("retry returned id %d, want the original id %d", id2, id1)
	}

	var count int
	if err := db.QueryRowContext(context.Background(),
		`SELECT count(*) FROM submissions WHERE audit_id = 'a1'`,
	).Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != 1 {
		t.Errorf("row count = %d, want 1 (no duplicate on retry)", count)
	}
}

func TestAdmit_DifferentlyFormattedEquivalentJSON_ReturnsErrSourceConflict(t *testing.T) {
	db := testutil.MigratedPostgres(t)

	original := []byte(`{"a":1,"b":2}`)
	id1, err := Admit(context.Background(), db, original, "a1", "ResponseComplete")
	if err != nil {
		t.Fatalf("first admit: %v", err)
	}

	// Same identity (auditID+stage), semantically-equivalent JSON, but
	// different raw bytes (reordered keys, extra whitespace) -- this must
	// be rejected as a conflict, not silently treated as "the same event",
	// even though a JSONB-level comparison would have called it equal.
	reformatted := []byte(`{"b": 2, "a": 1}`)
	_, err = Admit(context.Background(), db, reformatted, "a1", "ResponseComplete")
	if !errors.Is(err, ErrSourceConflict) {
		t.Fatalf("expected ErrSourceConflict, got %v", err)
	}

	got, getErr := Get(context.Background(), db, id1)
	if getErr != nil {
		t.Fatalf("get: %v", getErr)
	}
	if !bytes.Equal(got.RawEvent, original) {
		t.Errorf("raw_event after rejected conflict = %s, want unchanged original %s", got.RawEvent, original)
	}

	var count int
	if err := db.QueryRowContext(context.Background(),
		`SELECT count(*) FROM submissions WHERE audit_id = 'a1'`,
	).Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != 1 {
		t.Errorf("row count = %d, want 1 (no duplicate/second row from the conflicting attempt)", count)
	}
}
