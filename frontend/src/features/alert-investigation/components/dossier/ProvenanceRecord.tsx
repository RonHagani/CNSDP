import type { ProvenanceState } from "@/features/alert-investigation/lib/provenance";
import styles from "./dossier.module.css";

/**
 * Renders the detailed provenance record for one selected matched
 * condition (UX spec §3.5b; recomposed as a deeper evidentiary annotation
 * in visual-authorship pass 2). Exactly one of the three states is ever
 * shown; nothing here computes or reinterprets which state applies — that
 * decision was already made by `provenance.ts` (Step 1). Provenance state
 * is field-level inspection state, not traceability state — this
 * component never renders anything about `traceability.intact`/
 * `failedLink`.
 *
 * Reads as continuous annotation prose attached to the selected condition
 * (the caller already labels it "Selected condition provenance"), not a
 * repeated label/value form — each state states only what it actually
 * knows: Verified carries the full normalized→raw chain, Partial stops at
 * the normalized fact plus its named limitation (no "Raw path" field is
 * ever rendered here — nothing is fabricated), Unavailable names the
 * missing artifact plainly. The technical values that are independently
 * inspectable (paths/values) each stay in their own inline element.
 */
export function ProvenanceRecord({ provenance }: { provenance: ProvenanceState }) {
  return (
    <div
      className={`${styles.record} ${styles.provenanceRecord}`}
      role="group"
      aria-label="Provenance record"
    >
      <p className={`${styles.prose} ${styles.wrapLongValue} ${styles.annotationCondition}`}>
        {provenance.condition}
      </p>

      {provenance.kind === "verified" && (
        <>
          <p className={`${styles.prose} ${styles.wrapLongValue} ${styles.annotationLine}`}>
            Normalized as <code className={styles.recordValue}>{provenance.normalizedPath}</code>
            {" = "}
            <code className={styles.recordValue}>{provenance.normalizedValue}</code>. {provenance.behavior}
          </p>
          <p
            className={`${styles.prose} ${styles.wrapLongValue} ${styles.annotationLine} ${styles.annotationRaw}`}
          >
            Raw origin <code className={styles.recordValue}>{provenance.rawPath}</code>:{" "}
            <code className={styles.recordValue}>{provenance.rawValue}</code>
          </p>
        </>
      )}

      {provenance.kind === "partial" && (
        <>
          <p className={`${styles.prose} ${styles.wrapLongValue} ${styles.annotationLine}`}>
            Normalized as <code className={styles.recordValue}>{provenance.normalizedPath}</code>
            {" = "}
            <code className={styles.recordValue}>{provenance.normalizedValue}</code>.
          </p>
          <p
            className={`${styles.proseSecondary} ${styles.wrapLongValue} ${styles.annotationLine} ${styles.annotationLimitation}`}
          >
            {provenance.limitation}
          </p>
        </>
      )}

      {provenance.kind === "unavailable" && (
        <p
          className={`${styles.proseSecondary} ${styles.wrapLongValue} ${styles.annotationLine} ${styles.annotationLimitation}`}
        >
          <span className={styles.technical}>
            {provenance.missingArtifact === "source-event" ? "Source event" : "Normalized event"}
          </span>{" "}
          unavailable — {provenance.explanation}
        </p>
      )}
    </div>
  );
}
