//go:build integration

package retrieval

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"cnsdp/internal/submission"
	"cnsdp/internal/testutil"
	"cnsdp/internal/validation"
)

// unsupportedSubmissionsFixture / invalidSubmissionsFixture /
// incompleteSubmissionsFixture are additional boundary fixtures alongside
// this package's shared validEventJSON (retrieval_integration_test.go) --
// one per non-valid outcome, needed only by this file's outcome-filter
// coverage.
const (
	unsupportedSubmissionsFixture = `{"kind":"Pod","apiVersion":"v1"}`
	invalidSubmissionsFixture     = `{"kind":"Event","apiVersion":"audit.k8s.io/v1","auditID":123,"stage":"ResponseComplete","requestURI":"/x","verb":"get","user":{"username":"u"},"objectRef":{"resource":"pods"},"requestReceivedTimestamp":"2026-07-21T15:25:11.901891Z"}`
	incompleteSubmissionsFixture  = `{"kind":"Event","apiVersion":"audit.k8s.io/v1","auditID":"a1","stage":"ResponseComplete","requestReceivedTimestamp":"2026-07-21T15:25:11.901891Z"}`
)

func newSubmissionsTestServer(t *testing.T, db *sql.DB) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("GET /v1/submissions", &SubmissionsListHandler{DB: db, Token: testToken})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// seedPendingSubmission admits rawEvent and returns its id without ever
// calling validation.Advance -- it stays at submission.StatusAdmitted with
// no validation_outcomes row, exactly the "pending" filter's subject.
func seedPendingSubmission(t *testing.T, db *sql.DB, rawEvent, auditID, auditStage string) int64 {
	t.Helper()
	id, err := submission.Admit(context.Background(), db, json.RawMessage(rawEvent), auditID, auditStage)
	if err != nil {
		t.Fatalf("admit: %v", err)
	}
	return id
}

// seedValidatedSubmission admits rawEvent and runs validation.Advance,
// leaving the submission at submission.StatusValidated with a persisted
// validation_outcomes row -- whatever outcome rawEvent actually classifies
// as. It deliberately stops at validated: this file exercises the
// submissions list's outcome decoration and filtering, which needs only
// the validation_outcomes artifact, not a full normalize/detect/alert run.
func seedValidatedSubmission(t *testing.T, db *sql.DB, rawEvent, auditID, auditStage string) int64 {
	t.Helper()
	ctx := context.Background()
	id, err := submission.Admit(ctx, db, json.RawMessage(rawEvent), auditID, auditStage)
	if err != nil {
		t.Fatalf("admit: %v", err)
	}
	sub, err := submission.Get(ctx, db, id)
	if err != nil {
		t.Fatalf("get admitted: %v", err)
	}
	if err := validation.Advance(ctx, db, sub); err != nil {
		t.Fatalf("validation.Advance: %v", err)
	}
	return id
}

func TestSubmissionsServeHTTP_MissingToken_Returns401WithNoData(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	srv := newSubmissionsTestServer(t, db)

	resp, body := get(t, srv, "/v1/submissions", "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
	if len(body) != 0 {
		t.Errorf("body = %q, want empty (no data returned on denied access)", body)
	}
}

func TestSubmissionsServeHTTP_InvalidToken_Returns401WithNoData(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	srv := newSubmissionsTestServer(t, db)

	resp, body := get(t, srv, "/v1/submissions", "wrong-token")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
	if len(body) != 0 {
		t.Errorf("body = %q, want empty (no data returned on denied access)", body)
	}
}

func TestSubmissionsServeHTTP_MethodNotAllowed_Returns405(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	srv := newSubmissionsTestServer(t, db)

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/v1/submissions", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+testToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", resp.StatusCode)
	}
}

func TestSubmissionsServeHTTP_EmptyRepository_Returns200WithEmptyListAndNullCursor(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	srv := newSubmissionsTestServer(t, db)

	resp, body := get(t, srv, "/v1/submissions", testToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", resp.StatusCode, body)
	}

	var got submissionsListResponse
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got.Total != 0 {
		t.Errorf("total = %d, want 0", got.Total)
	}
	if len(got.Submissions) != 0 {
		t.Errorf("len(submissions) = %d, want 0", len(got.Submissions))
	}
	if got.NextCursor != nil {
		t.Errorf("nextCursor = %v, want nil", got.NextCursor)
	}
	if !bytes.Contains(body, []byte(`"submissions":[]`)) {
		t.Errorf("expected an empty JSON array, not a null field, got: %s", body)
	}
}

