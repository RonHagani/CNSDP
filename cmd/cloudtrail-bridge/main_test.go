package main

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	smithy "github.com/aws/smithy-go"
)

// noopLogger discards all output -- these tests assert on fake-SQS/HTTP
// side effects, not log content.
func noopLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

// fakeSQS is the minimal in-memory SQSAPI fake this package's tests
// need: a queue of canned ReceiveMessage responses (consumed in order,
// one per call), and a record of every DeleteMessage receipt handle. A
// call made once the queue is empty blocks on the caller's context,
// exactly like a real long poll with nothing to deliver -- this is what
// lets the graceful-shutdown test exercise a realistic "idle, waiting"
// bridge.
type fakeSQS struct {
	mu       sync.Mutex
	pending  []receiveStep
	deleted  []string
	received int
}

type receiveStep struct {
	out *sqs.ReceiveMessageOutput
	err error
}

func newMessage(id, receiptHandle, body string) sqstypes.Message {
	return sqstypes.Message{
		MessageId:     aws.String(id),
		ReceiptHandle: aws.String(receiptHandle),
		Body:          aws.String(body),
	}
}

func (f *fakeSQS) queue(step receiveStep) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pending = append(f.pending, step)
}

func (f *fakeSQS) ReceiveMessage(ctx context.Context, _ *sqs.ReceiveMessageInput, _ ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error) {
	f.mu.Lock()
	f.received++
	if len(f.pending) == 0 {
		f.mu.Unlock()
		<-ctx.Done()
		return nil, ctx.Err()
	}
	step := f.pending[0]
	f.pending = f.pending[1:]
	f.mu.Unlock()
	return step.out, step.err
}

// receiveCount reports how many times ReceiveMessage has been called --
// used by tests as a bounded, condition-pollable proof that the run()
// loop fully finished handling a message (including any backoff) and
// cycled back to poll again, rather than sleeping a fixed, arbitrary
// duration and hoping that was long enough.
func (f *fakeSQS) receiveCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.received
}

func (f *fakeSQS) DeleteMessage(_ context.Context, in *sqs.DeleteMessageInput, _ ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, aws.ToString(in.ReceiptHandle))
	return &sqs.DeleteMessageOutput{}, nil
}

func (f *fakeSQS) GetQueueAttributes(_ context.Context, _ *sqs.GetQueueAttributesInput, _ ...func(*sqs.Options)) (*sqs.GetQueueAttributesOutput, error) {
	return &sqs.GetQueueAttributesOutput{}, nil
}

func (f *fakeSQS) wasDeleted(receiptHandle string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, rh := range f.deleted {
		if rh == receiptHandle {
			return true
		}
	}
	return false
}

const validEnvelope = `{
	"source": "aws.iam",
	"detail-type": "AWS API Call via CloudTrail",
	"detail": {"eventID": "evt-1", "eventName": "CreateAccessKey", "eventSource": "iam.amazonaws.com"}
}`

// runUntilIdle starts run() in a goroutine and returns a channel that
// receives its error once it returns (nil on graceful shutdown). The
// caller drains fake-SQS/poster side effects, then cancels ctx and
// blocks on the channel with a generous bounded timeout -- never a bare
// sleep-and-check.
func runUntilIdle(t *testing.T, ctx context.Context, sqsClient SQSAPI, poster *Poster) <-chan error {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, sqsClient, poster, "https://example.invalid/queue", noopLogger())
	}()
	return done
}

func waitDone(t *testing.T, done <-chan error, timeout time.Duration) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		t.Fatal("run() did not return within the expected bound")
		return nil
	}
}

func TestRun_HappyPath_ReceivePostDelete(t *testing.T) {
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	fq := &fakeSQS{}
	fq.queue(receiveStep{out: &sqs.ReceiveMessageOutput{
		Messages: []sqstypes.Message{newMessage("m-1", "rh-1", validEnvelope)},
	}})

	ctx, cancel := context.WithCancel(context.Background())
	poster := &Poster{Client: &http.Client{Timeout: cnsdpPOSTTimeout}, Endpoint: srv.URL, Token: "t"}
	done := runUntilIdle(t, ctx, fq, poster)

	waitForCondition(t, func() bool { return fq.wasDeleted("rh-1") }, 2*time.Second)
	if requests != 1 {
		t.Errorf("CNSDP received %d requests, want 1", requests)
	}

	cancel()
	if err := waitDone(t, done, 2*time.Second); err != nil {
		t.Errorf("run() returned %v, want nil on graceful shutdown", err)
	}
}

