import type { ReactNode } from "react";
import styles from "./StatusBadge.module.css";

export type StatusTone = "intact" | "warning" | "broken" | "neutral";

const TONE_GLYPH: Record<StatusTone, string> = {
  intact: "✓",
  warning: "⚠",
  broken: "✕",
  neutral: "–",
};

/**
 * The one place status color is defined. Every status indicator in the
 * product renders through this component so color is never the sole
 * carrier of meaning — a glyph and a text label are structurally required,
 * not optional decoration (product-experience-brief.md §7).
 */
export function StatusBadge({
  tone,
  children,
  glyphOverride,
}: {
  tone: StatusTone;
  children: ReactNode;
  glyphOverride?: string;
}) {
  return (
    <span className={styles.badge} data-tone={tone}>
      <span className={styles.glyph} aria-hidden="true">
        {glyphOverride ?? TONE_GLYPH[tone]}
      </span>
      <span className={styles.label}>{children}</span>
    </span>
  );
}
