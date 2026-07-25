import type { ValidationOutcomeInspection } from "@/features/alert-investigation/lib/artifactInspection";
import styles from "./evidence-map.module.css";

/**
 * The Validation Outcome checkpoint (UX spec §6): a small, compact object
 * attached to the Source Submission — not a peer-weighted artifact.
 * Renders generically for all four contract-permitted outcome values
 * (`valid`/`invalid`/`incomplete`/`unsupported`), never hardcoded to
 * assume `valid` is the only reachable value, even though every alert
 * this screen can show has a `valid` outcome by construction (FR-014).
 *
 * Pass 1 scope: a circular badge conveys "checkpoint, not a peer
 * artifact" structurally; the prototype's rotated double-ring rubber-stamp
 * rendering is a later visual-polish pass, not attempted here.
 */
export function ValidationStamp({ validation }: { validation: ValidationOutcomeInspection }) {
  if (!validation.available) {
    return (
      <div className={`${styles.artifactShape} ${styles.specimen}`} role="group" aria-label="Validation outcome">
        <p className={styles.statusUnavailable}>Validation outcome unavailable</p>
      </div>
    );
  }

  return (
    <div role="group" aria-label="Validation outcome">
      <div className={styles.validationStamp} data-outcome={validation.outcome}>
        <span className={styles.validationStampLabel}>{validation.outcome}</span>
      </div>
      {validation.reason && <p className={styles.eyebrow}>{validation.reason}</p>}
    </div>
  );
}
