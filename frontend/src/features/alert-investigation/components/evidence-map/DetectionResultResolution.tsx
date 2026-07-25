import type { DetectionResultInspection } from "@/features/alert-investigation/lib/artifactInspection";
import type { EvidenceConcordance } from "@/features/alert-investigation/lib/concordance";
import styles from "./evidence-map.module.css";

/**
 * The Detection Result convergence object (UX spec §9): a resolved-match
 * fact plus a literal declared/satisfied tally — never a score,
 * percentage, or color-coded risk level (§9's binding "no invented
 * confidence" rule). Renders zero tally ticks and no fabricated "0 of 0"
 * when no characteristics are declared at all (`showCharacteristicCount`
 * already encodes this — computed by the unmodified `concordance.ts`).
 *
 * Pass 1 scope: a CSS `clip-path` wedge gives this object a distinct
 * shape from every rectangular artifact around it (never a circle, card,
 * badge, or flowchart diamond, per §9) without any absolute positioning —
 * the geometric "convergence" framing itself (wide edge facing the bus,
 * narrow edge feeding the alert) is a later visual-refinement pass.
 */
export function DetectionResultResolution({
  detectionResult,
  concordance,
}: {
  detectionResult: DetectionResultInspection;
  concordance: EvidenceConcordance;
}) {
  if (!detectionResult.available) {
    return (
      <section className={`${styles.artifactShape} ${styles.resultWedge}`} aria-labelledby="result-heading">
        <h2 id="result-heading" className={styles.eyebrow}>
          Detection result
        </h2>
        <p className={styles.statusUnavailable}>Detection result unavailable</p>
      </section>
    );
  }

  return (
    <section className={`${styles.artifactShape} ${styles.resultWedge}`} aria-labelledby="result-heading">
      <h2 id="result-heading" className={styles.eyebrow}>
        Detection result
      </h2>
      <p>Resolved match</p>
      {concordance.showCharacteristicCount && (
        <>
          <div className={styles.resultTally} aria-hidden="true">
            {Array.from({ length: concordance.declaredCharacteristicCount }).map((_, i) => (
              <span
                key={i}
                className={styles.resultTallyTick}
                data-filled={i < concordance.satisfiedCharacteristicCount || undefined}
              />
            ))}
          </div>
          <p className={styles.resultCount}>
            {concordance.satisfiedCharacteristicCount}/{concordance.declaredCharacteristicCount}
          </p>
          <p className={styles.eyebrow}>satisfied</p>
        </>
      )}
    </section>
  );
}
