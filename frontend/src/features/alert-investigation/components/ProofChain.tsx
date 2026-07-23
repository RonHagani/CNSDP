import { motion, useReducedMotion } from "framer-motion";
import type { TraceabilityResult } from "@/types/contract";
import styles from "./ProofChain.module.css";

const LINKS = ["Alert", "Detection result", "Normalized event", "Submission"];

const FAILED_LINK_EXPLANATION: Record<string, string> = {
  alert: "The alert itself does not resolve through the chain.",
  source_key: "The submission's re-derived source identity no longer matches its stored record.",
  raw_event_sha256: "The stored raw event no longer matches its recorded integrity digest.",
};

/** Which of the three link gaps breaks for a given real failedLink value. */
function brokenGapIndex(failedLink: string | undefined): number | null {
  if (!failedLink) return null;
  if (failedLink === "alert") return 0;
  return 2; // source_key or raw_event_sha256 — both concern the submission link
}

/**
 * Traceability rendered as a physical proof object — four interlocking
 * chain-links forged from the real four-artifact FK chain
 * (internal/traceability.VerifyAlert: alert → detection_result →
 * normalized_event → submission), not a bordered panel of text. A broken
 * chain shows a visible severed gap with a spark at the exact failed
 * joint, never a generic error banner. Live-recomputed on every
 * retrieval — the caption says so explicitly, so it is never mistaken for
 * a cached flag.
 */
export function ProofChain({ traceability }: { traceability: TraceabilityResult }) {
  const reduceMotion = useReducedMotion();
  const intact = traceability.intact;
  const gap = brokenGapIndex(traceability.failedLink);

  return (
    <section className={styles.proof} id="traceability-verdict" aria-labelledby="proof-chain-title">
      <header className={styles.head}>
        <p className={styles.eyebrow}>Traceability integrity · live-verified</p>
        <h2 className={styles.title} id="proof-chain-title">
          The proof chain
        </h2>
      </header>

      <div className={styles.forge} data-intact={intact}>
        <svg
          viewBox="0 0 640 90"
          className={styles.chainSvg}
          role="img"
          aria-label={
            intact
              ? "Proof chain intact: alert, detection result, normalized event, and submission all verified connected"
              : `Proof chain broken at ${traceability.failedLink}`
          }
        >
          {LINKS.map((label, i) => {
            const cx = 80 + i * 160;
            const brokenHere = gap !== null && (i === gap || i === gap + 1);
            return (
              <g key={label}>
                {i > 0 && (
                  <line
                    x1={cx - 160 + 70}
                    y1="45"
                    x2={cx - 70}
                    y2="45"
                    className={gap === i - 1 ? styles.linkGapBroken : styles.linkGap}
                  />
                )}
                {gap === i - 1 && (
                  <text x={cx - 80} y="30" className={styles.sparkGlyph}>
                    ✕
                  </text>
                )}
                <motion.rect
                  x={cx - 62}
                  y="22"
                  width="124"
                  height="46"
                  rx="23"
                  className={brokenHere ? styles.linkRingBroken : styles.linkRing}
                  initial={reduceMotion ? false : { opacity: 0, scale: 0.85 }}
                  animate={{ opacity: 1, scale: 1 }}
                  transition={{
                    duration: reduceMotion ? 0 : 0.4,
                    delay: reduceMotion ? 0 : i * 0.08,
                  }}
                />
                <rect
                  x={cx - 58}
                  y="26"
                  width="116"
                  height="6"
                  rx="3"
                  className={styles.linkHighlight}
                />
              </g>
            );
          })}
        </svg>
        <div className={styles.labels}>
          {LINKS.map((label) => (
            <span key={label} className={styles.linkLabel}>
              {label}
            </span>
          ))}
        </div>
      </div>

      <p className={styles.verdictCopy}>
        <strong>{intact ? "Traceability verified." : "Traceability broken."}</strong>{" "}
        {intact
          ? "The alert resolves through its detection result and normalized event to the source submission; the re-derived source identity and raw-event digest both match their stored records (NFR-017)."
          : `${traceability.failedLink ? FAILED_LINK_EXPLANATION[traceability.failedLink] : "A required link did not resolve."} The gap is visibly identified, never silently repaired or hidden (FR-035).`}{" "}
        Verified live, on every retrieval — not a cached flag.
      </p>
    </section>
  );
}
