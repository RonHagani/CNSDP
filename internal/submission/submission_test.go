//go:build integration

package submission

import (
	"context"
	"errors"
	"testing"

	"cnsdp/internal/testutil"
)

func seedSubmission(t *testing.T, db DB, status Status) int64 {
	t.Helper()
	var id int64
	err := db.QueryRowContext(context.Background(),
		`INSERT INTO submissions (status, raw_event, audit_id, audit_stage) VALUES ($1, '{}', 'a', 'ResponseComplete') RETURNING id`,
		string(status)).Scan(&id)
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
