// Command cloudtrail-bridge is the external AWS delivery bridge for
// CNSDP's second telemetry source family (ADR-0006, docs/adr/
// 0006-aws-cloudtrail-delivery-and-intake-topology.md): it long-polls a
// dedicated SQS queue fed by an EventBridge rule matching AWS IAM
// write-management-event CloudTrail activity, and POSTs each record to
// the existing, unchanged POST /v1/cloudtrail-events intake endpoint.
//
// This binary is deliberately NOT part of cnsdp's own module graph in
// spirit, even though it lives in this repository for convenience of a
// single go.mod/go.sum: it imports nothing from internal/*, and nothing
// under internal/* imports it. ADR-0006 frames the bridge as external to
// the platform's own two-service Compose deployment; keeping it
// structurally independent of the platform's Go packages extends that
// same boundary to the source tree, not only the deployment topology.
// The only coupling between this binary and cnsdp is the already-tested
// HTTP wire contract (internal/intake/cloudtrail.go).
//
// Credential model: this binary never reads, parses, holds, or logs an
// AWS access key, secret key, or session token itself. It calls the AWS
// SDK's standard default credential chain (config.LoadDefaultConfig) and
// nothing else -- in the approved default operating mode that chain
// resolves to short-lived STS credentials via an assumed IAM role
// (AWS_PROFILE pointing at a role_arn/source_profile or SSO profile in
// the operator's own ~/.aws/config), scoped by that role's own permission
// policy to exactly sqs:ReceiveMessage/DeleteMessage/GetQueueAttributes
// on the one queue this bridge is configured for. See
// scripts/provision-cloudtrail-bridge.ps1 and docs/reference-environment.md.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	smithy "github.com/aws/smithy-go"
)

const (
	// sqsWaitTimeSeconds is the SQS ReceiveMessage long-poll wait
	// parameter (API value, seconds) -- the approved 20s maximum.
	sqsWaitTimeSeconds = 20

	// sqsReceiveCallTimeout bounds the context used for each
	// ReceiveMessage call. It must exceed sqsWaitTimeSeconds with real
	// margin so a full long poll is never truncated by the context
	// itself -- deliberately independent of, and not derived from,
	// cnsdpPOSTTimeout (post.go): these are two unrelated transports
	// with two unrelated timing budgets, and this constant must never
	// be set to or reused as CNSDP's own HTTP client timeout.
	sqsReceiveCallTimeout = 25 * time.Second

	// sqsShortCallTimeout bounds DeleteMessage and GetQueueAttributes --
	// ordinary, non-long-polling SQS API calls. Chosen independently of
	// cnsdpPOSTTimeout; the two constants are unrelated even where their
	// magnitudes happen to be in the same range.
	sqsShortCallTimeout = 15 * time.Second

	// maxMessagesPerReceive is fixed at 1: the bridge processes exactly
	// one message at a time, sequentially -- matching this repository's
	// existing single-worker, no-concurrency precedent (ADR-0002,
	// internal/worker) rather than introducing independent concurrent
	// processing this slice's approved scope never asked for.
	maxMessagesPerReceive = 1
)

// initialBackoff/maxBackoff govern the local, process-level exponential
// backoff applied only after a transient outcome (429/5xx/network
// error) -- never after a message-specific deterministic failure, and
// never as a substitute for SQS's own per-message
// visibility-timeout-driven redelivery. Sequence: 2s, 4s, 8s, 16s, 30s,
// 30s, ... capped, resetting to initialBackoff after the next successful
// outcome or empty receive.
//
// Package-level vars, not consts, solely so tests can shrink them to
// keep backoff-timing assertions fast and non-flaky without touching
// production behavior (main never modifies these).
var (
	initialBackoff = 2 * time.Second
	maxBackoff     = 30 * time.Second
)