// TestSubmissionsServeHTTP_NoFilter_ReturnsEveryOutcomeIncludingPendingOrderedByID
// is the direct proof this endpoint closes the FR-011/FR-012/FR-013 gap:
// every one of the four validation outcomes, plus a submission that has not
// been validated yet, is present and reviewable in one unfiltered list --
// none of them reachable via GET /v1/alerts.
func TestSubmissionsServeHTTP_NoFilter_ReturnsEveryOutcomeIncludingPendingOrderedByID(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	validID := seedValidatedSubmission(t, db, validEventJSON, "aud-valid", "ResponseComplete")
	invalidID := seedValidatedSubmission(t, db, invalidSubmissionsFixture, "aud-invalid", "ResponseComplete")
	incompleteID := seedValidatedSubmission(t, db, incompleteSubmissionsFixture, "aud-incomplete", "ResponseComplete")
	unsupportedID := seedValidatedSubmission(t, db, unsupportedSubmissionsFixture, "aud-unsupported", "ResponseComplete")
	pendingID := seedPendingSubmission(t, db, validEventJSON, "aud-pending", "ResponseComplete")
	srv := newSubmissionsTestServer(t, db)

	resp, body := get(t, srv, "/v1/submissions", testToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", resp.StatusCode, body)
	}

	var got submissionsListResponse
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got.Total != 5 {
		t.Fatalf("total = %d, want 5", got.Total)
	}
	if len(got.Submissions) != 5 {
		t.Fatalf("len(submissions) = %d, want 5", len(got.Submissions))
	}

	wantOrder := []int64{validID, invalidID, incompleteID, unsupportedID, pendingID}
	for i, want := range wantOrder {
		if got.Submissions[i].SubmissionID != want {
			t.Errorf("submissions[%d].submissionId = %d, want %d (ascending admission order)", i, got.Submissions[i].SubmissionID, want)
		}
	}

	byID := make(map[int64]submissionItem, len(got.Submissions))
	for _, s := range got.Submissions {
		byID[s.SubmissionID] = s
	}

	if o := byID[validID].ValidationOutcome; !o.Available || o.Outcome != "valid" || o.Reason != "" {
		t.Errorf("valid row outcome = %+v, want available=true outcome=valid reason=\"\"", o)
	}
	if o := byID[invalidID].ValidationOutcome; !o.Available || o.Outcome != "invalid" || o.Reason == "" {
		t.Errorf("invalid row outcome = %+v, want available=true outcome=invalid non-empty reason", o)
	}
	if o := byID[incompleteID].ValidationOutcome; !o.Available || o.Outcome != "incomplete" || o.Reason == "" {
		t.Errorf("incomplete row outcome = %+v, want available=true outcome=incomplete non-empty reason", o)
	}
	if o := byID[unsupportedID].ValidationOutcome; !o.Available || o.Outcome != "unsupported" || o.Reason == "" {
		t.Errorf("unsupported row outcome = %+v, want available=true outcome=unsupported non-empty reason", o)
	}
	if o := byID[pendingID].ValidationOutcome; o.Available {
		t.Errorf("pending row outcome = %+v, want available=false", o)
	}
}

// TestSubmissionsServeHTTP_NeverFabricatesOutcomeForPendingSubmission proves
// the wire contract directly against the raw JSON: a pending submission's
// row carries no "outcome" or "reason" key at all (omitempty on an unset
// field), never an empty-string placeholder that could be misread as a
// real recorded value.
func TestSubmissionsServeHTTP_NeverFabricatesOutcomeForPendingSubmission(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	seedPendingSubmission(t, db, validEventJSON, "aud-pending", "ResponseComplete")
	srv := newSubmissionsTestServer(t, db)

	resp, body := get(t, srv, "/v1/submissions", testToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", resp.StatusCode, body)
	}
	if !bytes.Contains(body, []byte(`"validationOutcome":{"available":false}`)) {
		t.Errorf("expected a bare available:false validationOutcome with no outcome/reason keys, got: %s", body)
	}
}

func TestSubmissionsServeHTTP_FilterPending_ReturnsOnlyPendingSubmissions(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	pendingID := seedPendingSubmission(t, db, validEventJSON, "aud-pending", "ResponseComplete")
	seedValidatedSubmission(t, db, validEventJSON, "aud-valid", "ResponseComplete")
	srv := newSubmissionsTestServer(t, db)

	resp, body := get(t, srv, "/v1/submissions?outcome=pending", testToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", resp.StatusCode, body)
	}

	var got submissionsListResponse
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got.Total != 1 || len(got.Submissions) != 1 {
		t.Fatalf("got = %+v, want exactly 1 pending submission", got)
	}
	if got.Submissions[0].SubmissionID != pendingID {
		t.Errorf("submissionId = %d, want %d", got.Submissions[0].SubmissionID, pendingID)
	}
	if got.Submissions[0].ValidationOutcome.Available {
		t.Errorf("pending row outcome.available = true, want false")
	}
}

