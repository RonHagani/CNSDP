// Package normalization implements the normalization module (ARCH-01 §2
// module 3): FR-015 through FR-019's transformation of every valid
// submission's raw source event into exactly one normalized event, and
// FR-014's guarantee that only that transformation -- never validation or
// detection logic -- happens here. It owns every read and write against
// normalized_events (ADR-0002); no other package writes this table.
//
// Normalization is deliberately independent of internal/validation: both
// packages derive "which approved scenario, if any, does this operation
// belong to" from the same underlying audit-event signal (FR-007(d),(e)),
// but ARCH-01's module-ownership boundary does not permit normalization to
// depend on validation, so resolveScenario in this package is its own
// small, locally tested resolver, not an imported one.
package normalization

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"time"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"

	"cnsdp/internal/submission"
)

// RepresentationRevision identifies the version of the normalized
// representation this package produces (FR-016, NFR-026). Hardcoded for
// the walking skeleton -- the same "hardcoded revision-1 id" allowance
// ARCH-01 §8 grants detection definitions applies here; a content-hash- or
// history-derived revisioning scheme is not required at this checkpoint.
const RepresentationRevision = "v1"

// Event is the normalized representation of one valid source event
// (FR-015, FR-016): the core content required of every normalized event
// (FR-017), plus, where the source event records a scenario-relevant
// operation, the additional content required to evaluate that scenario's
// documented detection conditions (FR-018). At most one of Exec,
// PodCreation, and ClusterRoleBinding is non-nil, depending on which
// approved scenario, if any, the operation belongs to.
type Event struct {
	Subject     Subject   `json:"subject"`
	Operation   Operation `json:"operation"`
	Target      Target    `json:"target"`
	Outcome     Outcome   `json:"outcome"`
	RequestTime time.Time `json:"requestTime"`

	Exec                *ExecCharacteristics               `json:"exec,omitempty"`
	PodCreation         *PodCreationCharacteristics        `json:"podCreation,omitempty"`
	ClusterRoleBinding  *ClusterRoleBindingCharacteristics `json:"clusterRoleBinding,omitempty"`
	CloudTrailIAMAction *CloudTrailIAMAction               `json:"cloudTrailIAMAction,omitempty"`
}

// Subject conveys the requesting subject (FR-017).
type Subject struct {
	Username string `json:"username"`
}

// Operation conveys the operation performed (FR-017), including the
// verb and the recorded request URI. RequestURI is core content, not
// scenario-specific: it is the sole carrier of scenario 1's
// interactive-execution signal (ADR-0003 Q1, FR-018) and is preserved for
// every event regardless of scenario.
type Operation struct {
	Verb       string `json:"verb"`
	RequestURI string `json:"requestURI"`
}

// Target conveys the operation's target resource (FR-017). Resource is
// the Kubernetes API server's REST resource name as recorded in
// objectRef.resource (e.g. "pods", "clusterrolebindings") -- audit events
// do not carry the object's Kubernetes Kind (e.g. "Pod") in objectRef, so
// this field must not be labeled or treated as one.
type Target struct {
	Resource    string `json:"resource"`
	Name        string `json:"name"`
	Namespace   string `json:"namespace,omitempty"`
	Subresource string `json:"subresource,omitempty"`
}

// Outcome conveys the recorded outcome information of the request,
// sufficient to determine successful completion where a documented
// detection condition requires it (FR-017). Code is 0 when the source
// event carries no responseStatus -- permitted for non-scenario-2/3
// events by FR-007(f). ErrorCode is CloudTrail's own native outcome
// signal (empty means success) and is always empty for a Kubernetes
// event; Code is always 0 for a CloudTrail event. Both are retained,
// family-specific and descriptive only, for evidence/explainability
// fidelity (FR-029, FR-032).
//
// Successful is the single source-neutral signal every detection
// condition actually needs: derived here, per family, at normalization
// time (Kubernetes: an HTTP response code in 200-299; CloudTrail: an
// absent errorCode) so that internal/detection's generic evaluator never
// has to know how any one family encodes success -- it reads only this
// field, never Code or ErrorCode.
type Outcome struct {
	Code       int32  `json:"code,omitempty"`
	ErrorCode  string `json:"errorCode,omitempty"`
	Successful bool   `json:"successful"`
}

