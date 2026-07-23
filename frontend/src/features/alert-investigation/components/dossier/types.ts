import type { EvidenceRegisterEntry } from "@/features/alert-investigation/lib/evidenceRegister";
import type {
  AlertInspection,
  DetectionDefinitionInspection,
  DetectionResultInspection,
  NormalizedEventInspection,
  SourceEventInspection,
  ValidationOutcomeInspection,
} from "@/features/alert-investigation/lib/artifactInspection";

/**
 * Presentation-only support types for the dossier component family. These
 * are composed entirely from Step 1 view-model types — never from
 * `AlertInvestigationResponse` — and add no domain interpretation of
 * their own; they exist only so a typed value can be threaded through
 * component props.
 */

/** The six-artifact key, derived from the Step 1 register entry's own
 *  field rather than importing `EvidenceArtifactId` from the contract
 *  directly — the dossier components never import the wire contract. */
export type ArtifactKey = EvidenceRegisterEntry["key"];

/**
 * One artifact's inspection data, tagged with which of the six artifacts
 * it belongs to. `artifactInspection.ts`'s six inspection types do not
 * self-identify which artifact they are (each only discriminates on
 * `available`), so `InspectionLeaf` needs this tag to know which
 * artifact-specific view to delegate to. Resolving *which* tagged value
 * is currently active from an `ArtifactInspectionModel` and a selected key
 * is orchestration (Step 3), not this component's job — `InspectionLeaf`
 * only ever receives an already-resolved value.
 */
export type ActiveArtifactInspection =
  | { key: "source-event"; inspection: SourceEventInspection }
  | { key: "validation-outcome"; inspection: ValidationOutcomeInspection }
  | { key: "normalized-event"; inspection: NormalizedEventInspection }
  | { key: "detection-definition"; inspection: DetectionDefinitionInspection }
  | { key: "detection-result"; inspection: DetectionResultInspection }
  | { key: "alert"; inspection: AlertInspection };
