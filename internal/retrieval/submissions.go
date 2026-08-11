package retrieval

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"cnsdp/internal/auth"
	"cnsdp/internal/diagnostics"
	"cnsdp/internal/submission"
	"cnsdp/internal/validation"
)

// SubmissionsListHandler serves the submissions review endpoint (GET
// /v1/submissions): a keyset-paginated, optionally outcome-filtered list
// of every submission received at the defined intake, closing the
// FR-011/FR-012/FR-013 reviewability gap for submissions that never
// produce an alert -- every invalid, incomplete, or unsupported
// submission, and every valid submission that does not match any
// detection scenario, none of which is reachable through GET /v1/alerts or
// GET /v1/alerts/{id} (both keyed off an alert, which only a matching
// detection result ever produces). It owns no artifact table of its own:
// every row is composed from internal/submission's and
// internal/validation's own sanctioned reads, never a direct query against
// either module's table (ARCH-01 §2).
type SubmissionsListHandler struct {
	DB    *sql.DB
	Token string
}

// filterPending is the query-value for "no validation outcome recorded
// yet" -- a submission still at submission.StatusAdmitted. It is
// deliberately not a validation.Outcome value: FR-005 fixes validation
// classification at exactly four mutually exclusive outcomes, and this
// filter never touches the validation_outcomes table or its outcome
// column (see listSubmissionsPage).
const filterPending = "pending"

const (
	defaultSubmissionsLimit = 50
	maxSubmissionsLimit     = 200
)

func isValidOutcomeFilter(v string) bool {
	switch v {
	case "", filterPending,
		string(validation.OutcomeValid), string(validation.OutcomeInvalid),
		string(validation.OutcomeIncomplete), string(validation.OutcomeUnsupported):
		return true
	default:
		return false
	}
}

