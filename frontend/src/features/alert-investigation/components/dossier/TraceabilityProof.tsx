import type { TraceabilityProofModel } from "@/features/alert-investigation/lib/traceability";
import styles from "./dossier.module.css";

const CHAIN_LABELS: Record<string, string> = {
  alert: "Alert",
  "detection-result": "Detection result",
  "normalized-event": "Normalized event",
  submission: "Submission",
};

/**
 * Renders the Traceability Proof (UX spec §3.6, §7) as a plain,
 * declarative verification statement — no graphic nodes, chain pills,
 * glow, seals, or icons. Traceability is not one of the six evidence
 * artifacts and is never rendered inside the Evidence Register. Nothing
 * here interprets a matched condition's provenance state — this
 * component only ever renders `TraceabilityProofModel`, which is
 * independent of provenance by construction (Step 1).
 */
export function TraceabilityProof({ traceability }: { traceability: TraceabilityProofModel }) {
  return (
    <section aria-labelledby="traceability-proof-heading" role="status" className={styles.section}>
      <h2 id="traceability-proof-heading" className={styles.heading}>
        Traceability proof
      </h2>

      <p className={styles.prose}>
        <strong>{traceability.intact ? "Traceability verified." : "Traceability broken."}</strong>{" "}
        {traceability.explanation}
      </p>

      {!traceability.intact && traceability.failedLink && (
        <p className={styles.proseSecondary}>Failed link: {traceability.failedLink}</p>
      )}

      <div>
        <p className={styles.eyebrow}>Chain</p>
        <ol className={styles.fieldList} aria-label="Traceability chain, in resolution order">
          {traceability.chain.map((link) => (
            <li key={link} className={styles.technical}>
              {CHAIN_LABELS[link] ?? link}
            </li>
          ))}
        </ol>
      </div>
    </section>
  );
}
