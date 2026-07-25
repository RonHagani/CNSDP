import { useState } from "react";
import type { SourceEventInspection } from "@/features/alert-investigation/lib/artifactInspection";
import { JsonTree } from "../JsonTree";
import styles from "./evidence-map.module.css";

const EMPTY_PATHS = new Set<string>();

/**
 * The Source Submission specimen (UX spec §5): the origin object, in
 * three visual tiers — identity/request essentials, secondary ingestion
 * metadata, and the raw payload (last, quietest, never hidden). The raw
 * payload is `requestURI` for scenario 1 or a real excerpt of
 * `requestObject` for scenarios 2/3 (both real, contract-typed fields —
 * this distinction is not invented, it follows directly from which field
 * the response actually carries). The on-demand "view full raw record"
 * affordance reuses `JsonTree` unchanged for the complete `RawAuditEvent`
 * (UX spec §5's `annotations`/full `sourceIPs`/complete `requestObject`
 * requirement) — explicit and never auto-expanded.
 *
 * Pass 1 scope: the exact substring-of-raw-payload marking for Verified
 * provenance (UX spec §5, e.g. highlighting the `tty=true` fragment) is
 * deferred — `ProvenanceState` resolves to the containing raw field
 * (e.g. the whole `requestURI`), not a computed substring span; inventing
 * that span here would mean adding new parsing logic outside the domain
 * layer, which this pass does not do. See the migration report.
 */
export function SourceSubmissionSpecimen({ inspection }: { inspection: SourceEventInspection }) {
  const [fullRecordExpanded, setFullRecordExpanded] = useState(false);

  if (!inspection.available) {
    return (
      <section className={`${styles.artifactShape} ${styles.specimen}`} aria-labelledby="specimen-heading">
        <h2 id="specimen-heading" className={styles.eyebrow}>
          Source submission
        </h2>
        <p className={styles.statusUnavailable}>Source event unavailable</p>
      </section>
    );
  }

  const raw = inspection.rawEvent;
  const target = [raw.objectRef?.resource, raw.objectRef?.subresource].filter(Boolean).join("/");

  return (
    <section className={`${styles.artifactShape} ${styles.specimen}`} aria-labelledby="specimen-heading">
      <h2 id="specimen-heading" className={styles.eyebrow}>
        Source submission
      </h2>

      <div className={styles.specimenTier1}>
        <p className={`${styles.technical} ${styles.wrapLongValue}`}>auditID {raw.auditID}</p>
        <p className={styles.technical}>verb {raw.verb}</p>
        <p className={`${styles.technical} ${styles.wrapLongValue}`}>user {raw.user.username}</p>
        <p className={`${styles.technical} ${styles.wrapLongValue}`}>
          objectRef {target || "—"}
          {raw.objectRef?.name ? ` (${raw.objectRef.name})` : ""}
          {raw.objectRef?.namespace ? ` ns:${raw.objectRef.namespace}` : ""}
        </p>
        <p className={styles.technical}>response.code {raw.responseStatus?.code ?? "—"}</p>
      </div>

      <div className={styles.specimenTier2}>
        <p>
          {raw.level ?? "—"} / {raw.stage}
        </p>
        {raw.sourceIPs && raw.sourceIPs.length > 0 && (
          <p className={styles.wrapLongValue}>sourceIPs {raw.sourceIPs.join(", ")}</p>
        )}
        {raw.userAgent && <p className={styles.wrapLongValue}>userAgent {raw.userAgent}</p>}
        <p className={styles.wrapLongValue}>received {raw.requestReceivedTimestamp}</p>
      </div>

      <div className={styles.specimenTier3}>
        <p className={styles.eyebrow}>Raw payload</p>
        <div className={styles.specimenRawBox}>
          {raw.requestObject !== undefined ? (
            <JsonTree
              data={raw.requestObject}
              linkablePaths={EMPTY_PATHS}
              highlightedPaths={EMPTY_PATHS}
              dimOthers={false}
              onSelectPath={() => {}}
            />
          ) : (
            <p className={`${styles.technical} ${styles.wrapLongValue}`}>{raw.requestURI}</p>
          )}
        </div>

        <button
          type="button"
          className={styles.plainButton}
          aria-expanded={fullRecordExpanded}
          onClick={() => setFullRecordExpanded((v) => !v)}
        >
          {fullRecordExpanded ? "Hide full raw record" : "View full raw record"}
        </button>
        {fullRecordExpanded && (
          <div className={styles.specimenRawBox}>
            <JsonTree
              data={raw}
              linkablePaths={EMPTY_PATHS}
              highlightedPaths={EMPTY_PATHS}
              dimOthers={false}
              onSelectPath={() => {}}
            />
          </div>
        )}
      </div>
    </section>
  );
}
