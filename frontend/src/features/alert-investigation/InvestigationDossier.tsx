import { useEffect, useMemo, useState } from "react";
import type { AlertInvestigationResponse } from "@/types/contract";
import { CommandPalette } from "@/components/CommandPalette/CommandPalette";
import type { ArtifactInspectionModel } from "./lib/artifactInspection";
import type { EvidenceRegisterEntry } from "./lib/evidenceRegister";
import { buildInvestigationViewModel } from "./lib/investigationViewModel";
import { useInvestigationCommands } from "./hooks/useInvestigationCommands";
import {
  CaseOpening,
  EvidenceConcordance,
  EvidenceRegister,
  InspectionLeaf,
  TraceabilityProof,
  type ActiveArtifactInspection,
  type ArtifactKey,
} from "./components/dossier";
import styles from "./components/dossier/dossier.module.css";

function resolveActiveInspection(
  key: ArtifactKey,
  model: ArtifactInspectionModel,
): ActiveArtifactInspection {
  switch (key) {
    case "source-event":
      return { key, inspection: model.sourceEvent };
    case "validation-outcome":
      return { key, inspection: model.validationOutcome };
    case "normalized-event":
      return { key, inspection: model.normalizedEvent };
    case "detection-definition":
      return { key, inspection: model.detectionDefinition };
    case "detection-result":
      return { key, inspection: model.detectionResult };
    case "alert":
      return { key, inspection: model.alert };
  }
}

/**
 * The Forensic Case Folio orchestrator (Composition Reset). Builds the
 * Step 1 investigation view model exactly once per response and owns the
 * two pieces of interaction state the approved UX requires: the selected
 * evidence artifact and the selected concordance condition. Every value
 * passed to a presentation component is already-resolved, typed
 * view-model data — `data: AlertInvestigationResponse` is read here, once,
 * to build the view model, and never passed to any child.
 *
 * Renders the case folio's regions in one continuous reading order: the
 * Case Opening (identity + observed act, merged — UX spec §3.1/§3.2),
 * Evidence Concordance (admission threshold + evidentiary clauses),
 * Evidence Register (the folio index), the opened Inspection Leaf record,
 * and the Traceability custody closure. No processing-stage navigation
 * model, and no legacy stage-based context, exists anywhere in this tree.
 *
 * The Evidence Register and the Evidence Concordance are independent
 * selection axes (UX spec §4.3): selecting a condition never changes
 * which artifact is current, and selecting an artifact never clears a
 * selected condition. A selected condition's `ProvenanceRecord` expands
 * inline inside its own Evidence Concordance clause (Composition Reset
 * §5) — `EvidenceConcordance`/`ConcordanceConditionRow` read `provenance`
 * directly off the already-selected row, so no separate provenance value
 * needs threading through this orchestrator at all.
 */
export function InvestigationDossier({ data }: { data: AlertInvestigationResponse }) {
  const viewModel = useMemo(() => buildInvestigationViewModel(data), [data]);

  const [selectedArtifact, setSelectedArtifact] = useState<ArtifactKey>(
    viewModel.defaultSelectedArtifact,
  );
  const [selectedConditionKey, setSelectedConditionKey] = useState<string | null>(null);
  const [paletteOpen, setPaletteOpen] = useState(false);

  const commands = useInvestigationCommands({ onSelectArtifact: setSelectedArtifact });

  useEffect(() => {
    function handleKeyDown(event: KeyboardEvent) {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "k") {
        event.preventDefault();
        setPaletteOpen(true);
      }
    }
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, []);

  const activeInspection = resolveActiveInspection(selectedArtifact, viewModel.artifacts);

  const currentRegisterEntry = viewModel.register.find((entry) => entry.key === selectedArtifact);
  const relatedArtifacts: EvidenceRegisterEntry[] = currentRegisterEntry
    ? currentRegisterEntry.relatedArtifacts
        .map((key) => viewModel.register.find((entry) => entry.key === key))
        .filter((entry): entry is EvidenceRegisterEntry => entry !== undefined)
    : [];

  return (
    <div className={`${styles.dossierRoot} ${styles.contentWidth} ${styles.stack}`}>
      <CaseOpening
        docketHeader={viewModel.docketHeader}
        finding={viewModel.finding}
        onOpenCommandPalette={() => setPaletteOpen(true)}
      />
      <EvidenceConcordance
        concordance={viewModel.concordance}
        selectedConditionKey={selectedConditionKey}
        onSelectCondition={setSelectedConditionKey}
      />
      <EvidenceRegister
        entries={viewModel.register}
        selectedArtifact={selectedArtifact}
        onSelectArtifact={setSelectedArtifact}
      />
      <InspectionLeaf
        active={activeInspection}
        relatedArtifacts={relatedArtifacts}
        onSelectRelatedArtifact={setSelectedArtifact}
      />
      <TraceabilityProof traceability={viewModel.traceability} />

      <CommandPalette open={paletteOpen} onOpenChange={setPaletteOpen} commands={commands} />
    </div>
  );
}
