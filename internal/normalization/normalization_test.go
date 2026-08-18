package normalization

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"

	"cnsdp/internal/submission"
)

// readValidationFixture reuses internal/validation's existing testdata
// fixtures rather than duplicating scenario source-event content: both
// packages need the exact same real Spike-1-captured events, and a copy
// would risk silently drifting from what validation actually classifies
// valid.
func readValidationFixture(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "validation", "testdata", name))
	if err != nil {
		t.Fatalf("read validation fixture %s: %v", name, err)
	}
	return raw
}

func mustParseTime(t *testing.T, s string) time.Time {
	t.Helper()
	tm, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t.Fatalf("parse time %s: %v", s, err)
	}
	return tm
}

// TestNormalize_Scenario1Exec proves FR-017 core content and FR-018
// scenario 1 content for a real captured interactive exec request
// (validation/testdata/boundary-4-valid.json, also AC-003's fixture 4).
func TestNormalize_Scenario1Exec(t *testing.T) {
	raw := readValidationFixture(t, "boundary-4-valid.json")

	got, err := Normalize(raw, submission.FamilyKubernetes)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}

	if got.Subject.Username != "kubernetes-admin" {
		t.Errorf("Subject.Username = %q, want %q", got.Subject.Username, "kubernetes-admin")
	}
	if got.Operation.Verb != "get" {
		t.Errorf("Operation.Verb = %q, want %q", got.Operation.Verb, "get")
	}
	wantURI := "/api/v1/namespaces/default/pods/high-risk-pod/exec?command=C%3A%2FProgram+Files%2FGit%2Fusr%2Fbin%2Fsh.exe&command=-c&command=echo+interactive-exec-marker&container=high-risk-container&stdin=true&stdout=true&tty=true"
	if got.Operation.RequestURI != wantURI {
		t.Errorf("Operation.RequestURI = %q, want %q (must be preserved verbatim)", got.Operation.RequestURI, wantURI)
	}
	if got.Target.Resource != "pods" {
		t.Errorf("Target.Resource = %q, want %q (the REST resource name, not a Kubernetes Kind)", got.Target.Resource, "pods")
	}
	if got.Target.Name != "high-risk-pod" || got.Target.Namespace != "default" || got.Target.Subresource != "exec" {
		t.Errorf("Target = %+v, want Name=high-risk-pod Namespace=default Subresource=exec", got.Target)
	}
	if got.Outcome.Code != 101 {
		t.Errorf("Outcome.Code = %d, want 101", got.Outcome.Code)
	}
	wantTime := mustParseTime(t, "2026-07-21T15:25:11.901891Z")
	if !got.RequestTime.Equal(wantTime) {
		t.Errorf("RequestTime = %v, want %v", got.RequestTime, wantTime)
	}

	if got.Exec == nil {
		t.Fatal("Exec = nil, want scenario 1 characteristics")
	}
	if !got.Exec.Stdin || !got.Exec.TTY {
		t.Errorf("Exec = %+v, want Stdin=true TTY=true", got.Exec)
	}
	if got.PodCreation != nil {
		t.Errorf("PodCreation = %+v, want nil for a scenario 1 event", got.PodCreation)
	}
	if got.ClusterRoleBinding != nil {
		t.Errorf("ClusterRoleBinding = %+v, want nil for a scenario 1 event", got.ClusterRoleBinding)
	}
}

