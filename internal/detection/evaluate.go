package detection

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"cnsdp/internal/normalization"
	"cnsdp/internal/submission"
)

// characteristicSet builds the flat, ID-keyed set of named
// characteristics known to be true for event. This is the only
// scenario-specific mapping in this file: it translates normalized Event
// fields into the vocabulary the three definitions' requires_any /
// requires_all lists reference by id. The matching algorithm below
// (evaluate, operationMatches, outcomeSatisfied) never branches on
// scenario -- it only ever asks "is this characteristic id present in the
// set", so a fourth scenario's definition would need a new entry here,
// never a new evaluator.
func characteristicSet(event normalization.Event) map[string]bool {
	set := make(map[string]bool)
	if event.Exec != nil {
		set["stdin_streaming"] = event.Exec.Stdin
		set["tty_allocation"] = event.Exec.TTY
	}
	if event.PodCreation != nil {
		set["privileged_container"] = event.PodCreation.Privileged
		set["host_network"] = event.PodCreation.HostNetwork
		set["host_pid"] = event.PodCreation.HostPID
		set["host_ipc"] = event.PodCreation.HostIPC
		set["host_path_volume"] = event.PodCreation.HostPathVolume
	}
	if event.ClusterRoleBinding != nil {
		set["role_ref_cluster_admin"] = event.ClusterRoleBinding.RoleRef.Kind == "ClusterRole" &&
			event.ClusterRoleBinding.RoleRef.Name == "cluster-admin"
	}
	return set
}

// operationMatches checks a definition's declared operation against the
// event's target and verb. Resource and verb are wildcards when left
// empty in the definition; subresource is always matched exactly
// (including empty), since an empty subresource is itself a meaningful,
// deliberate constraint -- it is what distinguishes scenario 2's plain
// Pod creation from scenario 1's pods/exec subresource, both of which
// declare resource: pods.
func operationMatches(op Operation, event normalization.Event) bool {
	if op.Resource != "" && op.Resource != event.Target.Resource {
		return false
	}
	if op.Subresource != event.Target.Subresource {
		return false
	}
	if op.Verb != "" && op.Verb != event.Operation.Verb {
		return false
	}
	return true
}

// outcomeSatisfied checks a definition's requires_outcome constraint. An
// empty constraint means the definition matches regardless of outcome
// (scenario 1, FR-024). "success" reads normalization's own source-neutral
// Successful signal -- derived per source-event family at normalization
// time (Kubernetes: HTTP response code 200-299; CloudTrail: no recorded
// errorCode) -- so this generic evaluator never branches on family itself.
// Any other declared value is a definition authoring error that Validate
// does not currently reject; treated here as never satisfied rather than
// an error, so an unrecognized constraint fails closed (no match) instead
// of failing open.
func outcomeSatisfied(requiresOutcome string, event normalization.Event) bool {
	switch requiresOutcome {
	case "":
		return true
	case "success":
		return event.Outcome.Successful
	default:
		return false
	}
}

// satisfiedFrom returns the subset of list whose characteristic id is
// present (true) in set, preserving list's declared order.
func satisfiedFrom(list []Characteristic, set map[string]bool) []Characteristic {
	var out []Characteristic
	for _, c := range list {
		if set[c.ID] {
			out = append(out, c)
		}
	}
	return out
}

// evaluate is the generic, definition-driven matching algorithm (FR-023):
// it interprets def's declared operation, requires_outcome, requires_any,
// and requires_all fields against event, with no per-scenario branching.
// matched is true only if the operation matches, the outcome constraint
// (if any) is satisfied, every requires_all characteristic is present,
// and -- if requires_any is non-empty -- at least one requires_any
// characteristic is present. satisfied lists every documented
// characteristic actually present in the event, from both lists
// combined (FR-024: "every documented characteristic present in the
// request shall be recorded"), and is nil when matched is false.
func evaluate(event normalization.Event, def Definition) (matched bool, satisfied []Characteristic) {
	if !operationMatches(def.Conditions.Operation, event) {
		return false, nil
	}
	if !outcomeSatisfied(def.Conditions.RequiresOutcome, event) {
		return false, nil
	}

	set := characteristicSet(event)

	anyMatched := satisfiedFrom(def.Conditions.RequiresAny, set)
	if len(def.Conditions.RequiresAny) > 0 && len(anyMatched) == 0 {
		return false, nil
	}

	allMatched := satisfiedFrom(def.Conditions.RequiresAll, set)
	if len(allMatched) != len(def.Conditions.RequiresAll) {
		return false, nil
	}

	satisfied = append(satisfied, anyMatched...)
	satisfied = append(satisfied, allMatched...)
	return true, satisfied
}

