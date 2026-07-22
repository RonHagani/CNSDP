package normalization

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
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

	got, err := Normalize(raw)
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

	got, err := Normalize(raw)
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

	got, err := Normalize([]byte(raw))
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

	got, err := Normalize(raw)
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

	got, err := Normalize([]byte(raw))
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
			got, err := Normalize(raw)
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
			if _, err := Normalize([]byte(tt.raw)); err == nil {
				t.Fatal("Normalize: expected an error for a scenario-relevant event missing requestObject")
			}
		})
	}
}

// TestNormalize_JSONRoundTrip guards against a JSON-tag mistake silently
// dropping a field once content is stored as JSONB and later re-read.
func TestNormalize_JSONRoundTrip(t *testing.T) {
	raw := readValidationFixture(t, "scenario3-valid.json")
	want, err := Normalize(raw)
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
