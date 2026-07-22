// Package validation implements the validation and classification module
// (ARCH-01 §2 module 2): FR-004 through FR-014's exclusive four-way
// classification of each submission's raw source event, assessed in the
// documented precedence order — unsupported, invalid, incomplete, valid
// (FR-006) — producing exactly one outcome and, for every non-valid
// outcome, a stated reason (FR-011, FR-012). It contains no normalization
// or detection-evaluation logic: it checks only that a scenario-relevant
// submission carries enough structural content for a later checkpoint to
// evaluate it, never whether any documented characteristic actually holds
// (FR-023 remains a later checkpoint's responsibility). It owns every
// read and write against validation_outcomes (ADR-0002); no other
// package writes this table -- the worker only selects and dispatches
// work via Advance, the same ownership pattern internal/normalization
// and internal/detection follow for their own stages.
package validation

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"

	"cnsdp/internal/submission"
)

// Outcome is one of the four mutually exclusive validation outcomes
// (PC-G-002) — the only values the validation_outcomes.outcome CHECK
// constraint (migrations/0001_initial_schema.up.sql) permits.
type Outcome string

const (
	OutcomeUnsupported Outcome = "unsupported"
	OutcomeInvalid     Outcome = "invalid"
	OutcomeIncomplete  Outcome = "incomplete"
	OutcomeValid       Outcome = "valid"
)

// Result is the classification of one submission's raw source event.
// Reason is empty if and only if Outcome is OutcomeValid (FR-011, FR-012).
type Result struct {
	Outcome Outcome
	Reason  string
}

// supportedKind and supportedAPIVersion define the documented supported
// source-event form (FR-002): a Kubernetes audit.k8s.io/v1 Event.
const (
	supportedKind       = "Event"
	supportedAPIVersion = "audit.k8s.io/v1"
)

// Classify assesses one submission's raw source event — exactly the bytes
// stored in submissions.raw_event — in the documented precedence order
// (FR-006): unsupported, invalid, incomplete, valid. Exactly one outcome is
// returned (FR-005); Classify never panics on any input.
func Classify(raw []byte) Result {
	var ev auditv1.Event
	decodeErr := json.Unmarshal(raw, &ev)

	// Attribution (FR-010) is checked directly against the decoded
	// kind/apiVersion markers, never against decodeErr: encoding/json
	// populates every field it can — including kind/apiVersion — even when
	// a later field fails to type-decode, and leaves them at their zero
	// value whenever raw isn't a parseable JSON object at all. Either way,
	// checking the markers directly is sufficient and correct, and matches
	// the boundary-fixture set's requirement (AC-006) that an unsupported
	// submission is classified unsupported even when it also contains
	// malformed content.
	if ev.Kind != supportedKind || ev.APIVersion != supportedAPIVersion {
		return Result{OutcomeUnsupported, fmt.Sprintf(
			"submission is not attributable to the supported %s %s source form (found kind %q, apiVersion %q)",
			supportedAPIVersion, supportedKind, ev.Kind, ev.APIVersion)}
	}

	if decodeErr != nil {
		return Result{OutcomeInvalid, fmt.Sprintf(
			"submission does not conform to the documented %s %s structure: %v",
			supportedAPIVersion, supportedKind, decodeErr)}
	}

	if reason, missing := missingCoreField(ev); missing {
		return Result{OutcomeIncomplete, reason}
	}
	if reason, missing := missingScenarioField(ev); missing {
		return Result{OutcomeIncomplete, reason}
	}

	return Result{OutcomeValid, ""}
}