// MatchReason is the persisted content of a matching detection_results
// row (FR-028): the detection definition that matched, identified by
// scenario/name/revision, and the documented conditions that were
// satisfied. The full normalized event -- the event information that
// satisfied them -- remains separately available via its own artifact
// (normalized_events, reachable through the same normalized_event_id this
// row references), consistent with the minimum evidence set (FR-031)
// treating each artifact as a distinct, individually inspectable item
// rather than duplicating content across them.
type MatchReason struct {
	Scenario                 string           `json:"scenario"`
	DefinitionName           string           `json:"definitionName"`
	DefinitionRevision       string           `json:"definitionRevision"`
	SatisfiedCharacteristics []Characteristic `json:"satisfiedCharacteristics"`
}

// ErrResultConflict is returned by Advance when a detection_results row
// already exists for a (normalized event, detection definition) pair
// with a different matched or match_reason value than what this
// evaluation just computed -- a genuine disagreement between the newly
// computed result and the one already on record. It is never silently
// resolved by overwriting the previously persisted artifact (NFR-006,
// NFR-017): the submission is left at normalized and the caller must
// investigate.
var ErrResultConflict = errors.New("detection: existing detection result has different content")

// Advance evaluates sub's normalized event against every active
// detection definition (FR-023), persists one detection_results row per
// definition -- matched or not -- and advances the submission from
// normalized to evaluated -- all in one transaction (ADR-0002).
// internal/detection owns every read and write against detection_results;
// the worker only selects and dispatches work, and never writes this
// table itself. The normalized event is read exclusively through
// normalization.Get, never through direct SQL against normalized_events.
//
// Retry safety: a repeated call for the same submission_id is accepted as
// a safe no-op retry only when, for every definition, the newly computed
// matched and match_reason exactly match what is already persisted --
// detected by one race-safe INSERT ... ON CONFLICT ... DO UPDATE ...
// WHERE statement per definition, the same idempotent-insert pattern
// internal/submission.Admit, internal/normalization.Advance, and
// internal/validation.Advance already use. A mismatch on any one
// definition returns ErrResultConflict and aborts the whole transaction
// before any status change; no row is ever silently overwritten, and no
// submission is ever left with only some of its three results persisted.
func Advance(ctx context.Context, db *sql.DB, sub *submission.Submission) error {
	rec, err := normalization.Get(ctx, db, sub.ID)
	if err != nil {
		return fmt.Errorf("detection: advance submission %d: %w", sub.ID, err)
	}

	defs, err := ActiveDefinitions(ctx, db)
	if err != nil {
		return fmt.Errorf("detection: advance submission %d: %w", sub.ID, err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("detection: begin advance tx for submission %d: %w", sub.ID, err)
	}
	defer tx.Rollback()

	for _, def := range defs {
		matched, satisfied := evaluate(rec.Event, def.Definition)

		var reason []byte
		if matched {
			reason, err = json.Marshal(MatchReason{
				Scenario:                 def.Definition.Scenario,
				DefinitionName:           def.Definition.Name,
				DefinitionRevision:       def.Revision,
				SatisfiedCharacteristics: satisfied,
			})
			if err != nil {
				return fmt.Errorf("detection: advance submission %d: marshal match reason for %s: %w", sub.ID, def.Definition.Scenario, err)
			}
		}

		if err := upsertResult(ctx, tx, rec.ID, def.ID, matched, reason); err != nil {
			return fmt.Errorf("detection: advance submission %d: %w", sub.ID, err)
		}
	}

	if err := submission.Advance(ctx, tx, sub.ID, submission.StatusNormalized, submission.StatusEvaluated); err != nil {
		return fmt.Errorf("detection: advance submission %d to evaluated: %w", sub.ID, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("detection: commit advance for submission %d: %w", sub.ID, err)
	}
	return nil
}

// upsertResult performs one race-safe, conflict-aware insert of a single
// (normalized event, definition) evaluation result.
func upsertResult(ctx context.Context, tx *sql.Tx, normalizedEventID, definitionID int64, matched bool, matchReason []byte) error {
	var reasonParam any
	if matched {
		reasonParam = matchReason
	}

	var id int64
	err := tx.QueryRowContext(ctx,
		`INSERT INTO detection_results (normalized_event_id, detection_definition_id, matched, match_reason)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (normalized_event_id, detection_definition_id) DO UPDATE
		 SET matched = EXCLUDED.matched
		 WHERE detection_results.matched = EXCLUDED.matched
		   AND detection_results.match_reason IS NOT DISTINCT FROM EXCLUDED.match_reason
		 RETURNING id`,
		normalizedEventID, definitionID, matched, reasonParam,
	).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: normalized_event %d, definition %d", ErrResultConflict, normalizedEventID, definitionID)
	}
	if err != nil {
		return fmt.Errorf("insert detection result for normalized_event %d, definition %d: %w", normalizedEventID, definitionID, err)
	}
	return nil
}

// MatchedResult is one matched detection_results row as needed by
// downstream alert generation (ARCH-01 §2 module 5) and evidence
// assembly (module 6): the result's own id -- the value
// alerts.detection_result_id must reference -- the detection_definitions
// row it was evaluated against, and the match reason exactly as it was
// persisted at evaluation time.
type MatchedResult struct {
	ID                    int64
	DetectionDefinitionID int64
	MatchReason           MatchReason
}

// DB is the minimal subset of *sql.DB / *sql.Tx MatchedResults and
// GetDefinition need, so a caller can pass either a plain connection or
// an already-open transaction -- e.g. internal/evidence's verification
// transaction (ADR-0002).
type DB interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// MatchedResults returns every matched detection_results row for the
// given normalized event, ordered by id for determinism. This is the
// sanctioned way a downstream module reads detection_results:
// internal/detection owns every read and write against this table (see
// package doc), so internal/alerting and internal/evidence call this
// rather than querying the table directly. The match reason is returned
// exactly as persisted -- never re-derived from the currently active
// definition set, which could disagree with an older persisted result
// after a definition change (AC-014).
func MatchedResults(ctx context.Context, db DB, normalizedEventID int64) ([]MatchedResult, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, detection_definition_id, match_reason FROM detection_results
		 WHERE normalized_event_id = $1 AND matched
		 ORDER BY id`,
		normalizedEventID,
	)
	if err != nil {
		return nil, fmt.Errorf("detection: matched results for normalized event %d: %w", normalizedEventID, err)
	}
	defer rows.Close()

	var out []MatchedResult
	for rows.Next() {
		var (
			id           int64
			definitionID int64
			reason       []byte
		)
		if err := rows.Scan(&id, &definitionID, &reason); err != nil {
			return nil, fmt.Errorf("detection: matched results for normalized event %d: scan: %w", normalizedEventID, err)
		}
		var mr MatchReason
		if err := json.Unmarshal(reason, &mr); err != nil {
			return nil, fmt.Errorf("detection: matched results for normalized event %d: unmarshal match reason for result %d: %w", normalizedEventID, id, err)
		}
		out = append(out, MatchedResult{ID: id, DetectionDefinitionID: definitionID, MatchReason: mr})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("detection: matched results for normalized event %d: %w", normalizedEventID, err)
	}
	return out, nil
}

// ErrDefinitionNotFound is returned by GetDefinition when no
// detection_definitions row exists for the given id -- normally
// impossible for an id referenced by a persisted detection_results row,
// since detection_definition_id is a NOT NULL foreign key.
var ErrDefinitionNotFound = errors.New("detection: detection definition not found")

// GetDefinition returns the exact detection_definitions row content and
// revision for id -- the definition a specific persisted detection_result
// referenced, read back exactly as stored, never re-resolved from the
// currently active definition set (mirrors MatchedResults' AC-014
// rationale: a later definition change must not alter what an existing
// result or alert resolves to). This is the sanctioned way
// internal/evidence reads detection_definitions content for the evidence
// inventory's detection-definition artifact.
func GetDefinition(ctx context.Context, db DB, id int64) (Definition, string, error) {
	var (
		revision string
		content  []byte
	)
	err := db.QueryRowContext(ctx,
		`SELECT revision, content FROM detection_definitions WHERE id = $1`,
		id,
	).Scan(&revision, &content)
	if errors.Is(err, sql.ErrNoRows) {
		return Definition{}, "", ErrDefinitionNotFound
	}
	if err != nil {
		return Definition{}, "", fmt.Errorf("detection: get definition %d: %w", id, err)
	}

	var def Definition
	if err := json.Unmarshal(content, &def); err != nil {
		return Definition{}, "", fmt.Errorf("detection: get definition %d: unmarshal content: %w", id, err)
	}
	return def, revision, nil
}

// ErrResultNotFound is returned by GetResult when no detection_results row
// exists for the given id.
var ErrResultNotFound = errors.New("detection: detection result not found")

// GetResult returns a single detection_results row by its own id, read
// back exactly as persisted -- the reverse-lookup counterpart to
// MatchedResults (which is keyed by normalized_event_id, the forward-flow
// direction). This is the sanctioned way a downstream module reads one
// specific detection result once it has resolved the id through
// internal/traceability.Locate (internal/evidence, via
// internal/retrieval).
func GetResult(ctx context.Context, db DB, id int64) (MatchedResult, error) {
	var (
		definitionID int64
		reason       []byte
	)
	err := db.QueryRowContext(ctx,
		`SELECT detection_definition_id, match_reason FROM detection_results WHERE id = $1`,
		id,
	).Scan(&definitionID, &reason)
	if errors.Is(err, sql.ErrNoRows) {
		return MatchedResult{}, ErrResultNotFound
	}
	if err != nil {
		return MatchedResult{}, fmt.Errorf("detection: get result %d: %w", id, err)
	}

	var mr MatchReason
	if reason != nil {
		if err := json.Unmarshal(reason, &mr); err != nil {
			return MatchedResult{}, fmt.Errorf("detection: get result %d: unmarshal match reason: %w", id, err)
		}
	}
	return MatchedResult{ID: id, DetectionDefinitionID: definitionID, MatchReason: mr}, nil
}
