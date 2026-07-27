import type { RefObject } from "react";
import { useConnectorPath } from "./useConnectorPath";
import styles from "./evidence-map.module.css";

/**
 * Draws the one selection-driven connector the Dark Evidence Map canvas
 * needs: the routed line from the highlighted Normalized Event row to the
 * selected characteristic pin (UX requirement: "render the row-to-pin
 * provenance trace as an actual visible routed line"). Verified renders
 * solid teal; Partial renders dashed amber — the same two provenance
 * colors used everywhere else on the canvas, never a third invented one.
 *
 * Purely decorative (`aria-hidden`, `pointer-events: none`); renders
 * nothing when either endpoint isn't currently mounted (no selection, or a
 * declared-only/Unavailable selection that carries no normalized field to
 * point at).
 */
export function SelectionTraceOverlay({
  containerRef,
  rowRef,
  pinRef,
  kind,
  selectionKey,
}: {
  containerRef: RefObject<HTMLElement | null>;
  rowRef: RefObject<HTMLElement | null>;
  pinRef: RefObject<HTMLElement | null>;
  kind: "verified" | "partial" | null;
  /** The selected characteristic's own id — the real re-measurement
   *  trigger. `kind` alone can't tell two different selections of the same
   *  kind apart (e.g. switching between two Verified pins), which would
   *  leave a stale line pointing at the previous pin. */
  selectionKey: string | null;
}) {
  const path = useConnectorPath(containerRef, rowRef, pinRef, [selectionKey]);

  if (!path || !kind) return null;

  const midX = (path.from.x + path.to.x) / 2;
  const d = `M ${path.from.x} ${path.from.y} L ${midX} ${path.from.y} L ${midX} ${path.to.y} L ${path.to.x} ${path.to.y}`;

  return (
    <svg
      className={styles.traceOverlay}
      width={path.width}
      height={path.height}
      viewBox={`0 0 ${path.width} ${path.height}`}
      aria-hidden="true"
    >
      <path className={styles.traceLine} data-kind={kind} d={d} />
    </svg>
  );
}