// missingCoreField checks FR-007 items (a) through (f): the core event
// information every valid submission must contain, regardless of scenario.
func missingCoreField(ev auditv1.Event) (reason string, missing bool) {
	if ev.AuditID == "" || ev.Stage == "" {
		return "missing source event identity (auditID and/or audit stage) required by FR-007(a)", true
	}
	if ev.RequestReceivedTimestamp.IsZero() {
		return "missing request time (requestReceivedTimestamp) required by FR-007(b)", true
	}
	if ev.User.Username == "" {
		return "missing requesting subject (user.username) required by FR-007(c)", true
	}
	if ev.Verb == "" || ev.RequestURI == "" {
		return "missing operation performed (verb and/or requestURI) required by FR-007(d)", true
	}
	if ev.ObjectRef == nil || ev.ObjectRef.Resource == "" {
		return "missing target resource (objectRef.resource) required by FR-007(e)", true
	}
	if s := scenarioOf(ev); s == scenario2 || s == scenario3 {
		// FR-007(f): outcome information is required only where an
		// applicable documented detection condition requires successful
		// completion. Scenarios 2 and 3 match only successful operations
		// (FR-025, FR-026); scenario 1 matches regardless of outcome
		// (FR-024), and a non-scenario-relevant event has no such
		// condition at all — so this check does not apply to either.
		if ev.ResponseStatus == nil || ev.ResponseStatus.Code == 0 {
			return "missing recorded outcome information (responseStatus.code) required by FR-007(f) to determine successful completion", true
		}
	}
	return "", false
}

// scenario identifies which approved detection scenario, if any, a
// submission's recorded operation is relevant to (the "scenario-relevant
// operation" definition FR-007(g)/FR-009 refer to).
type scenario int

const (
	scenarioNone scenario = iota
	scenario1             // pods/exec
	scenario2             // Pod creation
	scenario3             // ClusterRoleBinding creation
)

// scenarioOf classifies ev using only the generically-required objectRef
// and verb fields (FR-007(d), (e)) — never requestObject content, which may
// be entirely absent for a scenario-relevant operation depending on audit
// stage (ADR-0003 Q2, confirmed by Spike 1's captured RequestReceived-stage
// Pod-creation and ClusterRoleBinding-creation events).
func scenarioOf(ev auditv1.Event) scenario {
	if ev.ObjectRef == nil {
		return scenarioNone
	}
	switch {
	case ev.ObjectRef.Resource == "pods" && ev.ObjectRef.Subresource == "exec":
		return scenario1
	case ev.ObjectRef.Resource == "pods" && ev.ObjectRef.Subresource == "" && ev.Verb == "create":
		return scenario2
	case ev.ObjectRef.Resource == "clusterrolebindings" &&
		ev.ObjectRef.APIGroup == "rbac.authorization.k8s.io" && ev.Verb == "create":
		return scenario3
	default:
		return scenarioNone
	}
}

// missingScenarioField checks FR-007(g)/FR-009's scenario-required
// completeness for a scenario-relevant operation. Scope is deliberately
// narrow: presence of the structural content the documented detection
// conditions (FR-024–FR-026) will need to be evaluated later — never
// whether any characteristic actually holds. Evaluating those conditions is
// FR-023's concern, left entirely to a later checkpoint.
func missingScenarioField(ev auditv1.Event) (reason string, missing bool) {
	switch scenarioOf(ev) {
	case scenario1:
		// The exec request's interactive-execution signal lives entirely in
		// requestURI (ADR-0003 Q1), already required by core item (d) —
		// no additional field is required here.
		return "", false

	case scenario2:
		if ev.RequestObject == nil || len(ev.RequestObject.Raw) == 0 {
			return "missing requestObject required by FR-007(g)/FR-018 to evaluate scenario 2's Pod specification", true
		}
		var pod struct {
			Kind string          `json:"kind"`
			Spec json.RawMessage `json:"spec"`
		}
		if err := json.Unmarshal(ev.RequestObject.Raw, &pod); err != nil {
			return fmt.Sprintf("requestObject required by FR-007(g)/FR-018 does not parse as a Pod: %v", err), true
		}
		if pod.Kind != "Pod" || len(pod.Spec) == 0 || string(pod.Spec) == "null" {
			return "requestObject required by FR-007(g)/FR-018 does not contain a Pod specification (spec)", true
		}
		return "", false

	case scenario3:
		if ev.RequestObject == nil || len(ev.RequestObject.Raw) == 0 {
			return "missing requestObject required by FR-007(g)/FR-018 to evaluate scenario 3's role reference and bound subjects", true
		}
		var crb struct {
			Kind     string `json:"kind"`
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
			RoleRef rbacv1.RoleRef `json:"roleRef"`
		}
		if err := json.Unmarshal(ev.RequestObject.Raw, &crb); err != nil {
			return fmt.Sprintf("requestObject required by FR-007(g)/FR-018 does not parse as a ClusterRoleBinding: %v", err), true
		}
		if crb.Kind != "ClusterRoleBinding" || crb.RoleRef.Name == "" {
			return "requestObject required by FR-007(g)/FR-018 does not contain a referenced role (roleRef.name)", true
		}
		// Binding identity (FR-018): objectRef.name is the primary source;
		// requestObject.metadata.name is the fallback for any captured
		// event shape where the former is absent.
		if (ev.ObjectRef == nil || ev.ObjectRef.Name == "") && crb.Metadata.Name == "" {
			return "missing target binding identity (objectRef.name / requestObject.metadata.name) required by FR-007(g)/FR-018", true
		}
		return "", false

	default:
		return "", false
	}
}

