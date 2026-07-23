import type { EvidenceRegisterEntry as EvidenceRegisterEntryModel } from "@/features/alert-investigation/lib/evidenceRegister";
import styles from "./dossier.module.css";

/**
 * One Evidence Register row (UX spec §3.4). Purely presentational: every
 * field it renders — index, total, label, availability, summary — is
 * already computed by Step 1 (`evidenceRegister.ts`); nothing here
 * recomputes a summary or an availability flag. An unavailable entry
 * remains fully visible and selectable — its own summary already states
 * the limitation (FR-035) in the model's own words, so this component
 * does not add a second, redundant "unavailable" claim beyond what the
 * model says.
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
    <li className={`${styles.ruledRow} ${selected ? styles.selected : ""} ${entry.available ? "" : styles.unavailable}`}>
      <button
        type="button"
        className={styles.plainButton}
        aria-pressed={selected}
        onClick={() => onSelect(entry.key)}
      >
        <div className={styles.accessionRow}>
          <span className={styles.accessionIndex}>
            {String(entry.index).padStart(2, "0")}/{entry.total}
          </span>
          <div className={styles.conditionTier}>
            <p className={styles.prose}>
              <span className={styles.recordLabel}>{entry.label}</span>
            </p>
            <p className={`${styles.proseSecondary} ${styles.wrapLongValue}`}>{entry.summary}</p>
          </div>
        </div>
      </button>
    </li>
  );
}
