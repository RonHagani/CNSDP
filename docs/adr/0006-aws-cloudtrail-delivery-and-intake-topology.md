# ADR-0006: AWS CloudTrail delivery mechanism and second-endpoint topology

| Field | Value |
| --- | --- |
| Status | Proposed (pending this scope gate's approval) |
| Document | `docs/architecture.md` (ARCH-01) §5 |

## Context

FR-002 and FR-007 (as extended for a second source family) and FR-037–040
require a second defined intake for AWS CloudTrail management events.
Unlike the Kubernetes case (ADR-0003), CNSDP holds no external cloud
credential today and the reference environment has no public network
exposure — so this decision must also resolve how data physically arrives,
not only what contract shape it takes. Candidates evaluated: (a) Amazon
EventBridge + SQS + a small bridge/poller process; (b) direct
CloudTrail-to-S3 log polling; (c) Lambda-push requiring public ingress;
(d) ingesting AWS GuardDuty or Security Hub findings instead of raw
CloudTrail activity; (e) one unified intake endpoint distinguishing
payload shape by content-sniffing, versus a second, dedicated endpoint.

## Decision

A second, dedicated HTTP(S) intake endpoint accepting AWS CloudTrail
management-event records, reusing ADR-0003's already-accepted dual-layer
pattern (external contract mapped onto the same small canonical internal
submission model) rather than re-deciding it.

Delivery: a CloudTrail Trail (Write management events only, `us-east-1`)
publishes matching events to Amazon EventBridge's default event bus; a
single EventBridge rule (`source: aws.iam`, default `ENABLED` rule state)
forwards matches to a dedicated Amazon SQS queue; a small bridge/poller
process, external to the platform's own two-service Compose deployment,
long-polls the queue and POSTs each event to the new intake endpoint using
the same shared bearer token already governing every other product-exposed
access path (NFR-012).

Root-account activity (scenario 7) is supported by the same detection and
normalization machinery but is exercised only via fixture/recorded
delivery, not the live EventBridge/SQS path: an IAM (non-root) user's
console sign-in is not reliably single-region, and root console sign-in
specifically would require a live root session that the demonstration
should not require operating.

## Consequences

- Existing Kubernetes intake, validation, normalization, and detection
  behavior for scenarios 1–3 is unchanged; the new endpoint is strictly
  additive.
- A schema migration adds a source-family discriminator to `submissions`
  (existing Kubernetes rows unaffected; additive column).
  `internal/submission`'s `SourceKey` dedup scheme gains a
  CloudTrail-specific branch keyed on the record's own `eventID`,
  following the same prefix-disambiguation pattern already used for
  Kubernetes identity.
- `internal/validation`, `internal/normalization`, and `internal/detection`
  each gain a per-source-family branch (the extended FR-002/FR-007
  CloudTrail form; a new characteristic bag; four new characteristic-set
  entries and four new declarative YAML definitions). The generic
  evaluation algorithm, alerting, evidence assembly, and traceability
  verification require no change — they already operate on
  `normalized_events`/`detection_results`/`alerts` rows without depending
  on how a row was produced.
- `internal/datasources`' ingestion-channel summary (FR-036, now plural)
  gains a second reported channel; its current single-hardcoded-channel
  implementation must be generalized — a genuine, if small, code change
  flagged here for the eventual implementation pass, not made by this
  documentation gate.
- The bridge/poller is a new standing component outside
  `docker-compose.yml`'s two services, holding a narrowly IAM-scoped
  credential (`sqs:ReceiveMessage`/`DeleteMessage`/`GetQueueAttributes` on
  one named queue only) — never an account-wide credential.
- Evidence for the AWS-side claims above was verified directly against
  current AWS documentation during design, not by an executed spike: the
  Trail-required-for-EventBridge-delivery behavior, IAM/root
  global-service `us-east-1` determinism, IAM-user-console-sign-in
  regional non-determinism, and the two-part EventBridge/SQS permission
  model (a queue resource policy authorizing EventBridge's `SendMessage`,
  source-ARN-scoped, separate from the bridge's own identity policy). This
  is a materially different evidence standard than Spike 1's executed,
  captured-cluster validation — recorded as an open assumption
  (`docs/architecture.md` §9), not silently equated with it.

## Alternatives considered

- **Ingesting AWS GuardDuty or Security Hub findings instead of raw
  CloudTrail activity** — rejected: both are already-computed judgments
  from another product; CNSDP's own detection engine would perform little
  to no real evaluation on them, undermining the platform's core value
  proposition (PC-008) of explainable, home-grown detection.
- **Unified single intake endpoint with content-sniffing** — rejected: the
  two wire formats share no common envelope; content-sniffing would add
  real branching complexity to the one part of the system ADR-0003
  deliberately kept simple, for no benefit over a second, equally simple,
  dedicated endpoint.
- **Requiring the live demo to also cover root-account console activity**
  — rejected: IAM-user console sign-in's regional non-determinism would
  force either multi-region infrastructure or an unreliable single-region
  rule; root console sign-in specifically would require operating a live
  root session as a routine demo step, which is avoidable without losing
  security relevance — the equivalent credential- and privilege-escalation
  risk is already covered live by scenarios 5 and 6.
- **Direct CloudTrail-to-S3 polling, bypassing EventBridge/SQS** —
  rejected: requires the bridge to poll and diff S3 objects itself, a more
  complex and higher-latency mechanism than EventBridge's native filtering
  plus SQS's simple long-poll consumption model, for no compensating
  benefit at this scale.

## References

FR-002, FR-007, FR-037, FR-038, FR-039, FR-040; `docs/scope.md` "Selected
telemetry source family — AWS CloudTrail management events"; "CNSDP AWS
Intake Decision" and "CNSDP Reality Audit" (this project's originating
design artifacts).
