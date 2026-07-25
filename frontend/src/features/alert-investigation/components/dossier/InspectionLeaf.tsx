import type { EvidenceRegisterEntry } from "@/features/alert-investigation/lib/evidenceRegister";
import { AlertInspection } from "./artifacts/AlertInspection";
import { DetectionDefinitionInspection } from "./artifacts/DetectionDefinitionInspection";
import { DetectionResultInspection } from "./artifacts/DetectionResultInspection";
import { NormalizedEventInspection } from "./artifacts/NormalizedEventInspection";
import { SourceEventInspection } from "./artifacts/SourceEventInspection";
import { ValidationOutcomeInspection } from "./artifacts/ValidationOutcomeInspection";
import type { ActiveArtifactInspection, ArtifactKey } from "./types";
import styles from "./dossier.module.css";

/**
 * The opened folio record (Composition Reset §7): the selected artifact
 * continuing directly out of the Evidence Register above it, not the
 * right-hand pane of a master-detail interface. Renders exactly one active
 * artifact inspection at a time, delegating to the correct typed
 * artifact-specific view — never a generic untyped key/value renderer.
 * Which artifact is "active" and which `ArtifactInspectionModel` field it
 * resolves to is decided by the caller (orchestration); this component
 * only ever receives an already-tagged, already-resolved value.
 *
 * `relatedArtifacts` render in the proof margin as compact cross-
 * references, not a navigation tab strip — the same kind of citation
 * every artifact's related-evidence list is, regardless of which artifact
 * is currently open. Matched-condition provenance no longer opens here —
 * it expands inline inside the selected Evidence Concordance clause
 * instead (Composition Reset §5), so this leaf carries only artifact
 * content and its cross-references.
 */
export function InspectionLeaf({
  active,
  relatedArtifacts,
  onSelectRelatedArtifact,
}: {
  active: ActiveArtifactInspection;
  relatedArtifacts?: EvidenceRegisterEntry[];
  onSelectRelatedArtifact?: (key: ArtifactKey) => void;
}) {
  return (
    <section
      aria-labelledby="inspection-leaf-heading"
      className={`${styles.section} ${styles.inspectionSection}`}
    >
      <h2 id="inspection-leaf-heading" className={styles.heading}>
        Inspection: {ARTIFACT_HEADINGS[active.key]}
      </h2>

      <div className={styles.inspectionBody}>{renderActive(active)}</div>

      {relatedArtifacts && relatedArtifacts.length > 0 && (
        <nav aria-label="Related evidence" className={styles.relatedEvidence}>
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
