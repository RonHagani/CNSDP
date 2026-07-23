import type { NormalizedEventInspection as NormalizedEventInspectionModel } from "@/features/alert-investigation/lib/artifactInspection";
import styles from "../dossier.module.css";

/**
 * Normalized Event Inspection Leaf content. Renders the core fields
 * (subject, operation, target, outcome, request time) plus whichever
 * single scenario-specific block — `exec`, `podCreation`, or
 * `clusterRoleBinding` — the event actually carries. All three are
 * handled generically here; none is assumed to be the only one that can
 * appear, and none is hardcoded as the default.
 */
export function NormalizedEventInspection({
  inspection,
}: {
  inspection: NormalizedEventInspectionModel;
}) {
  if (!inspection.available) {
    return <p className={styles.proseSecondary}>Normalized event is not available for this alert.</p>;
  }

  const event = inspection.event;
  const target = [event.target.resource, event.target.subresource].filter(Boolean).join("/");

  return (
    <div className={styles.stack}>
      <dl className={styles.fieldList}>
        <div className={styles.fieldRow}>
          <dt>Subject</dt>
          <dd className={styles.technical}>{event.subject.username}</dd>
        </div>
        <div className={styles.fieldRow}>
          <dt>Operation</dt>
          <dd className={`${styles.technical} ${styles.wrapLongValue}`}>{event.operation.verb}</dd>
        </div>
        <div className={styles.fieldRow}>
          <dt>Target</dt>
          <dd className={`${styles.technical} ${styles.wrapLongValue}`}>
            {target}
            {event.target.name ? ` / ${event.target.name}` : ""}
            {event.target.namespace ? ` (namespace: ${event.target.namespace})` : ""}
          </dd>
        </div>
        <div className={styles.fieldRow}>
          <dt>Outcome code</dt>
          <dd className={styles.technical}>{event.outcome.code ?? "—"}</dd>
        </div>
        <div className={styles.fieldRow}>
          <dt>Request time</dt>
          <dd className={`${styles.technical} ${styles.wrapLongValue}`}>{event.requestTime}</dd>
        </div>
      </dl>

      {event.exec && (
        <dl className={styles.fieldList} aria-label="Exec characteristics">
          <p className={styles.eyebrow}>exec</p>
          <div className={styles.fieldRow}>
            <dt>stdin</dt>
            <dd className={styles.technical}>{String(event.exec.stdin)}</dd>
          </div>
          <div className={styles.fieldRow}>
            <dt>tty</dt>
            <dd className={styles.technical}>{String(event.exec.tty)}</dd>
          </div>
        </dl>
      )}

      {event.podCreation && (
        <dl className={styles.fieldList} aria-label="Pod creation characteristics">
          <p className={styles.eyebrow}>podCreation</p>
          {(
            [
              ["privileged", event.podCreation.privileged],
              ["hostNetwork", event.podCreation.hostNetwork],
              ["hostPID", event.podCreation.hostPID],
              ["hostIPC", event.podCreation.hostIPC],
              ["hostPathVolume", event.podCreation.hostPathVolume],
            ] as const
          ).map(([label, value]) => (
            <div className={styles.fieldRow} key={label}>
              <dt>{label}</dt>
              <dd className={styles.technical}>{String(value)}</dd>
            </div>
          ))}
        </dl>
      )}

      {event.clusterRoleBinding && (
        <dl className={styles.fieldList} aria-label="ClusterRoleBinding characteristics">
          <p className={styles.eyebrow}>clusterRoleBinding</p>
          <div className={styles.fieldRow}>
            <dt>bindingName</dt>
            <dd className={`${styles.technical} ${styles.wrapLongValue}`}>
              {event.clusterRoleBinding.bindingName}
            </dd>
          </div>
          <div className={styles.fieldRow}>
            <dt>roleRef</dt>
            <dd className={`${styles.technical} ${styles.wrapLongValue}`}>
              {event.clusterRoleBinding.roleRef.kind}/{event.clusterRoleBinding.roleRef.name}
            </dd>
          </div>
          {event.clusterRoleBinding.subjects && event.clusterRoleBinding.subjects.length > 0 && (
            <div className={styles.fieldRow}>
              <dt>subjects</dt>
              <dd className={`${styles.technical} ${styles.wrapLongValue}`}>
                {event.clusterRoleBinding.subjects.map((s) => `${s.kind}:${s.name}`).join(", ")}
              </dd>
            </div>
          )}
        </dl>
      )}
    </div>
  );
}
