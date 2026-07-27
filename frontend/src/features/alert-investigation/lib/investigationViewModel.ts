import type { AlertInvestigationResponse, EvidenceArtifactId, FailedLink } from "@/types/contract";
import { buildArtifactInspection, type ArtifactInspectionModel } from "./artifactInspection";
import { buildEvidenceConcordance, type EvidenceConcordance } from "./concordance";
import {
  allArtifactsAvailable,
  buildEvidenceRegister,
  resolveDefaultSelection,
  type EvidenceRegisterEntry,
} from "./evidenceRegister";
import { buildFinding, type FindingViewModel } from "./finding";
import { buildTraceabilityProof, type TraceabilityProofModel } from "./traceability";

/**
 * The root investigation view model (implementation plan §4): the single
 * typed derivation from `AlertInvestigationResponse` that carries
 * everything the Causal Evidence Dossier's six information-architecture
 * elements need — Docket Header, Finding, Evidence Concordance, Evidence
 * Register, the default Inspection Leaf selection, and Traceability Proof
 * — without any CSS-oriented name, layout position, icon, animation
 * state, DOM identifier, or legacy stage terminology anywhere in it.
 *
 * `buildInvestigationViewModel` is pure: given the same response, it
 * always returns an equivalent, freshly-constructed result, and it never
 * reads a mutable field back out of `data` after writing to it — `data`
 * is only ever read, never mutated.
 */

export interface DocketHeaderViewModel {
  alertId: number;
  matchedScenario?: string;
  matchedDefinitionName?: string;
  pinnedRevision?: string;
  traceabilityIntact: boolean;
  /** The specific broken link (UX requirement: the canvas must name the
   *  real failed link, not just say "broken") — absent whenever
   *  `traceabilityIntact` is true. */
  failedLink?: FailedLink;
}

export interface InvestigationViewModel {
  alertId: number;
  docketHeader: DocketHeaderViewModel;
  finding: FindingViewModel;
  concordance: EvidenceConcordance;
  register: EvidenceRegisterEntry[];
  defaultSelectedArtifact: EvidenceArtifactId;
  artifacts: ArtifactInspectionModel;
  traceability: TraceabilityProofModel;
}

function buildDocketHeader(
  data: AlertInvestigationResponse,
  finding: FindingViewModel,
): DocketHeaderViewModel {
  return {
    alertId: data.alertId,
    matchedScenario: finding.matchedScenario,
    matchedDefinitionName: finding.matchedDefinitionName,
    pinnedRevision: data.detectionDefinition.available ? data.detectionDefinition.revision : undefined,
    traceabilityIntact: data.traceability.intact,
    failedLink: data.traceability.intact ? undefined : data.traceability.failedLink,
  };
}

export function buildInvestigationViewModel(data: AlertInvestigationResponse): InvestigationViewModel {
  const finding = buildFinding(data);
  const defaultSelectedArtifact = resolveDefaultSelection(data);

  return {
    alertId: data.alertId,
    docketHeader: buildDocketHeader(data, finding),
    finding,
    concordance: buildEvidenceConcordance(data),
    register: buildEvidenceRegister(data, defaultSelectedArtifact),
    defaultSelectedArtifact,
    artifacts: buildArtifactInspection(data),
    traceability: buildTraceabilityProof(data, allArtifactsAvailable(data)),
  };
}