// TestNormalize_Scenario2PodCreation proves FR-018 scenario 2 content
// against a real captured Pod-creation request with all five documented
// high-risk characteristics present (validation/testdata/scenario2-valid.json).
func TestNormalize_Scenario2PodCreation(t *testing.T) {
	raw := readValidationFixture(t, "scenario2-valid.json")

	got, err := Normalize(raw, submission.FamilyKubernetes)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}

	if got.Target.Resource != "pods" || got.Target.Subresource != "" {
		t.Errorf("Target = %+v, want Resource=pods Subresource=\"\"", got.Target)
	}
	if got.Outcome.Code != 201 {
		t.Errorf("Outcome.Code = %d, want 201", got.Outcome.Code)
	}
	if got.Exec != nil {
		t.Errorf("Exec = %+v, want nil for a scenario 2 event", got.Exec)
	}
	if got.ClusterRoleBinding != nil {
		t.Errorf("ClusterRoleBinding = %+v, want nil for a scenario 2 event", got.ClusterRoleBinding)
	}

	if got.PodCreation == nil {
		t.Fatal("PodCreation = nil, want scenario 2 characteristics")
	}
	want := PodCreationCharacteristics{
		Privileged:     true,
		HostNetwork:    true,
		HostPID:        true,
		HostIPC:        true,
		HostPathVolume: true,
	}
	if *got.PodCreation != want {
		t.Errorf("PodCreation = %+v, want %+v", *got.PodCreation, want)
	}
}

// TestNormalize_PodCreation_NoHighRiskCharacteristics proves normalization
// reports an explicit false for every characteristic that is absent,
// rather than omitting the field -- a later detection checkpoint must be
// able to distinguish "known absent" from "not evaluated".
func TestNormalize_PodCreation_NoHighRiskCharacteristics(t *testing.T) {
	const raw = `{"kind":"Event","apiVersion":"audit.k8s.io/v1","auditID":"22222222-2222-2222-2222-222222222222","stage":"ResponseComplete","requestURI":"/api/v1/namespaces/default/pods","verb":"create","user":{"username":"bob"},"objectRef":{"resource":"pods","namespace":"default","name":"safe-pod"},"responseStatus":{"code":201},"requestObject":{"kind":"Pod","apiVersion":"v1","metadata":{"name":"safe-pod"},"spec":{"containers":[{"name":"c","image":"busybox"}]}},"requestReceivedTimestamp":"2026-07-21T10:05:00.000000Z"}`

	got, err := Normalize([]byte(raw), submission.FamilyKubernetes)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if got.PodCreation == nil {
		t.Fatal("PodCreation = nil, want a populated (all-false) characteristics struct")
	}
	want := PodCreationCharacteristics{}
	if *got.PodCreation != want {
		t.Errorf("PodCreation = %+v, want all-false %+v", *got.PodCreation, want)
	}
}

// TestNormalize_Scenario3ClusterRoleBinding proves FR-018 scenario 3
// content against a real captured cluster-admin ClusterRoleBinding
// creation (validation/testdata/scenario3-valid.json).
func TestNormalize_Scenario3ClusterRoleBinding(t *testing.T) {
	raw := readValidationFixture(t, "scenario3-valid.json")

	got, err := Normalize(raw, submission.FamilyKubernetes)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}

	if got.Target.Resource != "clusterrolebindings" {
		t.Errorf("Target.Resource = %q, want %q", got.Target.Resource, "clusterrolebindings")
	}
	if got.Exec != nil {
		t.Errorf("Exec = %+v, want nil for a scenario 3 event", got.Exec)
	}
	if got.PodCreation != nil {
		t.Errorf("PodCreation = %+v, want nil for a scenario 3 event", got.PodCreation)
	}

	if got.ClusterRoleBinding == nil {
		t.Fatal("ClusterRoleBinding = nil, want scenario 3 characteristics")
	}
	crb := got.ClusterRoleBinding
	if crb.BindingName != "spike-crb-jsonpatch" {
		t.Errorf("BindingName = %q, want %q", crb.BindingName, "spike-crb-jsonpatch")
	}
	if crb.RoleRef.APIGroup != "rbac.authorization.k8s.io" || crb.RoleRef.Kind != "ClusterRole" || crb.RoleRef.Name != "cluster-admin" {
		t.Errorf("RoleRef = %+v, want cluster-admin ClusterRole", crb.RoleRef)
	}
	if len(crb.Subjects) != 1 || crb.Subjects[0].Kind != "User" || crb.Subjects[0].Name != "alice@example.com" {
		t.Errorf("Subjects = %+v, want one User alice@example.com", crb.Subjects)
	}
}

