import type { FailedLink } from "@/types/contract";
import type { TraceabilityProofModel } from "@/features/alert-investigation/lib/traceability";
import styles from "./evidence-map.module.css";

/**
 * The permanent Traceability Rail (UX spec §11): the real four-artifact
 * identity chain — Source Submission → Normalized Event → Detection
 * Result → Generated Alert — always visible, independent of any
 * selection. Consumes `TraceabilityProofModel` unchanged (`lib/traceability.ts`
 * is not modified by this pass).
 *
 * Segment-broken-state localization is presentation logic derived purely
 * from the already-typed `failedLink` union (three fixed values), not new
 * domain computation: `raw_event_sha256`/`source_key` concern the
 * Submission → Normalized Event segment; `alert` concerns the Detection
 * Result → Generated Alert segment (UX spec §11 "Failed-link
 * localization"). The middle segment (Normalized Event → Detection
 * Result) has no corresponding `failedLink` value and always renders
 * healthy — this mirrors the real model exactly, not an approximation.
 */

const RAIL_NODES = [
  { key: "submission", label: "Source submission" },
  { key: "normalized-event", label: "Normalized event" },
  { key: "detection-result", label: "Detection result" },
  { key: "alert", label: "Generated alert" },
] as const;

function segmentBroken(index: 0 | 1 | 2, failedLink: FailedLink | undefined): boolean {
  if (!failedLink) return false;
  if (index === 0) return failedLink === "raw_event_sha256" || failedLink === "source_key";
  if (index === 2) return failedLink === "alert";
  return false;
}

export function TraceabilityRail({ traceability }: { traceability: TraceabilityProofModel }) {
  return (
    <section
      className={`${styles.artifactShape} ${styles.rail}`}
      aria-labelledby="evidence-map-traceability-heading"
      role="status"
    >
      <h2 id="evidence-map-traceability-heading" className={styles.eyebrow}>
        Traceability
      </h2>

      <ol className={styles.railTrack} aria-label="Traceability chain, submission to alert">
        {RAIL_NODES.map((node, i) => (
          <li key={node.key} style={{ display: "contents" }}>
            <div className={styles.railNode}>
              <span className={styles.railNodeMark} aria-hidden="true" />
              <span className={styles.railNodeLabel}>{node.label}</span>
            </div>
            {i < RAIL_NODES.length - 1 && (
              <div
                className={styles.railSegment}
                data-testid={`rail-segment-${i}`}
                data-broken={
                  (!traceability.intact && segmentBroken(i as 0 | 1 | 2, traceability.failedLink)) ||
                  undefined
                }
                aria-hidden="true"
              />
            )}
          </li>
        ))}
      </ol>

      <p className={styles.railExplanation} data-broken={!traceability.intact || undefined}>
        {traceability.intact ? "Traceability intact. " : "Traceability broken. "}
        {traceability.explanation}
      </p>
    </section>
  );
}
