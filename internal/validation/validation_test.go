package validation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return raw
}

// TestClassify_BoundaryFixtureSet exercises the shared four-fixture set
// (AC-003–AC-006) proving the documented precedence order and mutual
// exclusivity of the four outcomes. Fixture 1 is unsupported (a real Pod
// object, wrong kind/apiVersion) despite also containing malformed content
// (a non-object "spec" value) — demonstrating that precedence assessment,
// not content quality, governs the outcome (AC-006).
func TestClassify_BoundaryFixtureSet(t *testing.T) {
	tests := []struct {
		fixture string
		want    Outcome
	}{
		{"boundary-1-unsupported.json", OutcomeUnsupported},
		{"boundary-2-invalid.json", OutcomeInvalid},
		{"boundary-3-incomplete.json", OutcomeIncomplete},
		{"boundary-4-valid.json", OutcomeValid},
	}
	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			got := Classify(readFixture(t, tt.fixture))
			if got.Outcome != tt.want {
				t.Fatalf("outcome = %q, want %q (reason: %s)", got.Outcome, tt.want, got.Reason)
			}
			if tt.want == OutcomeValid {
				if got.Reason != "" {
					t.Errorf("reason = %q, want empty for a valid outcome", got.Reason)
				}
			} else if got.Reason == "" {
				t.Errorf("reason is empty, want a stated reason for outcome %q (FR-012)", got.Outcome)
			}
		})
	}
}

// TestClassify_ScenarioIncomplete proves FR-009 for scenarios 2 and 3 using
// real Spike-1-captured RequestReceived-stage events. Per ADR-0003 Q2,
// ordinary synchronous requests (Pod/ClusterRoleBinding create) produce
// only RequestReceived and ResponseComplete stages, and RequestReceived
// carries neither responseStatus nor requestObject — so these real events
// are genuinely missing both core item (f) and scenario item (g)
// simultaneously; there is no real captured stage that isolates one from
// the other. Classify reports whichever core check (a)-(f) fails first, so
// the cited reason here is item (f), not (g) — see
// TestClassify_ScenarioSpecificFieldMissingOnly below for an isolated,
// synthetic proof of item (g) alone.
func TestClassify_ScenarioIncomplete(t *testing.T) {
	tests := []struct {
		fixture string
	}{
		{"scenario2-incomplete.json"},
		{"scenario3-incomplete.json"},
	}
	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			got := Classify(readFixture(t, tt.fixture))
			if got.Outcome != OutcomeIncomplete {
				t.Fatalf("outcome = %q, want %q (reason: %s)", got.Outcome, OutcomeIncomplete, got.Reason)
			}
			if !strings.Contains(got.Reason, "FR-007") {
				t.Errorf("reason = %q, want it to cite the FR-007 item it found missing", got.Reason)
			}
		})
	}
}

// TestClassify_ScenarioSpecificFieldMissingOnly isolates FR-007(g)/FR-018
// specifically, independent of item (f): each case starts from a complete
// scenario 2/3 event (real requestObject content) with a synthetic,
// present responseStatus, then removes only the requestObject. Real
// captured data cannot isolate this case (see TestClassify_ScenarioIncomplete
// above), so this uses constructed events for exactly this one field.
func TestClassify_ScenarioSpecificFieldMissingOnly(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
	}{
		{"scenario2_missing_pod_spec", "scenario2-valid.json"},
		{"scenario3_missing_role_and_subjects", "scenario3-valid.json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ev map[string]any
			if err := json.Unmarshal(readFixture(t, tt.fixture), &ev); err != nil {
				t.Fatalf("unmarshal fixture: %v", err)
			}
			delete(ev, "requestObject")
			got := Classify(mustMarshal(t, ev))
			if got.Outcome != OutcomeIncomplete {
				t.Fatalf("outcome = %q, want %q (reason: %s)", got.Outcome, OutcomeIncomplete, got.Reason)
			}
			if !strings.Contains(got.Reason, "requestObject") {
				t.Errorf("reason = %q, want it to identify the missing requestObject (FR-007(g))", got.Reason)
			}
		})
	}
}

// TestClassify_ScenarioValid confirms that a real, complete ResponseComplete
// -stage event for scenarios 2 and 3 classifies valid — and, per the locked
// Checkpoint 5 scope, regardless of whether any high-risk characteristic
// documented by FR-025 actually holds. Both fixtures do contain high-risk
// content, but Classify does not inspect it; only presence is checked.
func TestClassify_ScenarioValid(t *testing.T) {
	tests := []struct{ fixture string }{
		{"scenario2-valid.json"},
		{"scenario3-valid.json"},
	}
	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			got := Classify(readFixture(t, tt.fixture))
			if got.Outcome != OutcomeValid {
				t.Fatalf("outcome = %q, want %q (reason: %s)", got.Outcome, OutcomeValid, got.Reason)
			}
		})
	}
}

