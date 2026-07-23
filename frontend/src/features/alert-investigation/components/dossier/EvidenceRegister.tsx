import type { EvidenceRegisterEntry as EvidenceRegisterEntryModel } from "@/features/alert-investigation/lib/evidenceRegister";
import { EvidenceRegisterEntry } from "./EvidenceRegisterEntry";
import type { ArtifactKey } from "./types";
import styles from "./dossier.module.css";

/**
 * Renders the fixed six-artifact ledger (UX spec §3.4) in the exact
 * canonical order supplied. Traceability is deliberately not one of the
 * six — there is no seventh row here, and this component has no code
 * path that could add one. Fully controlled: `entries` and
 * `selectedArtifact` come from the caller, and selection is reported via
 * `onSelectArtifact`, never owned locally.
 */
export function EvidenceRegister({
  entries,
  selectedArtifact,
  onSelectArtifact,
}: {
  entries: EvidenceRegisterEntryModel[];
  selectedArtifact: ArtifactKey;
  onSelectArtifact: (key: ArtifactKey) => void;
}) {
  return (
    <section aria-labelledby="evidence-register-heading" className={styles.section}>
      <h2 id="evidence-register-heading" className={styles.heading}>
        Evidence register
      </h2>
      <ol className={styles.ruledList} aria-label="The six minimum evidence-set artifacts">
        {entries.map((entry) => (
          <EvidenceRegisterEntry
            key={entry.key}
            entry={entry}
            selected={entry.key === selectedArtifact}
            onSelect={onSelectArtifact}
          />
        ))}
      </ol>
    </section>
  );
}
