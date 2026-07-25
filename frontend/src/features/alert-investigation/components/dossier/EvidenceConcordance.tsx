import {
  isCharacteristicRow,
  type EvidenceConcordance as EvidenceConcordanceModel,
  type OperationConcordanceRow,
  type OutcomeConcordanceRow,
} from "@/features/alert-investigation/lib/concordance";
import { ConcordanceConditionRow } from "./ConcordanceConditionRow";
import styles from "./dossier.module.css";

/**
 * Renders the complete Evidence Concordance (UX spec §3.3), recomposed
 * as an ADMISSION THRESHOLD followed by one EVIDENTIARY CLAUSE per
 * matched characteristic (Composition Reset §3–§4) — never a three-column
 * table, never a repeated label/value block. Every row already comes
 * pre-filtered to satisfied conditions only by Step 1 (`concordance.ts`)
 * — this component renders whatever `rows` it is given, in order, with no
 * assumption about how many characteristics there are (zero, one, two,
 * five, or more) and no scenario-specific branch. Selection is fully
 * controlled: this component owns no state of its own and never derives
 * provenance — each row already carries its own provenance state.
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
      <section
        aria-labelledby="evidence-concordance-heading"
        className={`${styles.section} ${styles.concordanceSection}`}
      >
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

  const operation = concordance.rows.find(
    (row): row is OperationConcordanceRow => row.kind === "operation",
  );
  const outcome = concordance.rows.find((row): row is OutcomeConcordanceRow => row.kind === "outcome");
  const characteristicRows = concordance.rows.filter(isCharacteristicRow);

  return (
    <section
      aria-labelledby="evidence-concordance-heading"
      className={`${styles.section} ${styles.concordanceSection}`}
    >
      <h2 id="evidence-concordance-heading" className={styles.heading}>
        Evidence concordance
      </h2>
      <p className={`${styles.proseSecondary} ${styles.concordanceLede}`}>
        Why the detection matched — the declared conditions this alert satisfied.
      </p>
      {concordance.showCharacteristicCount && (
        <p className={styles.proseSecondary}>
          {concordance.satisfiedCharacteristicCount} of {concordance.declaredCharacteristicCount}{" "}
          declared characteristics satisfied.
        </p>
      )}

      {operation && <AdmissionThreshold operation={operation} outcome={outcome} />}

      <ol className={styles.clauseList} aria-label="Matched documented conditions">
        {characteristicRows.map((row) => (
          <ConcordanceConditionRow
            key={row.id}
            row={row}
            selected={row.id === selectedConditionKey}
            onSelect={onSelectCondition}
          />
        ))}
      </ol>
    </section>
  );
}

/**
 * The prerequisite threshold (Composition Reset §4): operation and
 * required outcome are admission requirements, not causal evidence — one
 * quiet, unaccessioned statement assembled only from what the current
 * detection definition actually declares. `outcome` is `undefined`
 * whenever no `requires_outcome` is declared (scenario 1); this component
 * never invents an outcome requirement to fill that gap.
 */
function AdmissionThreshold({
  operation,
  outcome,
}: {
  operation: OperationConcordanceRow;
  outcome?: OutcomeConcordanceRow;
}) {
  const target = [operation.resource, operation.subresource].filter(Boolean).join("/");
  return (
    <div className={styles.admissionThreshold}>
      <p className={`${styles.prose} ${styles.thresholdLine}`}>
        <span className={styles.eyebrow}>Admission</span>{" "}
        <span className={`${styles.technical} ${styles.wrapLongValue}`}>
          operation {operation.verb ? `${operation.verb} ` : ""}
          {target || "(any resource)"}
          {outcome && (
            <>
              {" — Required outcome: "}
              {outcome.requiredOutcome}
              {outcome.recordedOutcomeCode !== undefined &&
                ` (observed code ${outcome.recordedOutcomeCode})`}
            </>
          )}
        </span>
      </p>
      <div className={styles.admissionRule} aria-hidden="true" />
    </div>
  );
}
