import type { ReactNode } from "react";
import styles from "./evidence-map.module.css";

/**
 * The spatial map container (implementation plan §5; connector routing and
 * density added by Pass 7 Category A): owns layout only. Each of the six
 * artifacts and the rail is an explicit, named slot prop — not a generic
 * `children` array — so the causal-map's data relationships stay visible
 * in the component's own type signature, not just in CSS class names (the
 * "Avoid recreating the prototype as hardcoded absolute-positioned HTML"
 * requirement: geometry here is CSS Grid with named areas, never inline
 * absolute positioning or JS-measured coordinates).
 *
 * Renders the five artifact-level connector relationships UX spec §3/§18
 * require — Source Submission <-> Validation Outcome (a short stub, §6),
 * Source Submission -> Normalized Event, Normalized Event -> Detection
 * Definition, Detection Definition -> Detection Result, Detection Result
 * -> Generated Alert — as dedicated, non-overlapping cells (see
 * evidence-map.module.css's "Layout shell"). Each connector is purely
 * decorative (`aria-hidden`, no interactive role): it carries no
 * selection, provenance, or traceability state. The Traceability Rail
 * remains the sole, independent mechanism for traceability state (UX spec
 * §14) — unchanged here. A future selected-row-to-pin trace (UX spec §12
 * step 2) is a selection-triggered interaction overlay, not a sixth
 * connector relationship, and is not implemented by this component.
 *
 * Definition, its connector to Result, and the Result/Alert row are
 * nested together as one flex column (`canvasRightStack`) rather than
 * three independent grid cells sharing Source's row-track: Source's own
 * content (an unbounded raw-payload specimen) can be far taller than
 * Definition/Result/Alert combined, and letting that height force Result/
 * Alert's row to grow was exactly the "excessive empty band" the
 * implementation plan's Pass 7 finding identified. Nesting them means
 * their spacing — including the Definition -> Result connector — stays
 * relative to their own real height, never to Source's.
 *
 * The Source -> Normalized connector's vertical anchor accounts for the
 * Validation stamp/stub sitting above Source's own specimen (see
 * evidence-map.module.css's `.wireSourceNormalized.wireHorizontal`) —
 * without it, the connector would land beside Validation instead of
 * Source, since Normalized has no equivalent leading block above it.
 * `data-testid="source-validation-group"` on Source's own wrapper below
 * exists so a test can assert Validation stays nested inside it, not
 * promoted to an independent top-level sibling.
 *
 * Characteristic-bus branching, selection dimming, anchored provenance
 * annotation, and raw-substring marking (implementation plan §9 Pass 7
 * categories B-D) are not yet implemented — out of scope for this slice.
 */
export function EvidenceCanvas({
  header,
  source,
  validation,
  normalized,
  definition,
  result,
  alert,
  rail,
}: {
  header: ReactNode;
  source: ReactNode;
  validation: ReactNode;
  normalized: ReactNode;
  definition: ReactNode;
  result: ReactNode;
  alert: ReactNode;
  rail: ReactNode;
}) {
  return (
    <div className={styles.canvas}>
      <div className={styles.canvasHeader}>{header}</div>

      <div className={styles.canvasSource} data-testid="source-validation-group">
        <div className={styles.validationGroup}>
          {validation}
          <div className={styles.wireStub} data-wire="source-validation-stub" aria-hidden="true" />
        </div>
        {source}
      </div>

      <div
        className={`${styles.wireSourceNormalized} ${styles.wireHorizontal}`}
        data-wire="source-normalized"
        aria-hidden="true"
      />

      <div className={styles.canvasNormalized}>{normalized}</div>

      <div
        className={`${styles.wireNormalizedDefinition} ${styles.wireHorizontal}`}
        data-wire="normalized-definition"
        aria-hidden="true"
      />

      <div className={styles.canvasRightStack}>
        <div className={styles.canvasDefinition}>{definition}</div>

        <div className={styles.wireDefinitionResult} data-wire="definition-result" aria-hidden="true" />

        <div className={styles.resultAlertRow}>
          <div className={styles.canvasResult}>{result}</div>

          <div className={styles.wireResultAlert} data-wire="result-alert" aria-hidden="true" />

          <div className={styles.canvasAlert}>{alert}</div>
        </div>
      </div>

      <div className={styles.canvasRail}>{rail}</div>
    </div>
  );
}