// ExecCharacteristics conveys scenario 1's documented interactive-execution
// characteristics (FR-018, FR-024), derived from the request URI's query
// parameters (ADR-0003 Q1) -- never from requestObject, which exec
// requests do not carry.
type ExecCharacteristics struct {
	Stdin bool `json:"stdin"`
	TTY   bool `json:"tty"`
}

// PodCreationCharacteristics conveys scenario 2's five documented
// high-risk privilege and host-access characteristics (FR-018, FR-025).
type PodCreationCharacteristics struct {
	Privileged     bool `json:"privileged"`
	HostNetwork    bool `json:"hostNetwork"`
	HostPID        bool `json:"hostPID"`
	HostIPC        bool `json:"hostIPC"`
	HostPathVolume bool `json:"hostPathVolume"`
}

// ClusterRoleBindingCharacteristics conveys scenario 3's documented
// identity, referenced role, and bound subjects (FR-018, FR-026).
type ClusterRoleBindingCharacteristics struct {
	BindingName string           `json:"bindingName"`
	RoleRef     rbacv1.RoleRef   `json:"roleRef"`
	Subjects    []rbacv1.Subject `json:"subjects,omitempty"`
}

// CloudTrailIAMAction conveys the CloudTrail-family scenario-specific
// content FR-018 requires for scenarios 4-6: the identity of the affected
// user and MFA device (scenario 4, FR-037); the affected user (scenario 5,
// FR-038); or the affected principal and referenced policy (scenario 6,
// FR-039). One struct covers all three rather than three near-identical
// single-field types, since they share the same "affected principal, plus
// an optional device or policy" shape.
type CloudTrailIAMAction struct {
	AffectedUser   string `json:"affectedUser,omitempty"`
	AffectedDevice string `json:"affectedDevice,omitempty"`
	PolicyName     string `json:"policyName,omitempty"`
}

// Normalize transforms one submission's raw source event into its
// normalized representation. raw is assumed already classified valid
// (FR-014 ensures Normalize is only ever invoked for a valid submission;
// FR-007 guarantees a valid submission is normalizable by construction).
// An error return here reflects a source event that did not actually
// carry what its valid classification promised -- an engineering fault
// (functional-requirements.md, "Excluded requirement candidates" #8), not
// a product data-quality outcome. family selects which of the two
// approved source-event families raw is normalized as.
func Normalize(raw []byte, family submission.Family) (Event, error) {
	if family == submission.FamilyCloudTrail {
		return normalizeCloudTrail(raw)
	}
	return normalizeKubernetes(raw)
}

// normalizeKubernetes is Normalize's Kubernetes-family path — unchanged
// Kubernetes normalization logic, preserved exactly as it was before the
// CloudTrail family was added.
func normalizeKubernetes(raw []byte) (Event, error) {
	var ev auditv1.Event
	if err := json.Unmarshal(raw, &ev); err != nil {
		return Event{}, fmt.Errorf("normalization: unmarshal source event: %w", err)
	}

	out := Event{
		Subject:     Subject{Username: ev.User.Username},
		Operation:   Operation{Verb: ev.Verb, RequestURI: ev.RequestURI},
		Target:      targetOf(ev.ObjectRef),
		Outcome:     outcomeOf(ev.ResponseStatus),
		RequestTime: ev.RequestReceivedTimestamp.Time,
	}

	switch resolveScenario(ev.ObjectRef, ev.Verb) {
	case scenario1:
		out.Exec = execCharacteristics(ev.RequestURI)
	case scenario2:
		pc, err := podCreationCharacteristics(ev.RequestObject)
		if err != nil {
			return Event{}, fmt.Errorf("normalization: scenario 2 pod specification: %w", err)
		}
		out.PodCreation = pc
	case scenario3:
		crb, err := clusterRoleBindingCharacteristics(ev.ObjectRef, ev.RequestObject)
		if err != nil {
			return Event{}, fmt.Errorf("normalization: scenario 3 cluster role binding: %w", err)
		}
		out.ClusterRoleBinding = crb
	}

	return out, nil
}

