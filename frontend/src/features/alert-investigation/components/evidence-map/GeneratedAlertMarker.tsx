import type { AlertInspection } from "@/features/alert-investigation/lib/artifactInspection";
import styles from "./evidence-map.module.css";

/**
 * The Generated Alert terminal marker (UX spec §10): the map's terminal
 * object, fed by exactly one incoming connection from the Detection
 * Result. Content is `alertId` and a one-line summary composed from
 * `alert.summary` — the same underlying facts the identity header states
 * in full; this object is the literal artifact view of that data, not a
 * re-derivation of it.
 */
export function GeneratedAlertMarker({
  alertId,
  alert,
}: {
  alertId: number;
  alert: AlertInspection;
}) {
  if (!alert.available) {
    return (
      <section className={`${styles.artifactShape} ${styles.alertMarker}`} aria-labelledby="alert-heading">
        <h2 id="alert-heading" className={styles.eyebrow}>
          Generated alert
        </h2>
        <p className={styles.statusUnavailable}>Alert summary unavailable</p>
      </section>
    );
  }

  const { summary } = alert;
  const target = [summary.target.resource, summary.target.subresource].filter(Boolean).join("/");

  return (
    <section className={`${styles.artifactShape} ${styles.alertMarker}`} aria-labelledby="alert-heading">
      <h2 id="alert-heading" className={styles.eyebrow}>
        Generated alert
      </h2>
      <p>
        <span className={styles.alertMarkerPrompt} aria-hidden="true">
          {">"}
        </span>{" "}
        Alert #{alertId}
      </p>
      <p className={`${styles.technical} ${styles.wrapLongValue}`}>
        {summary.subject.username} {summary.operation.verb} {target}
        {summary.target.name ? `/${summary.target.name}` : ""} — outcome {summary.outcome.code ?? "—"}
      </p>
    </section>
  );
}