func TestSubmissionsServeHTTP_FilterKnownOutcome_ReturnsOnlyThatOutcomeWithReason(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	invalidID := seedValidatedSubmission(t, db, invalidSubmissionsFixture, "aud-invalid", "ResponseComplete")
	seedValidatedSubmission(t, db, validEventJSON, "aud-valid", "ResponseComplete")
	seedPendingSubmission(t, db, validEventJSON, "aud-pending", "ResponseComplete")
	srv := newSubmissionsTestServer(t, db)

	resp, body := get(t, srv, "/v1/submissions?outcome=invalid", testToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", resp.StatusCode, body)
	}

	var got submissionsListResponse
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got.Total != 1 || len(got.Submissions) != 1 {
		t.Fatalf("got = %+v, want exactly 1 invalid submission", got)
	}
	row := got.Submissions[0]
	if row.SubmissionID != invalidID {
		t.Errorf("submissionId = %d, want %d", row.SubmissionID, invalidID)
	}
	if !row.ValidationOutcome.Available || row.ValidationOutcome.Outcome != "invalid" || row.ValidationOutcome.Reason == "" {
		t.Errorf("outcome = %+v, want available=true outcome=invalid non-empty reason", row.ValidationOutcome)
	}
}

func TestSubmissionsServeHTTP_InvalidOutcomeQueryParam_Returns400(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	srv := newSubmissionsTestServer(t, db)

	resp, body := get(t, srv, "/v1/submissions?outcome=bogus", testToken)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (body: %s)", resp.StatusCode, body)
	}
}

func TestSubmissionsServeHTTP_InvalidCursorQueryParam_Returns400(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	srv := newSubmissionsTestServer(t, db)

	for _, cursor := range []string{"not-a-number", "-1"} {
		resp, body := get(t, srv, "/v1/submissions?cursor="+cursor, testToken)
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("cursor=%q: status = %d, want 400 (body: %s)", cursor, resp.StatusCode, body)
		}
	}
}

func TestSubmissionsServeHTTP_InvalidLimitQueryParam_Returns400(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	srv := newSubmissionsTestServer(t, db)

	for _, limit := range []string{"0", "-5", "201", "not-a-number"} {
		resp, body := get(t, srv, "/v1/submissions?limit="+limit, testToken)
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("limit=%q: status = %d, want 400 (body: %s)", limit, resp.StatusCode, body)
		}
	}
}

// TestSubmissionsServeHTTP_Pagination_CursorAdvancesThroughEveryPageWithoutDuplicatesOrGaps
// walks a five-submission repository two rows at a time using each
// response's own nextCursor, and proves the pages collectively reconstruct
// the full ordered set exactly once -- keyset pagination's core
// correctness property, not exercised by any single-page test above.
func TestSubmissionsServeHTTP_Pagination_CursorAdvancesThroughEveryPageWithoutDuplicatesOrGaps(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	var want []int64
	for i := 0; i < 5; i++ {
		// Distinct auditID per submission: submission.Admit derives its
		// dedup key from (auditID, auditStage) alone when both are
		// non-empty, so five calls sharing one auditID would collapse into
		// a single retry-safe row instead of five distinct submissions.
		auditID := fmt.Sprintf("aud-page-%d", i)
		want = append(want, seedPendingSubmission(t, db, validEventJSON, auditID, "ResponseComplete"))
	}
	srv := newSubmissionsTestServer(t, db)

	var collected []int64
	cursor := ""
	for page := 0; ; page++ {
		if page > 10 {
			t.Fatal("pagination did not terminate within 10 pages")
		}
		path := "/v1/submissions?limit=2"
		if cursor != "" {
			path += "&cursor=" + cursor
		}
		resp, body := get(t, srv, path, testToken)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("page %d: status = %d, want 200 (body: %s)", page, resp.StatusCode, body)
		}
		var got submissionsListResponse
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatalf("page %d: unmarshal: %v", page, err)
		}
		if len(got.Submissions) > 2 {
			t.Fatalf("page %d: len(submissions) = %d, want at most 2 (limit)", page, len(got.Submissions))
		}
		for _, s := range got.Submissions {
			collected = append(collected, s.SubmissionID)
		}
		if got.NextCursor == nil {
			break
		}
		cursor = strconv.FormatInt(*got.NextCursor, 10)
	}

	if len(collected) != len(want) {
		t.Fatalf("collected %d submissions across pages, want %d", len(collected), len(want))
	}
	for i, id := range want {
		if collected[i] != id {
			t.Errorf("collected[%d] = %d, want %d (ascending, no duplicates or gaps)", i, collected[i], id)
		}
	}
}

