import type { ReactNode } from "react";
import type { ConcordanceRow } from "@/features/alert-investigation/lib/concordance";
import type { ProvenanceState } from "@/features/alert-investigation/lib/provenance";
import styles from "./dossier.module.css";

/**
 * One Evidence Concordance row (UX spec §3.3), structured around
 * DOCUMENTED CONDITION → OBSERVED FACT → SOURCE EVIDENCE. A single
 * reusable component handles all four row kinds (operation, outcome,
 * requires_any, requires_all) — the `kind` discriminant already produced
 * by Step 1 decides layout, this component invents no new classification.
 *
 * Only characteristic rows (requires_any / requires_all) carry a
 * `ProvenanceState` and are selectable — the operation and outcome rows
 * are structural facts about what this definition requires, not
 * characteristics with their own documented-condition identity, so they
 * render as plain, non-interactive content (UX spec §3.3 row-level detail
 * 1–2). This mirrors Step 1's `concordance.ts`, which likewise attaches
 * provenance only to characteristic rows.
 */
export function ConcordanceConditionRow({
  row,
  selected,
  onSelect,
}: {
  row: ConcordanceRow;
  selected?: boolean;
  onSelect?: (conditionKey: string) => void;
}) {
  if (row.kind === "operation") {
    const target = [row.resource, row.subresource].filter(Boolean).join("/");
    return (
      <li className={styles.ruledRow}>
        <div className={`${styles.conditionRow} ${styles.conditionRowTwoTier}`}>
          <Tier label="Documented condition">
            <p className={styles.prose}>Operation match</p>
          </Tier>
          <Tier label="Observed fact">
            <p className={`${styles.technical} ${styles.wrapLongValue}`}>
              {row.verb ? `${row.verb} ` : ""}
              {target || "(any resource)"}
            </p>
          </Tier>
        </div>
      </li>
    );
  }

  if (row.kind === "outcome") {
    return (
      <li className={styles.ruledRow}>
        <div className={`${styles.conditionRow} ${styles.conditionRowTwoTier}`}>
          <Tier label="Documented condition">
            <p className={styles.prose}>Required outcome: {row.requiredOutcome}</p>
          </Tier>
          <Tier label="Observed fact">
            <p className={`${styles.technical} ${styles.wrapLongValue}`}>
              {row.recordedOutcomeCode !== undefined ? `code ${row.recordedOutcomeCode}` : "—"}
            </p>
          </Tier>
        </div>
      </li>
    );
  }

  // requires_any / requires_all characteristic row — the only selectable kind.
  const content = (
    <div className={styles.conditionRow}>
      <Tier label="Documented condition">
        <p className={`${styles.technical} ${styles.wrapLongValue}`}>{row.id}</p>
        <p className={styles.proseSecondary}>{row.description}</p>
      </Tier>
      <Tier label="Observed fact">
        <ObservedFact provenance={row.provenance} />
      </Tier>
      <Tier label="Source evidence">
        <SourceEvidenceSummary provenance={row.provenance} />
      </Tier>
    </div>
  );

  return (
    <li className={`${styles.ruledRow} ${selected ? styles.selected : ""}`}>
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

function Tier({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className={styles.conditionTier}>
      <p className={styles.eyebrow}>{label}</p>
      {children}
    </div>
  );
}

function ObservedFact({ provenance }: { provenance: ProvenanceState }) {
  if (provenance.kind === "unavailable") {
    return <p className={styles.proseSecondary}>{provenance.explanation}</p>;
  }
  return (
    <p className={`${styles.technical} ${styles.wrapLongValue}`}>
      {provenance.normalizedPath} = {provenance.normalizedValue}
    </p>
  );
}

/**
 * A concise provenance-state indicator — text only (no icon, checkmark,
 * badge, or colored block), and always the real text Step 1 already
 * computed (`behavior` / `limitation` / `explanation`), never a new
 * interpretation invented at this layer. The full detailed record is
 * `ProvenanceRecord`, reached separately once a row is selected — this is
 * deliberately not that.
 */
function SourceEvidenceSummary({ provenance }: { provenance: ProvenanceState }) {
  switch (provenance.kind) {
    case "verified":
      return <p className={styles.proseSecondary}>Verified — {provenance.behavior}</p>;
    case "partial":
      return <p className={styles.proseSecondary}>Partial — {provenance.limitation}</p>;
    case "unavailable":
      return <p className={styles.proseSecondary}>Unavailable — {provenance.explanation}</p>;
  }
}