// TestNormalize_NonScenarioEvent_NoScenarioContent proves that a valid
// event whose operation is not scenario-relevant normalizes its core
// content (FR-017) without populating any of the three scenario-specific
// sections (FR-018 does not apply).
func TestNormalize_NonScenarioEvent_NoScenarioContent(t *testing.T) {
	const raw = `{"kind":"Event","apiVersion":"audit.k8s.io/v1","auditID":"11111111-1111-1111-1111-111111111111","stage":"ResponseComplete","requestURI":"/api/v1/namespaces/default/configmaps/my-config","verb":"get","user":{"username":"alice"},"objectRef":{"resource":"configmaps","namespace":"default","name":"my-config"},"responseStatus":{"code":200},"requestReceivedTimestamp":"2026-07-21T10:00:00.000000Z"}`

	got, err := Normalize([]byte(raw), submission.FamilyKubernetes)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if got.Target.Resource != "configmaps" || got.Target.Name != "my-config" {
		t.Errorf("Target = %+v, want Resource=configmaps Name=my-config", got.Target)
	}
	if got.Exec != nil || got.PodCreation != nil || got.ClusterRoleBinding != nil {
		t.Errorf("scenario content = Exec:%v PodCreation:%v ClusterRoleBinding:%v, want all nil",
			got.Exec, got.PodCreation, got.ClusterRoleBinding)
	}
}

// TestNormalize_ExecCharacteristics_IndividualFlags proves each of
// scenario 1's two documented characteristics (FR-024) is derived
// independently from the request URI's query parameters, never coupled to
// the other.
func TestNormalize_ExecCharacteristics_IndividualFlags(t *testing.T) {
	const template = `{"kind":"Event","apiVersion":"audit.k8s.io/v1","auditID":"33333333-3333-3333-3333-333333333333","stage":"ResponseComplete","requestURI":"/api/v1/namespaces/default/pods/p/exec?%s","verb":"get","user":{"username":"carol"},"objectRef":{"resource":"pods","namespace":"default","name":"p","subresource":"exec"},"responseStatus":{"code":101},"requestReceivedTimestamp":"2026-07-21T10:10:00.000000Z"}`

	tests := []struct {
		name      string
		query     string
		wantStdin bool
		wantTTY   bool
	}{
		{"stdin only", "stdin=true", true, false},
		{"tty only", "tty=true", false, true},
		{"both", "stdin=true&tty=true", true, true},
		{"neither", "container=p", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := []byte(fmt.Sprintf(template, tt.query))
			got, err := Normalize(raw, submission.FamilyKubernetes)
			if err != nil {
				t.Fatalf("Normalize: %v", err)
			}
			if got.Exec == nil {
				t.Fatal("Exec = nil, want populated characteristics")
			}
			if got.Exec.Stdin != tt.wantStdin || got.Exec.TTY != tt.wantTTY {
				t.Errorf("Exec = %+v, want Stdin=%v TTY=%v", got.Exec, tt.wantStdin, tt.wantTTY)
			}
		})
	}
}