func targetOf(ref *auditv1.ObjectReference) Target {
	if ref == nil {
		return Target{}
	}
	return Target{
		Resource:    ref.Resource,
		Name:        ref.Name,
		Namespace:   ref.Namespace,
		Subresource: ref.Subresource,
	}
}

// outcomeOf derives Kubernetes' Outcome, including Successful: an HTTP
// response code in the inclusive range 200-299 (the same "success" rule
// internal/detection's generic evaluator previously applied directly to
// Code -- relocated here so the evaluator never needs family-specific
// knowledge of how "success" is encoded). rs == nil (permitted for
// non-scenario-2/3 events by FR-007(f)) yields Successful: false, which is
// never actually consulted for such an event: a definition's
// requires_outcome is empty exactly when no responseStatus was required.
func outcomeOf(rs *metav1.Status) Outcome {
	if rs == nil {
		return Outcome{}
	}
	return Outcome{Code: rs.Code, Successful: rs.Code >= 200 && rs.Code < 300}
}

func execCharacteristics(requestURI string) *ExecCharacteristics {
	u, err := url.Parse(requestURI)
	if err != nil {
		return &ExecCharacteristics{}
	}
	q := u.Query()
	return &ExecCharacteristics{
		Stdin: q.Get("stdin") == "true",
		TTY:   q.Get("tty") == "true",
	}
}

// podCreationCharacteristics decodes reqObj as a Pod and extracts the five
// documented high-risk characteristics (FR-025). It reuses the official
// corev1.PodSpec type rather than hand-rolled field parsing.
func podCreationCharacteristics(reqObj *runtime.Unknown) (*PodCreationCharacteristics, error) {
	if reqObj == nil || len(reqObj.Raw) == 0 {
		return nil, fmt.Errorf("missing requestObject")
	}
	var pod struct {
		Kind string         `json:"kind"`
		Spec corev1.PodSpec `json:"spec"`
	}
	if err := json.Unmarshal(reqObj.Raw, &pod); err != nil {
		return nil, fmt.Errorf("decode pod: %w", err)
	}

	privileged := false
	for _, c := range pod.Spec.Containers {
		if c.SecurityContext != nil && c.SecurityContext.Privileged != nil && *c.SecurityContext.Privileged {
			privileged = true
			break
		}
	}
	if !privileged {
		for _, c := range pod.Spec.InitContainers {
			if c.SecurityContext != nil && c.SecurityContext.Privileged != nil && *c.SecurityContext.Privileged {
				privileged = true
				break
			}
		}
	}

	hostPathVolume := false
	for _, v := range pod.Spec.Volumes {
		if v.HostPath != nil {
			hostPathVolume = true
			break
		}
	}

	return &PodCreationCharacteristics{
		Privileged:     privileged,
		HostNetwork:    pod.Spec.HostNetwork,
		HostPID:        pod.Spec.HostPID,
		HostIPC:        pod.Spec.HostIPC,
		HostPathVolume: hostPathVolume,
	}, nil
}

// clusterRoleBindingCharacteristics decodes reqObj as a ClusterRoleBinding
// and extracts the binding identity, referenced role, and bound subjects
// (FR-018, FR-026). Binding identity prefers objectRef.name, falling back
// to requestObject.metadata.name -- the same fallback validation applies
// for FR-007(g) completeness.
func clusterRoleBindingCharacteristics(objectRef *auditv1.ObjectReference, reqObj *runtime.Unknown) (*ClusterRoleBindingCharacteristics, error) {
	if reqObj == nil || len(reqObj.Raw) == 0 {
		return nil, fmt.Errorf("missing requestObject")
	}
	var crb struct {
		Kind     string `json:"kind"`
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
		RoleRef  rbacv1.RoleRef   `json:"roleRef"`
		Subjects []rbacv1.Subject `json:"subjects"`
	}
	if err := json.Unmarshal(reqObj.Raw, &crb); err != nil {
		return nil, fmt.Errorf("decode cluster role binding: %w", err)
	}

	name := crb.Metadata.Name
	if objectRef != nil && objectRef.Name != "" {
		name = objectRef.Name
	}

	return &ClusterRoleBindingCharacteristics{
		BindingName: name,
		RoleRef:     crb.RoleRef,
		Subjects:    crb.Subjects,
	}, nil
}