// SQSAPI is the narrow slice of *sqs.Client this program calls --
// introduced only so tests can substitute an in-memory fake, per the
// approved plan's "use an interface only where genuinely needed"
// instruction. *sqs.Client satisfies this with no adapter code.
type SQSAPI interface {
	ReceiveMessage(ctx context.Context, params *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error)
	DeleteMessage(ctx context.Context, params *sqs.DeleteMessageInput, optFns ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error)
	GetQueueAttributes(ctx context.Context, params *sqs.GetQueueAttributesInput, optFns ...func(*sqs.Options)) (*sqs.GetQueueAttributesOutput, error)
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	region := requireEnv(logger, "AWS_REGION")
	queueURL := requireEnv(logger, "SQS_QUEUE_URL")
	endpoint := requireEnv(logger, "CNSDP_ENDPOINT")
	token := requireEnv(logger, "CNSDP_API_TOKEN")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// LoadDefaultConfig resolves credentials via the SDK's standard
	// chain only (AWS_PROFILE, shared config/SSO, or ambient
	// environment credentials if the caller's shell already exports
	// them) -- this program never reads an access key or session token
	// into a variable of its own. See the package doc comment above.
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		logger.Error("load AWS config failed", "error", redactedError(err))
		os.Exit(1)
	}
	sqsClient := sqs.NewFromConfig(cfg)

	poster := &Poster{
		Client:   &http.Client{Timeout: cnsdpPOSTTimeout},
		Endpoint: endpoint,
		Token:    token,
	}

	// Fail-fast startup check: confirm the configured credential can
	// actually reach the configured queue before entering the long-poll
	// loop, using the third approved bridge permission
	// (sqs:GetQueueAttributes) rather than discovering a credential or
	// queue-URL mistake only on the first ReceiveMessage.
	attrCtx, cancel := context.WithTimeout(ctx, sqsShortCallTimeout)
	attrs, err := sqsClient.GetQueueAttributes(attrCtx, &sqs.GetQueueAttributesInput{
		QueueUrl:       aws.String(queueURL),
		AttributeNames: []sqstypes.QueueAttributeName{sqstypes.QueueAttributeNameQueueArn},
	})
	cancel()
	if err != nil {
		logger.Error("startup check failed: cannot access configured SQS queue", "error", redactedError(err))
		os.Exit(1)
	}
	logger.Info("cloudtrail-bridge starting",
		"region", region,
		"queueUrl", queueURL,
		"queueArn", attrs.Attributes[string(sqstypes.QueueAttributeNameQueueArn)],
		"endpoint", endpoint,
	)

	if err := run(ctx, sqsClient, poster, queueURL, logger); err != nil {
		logger.Error("cloudtrail-bridge exiting", "error", redactedError(err))
		os.Exit(1)
	}
	logger.Info("cloudtrail-bridge shutdown complete")
}

// requireEnv reads a required environment variable or exits with a
// clear, immediate error -- matching cmd/platform/main.go's own
// fail-clear-on-missing-required-config convention. Never logs the
// value: CNSDP_API_TOKEN is one of the variables this reads, and it must
// never appear in a log line.
func requireEnv(logger *slog.Logger, name string) string {
	v := os.Getenv(name)
	if v == "" {
		logger.Error(name + " is required")
		os.Exit(1)
	}
	return v
}

// run is the bridge's long-poll loop: receive at most one message,
// process it, decide delete/retain/backoff/fatal, repeat until ctx is
// canceled (graceful shutdown, returns nil) or a fatal outcome occurs
// (returns a non-nil error, causing main to exit non-zero).
func run(ctx context.Context, sqsClient SQSAPI, poster *Poster, queueURL string, logger *slog.Logger) error {
	backoff := initialBackoff

	for {
		if ctx.Err() != nil {
			return nil
		}

		rmCtx, cancel := context.WithTimeout(ctx, sqsReceiveCallTimeout)
		out, err := sqsClient.ReceiveMessage(rmCtx, &sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(queueURL),
			MaxNumberOfMessages: maxMessagesPerReceive,
			WaitTimeSeconds:     sqsWaitTimeSeconds,
		})
		cancel()

		if err != nil {
			if ctx.Err() != nil {
				// Shutting down: the ReceiveMessage error is an
				// artifact of context cancellation, not a real fault.
				return nil
			}
			if isFatalAWSError(err) {
				return fmt.Errorf("AWS authentication/permission failure on ReceiveMessage: %w", redactedError(err))
			}
			logger.Warn("receive message failed, will retry", "error", redactedError(err))
			sleepWithBackoff(ctx, &backoff)
			continue
		}

		if len(out.Messages) == 0 {
			// A clean, empty long poll is the healthy steady state --
			// reset backoff so a prior transient episode does not keep
			// throttling the bridge after it has recovered.
			backoff = initialBackoff
			continue
		}

		msg := out.Messages[0]
		outcome, fatalErr := handleMessage(ctx, sqsClient, poster, queueURL, msg, logger)
		if fatalErr != nil {
			return fatalErr
		}
		switch outcome {
		case outcomeDelete:
			backoff = initialBackoff
		case outcomeRetainBackoff:
			sleepWithBackoff(ctx, &backoff)
		case outcomeRetainImmediate:
			// No backoff: this message's own content is the problem,
			// not CNSDP's or AWS's health -- nothing about it predicts
			// trouble with the next message.
		}
	}
}

