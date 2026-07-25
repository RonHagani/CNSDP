import type { ReactNode } from "react";
import styles from "./evidence-map.module.css";

/**
 * The spatial map container (implementation plan §5): owns layout only.
 * Each of the six artifacts and the rail is an explicit, named slot prop —
 * not a generic `children` array — so the causal-map's data relationships
 * stay visible in the component's own type signature, not just in CSS
 * class names (the "Avoid recreating the prototype as hardcoded
 * absolute-positioned HTML" requirement: geometry here is CSS Grid with
 * named areas, never inline absolute positioning).
 *
 * Pass 1 scope: the grid arranges the six artifacts in their real causal
 * groups (source+validation left, normalized event center, definition/
 * result/alert right, rail along the bottom) without literal orthogonal
 * connector-line routing between them — that routing is a later
 * refinement pass (see the migration report).
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
      <div className={styles.canvasSource}>
        {validation}
        {source}
      </div>
      <div className={styles.canvasNormalized}>{normalized}</div>
      <div className={styles.canvasDefinition}>{definition}</div>
      <div className={styles.canvasResult}>{result}</div>
      <div className={styles.canvasAlert}>{alert}</div>
      <div className={styles.canvasRail}>{rail}</div>
    </div>
  );
}