// scenario identifies which approved detection scenario, if any, an
// operation belongs to -- resolveScenario's own local classification,
// independent of internal/validation (see package doc).
type scenario int

const (
	scenarioNone scenario = iota
	scenario1             // pods/exec
	scenario2             // pod creation
	scenario3             // clusterrolebinding creation
)

// resolveScenario classifies an operation using only the generically-
// required objectRef and verb fields (FR-007(d),(e)) -- never
// requestObject content, which is absent for scenario 1 and, depending on
// audit stage, may be structurally identical in shape to a non-matching
// request for scenarios 2 and 3 until these exact fields are checked.
func resolveScenario(ref *auditv1.ObjectReference, verb string) scenario {
	if ref == nil {
		return scenarioNone
	}
	switch {
	case ref.Resource == "pods" && ref.Subresource == "exec":
		return scenario1
	case ref.Resource == "pods" && ref.Subresource == "" && verb == "create":
		return scenario2
	case ref.Resource == "clusterrolebindings" &&
		ref.APIGroup == "rbac.authorization.k8s.io" && verb == "create":
		return scenario3
	default:
		return scenarioNone
	}
}

// cloudTrailRecord is the documented supported CloudTrail management-event
// record form (FR-002), decoded independently of internal/validation's own
// copy of this shape: ARCH-01's module-ownership boundary does not permit
// normalization to depend on validation (see package doc), so this is its
// own small, locally used decode target, not an imported one -- the same
// deliberate duplication resolveScenario's doc comment already explains
// for the Kubernetes scenario resolver.
type cloudTrailRecord struct {
	EventTime         time.Time              `json:"eventTime"`
	EventSource       string                 `json:"eventSource"`
	EventName         string                 `json:"eventName"`
	UserIdentity      cloudTrailUserIdentity `json:"userIdentity"`
	RequestParameters json.RawMessage        `json:"requestParameters"`
	ResponseElements  json.RawMessage        `json:"responseElements"`
	ErrorCode         string                 `json:"errorCode"`
}

type cloudTrailUserIdentity struct {
	Type     string `json:"type"`
	ARN      string `json:"arn"`
	UserName string `json:"userName"`
}

// ctScenario identifies which approved CloudTrail-family detection
// scenario, if any, a record's recorded API action belongs to -- the
// CloudTrail analog of scenario/resolveScenario above, named distinctly (ct
// prefix) to avoid colliding with the Kubernetes-family scenario type in
// this same package.
type ctScenario int

const (
	ctScenarioNone ctScenario = iota
	ctScenario4               // MFA device deactivated
	ctScenario5               // new IAM access key created
	ctScenario6               // IAM policy attachment
)

func resolveCloudTrailScenario(eventName string) ctScenario {
	switch eventName {
	case "DeactivateMFADevice", "DeleteVirtualMFADevice":
		return ctScenario4
	case "CreateAccessKey":
		return ctScenario5
	case "AttachUserPolicy", "AttachRolePolicy":
		return ctScenario6
	default:
		return ctScenarioNone
	}
}

// normalizeCloudTrail is Normalize's AWS CloudTrail-family path.
func normalizeCloudTrail(raw []byte) (Event, error) {
	var rec cloudTrailRecord
	if err := json.Unmarshal(raw, &rec); err != nil {
		return Event{}, fmt.Errorf("normalization: unmarshal cloudtrail record: %w", err)
	}

	out := Event{
		Subject:     Subject{Username: cloudTrailRequestingIdentity(rec)},
		Operation:   Operation{Verb: rec.EventName},
		Target:      cloudTrailTargetOf(rec),
		Outcome:     cloudTrailOutcomeOf(rec),
		RequestTime: rec.EventTime,
	}

	if resolveCloudTrailScenario(rec.EventName) != ctScenarioNone {
		out.CloudTrailIAMAction = cloudTrailIAMActionOf(rec)
	}

	return out, nil
}

