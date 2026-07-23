import type { AlertInspection as AlertInspectionModel } from "@/features/alert-investigation/lib/artifactInspection";
import styles from "../dossier.module.css";

/**
 * Alert Inspection Leaf content: the literal `alert.summary` fields
 * exactly as recorded, distinct from the Finding's plain-language prose
 * rendering of the same underlying data. Renders only fields the model
 * carries — no severity, confidence, assignment, case state, or
 * disposition exists on this artifact and none is introduced here.
 */
export function AlertInspection({ inspection }: { inspection: AlertInspectionModel }) {
  if (!inspection.available) {
    return <p className={styles.proseSecondary}>Alert is not available for this alert record.</p>;
  }

  const { summary } = inspection;
  const target = [summary.target.resource, summary.target.subresource].filter(Boolean).join("/");

  return (
    <dl className={styles.fieldList}>
      <div className={styles.fieldRow}>
        <dt>Matched definition</dt>
        <dd className={styles.prose}>{summary.matchReason.definitionName}</dd>
      </div>
      <div className={styles.fieldRow}>
        <dt>Subject</dt>
        <dd className={styles.technical}>{summary.subject.username}</dd>
      </div>
      <div className={styles.fieldRow}>
        <dt>Operation</dt>
        <dd className={`${styles.technical} ${styles.wrapLongValue}`}>{summary.operation.verb}</dd>
      </div>
      <div className={styles.fieldRow}>
        <dt>Target</dt>
        <dd className={`${styles.technical} ${styles.wrapLongValue}`}>
          {target}
          {summary.target.name ? ` / ${summary.target.name}` : ""}
        </dd>
      </div>
      <div className={styles.fieldRow}>
        <dt>Outcome code</dt>
        <dd className={styles.technical}>{summary.outcome.code ?? "—"}</dd>
      </div>
      <div className={styles.fieldRow}>
        <dt>Request time</dt>
        <dd className={`${styles.technical} ${styles.wrapLongValue}`}>{summary.requestTime}</dd>
      </div>
    </dl>
  );
}
