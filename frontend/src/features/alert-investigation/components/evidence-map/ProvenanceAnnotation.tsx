import type { ProvenanceState } from "@/features/alert-investigation/lib/provenance";
import styles from "./evidence-map.module.css";

/**
 * The anchored provenance annotation for the currently selected
 * characteristic (UX spec §12 step 4, §13). Renders exactly one of the
 * three states `provenance.ts` produces; nothing here computes or
 * reinterprets which state applies, and no raw path or value is ever
 * fabricated for the Partial/Unavailable cases (UX spec §13's binding
 * rule).
 *
 * Pass 1 scope: rendered as a docked panel beneath the characteristic bus
 * rather than a leader-line-anchored callout positioned at the trace's
 * exact landing point (UX spec §12 step 4's full spatial requirement) —
 * the leader-line anchoring mechanism is deferred to a later refinement
 * pass; see the migration report.
 */
export function ProvenanceAnnotation({ provenance }: { provenance: ProvenanceState }) {
  return (
    <div className={styles.annotation} data-kind={provenance.kind} role="group" aria-label="Provenance record">
      <p className={styles.annotationKind}>{kindLabel(provenance.kind)}</p>
      <p className={`${styles.annotationLine} ${styles.wrapLongValue}`}>{provenance.condition}</p>

      {provenance.kind === "verified" && (
        <>
          <p className={`${styles.annotationLine} ${styles.wrapLongValue}`}>
            Normalized <code className={styles.technical}>{provenance.normalizedPath}</code> ={" "}
            <code className={styles.technical}>{provenance.normalizedValue}</code>. {provenance.behavior}
          </p>
          <p className={`${styles.annotationLine} ${styles.wrapLongValue}`}>
            Raw origin <code className={styles.technical}>{provenance.rawPath}</code>:{" "}
            <code className={styles.technical}>{provenance.rawValue}</code>
          </p>
        </>
      )}

      {provenance.kind === "partial" && (
        <>
          <p className={`${styles.annotationLine} ${styles.wrapLongValue}`}>
            Normalized <code className={styles.technical}>{provenance.normalizedPath}</code> ={" "}
            <code className={styles.technical}>{provenance.normalizedValue}</code>.
          </p>
          <p className={`${styles.annotationLine} ${styles.wrapLongValue}`}>{provenance.limitation}</p>
        </>
      )}

      {provenance.kind === "unavailable" && (
        <p className={`${styles.annotationLine} ${styles.wrapLongValue}`}>
          <span className={styles.technical}>
            {provenance.missingArtifact === "source-event" ? "Source event" : "Normalized event"}
          </span>{" "}
          unavailable — {provenance.explanation}
        </p>
      )}
    </div>
  );
}

function kindLabel(kind: ProvenanceState["kind"]): string {
  switch (kind) {
    case "verified":
      return "Provenance — Verified";
    case "partial":
      return "Provenance — Partial";
    case "unavailable":
      return "Provenance — Unavailable";
  }
}