// cloudTrailRequestingIdentity derives the requesting subject (FR-017)
// from userIdentity: the ARN when present, else the bare username, else
// the literal "root" for AWS account root-user activity (scenario 7,
// FR-040), which carries neither an arn nor a userName the way an IAM
// principal does.
func cloudTrailRequestingIdentity(rec cloudTrailRecord) string {
	switch {
	case rec.UserIdentity.ARN != "":
		return rec.UserIdentity.ARN
	case rec.UserIdentity.UserName != "":
		return rec.UserIdentity.UserName
	case rec.UserIdentity.Type == "Root":
		return "root"
	default:
		return ""
	}
}

// cloudTrailTargetOf conveys the operation's target resource (FR-017):
// Resource is the AWS service that recorded the event (eventSource); Name
// is a best-effort affected-principal name drawn from the well-established,
// stable IAM request-parameter names shared across the platform's
// CloudTrail scenarios. This is deliberately not an exhaustive per-API
// field mapping -- exact per-scenario field extraction beyond this belongs
// to the future detection-scenario-authoring pass (no AWS detection
// scenarios exist yet).
func cloudTrailTargetOf(rec cloudTrailRecord) Target {
	params := decodeCloudTrailParams(rec.RequestParameters)
	name := stringField(params, "userName")
	if name == "" {
		name = stringField(params, "roleName")
	}
	return Target{Resource: rec.EventSource, Name: name}
}

// cloudTrailOutcomeOf derives CloudTrail's Outcome: ErrorCode conveyed
// verbatim for evidence/explainability (FR-029, FR-032), and Successful
// derived as "no recorded errorCode" -- CloudTrail's own native outcome
// signal, always structurally determinable once the record decodes.
func cloudTrailOutcomeOf(rec cloudTrailRecord) Outcome {
	return Outcome{ErrorCode: rec.ErrorCode, Successful: rec.ErrorCode == ""}
}

// cloudTrailIAMActionOf conveys FR-018's scenario 4-6 content (see
// CloudTrailIAMAction's doc comment) using the same well-established IAM
// request/response parameter names cloudTrailTargetOf relies on.
// CreateAccessKey's affected user falls back to the response's own
// accessKey.userName when the request omitted an explicit userName (AWS
// permits self-service key creation with no userName parameter, but always
// records the actual affected username in the response on success).
func cloudTrailIAMActionOf(rec cloudTrailRecord) *CloudTrailIAMAction {
	reqParams := decodeCloudTrailParams(rec.RequestParameters)
	respElements := decodeCloudTrailParams(rec.ResponseElements)

	action := &CloudTrailIAMAction{}
	switch resolveCloudTrailScenario(rec.EventName) {
	case ctScenario4:
		action.AffectedUser = stringField(reqParams, "userName")
		action.AffectedDevice = stringField(reqParams, "serialNumber")
	case ctScenario5:
		action.AffectedUser = stringField(reqParams, "userName")
		if action.AffectedUser == "" {
			if accessKey, ok := respElements["accessKey"].(map[string]any); ok {
				action.AffectedUser, _ = accessKey["userName"].(string)
			}
		}
	case ctScenario6:
		action.AffectedUser = stringField(reqParams, "userName")
		if action.AffectedUser == "" {
			action.AffectedUser = stringField(reqParams, "roleName")
		}
		action.PolicyName = stringField(reqParams, "policyArn")
	}
	return action
}

// decodeCloudTrailParams permissively decodes a requestParameters/
// responseElements payload into a generic map -- CloudTrail's per-API
// parameter shape varies by eventName, so no fixed struct fits every
// action. A nil or unparseable payload yields a nil map rather than an
// error: raw is assumed already classified valid, and a missing optional
// field here is not an engineering fault the way a missing required
// FR-007 item would be.
func decodeCloudTrailParams(raw json.RawMessage) map[string]any {
	if len(raw) == 0 {
		return nil
	}
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	return m
}

func stringField(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	s, _ := m[key].(string)
	return s
}

// ErrArtifactConflict is returned by Advance when a normalized_events row
// already exists for the submission with different content or a
// different representation revision than what this call just computed --
// a genuine disagreement between the newly derived artifact and the one
// already on record. It is never silently resolved by overwriting the
// previously persisted artifact (NFR-006, NFR-017): the submission is
// left at validated and the caller must investigate.
var ErrArtifactConflict = errors.New("normalization: existing normalized event has different content")