// TestNormalize_MissingRequestObject_ReturnsErrorNotPanic guards
// Normalize's engineering-fault contract (see package doc): a
// scenario-relevant operation without a requestObject cannot be
// classified valid by internal/validation (FR-009), so Normalize should
// never actually see one in production, but it must still fail
// explicitly rather than panic or silently produce empty content.
func TestNormalize_MissingRequestObject_ReturnsErrorNotPanic(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{
			"scenario 2 without requestObject",
			`{"kind":"Event","apiVersion":"audit.k8s.io/v1","auditID":"44444444-4444-4444-4444-444444444444","stage":"ResponseComplete","requestURI":"/api/v1/namespaces/default/pods","verb":"create","user":{"username":"dave"},"objectRef":{"resource":"pods","namespace":"default","name":"p"},"responseStatus":{"code":201},"requestReceivedTimestamp":"2026-07-21T10:15:00.000000Z"}`,
		},
		{
			"scenario 3 without requestObject",
			`{"kind":"Event","apiVersion":"audit.k8s.io/v1","auditID":"55555555-5555-5555-5555-555555555555","stage":"ResponseComplete","requestURI":"/apis/rbac.authorization.k8s.io/v1/clusterrolebindings","verb":"create","user":{"username":"erin"},"objectRef":{"resource":"clusterrolebindings","name":"b","apiGroup":"rbac.authorization.k8s.io"},"responseStatus":{"code":201},"requestReceivedTimestamp":"2026-07-21T10:20:00.000000Z"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Normalize([]byte(tt.raw), submission.FamilyKubernetes); err == nil {
				t.Fatal("Normalize: expected an error for a scenario-relevant event missing requestObject")
			}
		})
	}
}

// TestNormalize_JSONRoundTrip guards against a JSON-tag mistake silently
// dropping a field once content is stored as JSONB and later re-read.
func TestNormalize_JSONRoundTrip(t *testing.T) {
	raw := readValidationFixture(t, "scenario3-valid.json")
	want, err := Normalize(raw, submission.FamilyKubernetes)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}

	encoded, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got Event
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !want.RequestTime.Equal(got.RequestTime) {
		t.Errorf("RequestTime round-trip = %v, want %v", got.RequestTime, want.RequestTime)
	}
	// time.Time's internal representation (monotonic reading, Location
	// pointer) need not survive a JSON round-trip identically even when
	// the represented instant is unchanged (already checked above via
	// Equal) -- normalize both to the same value before the structural
	// comparison below.
	got.RequestTime = want.RequestTime
	if !reflect.DeepEqual(got, want) {
		t.Errorf("round-tripped Event = %+v, want %+v", got, want)
	}
}

// TestResolveScenario proves resolveScenario's own independent
// classification of the three approved scenarios and confirms
// near-miss operations are not misclassified.
func TestResolveScenario(t *testing.T) {
	tests := []struct {
		name string
		ref  *auditv1.ObjectReference
		verb string
		want scenario
	}{
		{"nil objectRef", nil, "get", scenarioNone},
		{"scenario 1: pods/exec", &auditv1.ObjectReference{Resource: "pods", Subresource: "exec"}, "get", scenario1},
		{"scenario 2: pods create", &auditv1.ObjectReference{Resource: "pods"}, "create", scenario2},
		{"not scenario 2: pods get (no subresource)", &auditv1.ObjectReference{Resource: "pods"}, "get", scenarioNone},
		{"not scenario 1: pods/logs (wrong subresource)", &auditv1.ObjectReference{Resource: "pods", Subresource: "logs"}, "get", scenarioNone},
		{
			"scenario 3: clusterrolebindings create with correct apiGroup",
			&auditv1.ObjectReference{Resource: "clusterrolebindings", APIGroup: "rbac.authorization.k8s.io"}, "create", scenario3,
		},
		{
			"not scenario 3: wrong apiGroup",
			&auditv1.ObjectReference{Resource: "clusterrolebindings", APIGroup: "example.com"}, "create", scenarioNone,
		},
		{
			"not scenario 3: update verb",
			&auditv1.ObjectReference{Resource: "clusterrolebindings", APIGroup: "rbac.authorization.k8s.io"}, "update", scenarioNone,
		},
		{"no scenario: unrelated resource", &auditv1.ObjectReference{Resource: "configmaps"}, "get", scenarioNone},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveScenario(tt.ref, tt.verb); got != tt.want {
				t.Errorf("resolveScenario(%+v, %q) = %v, want %v", tt.ref, tt.verb, got, tt.want)
			}
		})
	}
}

// TestOutcomeOf_SuccessfulDerivation proves the 200-299 "success" rule --
// relocated here from internal/detection's generic evaluator so that
// evaluator never needs family-specific knowledge of how success is
// encoded (see internal/detection's TestOutcomeSatisfied). This is the
// Kubernetes-family half of that source-neutral Successful signal; see
// TestNormalizeCloudTrail_Outcome below for the CloudTrail-family half.
func TestOutcomeOf_SuccessfulDerivation(t *testing.T) {
	tests := []struct {
		name string
		code int32
		want bool
	}{
		{"lower bound 200", 200, true},
		{"upper bound 299", 299, true},
		{"just below range", 199, false},
		{"just above range", 300, false},
		{"zero code", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := outcomeOf(&metav1.Status{Code: tt.code})
			if got.Successful != tt.want {
				t.Errorf("outcomeOf(code=%d).Successful = %v, want %v", tt.code, got.Successful, tt.want)
			}
			if got.Code != tt.code {
				t.Errorf("outcomeOf(code=%d).Code = %d, want %d (unchanged descriptive field)", tt.code, got.Code, tt.code)
			}
		})
	}
	t.Run("nil responseStatus", func(t *testing.T) {
		got := outcomeOf(nil)
		if got.Successful {
			t.Error("outcomeOf(nil).Successful = true, want false (never actually consulted for such an event)")
		}
	})
}

// --- AWS CloudTrail family (ADR-0006) ---

// cloudTrailScenario5JSON is a real-shaped CreateAccessKey management
// event with an explicit requestParameters.userName.
const cloudTrailScenario5JSON = `{"eventVersion":"1.08","eventTime":"2026-08-18T12:34:56Z","eventSource":"iam.amazonaws.com","eventName":"CreateAccessKey","eventCategory":"Management","awsRegion":"us-east-1","userIdentity":{"type":"IAMUser","arn":"arn:aws:iam::123456789012:user/alice","userName":"alice"},"requestParameters":{"userName":"bob"},"responseElements":{"accessKey":{"userName":"bob","accessKeyId":"AKIAEXAMPLE","status":"Active"}},"eventID":"5f6a2b1c-0000-0000-0000-000000000001","eventType":"AwsApiCall"}`

func TestNormalizeCloudTrail_Scenario5CreateAccessKey(t *testing.T) {
	got, err := normalizeCloudTrail([]byte(cloudTrailScenario5JSON))
	if err != nil {
		t.Fatalf("normalizeCloudTrail: %v", err)
	}
	if got.Subject.Username != "arn:aws:iam::123456789012:user/alice" {
		t.Errorf("Subject.Username = %q, want the requesting identity's ARN", got.Subject.Username)
	}
	if got.Operation.Verb != "CreateAccessKey" {
		t.Errorf("Operation.Verb = %q, want %q", got.Operation.Verb, "CreateAccessKey")
	}
	if got.Target.Resource != "iam.amazonaws.com" {
		t.Errorf("Target.Resource = %q, want the AWS source service", got.Target.Resource)
	}
	if !got.Outcome.Successful || got.Outcome.ErrorCode != "" {
		t.Errorf("Outcome = %+v, want Successful=true ErrorCode=\"\" (no errorCode recorded)", got.Outcome)
	}
	wantTime := mustParseTime(t, "2026-08-18T12:34:56Z")
	if !got.RequestTime.Equal(wantTime) {
		t.Errorf("RequestTime = %v, want %v", got.RequestTime, wantTime)
	}
	if got.Exec != nil || got.PodCreation != nil || got.ClusterRoleBinding != nil {
		t.Errorf("Kubernetes-only scenario fields populated for a CloudTrail event: %+v", got)
	}
	if got.CloudTrailIAMAction == nil {
		t.Fatal("CloudTrailIAMAction = nil, want scenario 5 content (FR-018)")
	}
	if got.CloudTrailIAMAction.AffectedUser != "bob" {
		t.Errorf("AffectedUser = %q, want %q (from requestParameters.userName)", got.CloudTrailIAMAction.AffectedUser, "bob")
	}
}

// TestNormalizeCloudTrail_Scenario5_FallsBackToResponseUserName proves the
// self-service CreateAccessKey case (no explicit requestParameters.userName
// -- AWS creates the key for the caller) still resolves the affected user,
// from responseElements.accessKey.userName.
func TestNormalizeCloudTrail_Scenario5_FallsBackToResponseUserName(t *testing.T) {
	const raw = `{"eventTime":"2026-08-18T12:00:00Z","eventSource":"iam.amazonaws.com","eventName":"CreateAccessKey","eventCategory":"Management","userIdentity":{"type":"IAMUser","userName":"carol"},"requestParameters":{},"responseElements":{"accessKey":{"userName":"carol","accessKeyId":"AKIASELF"}},"eventID":"evt-self"}`
	got, err := normalizeCloudTrail([]byte(raw))
	if err != nil {
		t.Fatalf("normalizeCloudTrail: %v", err)
	}
	if got.CloudTrailIAMAction == nil || got.CloudTrailIAMAction.AffectedUser != "carol" {
		t.Errorf("CloudTrailIAMAction = %+v, want AffectedUser=carol (from responseElements fallback)", got.CloudTrailIAMAction)
	}
}

// cloudTrailScenario4JSON is a real-shaped DeactivateMFADevice event.
const cloudTrailScenario4JSON = `{"eventTime":"2026-08-18T09:00:00Z","eventSource":"iam.amazonaws.com","eventName":"DeactivateMFADevice","eventCategory":"Management","userIdentity":{"type":"IAMUser","arn":"arn:aws:iam::123456789012:user/mallory"},"requestParameters":{"userName":"victim","serialNumber":"arn:aws:iam::123456789012:mfa/victim"},"responseElements":null,"eventID":"evt-mfa-1"}`

func TestNormalizeCloudTrail_Scenario4DeactivateMFA(t *testing.T) {
	got, err := normalizeCloudTrail([]byte(cloudTrailScenario4JSON))
	if err != nil {
		t.Fatalf("normalizeCloudTrail: %v", err)
	}
	if got.CloudTrailIAMAction == nil {
		t.Fatal("CloudTrailIAMAction = nil, want scenario 4 content")
	}
	if got.CloudTrailIAMAction.AffectedUser != "victim" {
		t.Errorf("AffectedUser = %q, want %q", got.CloudTrailIAMAction.AffectedUser, "victim")
	}
	if got.CloudTrailIAMAction.AffectedDevice != "arn:aws:iam::123456789012:mfa/victim" {
		t.Errorf("AffectedDevice = %q, want the MFA device ARN", got.CloudTrailIAMAction.AffectedDevice)
	}
}

// cloudTrailScenario6JSON is a real-shaped AttachUserPolicy event granting
// AdministratorAccess.
const cloudTrailScenario6JSON = `{"eventTime":"2026-08-18T09:30:00Z","eventSource":"iam.amazonaws.com","eventName":"AttachUserPolicy","eventCategory":"Management","userIdentity":{"type":"IAMUser","arn":"arn:aws:iam::123456789012:user/mallory"},"requestParameters":{"userName":"victim","policyArn":"arn:aws:iam::aws:policy/AdministratorAccess"},"eventID":"evt-policy-1"}`

func TestNormalizeCloudTrail_Scenario6AttachUserPolicy(t *testing.T) {
	got, err := normalizeCloudTrail([]byte(cloudTrailScenario6JSON))
	if err != nil {
		t.Fatalf("normalizeCloudTrail: %v", err)
	}
	if got.CloudTrailIAMAction == nil {
		t.Fatal("CloudTrailIAMAction = nil, want scenario 6 content")
	}
	if got.CloudTrailIAMAction.AffectedUser != "victim" {
		t.Errorf("AffectedUser = %q, want %q", got.CloudTrailIAMAction.AffectedUser, "victim")
	}
	if got.CloudTrailIAMAction.PolicyName != "arn:aws:iam::aws:policy/AdministratorAccess" {
		t.Errorf("PolicyName = %q, want the policy ARN", got.CloudTrailIAMAction.PolicyName)
	}
}

// TestNormalizeCloudTrail_RootIdentity proves scenario 7's (FR-040) "who"
// signal is fully carried by core Subject content alone -- FR-018 does not
// enumerate any scenario-7-specific field, so no CloudTrailIAMAction is
// expected here.
func TestNormalizeCloudTrail_RootIdentity(t *testing.T) {
	const raw = `{"eventTime":"2026-08-18T03:00:00Z","eventSource":"signin.amazonaws.com","eventName":"ConsoleLogin","eventCategory":"Management","userIdentity":{"type":"Root"},"requestParameters":{},"eventID":"evt-root-1"}`
	got, err := normalizeCloudTrail([]byte(raw))
	if err != nil {
		t.Fatalf("normalizeCloudTrail: %v", err)
	}
	if got.Subject.Username != "root" {
		t.Errorf("Subject.Username = %q, want %q", got.Subject.Username, "root")
	}
	if got.CloudTrailIAMAction != nil {
		t.Errorf("CloudTrailIAMAction = %+v, want nil for a non-scenario-4/5/6 event", got.CloudTrailIAMAction)
	}
}

// TestNormalizeCloudTrail_NonScenarioEvent_NoScenarioContent mirrors
// TestNormalize_NonScenarioEvent_NoScenarioContent for the CloudTrail
// family: a valid record whose eventName is not scenario-relevant
// normalizes its core content without populating CloudTrailIAMAction.
func TestNormalizeCloudTrail_NonScenarioEvent_NoScenarioContent(t *testing.T) {
	const raw = `{"eventTime":"2026-08-18T09:00:00Z","eventSource":"iam.amazonaws.com","eventName":"GetUser","eventCategory":"Management","userIdentity":{"type":"IAMUser","userName":"dave"},"requestParameters":{"userName":"dave"},"eventID":"evt-getuser-1"}`
	got, err := normalizeCloudTrail([]byte(raw))
	if err != nil {
		t.Fatalf("normalizeCloudTrail: %v", err)
	}
	if got.CloudTrailIAMAction != nil {
		t.Errorf("CloudTrailIAMAction = %+v, want nil for a non-scenario-relevant event", got.CloudTrailIAMAction)
	}
	if got.Target.Name != "dave" {
		t.Errorf("Target.Name = %q, want %q", got.Target.Name, "dave")
	}
}

// TestNormalizeCloudTrail_Outcome proves CloudTrail's own native outcome
// signal (errorCode presence/absence), independent of Kubernetes' Code
// field -- the CloudTrail-family half of the source-neutral Successful
// signal (see TestOutcomeOf_SuccessfulDerivation above).
func TestNormalizeCloudTrail_Outcome(t *testing.T) {
	t.Run("no errorCode: successful", func(t *testing.T) {
		got := cloudTrailOutcomeOf(cloudTrailRecord{})
		if !got.Successful || got.ErrorCode != "" {
			t.Errorf("Outcome = %+v, want Successful=true ErrorCode=\"\"", got)
		}
	})
	t.Run("errorCode present: not successful", func(t *testing.T) {
		got := cloudTrailOutcomeOf(cloudTrailRecord{ErrorCode: "AccessDenied"})
		if got.Successful || got.ErrorCode != "AccessDenied" {
			t.Errorf("Outcome = %+v, want Successful=false ErrorCode=AccessDenied", got)
		}
		if got.Code != 0 {
			t.Errorf("Code = %d, want 0 (unused for CloudTrail)", got.Code)
		}
	})
}

// TestNormalize_DispatchesByFamily proves the exported Normalize entry
// point routes to the correct family-specific path -- the same raw bytes
// classified valid for a given family must never be normalized as if it
// were the other family.
func TestNormalize_DispatchesByFamily(t *testing.T) {
	k8sEvent, err := Normalize(readValidationFixture(t, "boundary-4-valid.json"), submission.FamilyKubernetes)
	if err != nil {
		t.Fatalf("Normalize (kubernetes): %v", err)
	}
	if k8sEvent.Operation.Verb != "get" {
		t.Errorf("kubernetes Operation.Verb = %q, want %q", k8sEvent.Operation.Verb, "get")
	}

	ctEvent, err := Normalize([]byte(cloudTrailScenario5JSON), submission.FamilyCloudTrail)
	if err != nil {
		t.Fatalf("Normalize (cloudtrail): %v", err)
	}
	if ctEvent.Operation.Verb != "CreateAccessKey" {
		t.Errorf("cloudtrail Operation.Verb = %q, want %q", ctEvent.Operation.Verb, "CreateAccessKey")
	}
}