// TestClassify_Precedence constructs cases that would qualify for more than
// one outcome and confirms the higher-precedence outcome always wins
// (FR-006: unsupported > invalid > incomplete > valid).
func TestClassify_Precedence(t *testing.T) {
	const validBase = `{"kind":"Event","apiVersion":"audit.k8s.io/v1","auditID":"a1","stage":"ResponseComplete","requestURI":"/api/v1/namespaces/default/pods","verb":"list","user":{"username":"u"},"objectRef":{"resource":"pods"},"requestReceivedTimestamp":"2026-07-21T15:25:11.901891Z"}`

	t.Run("invalid_beats_incomplete", func(t *testing.T) {
		// Correct kind/apiVersion (attributable) with a type error
		// (auditID is a number) AND a missing core field (no "user") --
		// would be both invalid and incomplete if parsed; invalid must win.
		raw := []byte(`{"kind":"Event","apiVersion":"audit.k8s.io/v1","auditID":123,"stage":"ResponseComplete","requestURI":"/x","verb":"get","objectRef":{"resource":"pods"},"requestReceivedTimestamp":"2026-07-21T15:25:11.901891Z"}`)
		got := Classify(raw)
		if got.Outcome != OutcomeInvalid {
			t.Fatalf("outcome = %q, want %q", got.Outcome, OutcomeInvalid)
		}
	})

	t.Run("unsupported_beats_invalid_and_incomplete", func(t *testing.T) {
		// Wrong kind entirely, plus a type error, plus missing core fields.
		raw := []byte(`{"kind":"SomethingElse","apiVersion":"v2","auditID":123}`)
		got := Classify(raw)
		if got.Outcome != OutcomeUnsupported {
			t.Fatalf("outcome = %q, want %q", got.Outcome, OutcomeUnsupported)
		}
	})

	t.Run("valid_when_nothing_else_applies", func(t *testing.T) {
		got := Classify([]byte(validBase))
		if got.Outcome != OutcomeValid {
			t.Fatalf("outcome = %q, want %q (reason: %s)", got.Outcome, OutcomeValid, got.Reason)
		}
	})
}

// TestClassify_CoreFieldGaps targets each FR-007(a)-(f) item individually
// using minimal constructed events, isolating exactly one missing item per
// case.
func TestClassify_CoreFieldGaps(t *testing.T) {
	full := map[string]any{
		"kind": "Event", "apiVersion": "audit.k8s.io/v1",
		"auditID": "a1", "stage": "ResponseComplete",
		"requestURI": "/api/v1/namespaces/default/pods", "verb": "list",
		"user":                     map[string]any{"username": "u"},
		"objectRef":                map[string]any{"resource": "pods"},
		"requestReceivedTimestamp": "2026-07-21T15:25:11.901891Z",
	}

	tests := []struct {
		name   string
		mutate func(m map[string]any)
	}{
		{"missing_auditID", func(m map[string]any) { delete(m, "auditID") }},
		{"missing_stage", func(m map[string]any) { delete(m, "stage") }},
		{"missing_timestamp", func(m map[string]any) { delete(m, "requestReceivedTimestamp") }},
		{"missing_user", func(m map[string]any) { delete(m, "user") }},
		{"missing_verb", func(m map[string]any) { delete(m, "verb") }},
		{"missing_requestURI", func(m map[string]any) { delete(m, "requestURI") }},
		{"missing_objectRef", func(m map[string]any) { delete(m, "objectRef") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := map[string]any{}
			for k, v := range full {
				m[k] = v
			}
			tt.mutate(m)
			raw := mustMarshal(t, m)
			got := Classify(raw)
			if got.Outcome != OutcomeIncomplete {
				t.Fatalf("outcome = %q, want %q (reason: %s)", got.Outcome, OutcomeIncomplete, got.Reason)
			}
		})
	}
}

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return raw
}

// FuzzClassify proves the property architecture §7 requires beyond the
// boundary-fixture set: for any input, Classify never panics and always
// returns exactly one of the four defined outcomes, with a reason present
// if and only if the outcome is non-valid (FR-005, FR-011, FR-012).
func FuzzClassify(f *testing.F) {
	for _, fixture := range []string{
		"boundary-1-unsupported.json", "boundary-2-invalid.json",
		"boundary-3-incomplete.json", "boundary-4-valid.json",
		"scenario2-incomplete.json", "scenario3-incomplete.json",
		"scenario2-valid.json", "scenario3-valid.json",
	} {
		raw, err := os.ReadFile(filepath.Join("testdata", fixture))
		if err != nil {
			f.Fatalf("read seed fixture %s: %v", fixture, err)
		}
		f.Add(raw)
	}
	f.Add([]byte(`{}`))
	f.Add([]byte(`not json at all`))
	f.Add([]byte(``))

	f.Fuzz(func(t *testing.T, raw []byte) {
		got := Classify(raw)
		switch got.Outcome {
		case OutcomeUnsupported, OutcomeInvalid, OutcomeIncomplete:
			if got.Reason == "" {
				t.Fatalf("outcome %q has no stated reason", got.Outcome)
			}
		case OutcomeValid:
			if got.Reason != "" {
				t.Fatalf("valid outcome has a non-empty reason: %q", got.Reason)
			}
		default:
			t.Fatalf("unrecognized outcome %q", got.Outcome)
		}
	})
}
