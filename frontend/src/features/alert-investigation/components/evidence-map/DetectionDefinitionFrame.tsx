import { Fragment } from "react";
import type { DetectionDefinitionInspection } from "@/features/alert-investigation/lib/artifactInspection";
import { isCharacteristicRow, type EvidenceConcordance } from "@/features/alert-investigation/lib/concordance";
import { buildBusLayout } from "./characteristicBusLayout";
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
 * `CharacteristicPin` renders each row's own state) — as pins branching
 * left/right off a shared central spine (`characteristicBusLayout.ts`'s
 * `buildBusLayout`, Pass 7 Category B), never a stacked list. Each pin
 * alternates side by its position in the declared order (a plain array
 * index, not JS-measured geometry); subgroup boundaries (UX spec §8's
 * "Subgroup clustering," e.g. scenario 2's host-access/privilege split)
 * are rendered as a full-width label wherever the row's existing `group`
 * field (`lib/characteristicGroups.ts`) changes between adjacent rows —
 * consumed as-is, never inferred from scenario/fixture identity or
 * description text. A response where every row shares one group (today:
 * scenarios 1 and 3) renders with no label at all, matching §8's "None,
 * below the two-clustering-cases threshold." `CharacteristicPin` itself is
 * unmodified by this pass — only its position on the bus changed.
 */
export function DetectionDefinitionFrame({
  detectionDefinition,
  concordance,
  selectedConditionKey,
  onSelectCondition,
  registerPinRef,
}: {
  detectionDefinition: DetectionDefinitionInspection;
  concordance: EvidenceConcordance;
  selectedConditionKey: string | null;
  onSelectCondition: (id: string) => void;
  /** Attaches every rendered pin's own DOM node, keyed by its row id — the
   *  selection trace overlay reads back the currently-selected one, and
   *  the bus convergence overlay measures every pin at once to route its
   *  Definition -> Result rays. */
  registerPinRef?: (id: string, el: HTMLButtonElement | null) => void;
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
  const hasSelection = selectedConditionKey !== null;

  return (
    <section className={styles.ruleFrame} aria-labelledby="definition-heading">
      <span className={styles.ruleFrameCornerTr} aria-hidden="true" />
      <span className={styles.ruleFrameCornerBl} aria-hidden="true" />

      <div className={`${styles.ruleFrameHeading} ${hasSelection ? styles.dim : ""}`}>
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
      </div>

      {characteristicRows.length > 0 && (
        <div className={styles.bus} role="group" aria-label="Declared characteristics">
          <div className={styles.busSpine} aria-hidden="true" />
          {buildBusLayout(characteristicRows).map((item) =>
            item.kind === "label" ? (
              <p key={item.key} className={styles.busGroupLabel}>
                {item.group}
              </p>
            ) : (
              <Fragment key={item.key}>
                <div className={item.side === "left" ? styles.pinSlotLeft : styles.pinSlotRight}>
                  <CharacteristicPin
                    row={item.row}
                    selected={item.row.id === selectedConditionKey}
                    onSelect={onSelectCondition}
                    dimmed={hasSelection && item.row.id !== selectedConditionKey}
                    registerRef={(el) => registerPinRef?.(item.row.id, el)}
                  />
                </div>
                <div
                  className={item.side === "left" ? styles.pinSlotFillerRight : styles.pinSlotFillerLeft}
                />
              </Fragment>
            ),
          )}
        </div>
      )}
    </section>
  );
}
