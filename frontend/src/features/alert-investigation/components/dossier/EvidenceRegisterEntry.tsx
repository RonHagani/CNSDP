import type { EvidenceRegisterEntry as EvidenceRegisterEntryModel } from "@/features/alert-investigation/lib/evidenceRegister";
import styles from "./dossier.module.css";

/**
 * One Evidence Register row (Composition Reset §6): a single continuous
 * index line — accession, name, and summary read together as one entry in
 * the folio index — not a two-line label-then-summary block. Purely
 * presentational: every field it renders is already computed by Step 1
 * (`evidenceRegister.ts`); nothing here recomputes a summary or an
 * availability flag. An unavailable entry remains fully visible and
 * selectable — its own summary already states the limitation (FR-035) in
 * the model's own words.
 */
export function EvidenceRegisterEntry({
  entry,
  selected,
  onSelect,
}: {
  entry: EvidenceRegisterEntryModel;
  selected: boolean;
  onSelect: (key: EvidenceRegisterEntryModel["key"]) => void;
}) {
  return (
    <li
      className={`${styles.ruledRow} ${selected ? styles.selected : ""} ${
        entry.available ? "" : styles.unavailable
      }`}
    >
      <button
        type="button"
        className={styles.plainButton}
        aria-pressed={selected}
        onClick={() => onSelect(entry.key)}
      >
        <div className={styles.folioRow}>
          <span className={styles.accessionIndex}>
            {String(entry.index).padStart(2, "0")}/{entry.total}
          </span>
          <p className={`${styles.prose} ${styles.registerLine} ${styles.wrapLongValue}`}>
            <span className={styles.registerLabel}>{entry.label}</span>
            <span className={styles.proseSecondary}> — {entry.summary}</span>
          </p>
        </div>
      </button>
    </li>
  );
}
