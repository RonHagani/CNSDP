//go:build integration

package submission

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"testing"

	"cnsdp/internal/testutil"
)

func seedSubmission(t *testing.T, db DB, status Status) int64 {
	t.Helper()
	digest := sha256.Sum256([]byte("{}"))
	var id int64
	err := db.QueryRowContext(context.Background(),
		`INSERT INTO submissions (status, raw_event, audit_id, audit_stage, source_key, raw_event_sha256) VALUES ($1, '{}', 'a', 'ResponseComplete', $2, $3) RETURNING id`,
		string(status), testutil.UniqueKey(t), digest[:]).Scan(&id)
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

func TestOldestAtStatus_SelectsOldestAtGivenStatus(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	first := seedSubmission(t, db, StatusAdmitted)
	seedSubmission(t, db, StatusAdmitted) // a second, newer one

	got, err := OldestAtStatus(context.Background(), db, StatusAdmitted)
	if err != nil {
		t.Fatalf("oldest at status: %v", err)
	}
	if got.ID != first {
		t.Errorf("selected submission %d, want the oldest (%d)", got.ID, first)
	}
}

// TestOldestAtStatus_SkipsOtherStatuses is the direct proof behind the
// Checkpoint 5 claim-behavior decision: a submission parked at validated
// (e.g. after a non-valid classification, FR-014) must never be selected
// by a query for admitted work, even though it is older and even though it
// is non-terminal by OldestNonTerminal's definition.
func TestOldestAtStatus_SkipsOtherStatuses(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	seedSubmission(t, db, StatusValidated) // older, but not admitted: must never be selected
	admitted := seedSubmission(t, db, StatusAdmitted)

	got, err := OldestAtStatus(context.Background(), db, StatusAdmitted)
	if err != nil {
		t.Fatalf("oldest at status: %v", err)
	}
	if got.ID != admitted {
		t.Errorf("selected submission %d, want the admitted one (%d)", got.ID, admitted)
	}
}

func TestOldestAtStatus_NoWorkWhenNoneAtStatus(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	seedSubmission(t, db, StatusValidated)

	_, err := OldestAtStatus(context.Background(), db, StatusAdmitted)
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

	id, err := Admit(context.Background(), db, []byte(`{"auditID":"a1"}`), FamilyKubernetes, "a1", "ResponseComplete", "")
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
	if got.SourceFamily != FamilyKubernetes {
		t.Errorf("source_family = %q, want %q", got.SourceFamily, FamilyKubernetes)
	}
}

func TestAdmit_AssignsDistinctIDs(t *testing.T) {
	db := testutil.MigratedPostgres(t)

	id1, err := Admit(context.Background(), db, []byte(`{}`), FamilyKubernetes, "a1", "ResponseComplete", "")
	if err != nil {
		t.Fatalf("admit 1: %v", err)
	}
	id2, err := Admit(context.Background(), db, []byte(`{}`), FamilyKubernetes, "a2", "ResponseComplete", "")
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

	id1, err := Admit(context.Background(), db, raw, FamilyKubernetes, "a1", "ResponseComplete", "")
	if err != nil {
		t.Fatalf("first admit: %v", err)
	}
	id2, err := Admit(context.Background(), db, raw, FamilyKubernetes, "a1", "ResponseComplete", "")
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

// --- List (Checkpoint: submissions review, FR-011/FR-012/FR-013): the
// keyset-paginated read internal/retrieval's submissions list composes.

func TestList_OrdersByIDAscendingAndRespectsCursor(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	first := seedSubmission(t, db, StatusAdmitted)
	second := seedSubmission(t, db, StatusAdmitted)
	third := seedSubmission(t, db, StatusAdmitted)

	page, err := List(context.Background(), db, 0, nil, 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(page) != 3 {
		t.Fatalf("len(page) = %d, want 3", len(page))
	}
	if page[0].ID != first || page[1].ID != second || page[2].ID != third {
		t.Errorf("ids = [%d, %d, %d], want ascending [%d, %d, %d]", page[0].ID, page[1].ID, page[2].ID, first, second, third)
	}

	// A cursor of the first id excludes it and everything before it.
	rest, err := List(context.Background(), db, first, nil, 10)
	if err != nil {
		t.Fatalf("list after cursor: %v", err)
	}
	if len(rest) != 2 || rest[0].ID != second || rest[1].ID != third {
		t.Errorf("list after cursor %d = %+v, want [%d, %d]", first, rest, second, third)
	}
}

func TestList_RespectsLimit(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	seedSubmission(t, db, StatusAdmitted)
	seedSubmission(t, db, StatusAdmitted)
	seedSubmission(t, db, StatusAdmitted)

	page, err := List(context.Background(), db, 0, nil, 2)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(page) != 2 {
		t.Errorf("len(page) = %d, want 2 (limit enforced)", len(page))
	}
}

func TestList_NilStatusReturnsEverySubmissionRegardlessOfStatus(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	seedSubmission(t, db, StatusAdmitted)
	seedSubmission(t, db, StatusValidated)
	seedSubmission(t, db, StatusEvidenced)

	page, err := List(context.Background(), db, 0, nil, 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(page) != 3 {
		t.Errorf("len(page) = %d, want 3 (no status filter)", len(page))
	}
}

// TestList_StatusFilterSelectsOnlyPendingSubmissions is the direct proof of
// the "pending" filter design: filtering List by StatusAdmitted returns
// exactly the not-yet-validated submissions, and nothing already validated
// -- no validation_outcomes read is needed to answer this.
func TestList_StatusFilterSelectsOnlyPendingSubmissions(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	pending := seedSubmission(t, db, StatusAdmitted)
	seedSubmission(t, db, StatusValidated)
	seedSubmission(t, db, StatusEvidenced)

	admitted := StatusAdmitted
	page, err := List(context.Background(), db, 0, &admitted, 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(page) != 1 || page[0].ID != pending {
		t.Errorf("page = %+v, want exactly the one pending submission %d", page, pending)
	}
}

func TestList_EmptyTableReturnsNilSliceNoError(t *testing.T) {
	db := testutil.MigratedPostgres(t)

	page, err := List(context.Background(), db, 0, nil, 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if page != nil {
		t.Errorf("page = %+v, want nil", page)
	}
}

// --- GetMany: the bounded batch read internal/retrieval's submissions
// list uses to fetch a page of submissions already selected by
// internal/validation.ListByOutcome, without one query per id.

func TestGetMany_ReturnsMatchingRowsOrderedByID(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	first := seedSubmission(t, db, StatusAdmitted)
	second := seedSubmission(t, db, StatusValidated)
	seedSubmission(t, db, StatusEvidenced) // not requested, must not appear

	got, err := GetMany(context.Background(), db, []int64{second, first})
	if err != nil {
		t.Fatalf("get many: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[0].ID != first || got[1].ID != second {
		t.Errorf("ids = [%d, %d], want ascending [%d, %d]", got[0].ID, got[1].ID, first, second)
	}
}

func TestGetMany_EmptyIDsReturnsNilSliceNoQuery(t *testing.T) {
	db := testutil.MigratedPostgres(t)

	got, err := GetMany(context.Background(), db, nil)
	if err != nil {
		t.Fatalf("get many: %v", err)
	}
	if got != nil {
		t.Errorf("got = %+v, want nil", got)
	}
}

func TestGetMany_UnknownIDIsSilentlyAbsentNotError(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	real := seedSubmission(t, db, StatusAdmitted)

	got, err := GetMany(context.Background(), db, []int64{real, 999999})
	if err != nil {
		t.Fatalf("get many: %v", err)
	}
	if len(got) != 1 || got[0].ID != real {
		t.Errorf("got = %+v, want exactly [%d]", got, real)
	}
}

// --- Count: the total a paginated submissions list reports alongside a
// page, independent of that page's own LIMIT.

func TestCount_NilStatusCountsEverySubmission(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	seedSubmission(t, db, StatusAdmitted)
	seedSubmission(t, db, StatusValidated)

	n, err := Count(context.Background(), db, nil)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 2 {
		t.Errorf("count = %d, want 2", n)
	}
}

func TestCount_StatusFilterCountsOnlyThatStatus(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	seedSubmission(t, db, StatusAdmitted)
	seedSubmission(t, db, StatusAdmitted)
	seedSubmission(t, db, StatusValidated)

	admitted := StatusAdmitted
	n, err := Count(context.Background(), db, &admitted)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 2 {
		t.Errorf("count = %d, want 2", n)
	}
}

func TestCount_EmptyTableIsZero(t *testing.T) {
	db := testutil.MigratedPostgres(t)

	n, err := Count(context.Background(), db, nil)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Errorf("count = %d, want 0", n)
	}
}

func TestAdmit_DifferentlyFormattedEquivalentJSON_ReturnsErrSourceConflict(t *testing.T) {
	db := testutil.MigratedPostgres(t)

	original := []byte(`{"a":1,"b":2}`)
	id1, err := Admit(context.Background(), db, original, FamilyKubernetes, "a1", "ResponseComplete", "")
	if err != nil {
		t.Fatalf("first admit: %v", err)
	}

	// Same identity (auditID+stage), semantically-equivalent JSON, but
	// different raw bytes (reordered keys, extra whitespace) -- this must
	// be rejected as a conflict, not silently treated as "the same event",
	// even though a JSONB-level comparison would have called it equal.
	reformatted := []byte(`{"b": 2, "a": 1}`)
	_, err = Admit(context.Background(), db, reformatted, FamilyKubernetes, "a1", "ResponseComplete", "")
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

// --- AWS CloudTrail family (ADR-0006): Admit's eventID-keyed dedup branch
// and source-family attribution, proven at the same level of directness as
// the Kubernetes cases above.

func TestAdmit_CloudTrail_CreatesAdmittedSubmissionWithSourceEventID(t *testing.T) {
	db := testutil.MigratedPostgres(t)

	id, err := Admit(context.Background(), db, []byte(`{"eventID":"evt-1"}`), FamilyCloudTrail, "", "", "evt-1")
	if err != nil {
		t.Fatalf("admit: %v", err)
	}

	got, err := Get(context.Background(), db, id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.SourceFamily != FamilyCloudTrail {
		t.Errorf("source_family = %q, want %q", got.SourceFamily, FamilyCloudTrail)
	}
	if got.SourceEventID != "evt-1" {
		t.Errorf("source_event_id = %q, want %q", got.SourceEventID, "evt-1")
	}
	if got.AuditID != "" || got.AuditStage != "" {
		t.Errorf("audit_id/audit_stage = %q/%q, want both empty for a CloudTrail submission", got.AuditID, got.AuditStage)
	}
}

// TestAdmit_CloudTrail_ByteIdenticalRedeliveryReturnsSameID proves
// idempotent redelivery for the CloudTrail family: the same eventID and
// the same raw bytes twice resolves to the existing row, never a
// duplicate -- the CloudTrail analog of TestAdmit_RetryReturnsSameID above.
func TestAdmit_CloudTrail_ByteIdenticalRedeliveryReturnsSameID(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	raw := []byte(`{"eventID":"evt-2","eventName":"CreateAccessKey"}`)

	id1, err := Admit(context.Background(), db, raw, FamilyCloudTrail, "", "", "evt-2")
	if err != nil {
		t.Fatalf("first admit: %v", err)
	}
	id2, err := Admit(context.Background(), db, raw, FamilyCloudTrail, "", "", "evt-2")
	if err != nil {
		t.Fatalf("retry admit: %v", err)
	}
	if id1 != id2 {
		t.Errorf("retry returned id %d, want the original id %d", id2, id1)
	}

	var count int
	if err := db.QueryRowContext(context.Background(),
		`SELECT count(*) FROM submissions WHERE source_event_id = 'evt-2'`,
	).Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != 1 {
		t.Errorf("row count = %d, want 1 (no duplicate on byte-identical redelivery)", count)
	}
}

// TestAdmit_CloudTrail_SameEventIDDifferentContent_ReturnsErrSourceConflict
// is the CloudTrail analog of
// TestAdmit_DifferentlyFormattedEquivalentJSON_ReturnsErrSourceConflict:
// the same eventID with genuinely different raw bytes is a conflict, never
// silently accepted as a retry.
func TestAdmit_CloudTrail_SameEventIDDifferentContent_ReturnsErrSourceConflict(t *testing.T) {
	db := testutil.MigratedPostgres(t)

	original := []byte(`{"eventID":"evt-3","eventName":"CreateAccessKey"}`)
	id1, err := Admit(context.Background(), db, original, FamilyCloudTrail, "", "", "evt-3")
	if err != nil {
		t.Fatalf("first admit: %v", err)
	}

	different := []byte(`{"eventID":"evt-3","eventName":"DeactivateMFADevice"}`)
	_, err = Admit(context.Background(), db, different, FamilyCloudTrail, "", "", "evt-3")
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
}

// TestAdmit_KubernetesAndCloudTrail_SameIdentityStringNeverCollide proves
// families cannot be misclassified as one another at the Admit level, not
// only at the SourceKey level (sourcekey_test.go): a Kubernetes submission
// and a CloudTrail submission sharing the same identity string produce two
// distinct rows, never a false dedup collision across families.
func TestAdmit_KubernetesAndCloudTrail_SameIdentityStringNeverCollide(t *testing.T) {
	db := testutil.MigratedPostgres(t)

	k8sID, err := Admit(context.Background(), db, []byte(`{"a":1}`), FamilyKubernetes, "shared-id", "ResponseComplete", "")
	if err != nil {
		t.Fatalf("admit kubernetes: %v", err)
	}
	ctID, err := Admit(context.Background(), db, []byte(`{"a":1}`), FamilyCloudTrail, "", "", "shared-id")
	if err != nil {
		t.Fatalf("admit cloudtrail: %v", err)
	}
	if k8sID == ctID {
		t.Errorf("expected distinct submissions for the same identity string across families, got the same id %d for both", k8sID)
	}

	var count int
	if err := db.QueryRowContext(context.Background(), `SELECT count(*) FROM submissions`).Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != 2 {
		t.Errorf("row count = %d, want 2 (both families admitted as distinct rows)", count)
	}
}
