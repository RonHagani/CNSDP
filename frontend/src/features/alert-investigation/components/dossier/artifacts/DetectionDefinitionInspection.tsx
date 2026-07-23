import type { DetectionDefinitionInspection as DetectionDefinitionInspectionModel } from "@/features/alert-investigation/lib/artifactInspection";
import type { DeclaredConditionGroup } from "@/features/alert-investigation/lib/detectionConditions";
import styles from "../dossier.module.css";

/**
 * Detection Definition Inspection Leaf content: the full literal
 * definition, including any declared-but-unsatisfied `requires_any`
 * option (UX spec §6 item 4 — this is the only place an unsatisfied
 * declared characteristic is inspectable; it is definition data here,
 * never rendered as matched evidence). Declared and satisfied state stay
 * visually distinguishable per characteristic — by label text only, never
 * an icon, checkmark, badge, or color block.
 */
export function DetectionDefinitionInspection({
  inspection,
}: {
  inspection: DetectionDefinitionInspectionModel;
}) {
  if (!inspection.available) {
    return <p className={styles.proseSecondary}>Detection definition is not available for this alert.</p>;
  }

  const { definition, revision, declaredConditions } = inspection;
  const operationTarget = [
    declaredConditions?.operation.resource ?? definition.conditions.operation.resource,
    declaredConditions?.operation.subresource ?? definition.conditions.operation.subresource,
  ]
    .filter(Boolean)
    .join("/");
  const operationVerb = declaredConditions?.operation.verb ?? definition.conditions.operation.verb;

  return (
    <div className={styles.stack}>
      <div>
        <p className={styles.heading}>{definition.name}</p>
        <p className={styles.prose}>{definition.description}</p>
      </div>

      <dl className={styles.fieldList}>
        <div className={styles.fieldRow}>
          <dt>Revision (pinned)</dt>
          <dd className={`${styles.technical} ${styles.wrapLongValue}`}>{revision}</dd>
        </div>
        <div className={styles.fieldRow}>
          <dt>Operation</dt>
          <dd className={`${styles.technical} ${styles.wrapLongValue}`}>
            {operationVerb ? `${operationVerb} ` : ""}
            {operationTarget || "(any resource)"}
          </dd>
        </div>
        {declaredConditions?.outcome && (
          <div className={styles.fieldRow}>
            <dt>Required outcome</dt>
            <dd className={styles.technical}>{declaredConditions.outcome.requiredOutcome}</dd>
          </div>
        )}
      </dl>

      {declaredConditions?.requiresAny && (
        <CharacteristicGroup title="requires_any (at least one)" group={declaredConditions.requiresAny} />
      )}
      {declaredConditions?.requiresAll && (
        <CharacteristicGroup title="requires_all (every one)" group={declaredConditions.requiresAll} />
      )}
    </div>
  );
}

function CharacteristicGroup({ title, group }: { title: string; group: DeclaredConditionGroup }) {
  return (
    <div>
      <p className={styles.eyebrow}>{title}</p>
      <ul className={styles.ruledList}>
        {group.characteristics.map((c) => (
          <li key={c.id} className={styles.ruledRow}>
            <p className={`${styles.technical} ${styles.wrapLongValue}`}>{c.id}</p>
            <p className={styles.proseSecondary}>{c.description}</p>
            <p className={styles.proseSecondary}>{c.satisfied ? "Satisfied" : "Not satisfied"}</p>
          </li>
        ))}
      </ul>
    </div>
  );
}
