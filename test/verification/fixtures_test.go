package main

import (
	"encoding/json"
	"testing"

	"cnsdp/internal/submission"
	"cnsdp/internal/validation"
)

func loadTestFixtures(t *testing.T) *ScenarioFixtures {
	t.Helper()
	root, err := repoRootDir()
	if err != nil {
		t.Fatalf("repoRootDir: %v", err)
	}
	f, err := LoadScenarioFixtures(root)
	if err != nil {
		t.Fatalf("LoadScenarioFixtures: %v", err)
	}
	return f
}

func TestScenarioFixtures_Clone_FreshUniqueAuditIDPerCall(t *testing.T) {
	f := loadTestFixtures(t)

	for _, scenario := range []ScenarioID{Scenario1, Scenario2, Scenario3} {
		item1, id1, err := f.Clone(scenario)
		if err != nil {
			t.Fatalf("Clone(%s) #1: %v", scenario, err)
		}
		item2, id2, err := f.Clone(scenario)
		if err != nil {
			t.Fatalf("Clone(%s) #2: %v", scenario, err)
		}
		if id1 == "" || id2 == "" {
			t.Fatalf("Clone(%s) returned an empty auditID", scenario)
		}
		if id1 == id2 {
			t.Errorf("Clone(%s) returned the same auditID twice: %s", scenario, id1)
		}

		var m1, m2 map[string]json.RawMessage
		if err := json.Unmarshal(item1, &m1); err != nil {
			t.Fatalf("unmarshal clone #1: %v", err)
		}
		if err := json.Unmarshal(item2, &m2); err != nil {
			t.Fatalf("unmarshal clone #2: %v", err)
		}
		var auditID1, auditID2 string
		json.Unmarshal(m1["auditID"], &auditID1)
		json.Unmarshal(m2["auditID"], &auditID2)
		if auditID1 != id1 || auditID2 != id2 {
			t.Errorf("embedded auditID does not match returned id: embedded1=%s returned1=%s embedded2=%s returned2=%s", auditID1, id1, auditID2, id2)
		}

		// Every other field must be preserved unchanged from the template
		// (matching is keyed on objectRef/verb/requestURI/requestObject,
		// never auditID -- see fixtures.go's Clone doc comment).
		if string(m1["kind"]) != `"Event"` {
			t.Errorf("Clone(%s): kind = %s, want \"Event\"", scenario, m1["kind"])
		}
	}
}

func TestScenarioFixtures_Clone_UnknownScenario_Errors(t *testing.T) {
	f := loadTestFixtures(t)
	if _, _, err := f.Clone(ScenarioID("scenario-99")); err == nil {
		t.Error("expected an error for an unknown scenario id")
	}
}

func TestNewFillerEvent_ValidatesAsValid_NeverMatchesAnyScenario(t *testing.T) {
	item, id1 := NewFillerEvent(1)
	_, id2 := NewFillerEvent(2)
	if id1 == id2 {
		t.Error("NewFillerEvent should produce a fresh auditID per call")
	}

	// The name of this test promises OutcomeValid -- assert it against the
	// real classifier, not just structural field presence: a
	// requestReceivedTimestamp/stageTimestamp that fails to parse as
	// metav1.MicroTime makes json.Unmarshal itself fail, which
	// internal/validation.Classify reports as OutcomeInvalid, not
	// OutcomeIncomplete or a silently wrong date -- exactly the failure
	// mode this test must catch.
	if result := validation.Classify(item, submission.FamilyKubernetes); result.Outcome != validation.OutcomeValid {
		t.Errorf("validation.Classify(filler event) = %s (%s), want %s", result.Outcome, result.Reason, validation.OutcomeValid)
	}

	var m map[string]any
	if err := json.Unmarshal(item, &m); err != nil {
		t.Fatalf("unmarshal filler event: %v", err)
	}
	objectRef, _ := m["objectRef"].(map[string]any)
	if objectRef == nil {
		t.Fatal("filler event missing objectRef")
	}
	resource, _ := objectRef["resource"].(string)
	subresource, _ := objectRef["subresource"].(string)
	verb, _ := m["verb"].(string)

	// Mirrors internal/validation.scenarioOf's exact three match
	// conditions (pods/exec, pods+create with no subresource,
	// clusterrolebindings+create) -- the filler event must trip none of
	// them.
	if resource == "pods" && subresource == "exec" {
		t.Error("filler event would match scenario 1 (pods/exec)")
	}
	if resource == "pods" && subresource == "" && verb == "create" {
		t.Error("filler event would match scenario 2 (pods create)")
	}
	if resource == "clusterrolebindings" && verb == "create" {
		t.Error("filler event would match scenario 3 (clusterrolebindings create)")
	}

	for _, field := range []string{"auditID", "stage", "requestReceivedTimestamp", "verb", "requestURI"} {
		if _, ok := m[field]; !ok {
			t.Errorf("filler event missing required field %q (internal/validation.missingCoreField would classify this incomplete)", field)
		}
	}
	if m["user"] == nil {
		t.Error("filler event missing user")
	}
}
