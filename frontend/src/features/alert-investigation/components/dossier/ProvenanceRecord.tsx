import type { ProvenanceState } from "@/features/alert-investigation/lib/provenance";
import styles from "./dossier.module.css";

/**
 * Renders the detailed provenance record for one selected matched
 * condition (UX spec §3.5b). Exactly one of the three states is ever
 * shown; nothing here computes or reinterprets which state applies — that
 * decision was already made by `provenance.ts` (Step 1). Provenance state
 * is field-level inspection state, not traceability state — this
 * component never renders anything about `traceability.intact`/
 * `failedLink`.
 */
export function ProvenanceRecord({ provenance }: { provenance: ProvenanceState }) {
  return (
    <div className={styles.record} role="group" aria-label="Provenance record">
      <Field label="Documented condition" value={provenance.condition} />

      {provenance.kind === "verified" && (
        <>
          <Field label="Normalized path" value={provenance.normalizedPath} mono />
          <Field label="Normalized value" value={provenance.normalizedValue} mono />
          <Field label="Transformation or preservation behavior" value={provenance.behavior} />
          <Field label="Raw path" value={provenance.rawPath} mono />
          <Field label="Raw value" value={provenance.rawValue} mono />
        </>
      )}

      {provenance.kind === "partial" && (
        <>
          <Field label="Normalized path" value={provenance.normalizedPath} mono />
          <Field label="Normalized value" value={provenance.normalizedValue} mono />
          <Field label="Source field" value={provenance.limitation} />
        </>
      )}

      {provenance.kind === "unavailable" && (
        <>
          <Field
            label="Unavailable artifact"
            value={provenance.missingArtifact === "source-event" ? "Source event" : "Normalized event"}
          />
          <Field label="Explanation" value={provenance.explanation} />
        </>
      )}
    </div>
  );
}

function Field({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className={styles.recordField}>
      <p className={styles.recordLabel}>{label}</p>
      <p className={mono ? styles.recordValue : `${styles.prose} ${styles.wrapLongValue}`}>{value}</p>
    </div>
  );
}
