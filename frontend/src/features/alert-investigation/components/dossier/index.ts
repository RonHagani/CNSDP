/**
 * Barrel export for the Causal Evidence Dossier component family.
 *
 * Purely a re-export convenience for whichever future step composes these
 * components into the page shell — every line here is a direct,
 * one-to-one re-export of a named component from its own file, so
 * ownership stays fully traceable (no re-exported name is renamed,
 * merged, or reinterpreted). Nothing in this file is imported by any
 * component it re-exports, so it introduces no circular dependency.
 */

export { CaseOpening } from "./CaseOpening";
export { EvidenceConcordance } from "./EvidenceConcordance";
export { ConcordanceConditionRow } from "./ConcordanceConditionRow";
export { ProvenanceRecord } from "./ProvenanceRecord";
export { EvidenceRegister } from "./EvidenceRegister";
export { EvidenceRegisterEntry } from "./EvidenceRegisterEntry";
export { InspectionLeaf } from "./InspectionLeaf";
export { TraceabilityProof } from "./TraceabilityProof";

export { SourceEventInspection } from "./artifacts/SourceEventInspection";
export { ValidationOutcomeInspection } from "./artifacts/ValidationOutcomeInspection";
export { NormalizedEventInspection } from "./artifacts/NormalizedEventInspection";
export { DetectionDefinitionInspection } from "./artifacts/DetectionDefinitionInspection";
export { DetectionResultInspection } from "./artifacts/DetectionResultInspection";
export { AlertInspection } from "./artifacts/AlertInspection";

export type { ArtifactKey, ActiveArtifactInspection } from "./types";
