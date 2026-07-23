import type { EvidenceRegisterEntry } from "@/features/alert-investigation/lib/evidenceRegister";
import type { ProvenanceState } from "@/features/alert-investigation/lib/provenance";
import { AlertInspection } from "./artifacts/AlertInspection";
import { DetectionDefinitionInspection } from "./artifacts/DetectionDefinitionInspection";
import { DetectionResultInspection } from "./artifacts/DetectionResultInspection";
import { NormalizedEventInspection } from "./artifacts/NormalizedEventInspection";
import { SourceEventInspection } from "./artifacts/SourceEventInspection";
import { ValidationOutcomeInspection } from "./artifacts/ValidationOutcomeInspection";
import { ProvenanceRecord } from "./ProvenanceRecord";
import type { ActiveArtifactInspection, ArtifactKey } from "./types";
import styles from "./dossier.module.css";

/**
 * The single Inspection Leaf mount point (UX spec §3.5, implementation
 * plan §3): renders exactly one active artifact inspection at a time,
 * delegating to the correct typed artifact-specific view — never a
 * generic untyped key/value renderer. Which artifact is "active" and
 * which `ArtifactInspectionModel` field it resolves to is decided by the
 * caller (Step 3 orchestration); this component only ever receives an
 * already-tagged, already-resolved value.
 *
 * `relatedArtifacts` (when supplied) is rendered once, generically, here
 * — not duplicated inside each of the six artifact-specific views — since
 * every artifact's related-artifact list is the same kind of action
 * regardless of which artifact is showing.
 */
export function InspectionLeaf({
  active,
  selectedProvenance,
  relatedArtifacts,
  onSelectRelatedArtifact,
}: {
  active: ActiveArtifactInspection;
  selectedProvenance?: ProvenanceState;
  relatedArtifacts?: EvidenceRegisterEntry[];
  onSelectRelatedArtifact?: (key: ArtifactKey) => void;
}) {
  return (
    <section aria-labelledby="inspection-leaf-heading" className={styles.section}>
      <h2 id="inspection-leaf-heading" className={styles.heading}>
        Inspection: {ARTIFACT_HEADINGS[active.key]}
      </h2>

      {renderActive(active)}

      {selectedProvenance && (
        <div>
          <p className={styles.eyebrow}>Selected condition provenance</p>
          <ProvenanceRecord provenance={selectedProvenance} />
        </div>
      )}

      {relatedArtifacts && relatedArtifacts.length > 0 && (
        <nav aria-label="Related evidence">
          <p className={styles.eyebrow}>Related evidence</p>
          <ul className={styles.ruledList}>
            {relatedArtifacts.map((entry) => (
              <li key={entry.key} className={styles.ruledRow}>
                <button
                  type="button"
                  className={styles.plainButton}
                  onClick={() => onSelectRelatedArtifact?.(entry.key)}
                >
                  {entry.label}
                </button>
              </li>
            ))}
          </ul>
        </nav>
      )}
    </section>
  );
}

const ARTIFACT_HEADINGS: Record<ArtifactKey, string> = {
  "source-event": "Source event",
  "validation-outcome": "Validation outcome",
  "normalized-event": "Normalized event",
  "detection-definition": "Detection definition",
  "detection-result": "Detection result",
  alert: "Alert",
};

function renderActive(active: ActiveArtifactInspection) {
  switch (active.key) {
    case "source-event":
      return <SourceEventInspection inspection={active.inspection} />;
    case "validation-outcome":
      return <ValidationOutcomeInspection inspection={active.inspection} />;
    case "normalized-event":
      return <NormalizedEventInspection inspection={active.inspection} />;
    case "detection-definition":
      return <DetectionDefinitionInspection inspection={active.inspection} />;
    case "detection-result":
      return <DetectionResultInspection inspection={active.inspection} />;
    case "alert":
      return <AlertInspection inspection={active.inspection} />;
  }
}
