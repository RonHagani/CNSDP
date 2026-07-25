import type { TraceabilityProofModel } from "@/features/alert-investigation/lib/traceability";
import styles from "./dossier.module.css";

const CHAIN_LABELS: Record<string, string> = {
  alert: "Alert",
  "detection-result": "Detection result",
  "normalized-event": "Normalized event",
  submission: "Submission",
};

/**
 * The CUSTODY CLOSURE (Composition Reset §8): the case folio's physical
 * terminus, not a small footer beneath an unrelated short line. Spans the
 * evidence field (verification statement, ordered custody references) and
 * the proof margin (a compact restatement of the same traceability state
 * the case opening already declared — `.rustMark` is the identical class
 * used there, so "the same mark at both locations" is structural, not
 * merely similar-looking), then physically ends the sheet with one
 * terminal rule. No graphic nodes, chain pills, glow, seals, or icons —
 * the ordered references are compact citations, not a decorative chain.
 * Nothing here interprets a matched condition's provenance state — this
 * component only ever renders `TraceabilityProofModel`, independent of
 * provenance by construction (Step 1).
 */
export function TraceabilityProof({ traceability }: { traceability: TraceabilityProofModel }) {
  return (
    <section
      aria-labelledby="traceability-proof-heading"
      role="status"
      className={`${styles.section} ${styles.traceabilitySection} ${
        traceability.intact ? "" : styles.traceabilityBroken
      }`}
    >
      <h2 id="traceability-proof-heading" className={styles.heading}>
        Traceability proof
      </h2>

      <div className={styles.custodySplit}>
        <div>
          <p className={`${styles.prose} ${styles.traceabilityVerdict}`}>
            <strong>{traceability.intact ? "Traceability verified." : "Traceability broken."}</strong>{" "}
            {traceability.explanation}
          </p>

          {!traceability.intact && traceability.failedLink && (
            <p className={`${styles.proseSecondary} ${styles.traceabilityFailure}`}>
              Failed link: {traceability.failedLink}
            </p>
          )}

          <ol
            className={styles.custodyChainList}
            aria-label="Traceability chain, in resolution order"
          >
            {traceability.chain.map((link) => (
              <li key={link} className={styles.technical}>
                {CHAIN_LABELS[link] ?? link}
              </li>
            ))}
          </ol>
        </div>

        <div className={styles.custodyMargin}>
          <p className={styles.eyebrow}>Custody</p>
          <p className={`${styles.technical} ${traceability.intact ? "" : styles.rustMark}`}>
            {traceability.intact ? "Intact" : "Broken"}
          </p>
        </div>
      </div>

      <div className={styles.custodyTerminal} aria-hidden="true" />
    </section>
  );
}
