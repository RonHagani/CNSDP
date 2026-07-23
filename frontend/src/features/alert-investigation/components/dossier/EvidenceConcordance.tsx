import type { EvidenceConcordance as EvidenceConcordanceModel } from "@/features/alert-investigation/lib/concordance";
import { ConcordanceConditionRow } from "./ConcordanceConditionRow";
import styles from "./dossier.module.css";

/**
 * Renders the complete Evidence Concordance (UX spec §3.3): the CLAIM's
 * DOCUMENTED CONDITION → OBSERVED FACT → SOURCE EVIDENCE record. Every
 * row already comes pre-filtered to satisfied conditions only by Step 1
 * (`concordance.ts`) — this component renders whatever `rows` it is
 * given, in order, with no assumption about how many there are (zero,
 * one, two, five, or more) and no scenario-specific branch. Selection is
 * fully controlled: this component owns no state of its own and never
 * derives provenance — each row already carries its own provenance state.
 */
export function EvidenceConcordance({
  concordance,
  selectedConditionKey,
  onSelectCondition,
}: {
  concordance: EvidenceConcordanceModel;
  selectedConditionKey?: string | null;
  onSelectCondition: (conditionKey: string) => void;
}) {
  if (!concordance.available) {
    return (
      <section aria-labelledby="evidence-concordance-heading" className={styles.section}>
        <h2 id="evidence-concordance-heading" className={styles.heading}>
          Evidence concordance
        </h2>
        <p className={styles.proseSecondary}>
          The detection definition or detection result is unavailable, so the matched-condition
          concordance cannot be shown.
        </p>
      </section>
    );
  }

  return (
    <section aria-labelledby="evidence-concordance-heading" className={styles.section}>
      <h2 id="evidence-concordance-heading" className={styles.heading}>
        Evidence concordance
      </h2>
      {concordance.showCharacteristicCount && (
        <p className={styles.proseSecondary}>
          {concordance.satisfiedCharacteristicCount} of {concordance.declaredCharacteristicCount}{" "}
          declared characteristics satisfied.
        </p>
      )}
      <ol className={styles.ruledList} aria-label="Matched documented conditions">
        {concordance.rows.map((row) => {
          const key = row.kind === "requires_any" || row.kind === "requires_all" ? row.id : row.kind;
          const selectable = row.kind === "requires_any" || row.kind === "requires_all";
          return (
            <ConcordanceConditionRow
              key={key}
              row={row}
              selected={selectable && row.id === selectedConditionKey}
              onSelect={selectable ? onSelectCondition : undefined}
            />
          );
        })}
      </ol>
    </section>
  );
}
