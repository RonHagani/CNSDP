import type { CharacteristicConcordanceRow } from "@/features/alert-investigation/lib/concordance";
import styles from "./evidence-map.module.css";

/**
 * One characteristic pin on the Detection Definition's characteristic bus
 * (UX spec §8, §12) — satisfied or declared-only. A satisfied row
 * (`row.satisfied`) always carries a real `ProvenanceState` and renders its
 * provenance kind as a visible text label plus a `data-provenance` accent,
 * so Verified/Partial/Unavailable are never distinguished by color alone.
 * A declared-but-unsatisfied row carries no `provenance` (never fabricated,
 * UX spec §8) and instead renders the plain, honest "Declared, not
 * satisfied" label — distinguished from satisfied pins structurally via
 * `data-satisfied`, not by color alone. It remains independently selectable
 * either way (FR-022) — selecting it is `InvestigationMap`'s/
 * `ProvenanceAnnotation`'s job to resolve into the correct annotation.
 *
 * A native, keyboard-operable `<button>` (UX spec §16): reachable by Tab,
 * activated by Enter/Space, `aria-pressed` reflects selection.
 */
export function CharacteristicPin({
  row,
  selected,
  onSelect,
  dimmed = false,
  registerRef,
}: {
  row: CharacteristicConcordanceRow;
  selected: boolean;
  onSelect: (id: string) => void;
  /** True while a *different* characteristic is selected (UX requirement:
   *  keep the selected pin visually dominant while every other pin
   *  recedes). Never true for the selected pin itself. */
  dimmed?: boolean;
  /** Attaches this pin's own DOM node. `DetectionDefinitionFrame` invokes
   *  this for every rendered pin, not just the selected one — the
   *  resulting nodes back two independent overlays: `BusConvergenceOverlay`
   *  measures all of them at once for its permanent pin-to-Result rays,
   *  while `SelectionTraceOverlay` reads back only whichever one is
   *  currently selected to route its row-to-pin trace. */
  registerRef?: (el: HTMLButtonElement | null) => void;
}) {
  return (
    <button
      type="button"
      ref={registerRef}
      className={`${styles.pin} ${dimmed ? styles.dim : ""}`}
      data-satisfied={row.satisfied}
      data-provenance={row.satisfied ? row.provenance.kind : undefined}
      data-selected={selected || undefined}
      aria-pressed={selected}
      onClick={() => onSelect(row.id)}
    >
      <span className={styles.pinId}>{row.id}</span>
      <span className={styles.pinState}>
        {row.satisfied ? provenanceLabel(row.provenance.kind) : "Declared, not satisfied"}
      </span>
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
