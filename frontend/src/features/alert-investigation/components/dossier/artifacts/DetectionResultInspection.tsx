import type { DetectionResultInspection as DetectionResultInspectionModel } from "@/features/alert-investigation/lib/artifactInspection";
import styles from "../dossier.module.css";

/**
 * Detection Result Inspection Leaf content: the literal `matchReason`
 * artifact exactly as the backend recorded it — definition identity,
 * pinned revision, and the full satisfied-characteristics list. This is
 * the raw artifact; the interpreted declared-vs-satisfied comparison
 * lives in the Evidence Concordance, not here. Related-artifact actions
 * for this (and every) artifact are rendered once, generically, by the
 * enclosing `InspectionLeaf` — not duplicated per artifact view.
 */
export function DetectionResultInspection({
  inspection,
}: {
  inspection: DetectionResultInspectionModel;
}) {
  if (!inspection.available) {
    return <p className={styles.proseSecondary}>Detection result is not available for this alert.</p>;
  }

  const { matchReason } = inspection;

  return (
    <div className={styles.stack}>
      <dl className={styles.fieldList}>
        <div className={styles.fieldRow}>
          <dt>Scenario</dt>
          <dd className={styles.technical}>{matchReason.scenario}</dd>
        </div>
        <div className={styles.fieldRow}>
          <dt>Definition name</dt>
          <dd className={styles.prose}>{matchReason.definitionName}</dd>
        </div>
        <div className={styles.fieldRow}>
          <dt>Definition revision (pinned)</dt>
          <dd className={`${styles.technical} ${styles.wrapLongValue}`}>
            {matchReason.definitionRevision}
          </dd>
        </div>
      </dl>

      <div>
        <p className={styles.eyebrow}>Satisfied characteristics</p>
        {matchReason.satisfiedCharacteristics.length === 0 ? (
          <p className={styles.proseSecondary}>None declared for this scenario.</p>
        ) : (
          <ul className={styles.ruledList}>
            {matchReason.satisfiedCharacteristics.map((c) => (
              <li key={c.id} className={styles.ruledRow}>
                <p className={`${styles.technical} ${styles.wrapLongValue}`}>{c.id}</p>
                <p className={styles.proseSecondary}>{c.description}</p>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  );
}