func TestRun_NonDeleteOutcomes_MessageNeverDeleted(t *testing.T) {
	origInitial, origMax := initialBackoff, maxBackoff
	initialBackoff, maxBackoff = time.Millisecond, 5*time.Millisecond
	t.Cleanup(func() { initialBackoff, maxBackoff = origInitial, origMax })

	cases := []struct {
		name   string
		status int
	}{
		{"409_conflict", http.StatusConflict},
		{"429_too_many_requests", http.StatusTooManyRequests},
		{"500_internal_error", http.StatusInternalServerError},
		{"503_unavailable", http.StatusServiceUnavailable},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var requests atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				w.WriteHeader(c.status)
			}))
			defer srv.Close()

			fq := &fakeSQS{}
			fq.queue(receiveStep{out: &sqs.ReceiveMessageOutput{
				Messages: []sqstypes.Message{newMessage("m-1", "rh-1", validEnvelope)},
			}})

			ctx, cancel := context.WithCancel(context.Background())
			poster := &Poster{Client: &http.Client{Timeout: cnsdpPOSTTimeout}, Endpoint: srv.URL, Token: "t"}
			done := runUntilIdle(t, ctx, fq, poster)

			// Wait for CNSDP to actually observe the POST -- an
			// observable post-handling signal, not an arbitrary sleep --
			// then shut down and confirm the message was never deleted.
			waitForCondition(t, func() bool { return requests.Load() >= 1 }, 2*time.Second)
			cancel()
			if err := waitDone(t, done, 2*time.Second); err != nil {
				t.Errorf("run() returned %v, want nil on graceful shutdown", err)
			}
			if fq.wasDeleted("rh-1") {
				t.Errorf("status %d: message was deleted, want retained", c.status)
			}
		})
	}
}

func TestRun_NetworkError_MessageRetained(t *testing.T) {
	origInitial, origMax := initialBackoff, maxBackoff
	initialBackoff, maxBackoff = time.Millisecond, 5*time.Millisecond
	t.Cleanup(func() { initialBackoff, maxBackoff = origInitial, origMax })

	fq := &fakeSQS{}
	fq.queue(receiveStep{out: &sqs.ReceiveMessageOutput{
		Messages: []sqstypes.Message{newMessage("m-1", "rh-1", validEnvelope)},
	}})

	ctx, cancel := context.WithCancel(context.Background())
	poster := &Poster{Client: &http.Client{Timeout: 200 * time.Millisecond}, Endpoint: "http://127.0.0.1:1", Token: "t"}
	done := runUntilIdle(t, ctx, fq, poster)

	// Wait for the loop to fully handle the network-error outcome
	// (including backoff) and cycle back to a second receive attempt --
	// an observable post-handling signal, not an arbitrary sleep.
	waitForCondition(t, func() bool { return fq.receiveCount() >= 2 }, 2*time.Second)
	cancel()
	if err := waitDone(t, done, 2*time.Second); err != nil {
		t.Errorf("run() returned %v, want nil on graceful shutdown", err)
	}
	if fq.wasDeleted("rh-1") {
		t.Error("message was deleted despite a network error reaching CNSDP")
	}
}

func TestRun_MalformedMessage_RetainedNoBackoff(t *testing.T) {
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	fq := &fakeSQS{}
	fq.queue(receiveStep{out: &sqs.ReceiveMessageOutput{
		Messages: []sqstypes.Message{newMessage("m-1", "rh-1", "not valid json at all")},
	}})

	ctx, cancel := context.WithCancel(context.Background())
	poster := &Poster{Client: &http.Client{Timeout: cnsdpPOSTTimeout}, Endpoint: srv.URL, Token: "t"}
	done := runUntilIdle(t, ctx, fq, poster)

	// Wait for the loop to fully handle the malformed-envelope outcome
	// and cycle back to a second receive attempt -- an observable
	// post-handling signal, not an arbitrary sleep.
	waitForCondition(t, func() bool { return fq.receiveCount() >= 2 }, 2*time.Second)
	cancel()
	if err := waitDone(t, done, 2*time.Second); err != nil {
		t.Errorf("run() returned %v, want nil on graceful shutdown", err)
	}
	if fq.wasDeleted("rh-1") {
		t.Error("malformed message was deleted, want retained for redelivery/DLQ")
	}
	if requests != 0 {
		t.Errorf("CNSDP received %d requests for an unparseable envelope, want 0 (never POSTed)", requests)
	}
}