func (h *SubmissionsListHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if !auth.Bearer(r.Header.Get("Authorization"), h.Token) {
		diagnostics.LogAccessDenied(r)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	q := r.URL.Query()

	filter := q.Get("outcome")
	if !isValidOutcomeFilter(filter) {
		http.Error(w, fmt.Sprintf(
			"outcome must be one of %q, %q, %q, %q, %q; got %q",
			filterPending, validation.OutcomeValid, validation.OutcomeInvalid,
			validation.OutcomeIncomplete, validation.OutcomeUnsupported, filter,
		), http.StatusBadRequest)
		return
	}

	after := int64(0)
	if v := q.Get("cursor"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n < 0 {
			http.Error(w, "cursor must be a non-negative integer", http.StatusBadRequest)
			return
		}
		after = n
	}

	limit := defaultSubmissionsLimit
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 || n > maxSubmissionsLimit {
			http.Error(w, fmt.Sprintf("limit must be a positive integer no greater than %d", maxSubmissionsLimit), http.StatusBadRequest)
			return
		}
		limit = n
	}

	rows, nextCursor, total, err := listSubmissionsPage(r.Context(), h.DB, filter, after, limit)
	if err != nil {
		slog.Error("retrieval: list submissions failed", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(toSubmissionsListResponse(rows, nextCursor, total)); err != nil {
		// The response status and headers are already committed at this
		// point (NFR-022: never conflate a platform fault with a product
		// outcome already sent to the client).
		slog.Error("retrieval: encode submissions response failed", "error", err)
	}
}

// ErrCompositionMismatch is returned when an outcome-filtered page resolves
// a validation_outcomes record whose submission id has no matching row in
// the just-fetched submission set. This should be structurally impossible
// under ordinary operation: validation_outcomes.submission_id is a NOT
// NULL foreign key into submissions(id), and no delete path for a
// submissions row exists anywhere in this codebase. It is never silently
// dropped: a broken invariant here must surface as a platform fault
// (ServeHTTP maps this to 500), not a quietly under-filled page with a
// nextCursor derived from the wrong row.
var ErrCompositionMismatch = errors.New("retrieval: outcome-filtered submissions page: validation outcome references a submission id absent from the fetched submission set")

// submissionRow is one page row's composed content: the submission itself,
// plus its validation outcome when one has been recorded. OutcomeAvailable
// mirrors the "available" flag convention response.go already uses for
// GET /v1/alerts/{id} -- false is a distinct, visible state (not yet
// validated), never a fabricated or zero-valued outcome.
type submissionRow struct {
	Submission        submission.Submission
	ValidationOutcome validation.Result
	OutcomeAvailable  bool
}

// listSubmissionsPage composes one keyset page for filter, ordered by
// submission id ascending, using a bounded number of queries regardless of
// page size or total submission volume (never one validation query per
// row):
//
//   - filter == filterPending: submission.List filtered to
//     StatusAdmitted, plus submission.Count with the same filter. No
//     validation read at all -- StatusAdmitted if and only if no
//     validation outcome has been recorded (internal/validation.Advance
//     writes both in one transaction), so this filter is answerable from
//     internal/submission alone.
//   - filter == "" (no filter): submission.List across every status,
//     decorated with one validation.BatchGet call for that page's ids,
//     plus submission.Count(nil).
//   - filter is one of the four validation.Outcome values:
//     validation.ListByOutcome resolves the page directly against
//     validation_outcomes (submission_id keyset, same ordering key), then
//     one submission.GetMany call fetches the matching submission rows,
//     plus validation.CountByOutcome.
//
// Every branch fetches limit+1 rows to determine whether a next page
// exists without a second, separate query.
func listSubmissionsPage(ctx context.Context, db *sql.DB, filter string, after int64, limit int) ([]submissionRow, *int64, int64, error) {
	fetch := limit + 1

	switch filter {
	case filterPending:
		pending := submission.StatusAdmitted
		subs, err := submission.List(ctx, db, after, &pending, fetch)
		if err != nil {
			return nil, nil, 0, fmt.Errorf("retrieval: list submissions page (pending): %w", err)
		}
		total, err := submission.Count(ctx, db, &pending)
		if err != nil {
			return nil, nil, 0, fmt.Errorf("retrieval: count submissions (pending): %w", err)
		}
		rows := make([]submissionRow, len(subs))
		for i, s := range subs {
			rows[i] = submissionRow{Submission: s}
		}
		page, next := trimSubmissionsPage(rows, limit)
		return page, next, total, nil

	case "":
		subs, err := submission.List(ctx, db, after, nil, fetch)
		if err != nil {
			return nil, nil, 0, fmt.Errorf("retrieval: list submissions page: %w", err)
		}
		ids := make([]int64, len(subs))
		for i, s := range subs {
			ids[i] = s.ID
		}
		outcomes, err := validation.BatchGet(ctx, db, ids)
		if err != nil {
			return nil, nil, 0, fmt.Errorf("retrieval: batch get validation outcomes: %w", err)
		}
		total, err := submission.Count(ctx, db, nil)
		if err != nil {
			return nil, nil, 0, fmt.Errorf("retrieval: count submissions: %w", err)
		}
		rows := make([]submissionRow, len(subs))
		for i, s := range subs {
			result, ok := outcomes[s.ID]
			rows[i] = submissionRow{Submission: s, ValidationOutcome: result, OutcomeAvailable: ok}
		}
		page, next := trimSubmissionsPage(rows, limit)
		return page, next, total, nil

	default:
		outcome := validation.Outcome(filter)
		records, err := validation.ListByOutcome(ctx, db, outcome, after, fetch)
		if err != nil {
			return nil, nil, 0, fmt.Errorf("retrieval: list submissions page (outcome %s): %w", outcome, err)
		}
		ids := make([]int64, len(records))
		for i, rec := range records {
			ids[i] = rec.SubmissionID
		}
		subs, err := submission.GetMany(ctx, db, ids)
		if err != nil {
			return nil, nil, 0, fmt.Errorf("retrieval: get many submissions: %w", err)
		}
		subByID := make(map[int64]submission.Submission, len(subs))
		for _, s := range subs {
			subByID[s.ID] = s
		}
		total, err := validation.CountByOutcome(ctx, db, outcome)
		if err != nil {
			return nil, nil, 0, fmt.Errorf("retrieval: count by outcome %s: %w", outcome, err)
		}
		rows, err := mergeOutcomeFilteredRows(records, subByID)
		if err != nil {
			return nil, nil, 0, fmt.Errorf("retrieval: compose outcome-filtered page (outcome %s): %w", outcome, err)
		}
		page, next := trimSubmissionsPage(rows, limit)
		return page, next, total, nil
	}
}

// mergeOutcomeFilteredRows pairs each validation_outcomes record with its
// resolved submission row (via subByID) into one composed submissionRow
// per record, preserving record order. Returns ErrCompositionMismatch, and
// no rows, at the first record whose SubmissionID is absent from subByID
// -- see ErrCompositionMismatch's doc comment for why this is never a
// silent skip. Pulled out as its own pure function -- no database
// argument -- so the mismatch path is directly unit-testable without
// requiring (or weakening) the real foreign-key constraint that makes it
// unreachable in production.
func mergeOutcomeFilteredRows(records []validation.Record, subByID map[int64]submission.Submission) ([]submissionRow, error) {
	rows := make([]submissionRow, 0, len(records))
	for _, rec := range records {
		s, ok := subByID[rec.SubmissionID]
		if !ok {
			return nil, fmt.Errorf("%w: submission %d", ErrCompositionMismatch, rec.SubmissionID)
		}
		rows = append(rows, submissionRow{Submission: s, ValidationOutcome: rec.Result, OutcomeAvailable: true})
	}
	return rows, nil
}

// trimSubmissionsPage drops the lookahead row a limit+1 fetch may have
// returned, and derives nextCursor from the last row of the trimmed page
// -- nil once fewer than limit+1 rows came back, meaning there is no
// further page.
func trimSubmissionsPage(rows []submissionRow, limit int) ([]submissionRow, *int64) {
	if len(rows) > limit {
		trimmed := rows[:limit]
		next := trimmed[len(trimmed)-1].Submission.ID
		return trimmed, &next
	}
	return rows, nil
}

// submissionItem is one list row's wire shape. validationOutcome reuses
// response.go's validationOutcomeField -- the same "available flag first"
// contract GET /v1/alerts/{id} already uses for an artifact that may be
// legitimately absent, so a pending submission is never represented with a
// fabricated or zero-valued outcome.
type submissionItem struct {
	SubmissionID      int64                  `json:"submissionId"`
	Status            string                 `json:"status"`
	AuditID           string                 `json:"auditId"`
	AuditStage        string                 `json:"auditStage"`
	CreatedAt         time.Time              `json:"createdAt"`
	ValidationOutcome validationOutcomeField `json:"validationOutcome"`
}

// submissionsListResponse is the wire shape for a successful GET
// /v1/submissions: one keyset page plus the cursor for the next page (nil
// when this is the last page) and the total count of submissions matching
// the active filter, independent of pagination.
type submissionsListResponse struct {
	Submissions []submissionItem `json:"submissions"`
	NextCursor  *int64           `json:"nextCursor"`
	Total       int64            `json:"total"`
}

func toSubmissionsListResponse(rows []submissionRow, nextCursor *int64, total int64) submissionsListResponse {
	items := make([]submissionItem, 0, len(rows))
	for _, row := range rows {
		item := submissionItem{
			SubmissionID: row.Submission.ID,
			Status:       string(row.Submission.Status),
			AuditID:      row.Submission.AuditID,
			AuditStage:   row.Submission.AuditStage,
			CreatedAt:    row.Submission.CreatedAt,
		}
		item.ValidationOutcome.Available = row.OutcomeAvailable
		if row.OutcomeAvailable {
			item.ValidationOutcome.Outcome = string(row.ValidationOutcome.Outcome)
			item.ValidationOutcome.Reason = row.ValidationOutcome.Reason
		}
		items = append(items, item)
	}
	return submissionsListResponse{Submissions: items, NextCursor: nextCursor, Total: total}
}