// handleMessage processes exactly one received SQS message: extract the
// CloudTrail record, POST it, delete on success, and log enough
// non-sensitive context to operate the bridge without ever logging the
// bearer token, AWS credentials, or the full raw record body.
func handleMessage(ctx context.Context, sqsClient SQSAPI, poster *Poster, queueURL string, msg sqstypes.Message, logger *slog.Logger) (postOutcome, error) {
	messageID := aws.ToString(msg.MessageId)

	record, err := extractCloudTrailRecord([]byte(aws.ToString(msg.Body)))
	if err != nil {
		logger.Warn("message-specific failure: envelope not usable, retaining for redelivery/DLQ",
			"messageId", messageID, "error", err)
		return outcomeRetainImmediate, nil
	}

	eventID, eventName, eventSource := logEnvelopeContext(record)
	result := poster.Post(ctx, record)

	switch result.outcome {
	case outcomeDelete:
		logger.Info("admitted",
			"messageId", messageID, "eventID", eventID, "eventName", eventName, "eventSource", eventSource,
			"status", result.statusCode)
		delCtx, cancel := context.WithTimeout(ctx, sqsShortCallTimeout)
		_, delErr := sqsClient.DeleteMessage(delCtx, &sqs.DeleteMessageInput{
			QueueUrl:      aws.String(queueURL),
			ReceiptHandle: msg.ReceiptHandle,
		})
		cancel()
		if delErr != nil {
			if isFatalAWSError(delErr) {
				return outcomeDelete, fmt.Errorf("AWS authentication/permission failure on DeleteMessage: %w", redactedError(delErr))
			}
			// CNSDP already admitted this record (idempotently, keyed
			// on eventID) -- a failed delete here only risks a harmless
			// redelivery that CNSDP will itself deduplicate, never a
			// lost or duplicated admission.
			logger.Warn("delete message failed after successful admission; CNSDP's own eventID idempotency makes any resulting redelivery safe",
				"messageId", messageID, "error", redactedError(delErr))
		}
		return outcomeDelete, nil

	case outcomeRetainImmediate:
		logger.Warn("message-specific failure: CNSDP rejected this record deterministically, retaining for redelivery/DLQ",
			"messageId", messageID, "eventID", eventID, "eventName", eventName, "status", result.statusCode)
		return outcomeRetainImmediate, nil

	case outcomeRetainBackoff:
		logger.Warn("transient failure, retaining and backing off",
			"messageId", messageID, "status", result.statusCode, "error", redactedError(result.err))
		return outcomeRetainBackoff, nil

	default: // outcomeFatal
		return outcomeFatal, fmt.Errorf("CNSDP returned %d (authentication/authorization or endpoint-configuration failure) -- stopping consumer, message left untouched", result.statusCode)
	}
}

// sleepWithBackoff waits the current backoff duration (or until ctx is
// canceled, whichever is first) and doubles the caller's backoff value,
// capped at maxBackoff -- the sole mechanism preventing a hot retry loop
// against a struggling CNSDP or AWS. It never calls SQS's
// ChangeMessageVisibility: each receive is exactly one bounded attempt,
// so nothing here ever needs to extend a message's hold.
func sleepWithBackoff(ctx context.Context, backoff *time.Duration) {
	select {
	case <-time.After(*backoff):
	case <-ctx.Done():
	}
	next := *backoff * 2
	if next > maxBackoff {
		next = maxBackoff
	}
	*backoff = next
}

// fatalAWSErrorCodes are the well-known SQS/STS error codes that
// indicate an authentication or authorization failure specifically, as
// opposed to throttling, validation, or other transient/client errors
// which use distinct codes (e.g. "ThrottlingException",
// "RequestTimeout"). AWS does not expose a single canonical "this was an
// auth failure" error type, so this is a maintained allow-list rather
// than a structural check -- a reasonable, documented heuristic for this
// slice's scope, not a certified-complete enumeration; see the
// implementation report's "concerns" section.
var fatalAWSErrorCodes = map[string]bool{
	"AccessDenied":                true,
	"AccessDeniedException":       true,
	"UnauthorizedAccess":          true,
	"UnrecognizedClientException": true,
	"InvalidClientTokenId":        true,
	"ExpiredToken":                true,
	"ExpiredTokenException":       true,
	"AuthFailure":                 true,
	"InvalidSignatureException":   true,
	"MissingAuthenticationToken":  true,
	"SignatureDoesNotMatch":       true,
}

// isFatalAWSError reports whether err represents an AWS-side
// authentication or permission failure (as opposed to a transient
// network/throttling error), warranting the same fail-fast treatment as
// a 401/403 from CNSDP.
func isFatalAWSError(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		return fatalAWSErrorCodes[apiErr.ErrorCode()]
	}
	return false
}

// redactedError wraps err for logging. AWS SDK and net/http errors do
// not embed credential material in their Error() text under normal
// operation, but this wrapper exists as a single, auditable place to
// extend redaction if a future AWS SDK error type ever changes that --
// today it returns err unchanged.
func redactedError(err error) error {
	return err
}
