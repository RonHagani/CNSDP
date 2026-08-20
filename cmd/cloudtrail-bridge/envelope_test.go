package main

import (
	"errors"
	"testing"
)

func TestExtractCloudTrailRecord_ValidEnvelope(t *testing.T) {
	body := []byte(`{
		"version": "0",
		"id": "abcd-1234",
		"detail-type": "AWS API Call via CloudTrail",
		"source": "aws.iam",
		"account": "123456789012",
		"time": "2026-08-20T12:00:00Z",
		"region": "us-east-1",
		"detail": {
			"eventVersion": "1.08",
			"eventTime": "2026-08-20T12:00:00Z",
			"eventSource": "iam.amazonaws.com",
			"eventName": "CreateAccessKey",
			"eventCategory": "Management",
			"userIdentity": {"type": "IAMUser", "arn": "arn:aws:iam::123456789012:user/demo", "userName": "demo"},
			"requestParameters": {"userName": "demo"},
			"responseElements": {"accessKey": {"accessKeyId": "AKIAEXAMPLE", "status": "Active"}},
			"eventID": "evt-1",
			"errorCode": ""
		}
	}`)

	record, err := extractCloudTrailRecord(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	eventID, eventName, eventSource := logEnvelopeContext(record)
	if eventID != "evt-1" {
		t.Errorf("eventID = %q, want evt-1", eventID)
	}
	if eventName != "CreateAccessKey" {
		t.Errorf("eventName = %q, want CreateAccessKey", eventName)
	}
	if eventSource != "iam.amazonaws.com" {
		t.Errorf("eventSource = %q, want iam.amazonaws.com", eventSource)
	}
}

func TestExtractCloudTrailRecord_MalformedSQSBody(t *testing.T) {
	cases := []string{
		``,
		`not json at all`,
		`{"detail-type": "AWS API Call via CloudTrail", "detail": `, // truncated
		`42`,
		`"just a string"`,
	}
	for _, c := range cases {
		_, err := extractCloudTrailRecord([]byte(c))
		if !errors.Is(err, ErrMalformedEnvelope) {
			t.Errorf("extractCloudTrailRecord(%q): err = %v, want ErrMalformedEnvelope", c, err)
		}
	}
}

func TestExtractCloudTrailRecord_MissingDetail(t *testing.T) {
	cases := []string{
		`{"source": "aws.iam", "detail-type": "AWS API Call via CloudTrail"}`, // detail absent
		`{"source": "aws.iam", "detail-type": "AWS API Call via CloudTrail", "detail": null}`,
	}
	for _, c := range cases {
		_, err := extractCloudTrailRecord([]byte(c))
		if !errors.Is(err, ErrMissingDetail) {
			t.Errorf("extractCloudTrailRecord(%q): err = %v, want ErrMissingDetail", c, err)
		}
	}
}

// TestExtractCloudTrailRecord_DetailNotAnObject covers "detail" values
// that are present and non-null but structurally not a JSON object --
// including the empty string and a whitespace-only string, which are
// themselves non-empty JSON string values (not an absent/null field),
// so they are correctly classified as malformed rather than missing.
func TestExtractCloudTrailRecord_DetailNotAnObject(t *testing.T) {
	cases := []string{
		`{"detail": "not-an-object"}`,
		`{"detail": 42}`,
		`{"detail": ["a", "b"]}`,
		`{"detail": ""}`,
		`{"detail": "   "}`,
	}
	for _, c := range cases {
		_, err := extractCloudTrailRecord([]byte(c))
		if !errors.Is(err, ErrMalformedEnvelope) {
			t.Errorf("extractCloudTrailRecord(%q): err = %v, want ErrMalformedEnvelope", c, err)
		}
	}
}

func TestLogEnvelopeContext_UndecodableDetailYieldsEmptyFields(t *testing.T) {
	eventID, eventName, eventSource := logEnvelopeContext([]byte(`not json`))
	if eventID != "" || eventName != "" || eventSource != "" {
		t.Errorf("got (%q,%q,%q), want all empty", eventID, eventName, eventSource)
	}
}
