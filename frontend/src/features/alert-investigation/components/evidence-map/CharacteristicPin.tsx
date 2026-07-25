import type { CharacteristicConcordanceRow } from "@/features/alert-investigation/lib/concordance";
import styles from "./evidence-map.module.css";

/**
 * One satisfied-characteristic pin on the Detection Definition's
 * characteristic bus (UX spec §8, §12). Pass 1 scope: only satisfied
 * characteristics are rendered as pins — `row` always carries a real
 * `ProvenanceState` (Step 1's `concordance.ts` only produces rows for
 * satisfied characteristics today). Declared-but-unsatisfied pins are a
 * known, deferred gap (see the migration report) pending the domain-layer
 * change to `concordance.ts` the implementation plan's Pass 2 describes —
 * not made in this pass.
 *
 * A native, keyboard-operable `<button>` (UX spec §16): reachable by Tab,
 * activated by Enter/Space, `aria-pressed` reflects selection. Provenance
 * state is carried by a text label (`pinState`) in addition to the
 * `data-provenance` accent, so Verified/Partial/Unavailable are never
 * distinguished by color alone.
 */
export function CharacteristicPin({
  row,
  selected,
  onSelect,
}: {
  row: CharacteristicConcordanceRow;
  selected: boolean;
  onSelect: (id: string) => void;
}) {
  return (
    <button
      type="button"
      className={styles.pin}
      data-provenance={row.provenance.kind}
      data-selected={selected || undefined}
      aria-pressed={selected}
      onClick={() => onSelect(row.id)}
    >
      <span className={`${styles.pinId} ${styles.wrapLongValue}`}>{row.id}</span>
      <span className={styles.pinState}>{provenanceLabel(row.provenance.kind)}</span>
    </button>
  );
}

function provenanceLabel(kind: "verified" | "partial" | "unavailable"): string {
  switch (kind) {
    case "verified":
      return "Verified";
    case "partial":
      return "Partial";
    case "unavailable":
      return "Unavailable";
  }
}