// TestSubmissionsServeHTTP_Pagination_OutcomeFiltered_CursorAdvancesThroughEveryPageWithoutDuplicatesOrGaps
// is the outcome-filtered counterpart to the unfiltered walk above. It
// exercises listSubmissionsPage's "default" branch specifically --
// validation.ListByOutcome's own keyset query composed with
// submission.GetMany and mergeOutcomeFilteredRows -- which the unfiltered
// walk never reaches. Five submissions share one real "unsupported"
// outcome; a sixth, unrelated "valid" submission proves the filter and its
// total exclude what doesn't match. Runs against real PostgreSQL, through
// the real HTTP handler -- no mocked repository or query path.
func TestSubmissionsServeHTTP_Pagination_OutcomeFiltered_CursorAdvancesThroughEveryPageWithoutDuplicatesOrGaps(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	var want []int64
	for i := 0; i < 5; i++ {
		auditID := fmt.Sprintf("aud-outcome-page-%d", i)
		want = append(want, seedValidatedSubmission(t, db, unsupportedSubmissionsFixture, auditID, "ResponseComplete"))
	}
	// A differently-outcomed submission: must never appear in this
	// filter's pages, and must not inflate its total.
	seedValidatedSubmission(t, db, validEventJSON, "aud-outcome-page-other", "ResponseComplete")
	srv := newSubmissionsTestServer(t, db)

	var collected []int64
	var pageCursors []int64 // each non-terminal page's own reported nextCursor, in fetch order
	sawTerminalPage := false
	cursor := ""
	for page := 0; ; page++ {
		if page > 10 {
			t.Fatal("pagination did not terminate within 10 pages")
		}
		path := "/v1/submissions?outcome=unsupported&limit=2"
		if cursor != "" {
			path += "&cursor=" + cursor
		}
		resp, body := get(t, srv, path, testToken)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("page %d: status = %d, want 200 (body: %s)", page, resp.StatusCode, body)
		}
		var got submissionsListResponse
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatalf("page %d: unmarshal: %v", page, err)
		}
		if len(got.Submissions) > 2 {
			t.Fatalf("page %d: len(submissions) = %d, want at most 2 (limit)", page, len(got.Submissions))
		}
		if got.Total != 5 {
			t.Errorf("page %d: total = %d, want 5 (only the shared-outcome submissions, never the unrelated one)", page, got.Total)
		}
		for _, s := range got.Submissions {
			if s.ValidationOutcome.Outcome != "unsupported" {
				t.Errorf("page %d: submission %d outcome = %q, want unsupported", page, s.SubmissionID, s.ValidationOutcome.Outcome)
			}
			collected = append(collected, s.SubmissionID)
		}
		if got.NextCursor == nil {
			sawTerminalPage = true
			break
		}
		// nextCursor must be exactly this page's own last row -- not some
		// other value that would silently skip or re-fetch rows.
		lastID := got.Submissions[len(got.Submissions)-1].SubmissionID
		if *got.NextCursor != lastID {
			t.Fatalf("page %d: nextCursor = %d, want %d (this page's own last row)", page, *got.NextCursor, lastID)
		}
		pageCursors = append(pageCursors, *got.NextCursor)
		cursor = strconv.FormatInt(*got.NextCursor, 10)
	}

	if !sawTerminalPage {
		t.Fatal("walk exited without ever reaching a terminal page (nextCursor == nil)")
	}
	if len(collected) != len(want) {
		t.Fatalf("collected %d submissions across pages, want %d", len(collected), len(want))
	}
	for i, id := range want {
		if collected[i] != id {
			t.Errorf("collected[%d] = %d, want %d (ascending, no duplicates or gaps)", i, collected[i], id)
		}
	}
	// The walk must have actually crossed more than one page, and each
	// page's cursor must strictly progress -- otherwise cursor continuity
	// specifically would go unexercised.
	if len(pageCursors) < 2 {
		t.Fatalf("walk produced %d non-terminal page(s), want at least 2 to exercise cursor continuity", len(pageCursors))
	}
	for i := 1; i < len(pageCursors); i++ {
		if pageCursors[i] <= pageCursors[i-1] {
			t.Errorf("cursor progression not strictly ascending: pageCursors[%d]=%d <= pageCursors[%d]=%d", i, pageCursors[i], i-1, pageCursors[i-1])
		}
	}
}