// Advance normalizes sub's raw source event, persists it as this
// submission's normalized_events artifact, and advances the submission
// from validated to normalized -- all in one transaction (ADR-0002).
// internal/normalization owns every read and write against
// normalized_events; the worker only selects and dispatches work, and
// never writes this table itself.
//
// Retry safety: a repeated call for the same submission_id is accepted as
// a safe no-op retry only when the newly computed content and
// representation revision exactly match what is already persisted (JSONB
// semantic equality via Postgres's own jsonb equality operator, applied
// to content; ordinary equality for representation_revision) -- detected
// by one race-safe INSERT ... ON CONFLICT ... DO UPDATE ... WHERE
// statement, the same idempotent-insert pattern
// internal/submission.Admit already uses. A mismatch returns
// ErrArtifactConflict and leaves the submission at validated; the
// existing artifact is never overwritten. Once persisted, the guarded
// submission.Advance call behaves exactly as it does for every other
// stage: a submission no longer at validated (e.g. a true retry after a
// prior fully-committed call) yields ErrStatusConflict, not a silent
// reapplication.
func Advance(ctx context.Context, db *sql.DB, sub *submission.Submission) error {
	event, err := Normalize(sub.RawEvent, sub.SourceFamily)
	if err != nil {
		return fmt.Errorf("normalization: advance submission %d: %w", sub.ID, err)
	}
	content, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("normalization: advance submission %d: marshal content: %w", sub.ID, err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("normalization: begin advance tx for submission %d: %w", sub.ID, err)
	}
	defer tx.Rollback()

	var normalizedID int64
	err = tx.QueryRowContext(ctx,
		`INSERT INTO normalized_events (submission_id, content, representation_revision)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (submission_id) DO UPDATE
		 SET content = EXCLUDED.content
		 WHERE normalized_events.content = EXCLUDED.content
		   AND normalized_events.representation_revision = EXCLUDED.representation_revision
		 RETURNING id`,
		sub.ID, []byte(content), RepresentationRevision,
	).Scan(&normalizedID)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: submission %d", ErrArtifactConflict, sub.ID)
	}
	if err != nil {
		return fmt.Errorf("normalization: insert normalized event for submission %d: %w", sub.ID, err)
	}

	if err := submission.Advance(ctx, tx, sub.ID, submission.StatusValidated, submission.StatusNormalized); err != nil {
		return fmt.Errorf("normalization: advance submission %d to normalized: %w", sub.ID, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("normalization: commit advance for submission %d: %w", sub.ID, err)
	}
	return nil
}

// ErrNotFound is returned by Get when no normalized event has been
// persisted for the given submission id.
var ErrNotFound = errors.New("normalization: normalized event not found")

// Record is one persisted normalized_events row: its own id (the value a
// downstream stage's foreign key, e.g. detection_results.normalized_event_id,
// must reference), the submission it was produced from, and its
// normalized content.
type Record struct {
	ID           int64
	SubmissionID int64
	Event        Event
}

// DB is the minimal subset of *sql.DB / *sql.Tx Get needs, so a caller
// can pass either a plain connection or an already-open transaction --
// e.g. internal/evidence's verification transaction (ADR-0002).
type DB interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Get retrieves the normalized event already persisted for submissionID.
// It is the sanctioned way a downstream module reads normalized_events:
// internal/normalization owns every read and write against this table
// (see package doc), so a stage that needs the normalized representation
// -- detection evaluation, evidence assembly -- calls Get rather than
// querying the table directly. Returns ErrNotFound if no normalized event
// has been persisted for this submission yet.
func Get(ctx context.Context, db DB, submissionID int64) (*Record, error) {
	var (
		id      int64
		content []byte
	)
	err := db.QueryRowContext(ctx,
		`SELECT id, content FROM normalized_events WHERE submission_id = $1`,
		submissionID,
	).Scan(&id, &content)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("normalization: get normalized event for submission %d: %w", submissionID, err)
	}

	var event Event
	if err := json.Unmarshal(content, &event); err != nil {
		return nil, fmt.Errorf("normalization: get normalized event for submission %d: unmarshal content: %w", submissionID, err)
	}

	return &Record{ID: id, SubmissionID: submissionID, Event: event}, nil
}
