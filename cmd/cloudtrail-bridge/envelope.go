// Envelope unwrapping for the CloudTrail bridge: pure, side-effect-free
// logic separated from main.go's AWS/HTTP calls so it can be unit tested
// without any fake SQS or network dependency.
//
// This file implements exactly one transformation, per the approved
// investigation (docs review, AWS-doc verification): an SQS message body
// delivered by the EventBridge->SQS target is an EventBridge event
// envelope whose "detail" field already holds the raw CloudTrail record
// in the exact shape cnsdp's own /v1/cloudtrail-events endpoint expects
// (internal/intake/cloudtrail.go, internal/validation's cloudTrailRecord).
// No field remapping is performed or needed -- only envelope unwrapping.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// ErrMalformedEnvelope is returned when an SQS message body cannot be
// parsed as a plausible EventBridge event envelope at all (not valid
// JSON, or "detail" is present but not a JSON object).
var ErrMalformedEnvelope = errors.New("cloudtrail-bridge: malformed EventBridge envelope")

// ErrMissingDetail is returned when the envelope parses but carries no
// (or a null/empty) "detail" payload -- nothing to forward to CNSDP.
var ErrMissingDetail = errors.New("cloudtrail-bridge: missing or empty EventBridge detail")

// eventBridgeEnvelope is the minimal subset of an EventBridge event's
// fields the bridge needs. Source/DetailType are decoded only for log
// context (see logEnvelopeContext) -- the bridge does not re-validate the
// EventBridge rule's own filtering (source: aws.iam, detail-type: "AWS
// API Call via CloudTrail"); that already happened before this message
// ever reached the queue.
type eventBridgeEnvelope struct {
	Source     string          `json:"source"`
	DetailType string          `json:"detail-type"`
	Detail     json.RawMessage `json:"detail"`
}

// extractCloudTrailRecord unwraps one SQS message body down to the raw
// CloudTrail record CNSDP's /v1/cloudtrail-events endpoint expects
// verbatim as the POST body. It performs no CloudTrail-record-level
// validation (missing eventID, wrong eventCategory, etc.) -- that
// judgment belongs entirely to CNSDP's own validation checkpoint
// (FR-004-FR-010); this function only confirms the SQS message is a
// well-formed EventBridge envelope carrying a non-empty JSON object as
// its detail, i.e. something worth POSTing at all.
func extractCloudTrailRecord(sqsBody []byte) (json.RawMessage, error) {
	var env eventBridgeEnvelope
	if err := json.Unmarshal(sqsBody, &env); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMalformedEnvelope, err)
	}

	trimmed := bytes.TrimSpace(env.Detail)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return nil, ErrMissingDetail
	}
	if trimmed[0] != '{' {
		return nil, fmt.Errorf("%w: detail is not a JSON object", ErrMalformedEnvelope)
	}

	return env.Detail, nil
}

// logEnvelopeContext best-effort-extracts non-sensitive identifying
// fields for structured logging -- eventID/eventName/eventSource only,
// never requestParameters/responseElements/userIdentity, which can carry
// sensitive request content (docs/security/threat-model.md asset A6's own
// concern about raw telemetry sensitivity applies here too: the bridge
// must not casually widen where that content gets copied). A decode
// failure yields all-empty fields rather than an error -- this is a
// logging aid only, never a correctness gate.
func logEnvelopeContext(detail json.RawMessage) (eventID, eventName, eventSource string) {
	var rec struct {
		EventID     string `json:"eventID"`
		EventName   string `json:"eventName"`
		EventSource string `json:"eventSource"`
	}
	_ = json.Unmarshal(detail, &rec)
	return rec.EventID, rec.EventName, rec.EventSource
}
