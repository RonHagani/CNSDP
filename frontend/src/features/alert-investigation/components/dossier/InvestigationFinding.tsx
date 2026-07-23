import type { FindingViewModel } from "@/features/alert-investigation/lib/finding";
import styles from "./dossier.module.css";

/**
 * Renders the Finding (UX spec §3.2) — the CLAIM. The detected claim (what
 * matched, and why) is a distinct prose statement from the supporting
 * factual context (subject/operation/target/outcome/time), rendered as a
 * plain definition list beneath it — not a marketing headline, not a
 * decorative pull-quote, no card container. Every value comes straight
 * from `FindingViewModel`, already preserved verbatim by Step 1 from the
 * backend's own `alert.summary`/`matchReason` — nothing here infers
 * severity, intent, confidence, or maliciousness.
 */
export function InvestigationFinding({ finding }: { finding: FindingViewModel }) {
  if (!finding.available) {
    return (
      <section aria-labelledby="finding-heading" className={styles.section}>
        <h2 id="finding-heading" className={styles.heading}>
          Finding
        </h2>
        <p className={styles.proseSecondary}>Alert summary is not available for this alert.</p>
      </section>
    );
  }

  const { subject, operation, target, outcome, requestTime, matchedScenario, matchedDefinitionName } =
    finding;
  const targetLabel = target ? [target.resource, target.subresource].filter(Boolean).join("/") : "—";

  return (
    <section aria-labelledby="finding-heading" className={styles.section}>
      <h2 id="finding-heading" className={styles.heading}>
        Finding
      </h2>

      <p className={styles.prose}>
        {matchedDefinitionName
          ? `Matched detection: ${matchedDefinitionName}${matchedScenario ? ` (${matchedScenario})` : ""}.`
          : "Detection identity unavailable for this alert."}
      </p>

      <dl className={styles.fieldList} aria-label="Supporting factual context">
        <div className={styles.fieldRow}>
          <dt>Subject</dt>
          <dd className={styles.technical}>{subject?.username ?? "—"}</dd>
        </div>
        <div className={styles.fieldRow}>
          <dt>Operation</dt>
          <dd className={`${styles.technical} ${styles.wrapLongValue}`}>{operation?.verb ?? "—"}</dd>
        </div>
        <div className={styles.fieldRow}>
          <dt>Target</dt>
          <dd className={`${styles.technical} ${styles.wrapLongValue}`}>
            {targetLabel}
            {target?.name ? ` / ${target.name}` : ""}
          </dd>
        </div>
        <div className={styles.fieldRow}>
          <dt>Outcome code</dt>
          <dd className={styles.technical}>{outcome?.code ?? "—"}</dd>
        </div>
        <div className={styles.fieldRow}>
          <dt>Request time</dt>
          <dd className={`${styles.technical} ${styles.wrapLongValue}`}>{requestTime ?? "—"}</dd>
        </div>
      </dl>
    </section>
  );
}
