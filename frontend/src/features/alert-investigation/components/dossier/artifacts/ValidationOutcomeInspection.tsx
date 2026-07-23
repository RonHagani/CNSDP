import type { ValidationOutcomeInspection as ValidationOutcomeInspectionModel } from "@/features/alert-investigation/lib/artifactInspection";
import styles from "../dossier.module.css";

/**
 * Validation Outcome Inspection Leaf content. Renders exactly the outcome
 * and, when present, the reason — nothing more. A `valid` outcome
 * legitimately carries no reason; this component states that fact plainly
 * rather than adding explanatory policy prose (e.g. citing FR-011/FR-012)
 * that is not part of the model itself.
 */
export function ValidationOutcomeInspection({
  inspection,
}: {
  inspection: ValidationOutcomeInspectionModel;
}) {
  if (!inspection.available) {
    return <p className={styles.proseSecondary}>Validation outcome is not available for this alert.</p>;
  }

  return (
    <dl className={styles.fieldList}>
      <div className={styles.fieldRow}>
        <dt>Outcome</dt>
        <dd className={styles.technical}>{inspection.outcome}</dd>
      </div>
      <div className={styles.fieldRow}>
        <dt>Reason</dt>
        <dd>{inspection.reason ?? "None recorded."}</dd>
      </div>
    </dl>
  );
}
