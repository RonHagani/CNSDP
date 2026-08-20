// CNSDP POST + HTTP-response classification for the CloudTrail bridge.
//
// classifyStatus is pure and unit-tested directly. Poster.Post performs
// the one side effect (the HTTP call) and is exercised in tests against
// an httptest.Server, never a hand-rolled fake http.RoundTripper -- no
// interface abstraction is introduced here, per the approved plan's
// "use an interface only where genuinely needed to fake the small SQS
// surface" instruction: there is exactly one HTTP client in this
// program, and net/http/httptest already lets it be tested for real.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// cnsdpPOSTTimeout bounds exactly one admission POST to CNSDP. Set
// independently of every AWS SDK timeout in main.go -- CNSDP's own
// intake handler is a synchronous DB insert with no long-poll or
// multi-second pipeline work in the request path (AC-020's 60s budget
// covers the durable worker's later stages, not this POST), so 10s is
// generous headroom, not a tight bound. This constant must never be
// reused for any AWS SDK call (see main.go's own timeout constants).
const cnsdpPOSTTimeout = 10 * time.Second

// postOutcome classifies what the bridge's message-processing loop
// should do next after one CNSDP admission attempt -- the exact
// four-way state machine approved for this slice.
type postOutcome int

const (
	// outcomeDelete: CNSDP admitted (or idempotently re-admitted) the
	// record. Delete the SQS message and continue polling.
	outcomeDelete postOutcome = iota
	// outcomeRetainImmediate: a deterministic, message-specific failure
	// (CNSDP 409 source conflict, or any other unexpected non-auth 4xx).
	// Retrying the identical content cannot help, but it must not be
	// silently discarded either -- do not delete; do not locally back
	// off (this message's problem says nothing about the next message);
	// keep polling immediately. SQS's own redrive policy carries it to
	// the DLQ after maxReceiveCount attempts if it is never resolved by
	// hand.
	outcomeRetainImmediate
	// outcomeRetainBackoff: a transient failure (429 over capacity, 5xx,
	// or a network/timeout error reaching CNSDP at all). Do not delete;
	// apply local exponential backoff before the next receive so a
	// struggling CNSDP is not hammered; SQS's natural redelivery will
	// retry this specific message regardless.
	outcomeRetainBackoff
	// outcomeFatal: CNSDP returned 401/403 (authentication/authorization
	// failure) or 404/405 (CNSDP_ENDPOINT points at the wrong path or
	// method -- this bridge always POSTs to one fixed, known-good path,
	// so a 404/405 can only mean the endpoint is misconfigured, never
	// that one particular message is bad). Either way this is a
	// bridge-wide configuration failure, not specific to this message:
	// every other queued message would fail identically, so the
	// consumer must stop rather than churn through maxReceiveCount for
	// every message in the queue -- silently draining an entire backlog
	// to the DLQ because of one wrong URL would hide the real problem.
	// The in-flight message is left completely untouched (not deleted,
	// visibility not extended) so it becomes redeliverable again,
	// unaffected, once an operator fixes the configuration and restarts
	// the bridge.
	outcomeFatal
)

// postResult is Poster.Post's return value: the classified outcome, the
// raw HTTP status observed (0 if the request never got a response), and
// the underlying error for logging (nil on a normal HTTP response, even
// a non-2xx one -- only transport-level failures set it).
type postResult struct {
	outcome    postOutcome
	statusCode int
	err        error
}

// Poster POSTs one CloudTrail record to CNSDP's /v1/cloudtrail-events
// endpoint, authenticated with the same shared bearer token the
// reference environment already documents for every other product-
// exposed path (NFR-012).
type Poster struct {
	Client   *http.Client
	Endpoint string
	Token    string
}

// Post sends one CloudTrail record (the bridge's own extractCloudTrailRecord
// output, forwarded verbatim as the request body) and classifies the
// result. It never logs or returns the token, and never returns the
// response body content (CNSDP's admission response carries only an id
// or a short error string -- nothing the bridge's own decision logic
// needs to inspect).
func (p *Poster) Post(ctx context.Context, record json.RawMessage) postResult {
	ctx, cancel := context.WithTimeout(ctx, cnsdpPOSTTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.Endpoint, bytes.NewReader(record))
	if err != nil {
		// Construction failure (e.g. a malformed configured Endpoint) is
		// a local/configuration problem, not a property of this
		// message -- but it is not distinguishable from "CNSDP
		// unreachable" from the message's point of view, so it is
		// classified the same as a transient network failure: retained,
		// backed off, and retried against the same fixed Endpoint value
		// (which will fail identically every time until an operator
		// fixes CNSDP_ENDPOINT and restarts the bridge -- functionally
		// equivalent to the fatal path, but arrived at through the
		// existing transient-retry channel since there is no HTTP
		// status to classify here at all).
		return postResult{outcome: outcomeRetainBackoff, err: fmt.Errorf("build request: %w", err)}
	}
	req.Header.Set("Authorization", "Bearer "+p.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.Client.Do(req)
	if err != nil {
		return postResult{outcome: outcomeRetainBackoff, err: err}
	}
	defer resp.Body.Close()
	// Drain (bounded) so the underlying connection can be reused; the
	// bridge never needs the response body's content.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))

	return postResult{outcome: classifyStatus(resp.StatusCode), statusCode: resp.StatusCode}
}

// classifyStatus maps one CNSDP HTTP response status to the approved
// four-way outcome. Pure and exhaustively unit tested.
func classifyStatus(status int) postOutcome {
	switch {
	case status == http.StatusOK:
		return outcomeDelete
	case status == http.StatusUnauthorized, status == http.StatusForbidden:
		return outcomeFatal
	case status == http.StatusNotFound, status == http.StatusMethodNotAllowed:
		// CNSDP's /v1/cloudtrail-events contract is a single fixed
		// path accepting only POST -- a 404 or 405 cannot be a
		// property of this message's content, only of a misconfigured
		// CNSDP_ENDPOINT (wrong path or method). Treated the same as
		// 401/403: fail fast rather than retaining-immediate, which
		// would otherwise silently drain every queued message to the
		// DLQ as if each one individually had bad content.
		return outcomeFatal
	case status == http.StatusTooManyRequests:
		return outcomeRetainBackoff
	case status >= 500:
		return outcomeRetainBackoff
	default:
		// 409 (source conflict), 413 (should not occur for a
		// KB-scale CloudTrail record), and any other unanticipated 4xx
		// other than 404/405 (handled above): deterministic and
		// message-specific, not transient.
		return outcomeRetainImmediate
	}
}
