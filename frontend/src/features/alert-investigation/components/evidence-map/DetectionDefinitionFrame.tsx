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
 * Deliberately renders `.ruleFrame` alone, never `.artifactShape` (the
 * shared background+border treatment every other artifact uses): the
 * approved prototype (`dark-evidence-map/prototype.css`'s `.ruleframe`)
 * and UX spec §8 ("never a filled card") both show this frame with no
 * fill and no border — only the two corner brackets. Applying
 * `.artifactShape` here (corrected by this pass) had rendered it as a
 * solid bordered box, contradicting both.
 *
 * The bus renders every declared characteristic row `concordance.ts`
 * produces — satisfied and declared-but-unsatisfied alike (UX spec §8;
 * `CharacteristicPin` renders each row's own state). Each row also carries
 * a `group` label (`lib/characteristicGroups.ts`) for the later visual
 * clustering pass (UX spec §8's subgroup brackets, e.g. scenario 2's
 * host-access/privilege split); this pass exposes that data but does not
 * yet render a visual cluster boundary — all rows render on one bus,
 * ordered as `concordance.ts` returns them.
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
      <section className={styles.ruleFrame} aria-labelledby="definition-heading">
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
    <section className={styles.ruleFrame} aria-labelledby="definition-heading">
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
        <div className={styles.bus} role="group" aria-label="Declared characteristics">
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
