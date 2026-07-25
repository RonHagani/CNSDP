import type { DetectionDefinitionInspection } from "@/features/alert-investigation/lib/artifactInspection";
import { isCharacteristicRow, type EvidenceConcordance } from "@/features/alert-investigation/lib/concordance";
import { CharacteristicPin } from "./CharacteristicPin";
import styles from "./evidence-map.module.css";

/**
 * The Detection Definition rule frame and characteristic bus (UX spec §8):
 * an open, corner-bracketed instrument boundary containing the
 * definition's reviewable content (name, description, operation clause,
 * outcome clause when declared) and a bus of characteristic pins.
 *
 * Pass 1 scope: the bus renders satisfied characteristics only
 * (`concordance.rows`, unmodified `concordance.ts` output) — see
 * `CharacteristicPin`'s own doc comment for why declared-but-unsatisfied
 * pins are a deferred gap in this pass. Subgroup clustering (UX spec §8,
 * e.g. scenario 2's host-access/privilege grouping) requires the new
 * `lib/characteristicGroups.ts` the implementation plan's Pass 2
 * describes — also not added in this pass; all satisfied characteristics
 * render on one ungrouped bus for now.
 */
export function DetectionDefinitionFrame({
  detectionDefinition,
  concordance,
  selectedConditionKey,
  onSelectCondition,
}: {
  detectionDefinition: DetectionDefinitionInspection;
  concordance: EvidenceConcordance;
  selectedConditionKey: string | null;
  onSelectCondition: (id: string) => void;
}) {
  if (!detectionDefinition.available) {
    return (
      <section className={`${styles.artifactShape} ${styles.ruleFrame}`} aria-labelledby="definition-heading">
        <h2 id="definition-heading" className={styles.eyebrow}>
          Detection definition
        </h2>
        <p className={styles.statusUnavailable}>Detection definition unavailable</p>
      </section>
    );
  }

  const { definition } = detectionDefinition;
  const { operation, requires_outcome: requiresOutcome } = definition.conditions;
  const target = [operation.resource, operation.subresource].filter(Boolean).join("/");
  const characteristicRows = concordance.available ? concordance.rows.filter(isCharacteristicRow) : [];

  return (
    <section className={`${styles.artifactShape} ${styles.ruleFrame}`} aria-labelledby="definition-heading">
      <h2 id="definition-heading" className={styles.eyebrow}>
        Detection definition
      </h2>
      <p>
        <strong>{definition.name}</strong>
      </p>
      <p className={styles.ruleFrameClause}>{definition.description}</p>
      <p className={`${styles.ruleFrameClause} ${styles.technical}`}>
        operation {operation.verb ? `${operation.verb} ` : ""}
        {target || "(any resource)"}
      </p>
      {requiresOutcome && (
        <p className={`${styles.ruleFrameClause} ${styles.technical}`}>
          requires_outcome: {requiresOutcome}
        </p>
      )}

      {characteristicRows.length > 0 && (
        <div className={styles.bus} role="group" aria-label="Satisfied characteristics">
          {characteristicRows.map((row) => (
            <CharacteristicPin
              key={row.id}
              row={row}
              selected={row.id === selectedConditionKey}
              onSelect={onSelectCondition}
            />
          ))}
        </div>
      )}
    </section>
  );
}
