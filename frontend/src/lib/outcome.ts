import type { Outcome } from "@/types/contract";

/**
 * Formats an `Outcome` for display, generic across both source families
 * rather than assuming Kubernetes' `code` is the only real signal.
 *
 * `code` (Kubernetes' HTTP response code) takes precedence when present --
 * unchanged behavior for every existing Kubernetes call site. When absent,
 * `errorCode`/`successful` (AWS CloudTrail's own signal, ADR-0006) are
 * consulted instead: a non-empty `errorCode` means the call failed; a true
 * `successful` with no `errorCode` means it succeeded.
 *
 * The remaining case -- `code` absent, `errorCode` absent, `successful`
 * false or undefined -- is Kubernetes' own "no outcome recorded" state
 * (scenario 1, whose match condition does not require one; see
 * `internal/normalization/normalization.go`'s `Outcome` doc comment). It is
 * never rendered as an error: the backend derivation guarantees a
 * CloudTrail event can only report `successful: false` when `errorCode` is
 * also non-empty (`Successful` is defined as "no recorded errorCode"), so
 * this state can only arise from a Kubernetes event with no recorded
 * outcome at all.
 */
export function formatOutcome(outcome: Outcome): string {
  if (outcome.code !== undefined) return String(outcome.code);
  if (outcome.errorCode) return `error: ${outcome.errorCode}`;
  if (outcome.successful) return "success";
  return "—";
}
