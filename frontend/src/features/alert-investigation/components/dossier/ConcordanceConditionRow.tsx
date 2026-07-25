import type { SatisfiedCharacteristicRow } from "@/features/alert-investigation/lib/concordance";
import type { ProvenanceState } from "@/features/alert-investigation/lib/provenance";
import { ProvenanceRecord } from "./ProvenanceRecord";
import styles from "./dossier.module.css";

/**
 * One EVIDENTIARY CLAUSE (Composition Reset §3): a matched characteristic
 * read as a single causal statement — DECLARED CHARACTERISTIC satisfied
 * by OBSERVED FACT, supported or limited by SOURCE PROVENANCE — rather
 * than a repeated label/value/description/label/value/source-statement
 * block. The clause's own grammar ("`id` was satisfied by `path` =
 * `value`.") carries the declared/observed relationship; nothing here
 * repeats "Documented condition" / "Observed fact" / "Source evidence"
 * captions per row.
 *
 * The accession mark (`c01`, `c02`, …) is a CSS counter on the enclosing
 * list, not a rendered prop — this component never invents its own index.
 * Selecting the clause expands its `ProvenanceRecord` directly inside the
 * clause (Composition Reset §5), not in an unrelated details region
 * elsewhere on the page.
 */
export function ConcordanceConditionRow({
  row,
  selected,
  onSelect,
}: {
  // `EvidenceConcordance` (the caller) narrows to `SatisfiedCharacteristicRow`
  // via `isSatisfiedCharacteristicRow` before rendering this component —
  // `concordance.ts` itself now retains declared-but-unsatisfied rows too
  // (UX spec §8), but this retired presentation was never extended to
  // render that state. Typing this prop as the satisfied-only member (not
  // the full `CharacteristicConcordanceRow` union) means `row.provenance`
  // is guaranteed present by the type system itself — no cast or non-null
  // assertion needed anywhere in this component.
  row: SatisfiedCharacteristicRow;
  selected?: boolean;
  onSelect?: (conditionKey: string) => void;
}) {
  const { provenance } = row;

  const content = (
    <div className={styles.folioSplit}>
      <div>
        <p className={`${styles.prose} ${styles.clauseSentence}`}>
          <span className={`${styles.technical} ${styles.characteristicId} ${styles.wrapLongValue}`}>
            {row.id}
          </span>{" "}
          was satisfied by <ObservedFactInline provenance={provenance} />
        </p>
        <p className={`${styles.proseSecondary} ${styles.clauseDescription}`}>{row.description}</p>

        {selected && (
          <div className={styles.clauseExpansion}>
            <ProvenanceRecord provenance={provenance} />
          </div>
        )}
      </div>

      <div>
        <SourceEvidenceSummary provenance={provenance} />
      </div>
    </div>
  );

  return (
    <li
      className={`${styles.evidentiaryClause} ${selected ? styles.selected : ""}`}
      data-kind={row.kind}
    >
      {onSelect ? (
        <button
          type="button"
          className={styles.plainButton}
          aria-pressed={Boolean(selected)}
          onClick={() => onSelect(row.id)}
        >
          {content}
        </button>
      ) : (
        content
      )}
    </li>
  );
}

function ObservedFactInline({ provenance }: { provenance: ProvenanceState }) {
  if (provenance.kind === "unavailable") {
    return <span className={styles.proseSecondary}>{provenance.explanation}</span>;
  }
  return (
    <>
      <code className={styles.recordValue}>{provenance.normalizedPath}</code>
      {" = "}
      <code className={styles.recordValue}>{provenance.normalizedValue}</code>.
    </>
  );
}

/**
 * A concise provenance-state indicator — text only (no icon, checkmark,
 * badge, or colored block), and always the real text Step 1 already
 * computed (`behavior` / `limitation` / `explanation`), never a new
 * interpretation invented at this layer. The full detailed record is the
 * inline `ProvenanceRecord` expansion above, reached separately once the
 * clause is selected — this is deliberately not that.
 *
 * Partial provenance's limitation sentence is identical across every
 * characteristic that shares it (scenario 2's five clauses all cite the
 * same `requestObject`-typing gap) — quieted a shade further than
 * Verified/Unavailable so a repeated caveat doesn't compete for attention
 * with the one real, varying fact per clause. This is a weight change
 * within the same neutral text scale, not a new semantic color —
 * Verified/Partial/Unavailable remain distinguished by wording, never by
 * hue.
 */
function SourceEvidenceSummary({ provenance }: { provenance: ProvenanceState }) {
  switch (provenance.kind) {
    case "verified":
      return <p className={styles.sourceEvidence}>Verified — {provenance.behavior}</p>;
    case "partial":
      return (
        <p className={`${styles.sourceEvidence} ${styles.sourceEvidenceQuiet}`}>
          Partial — {provenance.limitation}
        </p>
      );
    case "unavailable":
      return <p className={styles.sourceEvidence}>Unavailable — {provenance.explanation}</p>;
  }
}
