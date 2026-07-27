import type { CharacteristicConcordanceRow } from "@/features/alert-investigation/lib/concordance";
import type { ProvenanceState } from "@/features/alert-investigation/lib/provenance";
import styles from "./evidence-map.module.css";

/**
 * The anchored provenance annotation for the currently selected
 * characteristic (UX spec §12 step 4, §13). Renders exactly one of the
 * three provenance states `provenance.ts` produces for a satisfied
 * characteristic, or — per this component's own implementation-plan
 * specification (§5's component table: "`ProvenanceState` of the selected
 * characteristic, or the declared-only statement") — the plain
 * declared-only fact for a characteristic that was declared but not
 * satisfied. Nothing here computes or reinterprets which state applies,
 * and no raw path or value is ever fabricated for the Partial/Unavailable/
 * declared-only cases (UX spec §13's binding rule, §8's declared-vs-
 * satisfied rule).
 *
 * Pass 1 scope note (still applicable): rendered as a docked panel beneath
 * the characteristic bus rather than a leader-line-anchored callout
 * positioned at the trace's exact landing point (UX spec §12 step 4's full
 * spatial requirement) — the leader-line anchoring mechanism is deferred
 * to a later refinement pass.
 */
export function ProvenanceAnnotation({
  row,
  side = "left",
}: {
  row: CharacteristicConcordanceRow;
  /** Which side of the characteristic bus the selected pin branches to
   *  (`characteristicBusLayout.ts`'s own left/right assignment) — anchors
   *  this callout near that pin instead of spanning the full frame width. */
  side?: "left" | "right";
}) {
  if (!row.satisfied) {
    return (
      <div
        className={styles.annotation}
        data-kind="declared-only"
        data-side={side}
        role="group"
        aria-label="Provenance record"
      >
        <p className={styles.annotationKind}>Declared, not satisfied</p>
        <p className={`${styles.annotationLine} ${styles.wrapLongValue}`}>{row.description}</p>
        <p className={styles.annotationLine}>
          Declared in this definition; not satisfied by this alert&apos;s recorded event.
        </p>
      </div>
    );
  }

  const { provenance } = row;

  return (
    <div
      className={styles.annotation}
      data-kind={provenance.kind}
      data-side={side}
      role="group"
      aria-label="Provenance record"
    >
      <p className={styles.annotationKind}>{kindLabel(provenance.kind)}</p>
      <p className={`${styles.annotationLine} ${styles.wrapLongValue}`}>{provenance.condition}</p>

      {provenance.kind === "verified" && (
        <>
          <p className={`${styles.annotationLine} ${styles.wrapLongValue}`}>
            Normalized <code className={styles.technical}>{provenance.normalizedPath}</code> ={" "}
            <code className={styles.technical}>{provenance.normalizedValue}</code>. {provenance.behavior}
          </p>
          <p className={`${styles.annotationLine} ${styles.wrapLongValue}`}>
            Raw origin <code className={styles.technical}>{provenance.rawPath}</code>:{" "}
            <code className={styles.technical}>{provenance.rawValue}</code>
          </p>
        </>
      )}

      {provenance.kind === "partial" && (
        <>
          <p className={`${styles.annotationLine} ${styles.wrapLongValue}`}>
            Normalized <code className={styles.technical}>{provenance.normalizedPath}</code> ={" "}
            <code className={styles.technical}>{provenance.normalizedValue}</code>.
          </p>
          <p className={`${styles.annotationLine} ${styles.wrapLongValue}`}>{provenance.limitation}</p>
        </>
      )}

      {provenance.kind === "unavailable" && (
        <p className={`${styles.annotationLine} ${styles.wrapLongValue}`}>
          <span className={styles.technical}>
            {provenance.missingArtifact === "source-event" ? "Source event" : "Normalized event"}
          </span>{" "}
          unavailable — {provenance.explanation}
        </p>
      )}
    </div>
  );
}

function kindLabel(kind: ProvenanceState["kind"]): string {
  switch (kind) {
    case "verified":
      return "Provenance — Verified";
    case "partial":
      return "Provenance — Partial";
    case "unavailable":
      return "Provenance — Unavailable";
  }
}
