package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ScenarioID names the three approved detection scenarios, matching the
// scenario field in definitions/scenario-{1,2,3}.yaml.
type ScenarioID string

const (
	Scenario1 ScenarioID = "scenario-1" // pods/exec, interactive characteristics
	Scenario2 ScenarioID = "scenario-2" // high-risk Pod creation
	Scenario3 ScenarioID = "scenario-3" // cluster-admin ClusterRoleBinding grant
)

// ScenarioFixtures holds one template Event per approved scenario, loaded
// once from this repository's own already-committed, real fixtures --
// never synthetic data invented for this harness. Matching, confirmed
// against definitions/scenario-{1,2,3}.yaml, is keyed on objectRef,
// verb, requestURI, and (for scenario 2/3) requestObject content -- never
// auditID -- so cloning with only auditID rewritten preserves the match.
type ScenarioFixtures struct {
	templates map[ScenarioID]map[string]json.RawMessage
}

// LoadScenarioFixtures reads the three real fixtures this repository
// already uses for its own tests and dev-seed-alerts.ps1:
//   - internal/intake/testdata/scenario-1-eventlist.json (an EventList
//     envelope; item [0] is used)
//   - internal/validation/testdata/scenario2-valid.json (a bare Event)
//   - internal/validation/testdata/scenario3-valid.json (a bare Event)
func LoadScenarioFixtures(repoRoot string) (*ScenarioFixtures, error) {
	f := &ScenarioFixtures{templates: make(map[ScenarioID]map[string]json.RawMessage, 3)}

	s1, err := loadEventListFirstItem(filepath.Join(repoRoot, "internal", "intake", "testdata", "scenario-1-eventlist.json"))
	if err != nil {
		return nil, fmt.Errorf("load scenario-1 fixture: %w", err)
	}
	f.templates[Scenario1] = s1

	s2, err := loadBareEvent(filepath.Join(repoRoot, "internal", "validation", "testdata", "scenario2-valid.json"))
	if err != nil {
		return nil, fmt.Errorf("load scenario-2 fixture: %w", err)
	}
	f.templates[Scenario2] = s2

	s3, err := loadBareEvent(filepath.Join(repoRoot, "internal", "validation", "testdata", "scenario3-valid.json"))
	if err != nil {
		return nil, fmt.Errorf("load scenario-3 fixture: %w", err)
	}
	f.templates[Scenario3] = s3

	return f, nil
}

func loadEventListFirstItem(path string) (map[string]json.RawMessage, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("unmarshal EventList envelope: %w", err)
	}
	if len(envelope.Items) == 0 {
		return nil, fmt.Errorf("%s: EventList has no items", path)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(envelope.Items[0], &m); err != nil {
		return nil, fmt.Errorf("unmarshal first item: %w", err)
	}
	return m, nil
}

func loadBareEvent(path string) (map[string]json.RawMessage, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("unmarshal event: %w", err)
	}
	return m, nil
}

// Clone produces one fresh EventList item for scenario: a deep copy of
// the loaded template with auditID replaced by a fresh RFC4122 UUID
// (real-format, matching genuine event shape, even though
// internal/validation.Classify only requires auditID to be non-empty --
// see missingCoreField). This is the harness's correlation id: unique per
// clone, so submission.SourceKey (identity-based branch: a canonical hash
// of auditID+auditStage) never coalesces two harness-generated events into
// one submission the way a repeated real auditID would.
func (f *ScenarioFixtures) Clone(scenario ScenarioID) (item json.RawMessage, auditID string, err error) {
	tmpl, ok := f.templates[scenario]
	if !ok {
		return nil, "", fmt.Errorf("no fixture loaded for scenario %q", scenario)
	}
	return cloneWithFreshAuditID(tmpl)
}

// cloneWithFreshAuditID deep-copies m (never mutates the caller's map:
// the returned map is a fresh copy sharing only the unmodified
// json.RawMessage byte slices, and json.Marshal never mutates its input)
// and overwrites auditID with a fresh UUID.
func cloneWithFreshAuditID(m map[string]json.RawMessage) (item json.RawMessage, auditID string, err error) {
	clone := make(map[string]json.RawMessage, len(m))
	for k, v := range m {
		clone[k] = v
	}
	id := uuid.NewString()
	idJSON, err := json.Marshal(id)
	if err != nil {
		return nil, "", err
	}
	clone["auditID"] = idJSON

	out, err := json.Marshal(clone)
	if err != nil {
		return nil, "", err
	}
	return out, id, nil
}

// NewFillerEvent builds one synthetic, schema-valid, non-matching K8s
// audit event -- a plain "get" against a configmap, which is not
// scenario-relevant under any of the three approved detection conditions
// (definitions/scenario-{1,2,3}.yaml: pods/exec, pods+create, or
// clusterrolebindings+create -- "configmaps"+"get" matches none), so it is
// classified valid (every FR-007 core field present, no scenario-required
// field applies) and normalized/evaluated but never alerts. Used as bulk
// throughput filler in the sustained-run workload, distinct from the
// scenario clones that must accumulate matching alerts.
func NewFillerEvent(seq int) (item json.RawMessage, auditID string) {
	id := uuid.NewString()
	now := time.Now().UTC().Format(metav1.RFC3339Micro)
	name := fmt.Sprintf("verification-filler-%d", seq)

	m := map[string]any{
		"kind":                     "Event",
		"apiVersion":               "audit.k8s.io/v1",
		"level":                    "RequestResponse",
		"auditID":                  id,
		"stage":                    "ResponseComplete",
		"requestURI":               "/api/v1/namespaces/default/configmaps/" + name,
		"verb":                     "get",
		"user":                     map[string]any{"username": "verification-harness", "groups": []string{"system:authenticated"}},
		"sourceIPs":                []string{"127.0.0.1"},
		"userAgent":                "cnsdp-verification-harness",
		"objectRef":                map[string]any{"resource": "configmaps", "namespace": "default", "name": name, "apiVersion": "v1"},
		"responseStatus":           map[string]any{"metadata": map[string]any{}, "code": 200},
		"requestReceivedTimestamp": now,
		"stageTimestamp":           now,
	}
	out, err := json.Marshal(m)
	if err != nil {
		// Every field above is a static, well-formed literal or a value
		// this function itself just generated (uuid.NewString,
		// time.Format) -- marshaling cannot fail for this input shape.
		panic(fmt.Sprintf("verification: marshal filler event: %v", err))
	}
	return out, id
}
