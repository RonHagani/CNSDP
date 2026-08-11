package retrieval

import (
	"errors"
	"testing"

	"cnsdp/internal/submission"
	"cnsdp/internal/validation"
)

// TestMergeOutcomeFilteredRows_AllResolved is the happy-path proof: every
// record's SubmissionID resolves through subByID, and the composed rows
// preserve record order and carry the record's own Result.
func TestMergeOutcomeFilteredRows_AllResolved(t *testing.T) {
	records := []validation.Record{
		{SubmissionID: 2, Result: validation.Result{Outcome: validation.OutcomeInvalid, Reason: "r2"}},
		{SubmissionID: 1, Result: validation.Result{Outcome: validation.OutcomeInvalid, Reason: "r1"}},
	}
	subByID := map[int64]submission.Submission{
		1: {ID: 1, Status: submission.StatusValidated},
		2: {ID: 2, Status: submission.StatusValidated},
	}

	rows, err := mergeOutcomeFilteredRows(records, subByID)
	if err != nil {
		t.Fatalf("mergeOutcomeFilteredRows: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2", len(rows))
	}
	// Record order (2 then 1), not submission-id order -- proves this
	// function doesn't silently reorder what its caller already ordered.
	if rows[0].Submission.ID != 2 || rows[1].Submission.ID != 1 {
		t.Errorf("rows = %+v, want submission 2 then 1 (record order)", rows)
	}
	if !rows[0].OutcomeAvailable || rows[0].ValidationOutcome.Reason != "r2" {
		t.Errorf("rows[0] = %+v, want OutcomeAvailable=true reason=r2", rows[0])
	}
}

// TestMergeOutcomeFilteredRows_MissingSubmissionReturnsErrCompositionMismatch
// simulates -- entirely in Go values, without touching the database -- the
// drift ErrCompositionMismatch's doc comment describes as structurally
// unreachable in production (validation_outcomes.submission_id is a NOT
// NULL foreign key into submissions(id) with no delete path). The real
// foreign-key constraint is never relaxed to make this reachable; this
// test only proves that if the fetched submission set and the fetched
// validation-outcome set were ever inconsistent, the merge fails visibly
// rather than silently dropping the affected row.
func TestMergeOutcomeFilteredRows_MissingSubmissionReturnsErrCompositionMismatch(t *testing.T) {
	records := []validation.Record{
		{SubmissionID: 1, Result: validation.Result{Outcome: validation.OutcomeInvalid, Reason: "r1"}},
		{SubmissionID: 2, Result: validation.Result{Outcome: validation.OutcomeInvalid, Reason: "r2"}},
	}
	subByID := map[int64]submission.Submission{
		1: {ID: 1, Status: submission.StatusValidated},
		// 2 is deliberately absent.
	}

	rows, err := mergeOutcomeFilteredRows(records, subByID)
	if !errors.Is(err, ErrCompositionMismatch) {
		t.Fatalf("err = %v, want ErrCompositionMismatch", err)
	}
	if rows != nil {
		t.Errorf("rows = %+v, want nil on error (no partial page)", rows)
	}
}

func TestMergeOutcomeFilteredRows_EmptyRecordsReturnsEmptySlice(t *testing.T) {
	rows, err := mergeOutcomeFilteredRows(nil, map[int64]submission.Submission{})
	if err != nil {
		t.Fatalf("mergeOutcomeFilteredRows: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("rows = %+v, want empty", rows)
	}
}

// TestTrimSubmissionsPage_ExactlyLimitRows_NoNextCursor is the exact-limit
// boundary proof: every caller fetches limit+1 rows specifically so a
// returned count of exactly limit+1 signals "there is more," while a
// returned count of exactly limit means the underlying query already
// exhausted every matching row. trimSubmissionsPage's len(rows) > limit
// check must not become len(rows) >= limit -- that off-by-one would
// wrongly trim a genuinely final, exactly-sized page and report a
// nextCursor pointing at a page that doesn't exist.
func TestTrimSubmissionsPage_ExactlyLimitRows_NoNextCursor(t *testing.T) {
	rows := []submissionRow{
		{Submission: submission.Submission{ID: 1}},
		{Submission: submission.Submission{ID: 2}},
	}

	page, next := trimSubmissionsPage(rows, 2)

	if len(page) != 2 {
		t.Fatalf("len(page) = %d, want 2 (both rows returned, none trimmed)", len(page))
	}
	if next != nil {
		t.Errorf("nextCursor = %d, want nil (exactly limit rows means no further page)", *next)
	}
}

// TestTrimSubmissionsPage_LimitPlusOneRows_TrimsAndSetsNextCursor is the
// complementary case: with the lookahead row present, the page is trimmed
// to limit and nextCursor is derived from the trimmed page's own last row
// -- never the dropped lookahead row itself.
func TestTrimSubmissionsPage_LimitPlusOneRows_TrimsAndSetsNextCursor(t *testing.T) {
	rows := []submissionRow{
		{Submission: submission.Submission{ID: 1}},
		{Submission: submission.Submission{ID: 2}},
		{Submission: submission.Submission{ID: 3}}, // the lookahead row
	}

	page, next := trimSubmissionsPage(rows, 2)

	if len(page) != 2 {
		t.Fatalf("len(page) = %d, want 2 (trimmed to limit)", len(page))
	}
	if next == nil || *next != 2 {
		t.Fatalf("nextCursor = %v, want 2 (the trimmed page's own last row, not the dropped lookahead row)", next)
	}
}
