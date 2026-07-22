package alerting

import (
	"reflect"
	"testing"
	"time"

	"cnsdp/internal/detection"
	"cnsdp/internal/normalization"
)

// TestBuildSummary_CarriesMatchReasonAndNormalizedContentVerbatim proves
// FR-029's required alert content: buildSummary must convey the matched
// definition's identity and satisfied conditions exactly as recorded by
// detection evaluation, plus the normalized event's subject, operation,
// target, outcome, and request time -- nothing more, nothing derived.
func TestBuildSummary_CarriesMatchReasonAndNormalizedContentVerbatim(t *testing.T) {
	reason := detection.MatchReason{
		Scenario:           "scenario-1",
		DefinitionName:     "Interactive exec with TTY and stdin",
		DefinitionRevision: "abc123",
		SatisfiedCharacteristics: []detection.Characteristic{
			{ID: "tty_allocation", Description: "TTY was allocated."},
			{ID: "stdin_streaming", Description: "Stdin was streamed."},
		},
	}
	requestTime := time.Date(2026, 7, 21, 15, 25, 11, 0, time.UTC)
	event := normalization.Event{
		Subject:     normalization.Subject{Username: "kubernetes-admin"},
		Operation:   normalization.Operation{Verb: "get", RequestURI: "/api/v1/namespaces/default/pods/high-risk-pod/exec?stdin=true&tty=true"},
		Target:      normalization.Target{Resource: "pods", Name: "high-risk-pod", Namespace: "default", Subresource: "exec"},
		Outcome:     normalization.Outcome{Code: 101},
		RequestTime: requestTime,
	}

	got := buildSummary(reason, event)

	if !reflect.DeepEqual(got.MatchReason, reason) {
		t.Errorf("MatchReason = %+v, want %+v (verbatim, never re-derived)", got.MatchReason, reason)
	}
	if got.Subject != event.Subject {
		t.Errorf("Subject = %+v, want %+v", got.Subject, event.Subject)
	}
	if got.Operation != event.Operation {
		t.Errorf("Operation = %+v, want %+v", got.Operation, event.Operation)
	}
	if got.Target != event.Target {
		t.Errorf("Target = %+v, want %+v", got.Target, event.Target)
	}
	if got.Outcome != event.Outcome {
		t.Errorf("Outcome = %+v, want %+v", got.Outcome, event.Outcome)
	}
	if !got.RequestTime.Equal(event.RequestTime) {
		t.Errorf("RequestTime = %v, want %v", got.RequestTime, event.RequestTime)
	}
}