func TestRun_CNSDPFatalStatus_StopsConsumer(t *testing.T) {
	// 401/403 are authentication/authorization failures; 404/405 mean
	// CNSDP_ENDPOINT is misconfigured (wrong path/method) -- both are
	// bridge-wide configuration failures, not properties of one
	// message, so all four must fail fast rather than draining the
	// queue toward the DLQ as if each message were individually bad.
	cases := []struct {
		name   string
		status int
	}{
		{"401_unauthorized", http.StatusUnauthorized},
		{"403_forbidden", http.StatusForbidden},
		{"404_not_found", http.StatusNotFound},
		{"405_method_not_allowed", http.StatusMethodNotAllowed},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(c.status)
			}))
			defer srv.Close()

			fq := &fakeSQS{}
			fq.queue(receiveStep{out: &sqs.ReceiveMessageOutput{
				Messages: []sqstypes.Message{newMessage("m-1", "rh-1", validEnvelope)},
			}})
			// A second message that must never be reached: the consumer
			// should stop after the first fatal outcome, not churn
			// through the rest of the queue.
			fq.queue(receiveStep{out: &sqs.ReceiveMessageOutput{
				Messages: []sqstypes.Message{newMessage("m-2", "rh-2", validEnvelope)},
			}})

			ctx := context.Background()
			poster := &Poster{Client: &http.Client{Timeout: cnsdpPOSTTimeout}, Endpoint: srv.URL, Token: "t"}
			done := runUntilIdle(t, ctx, fq, poster)

			err := waitDone(t, done, 2*time.Second)
			if err == nil {
				t.Fatalf("run() returned nil, want a fatal error on a %d outcome", c.status)
			}
			if fq.wasDeleted("rh-1") {
				t.Error("message was deleted on a fatal outcome, want it left untouched")
			}
			fq.mu.Lock()
			remaining := len(fq.pending)
			fq.mu.Unlock()
			if remaining == 0 {
				t.Error("consumer drained the second queued message after a fatal outcome, want it to stop immediately")
			}
		})
	}
}

func TestRun_AWSAuthErrorOnReceive_Fatal(t *testing.T) {
	fq := &fakeSQS{}
	fq.queue(receiveStep{err: &smithy.GenericAPIError{Code: "AccessDenied", Message: "not authorized"}})

	ctx := context.Background()
	poster := &Poster{Client: &http.Client{Timeout: cnsdpPOSTTimeout}, Endpoint: "http://127.0.0.1:1", Token: "t"}
	done := runUntilIdle(t, ctx, fq, poster)

	if err := waitDone(t, done, 2*time.Second); err == nil {
		t.Fatal("run() returned nil, want a fatal error on an AWS AccessDenied ReceiveMessage error")
	}
}

func TestRun_GracefulShutdown_WhileIdle(t *testing.T) {
	fq := &fakeSQS{} // never has anything queued -- ReceiveMessage blocks

	ctx, cancel := context.WithCancel(context.Background())
	poster := &Poster{Client: &http.Client{Timeout: cnsdpPOSTTimeout}, Endpoint: "http://127.0.0.1:1", Token: "t"}
	done := runUntilIdle(t, ctx, fq, poster)

	time.Sleep(20 * time.Millisecond) // let run() enter its blocking receive
	cancel()

	if err := waitDone(t, done, 2*time.Second); err != nil {
		t.Errorf("run() returned %v, want nil on graceful shutdown while idle", err)
	}
}

// waitForCondition polls cond until it is true or the timeout elapses --
// used only to detect that an already-fast, already-async operation
// (delivering one queued message) has completed, never to assert a
// precise timing relationship.
func waitForCondition(t *testing.T, cond func() bool, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !cond() {
		t.Fatal("condition was not met within the expected bound")
	}
}
