package worker

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"cnsdp/internal/submission"
	"cnsdp/internal/validation"
)

// TestErrorFamily_ResourceExhausted proves a SQLSTATE 53100 (disk_full)
// error -- however deeply wrapped by a stage's own Advance function, as a
// real one would be via internal/validation's "insert validation outcome
// for submission %d: %w" convention -- classifies as the distinct
// "resource_exhausted" family, never falling through to the generic
// "processing_error" default (AC-023; NFR-036).
func TestErrorFamily_ResourceExhausted(t *testing.T) {
	pgErr := &pgconn.PgError{Code: "53100", Message: "could not extend file: No space left on device"}
	wrapped := fmt.Errorf("validation: insert validation outcome for submission %d: %w", 7, pgErr)

	if got := errorFamily(wrapped); got != "resource_exhausted" {
		t.Errorf("errorFamily(wrapped 53100) = %q, want %q", got, "resource_exhausted")
	}
}

// TestErrorFamily_NeighboringClass53CodesNotResourceExhausted proves
// out_of_memory (53200) and too_many_connections (53300) -- different
// resource classes from disk_full -- are never classified
// resource_exhausted; they fall through to the generic default, matching
// this design's explicit requirement not to treat all of SQLSTATE class
// 53 as storage exhaustion.
func TestErrorFamily_NeighboringClass53CodesNotResourceExhausted(t *testing.T) {
	tests := []struct {
		name string
		code string
	}{
		{"out_of_memory", "53200"},
		{"too_many_connections", "53300"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pgErr := &pgconn.PgError{Code: tt.code}
			if got := errorFamily(pgErr); got == "resource_exhausted" {
				t.Errorf("errorFamily(%s %s) = %q, want anything but resource_exhausted", tt.name, tt.code, got)
			}
		})
	}
}

// TestErrorFamily_PreservesExistingClassification is a regression guard:
// adding the resource_exhausted branch must not change any pre-existing
// classification's outcome.
func TestErrorFamily_PreservesExistingClassification(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"context canceled", context.Canceled, "context_canceled"},
		{"validation outcome conflict", validation.ErrOutcomeConflict, "artifact_conflict"},
		{"submission status conflict", submission.ErrStatusConflict, "status_conflict"},
		{"submission not found", submission.ErrNotFound, "not_found"},
		{"unrelated error", errors.New("boom"), "processing_error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := errorFamily(tt.err); got != tt.want {
				t.Errorf("errorFamily(%v) = %q, want %q", tt.err, got, tt.want)
			}
		})
	}
}