// ErrOutcomeConflict is returned by Advance when a validation_outcomes row
// already exists for the submission with a different outcome or reason
// than what this call just computed -- a genuine disagreement between the
// newly derived classification and the one already on record. It is
// never silently resolved by overwriting the previously persisted
// artifact (NFR-006, NFR-017): the submission is left at admitted and the
// caller must investigate.
var ErrOutcomeConflict = errors.New("validation: existing validation outcome has different content")

// Advance classifies sub's raw source event, persists it as this
// submission's validation_outcomes artifact, and advances the submission
// from admitted to validated -- all in one transaction (ADR-0002),
// unconditionally, regardless of which of the four outcomes was recorded.
// "validated" represents workflow progress through this stage, not a
// successful validation result (FR-014): a non-valid outcome still
// advances here and then stays parked at validated, since no later stage
// handler ever claims it. internal/validation owns every read and write
// against validation_outcomes; the worker only selects and dispatches
// work, and never writes this table itself.
//
// Retry safety: a repeated call for the same submission_id is accepted as
// a safe no-op retry only when the newly computed outcome and reason
// exactly match what is already persisted -- detected by one race-safe
// INSERT ... ON CONFLICT ... DO UPDATE ... WHERE statement, the same
// idempotent-insert pattern internal/submission.Admit and
// internal/normalization.Advance already use. A mismatch returns
// ErrOutcomeConflict and leaves the submission at admitted; the existing
// artifact is never overwritten. Once persisted, the guarded
// submission.Advance call behaves exactly as it does for every other
// stage: a submission no longer at admitted (e.g. a true retry after a
// prior fully-committed call) yields ErrStatusConflict, not a silent
// reapplication.
func Advance(ctx context.Context, db *sql.DB, sub *submission.Submission) error {
	result := Classify(sub.RawEvent)

	var reason sql.NullString
	if result.Reason != "" {
		reason = sql.NullString{String: result.Reason, Valid: true}
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("validation: begin advance tx for submission %d: %w", sub.ID, err)
	}
	defer tx.Rollback()

	var outcomeID int64
	err = tx.QueryRowContext(ctx,
		`INSERT INTO validation_outcomes (submission_id, outcome, reason)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (submission_id) DO UPDATE
		 SET outcome = EXCLUDED.outcome
		 WHERE validation_outcomes.outcome = EXCLUDED.outcome
		   AND validation_outcomes.reason IS NOT DISTINCT FROM EXCLUDED.reason
		 RETURNING id`,
		sub.ID, string(result.Outcome), reason,
	).Scan(&outcomeID)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: submission %d", ErrOutcomeConflict, sub.ID)
	}
	if err != nil {
		return fmt.Errorf("validation: insert validation outcome for submission %d: %w", sub.ID, err)
	}

	if err := submission.Advance(ctx, tx, sub.ID, submission.StatusAdmitted, submission.StatusValidated); err != nil {
		return fmt.Errorf("validation: advance submission %d to validated: %w", sub.ID, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("validation: commit advance for submission %d: %w", sub.ID, err)
	}
	return nil
}
