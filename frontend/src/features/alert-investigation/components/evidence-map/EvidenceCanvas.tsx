import type { ReactNode, Ref } from "react";
import styles from "./evidence-map.module.css";

export type SourceNormalizedWireState = "default" | "verified" | "partial" | "broken";

/**
 * The spatial map container (Track B §5; Track C — Dark Evidence Map
 * Visual Fidelity Reset — restores the approved canvas composition). Owns
 * layout only. Each of the six artifacts and the rail is an explicit,
 * named slot prop — not a generic `children` array — so the causal-map's
 * data relationships stay visible in the component's own type signature,
 * not just in CSS class names. Geometry is CSS Grid with named areas,
 * never inline absolute positioning or JS-measured coordinates.
 *
 * Source, Normalized, and Definition sit in the row as three content-height
 * (top-aligned) columns; Result and Alert are real siblings of Definition
 * on the same row, vertically centered — never nested below Definition —
 * so the canvas reads as Source | Normalized | Definition | Result | Alert
 * left to right, matching the approved screenshots, with Result/Alert
 * floating at the row's vertical center rather than forcing a tall empty
 * band beneath a short Definition or a short Source.
 *
 * `overlay` is an optional absolutely-positioned layer — the bus
 * convergence rays (permanent) and the selection trace (selection-driven),
 * both measured SVGs painted above every artifact but below nothing
 * interactive — each is `aria-hidden`/`pointer-events: none` at its own
 * definition site, not here.
 */
export function EvidenceCanvas({
  canvasRef,
  header,
  source,
  validation,
  normalized,
  definition,
  result,
  alert,
  rail,
  brokenCallout,
  overlay,
  dimmed = false,
  sourceNormalizedState = "default",
  resultAlertBroken = false,
}: {
  canvasRef?: Ref<HTMLDivElement>;
  header: ReactNode;
  source: ReactNode;
  validation: ReactNode;
  normalized: ReactNode;
  definition: ReactNode;
  result: ReactNode;
  alert: ReactNode;
  rail: ReactNode;
  brokenCallout?: ReactNode;
  overlay?: ReactNode;
  dimmed?: boolean;
  sourceNormalizedState?: SourceNormalizedWireState;
  /** True only when the response's real `failedLink` is `"alert"` — the
   *  Detection Result -> Generated Alert segment localizes the break on
   *  the canvas itself, matching whichever segment the Traceability Rail
   *  also marks broken. */
  resultAlertBroken?: boolean;
}) {
  return (
    <div className={styles.canvas} ref={canvasRef}>
      <div className={styles.canvasHeader}>{header}</div>

      <div
        className={styles.canvasSource}
        data-testid="source-validation-group"
        data-dim={dimmed || undefined}
      >
        <div className={styles.validationGroup}>
          {validation}
          <div className={styles.wireStub} data-wire="source-validation-stub" aria-hidden="true" />
        </div>
        {source}
      </div>

      <div
        className={`${styles.wireSourceNormalized} ${styles.wireHorizontal}`}
        data-wire="source-normalized"
        data-state={sourceNormalizedState !== "default" ? sourceNormalizedState : undefined}
        aria-hidden="true"
      >
        {sourceNormalizedState === "partial" && (
          <span className={styles.wireGapGlyph} title="Gap — source location not structurally identified">
            ⋯
          </span>
        )}
        {sourceNormalizedState === "broken" && (
          <span className={styles.wireBreakGlyph} title="Broken link">
            ✕
          </span>
        )}
      </div>

      <div className={styles.canvasNormalized}>{normalized}</div>

      <div
        className={`${styles.wireNormalizedDefinition} ${styles.wireHorizontal} ${dimmed ? styles.dim : ""}`}
        data-wire="normalized-definition"
        aria-hidden="true"
      />

      <div className={styles.canvasDefinition}>{definition}</div>

      {/* No `.wireHorizontal` line here — this segment is the one relationship
          the bus convergence overlay (`overlay` prop) draws instead, as a ray
          from every declared pin rather than one flat gutter line (matching
          the approved screenshots, which show the same). The div itself stays,
          purely to reserve the grid gutter's spacing and satisfy the "each
          connector is a real DOM neighbor of its two artifacts" contract. */}
      <div className={styles.wireDefinitionResult} data-wire="definition-result" aria-hidden="true" />

      <div className={styles.canvasResult} data-dim={dimmed || undefined}>
        {result}
      </div>

      <div
        className={`${styles.wireResultAlert} ${styles.wireHorizontal} ${
          dimmed && !resultAlertBroken ? styles.dim : ""
        }`}
        data-wire="result-alert"
        data-state={resultAlertBroken ? "broken" : undefined}
        aria-hidden="true"
      >
        {resultAlertBroken && (
          <span className={styles.wireBreakGlyph} title="Broken link">
            ✕
          </span>
        )}
      </div>

      <div className={styles.canvasAlert} data-dim={dimmed || undefined}>
        {alert}
      </div>

      {brokenCallout && <div className={styles.canvasBrokenCallout}>{brokenCallout}</div>}

      <div className={styles.canvasRail}>{rail}</div>

      {overlay}
    </div>
  );
}
