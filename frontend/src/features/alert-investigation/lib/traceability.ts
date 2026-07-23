import type { AlertInvestigationResponse, FailedLink } from "@/types/contract";

/**
 * The Traceability Proof model (UX spec §3.6, §7): a plain, declarative
 * verification statement, never a chain-link graphic. `failedLinkExplanation`
 * is generalized from `ProofChain.tsx`'s `FAILED_LINK_EXPLANATION` map
 * (ProofChain.tsx itself is not imported or modified — only its
 * already-correct explanatory text is preserved here, as UX spec §7
 * explicitly calls for).
 *
 * Traceability and per-condition provenance (provenance.ts) are
 * independent concepts computed at different levels: this model verifies
 * the alert-to-source artifact chain as a whole; provenance describes
 * whether one specific matched condition's raw field can be identified
 * within an already-available artifact. `evidenceSetComplete` — mirroring
 * `internal/evidence/evidence.go`'s own `Inventory.Complete()` formula —
 * is an additional, explicitly-named joint fact, not a replacement for
 * either `intact` or per-artifact availability, both of which remain
 * independently present in the root investigation view model.
 */

export type TraceabilityChainLink = "alert" | "detection-result" | "normalized-event" | "submission";

/** The real four-artifact chain internal/traceability.VerifyAlert walks:
 *  alert → detection result → normalized event → submission. */
export const TRACEABILITY_CHAIN_ORDER: readonly TraceabilityChainLink[] = [
  "alert",
  "detection-result",
  "normalized-event",
  "submission",
];

const FAILED_LINK_EXPLANATION: Record<FailedLink, string> = {
  alert: "The alert itself does not resolve through the chain.",
  source_key: "The submission's re-derived source identity no longer matches its stored record.",
  raw_event_sha256: "The stored raw event no longer matches its recorded integrity digest.",
};

const INTACT_EXPLANATION =
  "The alert resolves through its detection result and normalized event to the source submission; the re-derived source identity and raw-event digest both match their stored records.";

const UNSPECIFIED_BROKEN_EXPLANATION = "A required link did not resolve.";

export interface TraceabilityProofModel {
  intact: boolean;
  /** The real four-link chain, in resolution order — reference data, not
   *  a claim that every link independently resolved (see `intact`). */
  chain: readonly TraceabilityChainLink[];
  failedLink?: FailedLink;
  /** Restrained, factual, product-language explanation — for `intact`,
   *  what was verified; for a broken chain, the specific named link's
   *  real-world meaning. Always framed as computed on this retrieval, not
   *  a cached flag (ARCH-01 §4). */
  explanation: string;
  /** intact && every one of the six evidence artifacts independently
   *  available — the same formula internal/evidence.Inventory.Complete()
   *  uses server-side. Does not replace or substitute for either fact. */
  evidenceSetComplete: boolean;
}

/**
 * Builds the Traceability Proof. `allArtifactsAvailable` is supplied by
 * the caller (see evidenceRegister.ts's function of the same name) so
 * this module does not duplicate the six-artifact availability check.
 */
export function buildTraceabilityProof(
  data: AlertInvestigationResponse,
  allArtifactsAvailable: boolean,
): TraceabilityProofModel {
  const { intact, failedLink } = data.traceability;

  const explanation = intact
    ? INTACT_EXPLANATION
    : failedLink
      ? FAILED_LINK_EXPLANATION[failedLink]
      : UNSPECIFIED_BROKEN_EXPLANATION;

  return {
    intact,
    chain: TRACEABILITY_CHAIN_ORDER,
    failedLink,
    explanation,
    evidenceSetComplete: intact && allArtifactsAvailable,
  };
}
