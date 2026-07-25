import { useMemo, useState } from "react";
import type { AlertInvestigationResponse } from "@/types/contract";
import { isCharacteristicRow } from "./lib/concordance";
import { buildInvestigationViewModel } from "./lib/investigationViewModel";
import {
  AlertIdentityHeader,
  DetectionDefinitionFrame,
  DetectionResultResolution,
  EvidenceCanvas,
  GeneratedAlertMarker,
  NormalizedEventSurface,
  ProvenanceAnnotation,
  SourceSubmissionSpecimen,
  TraceabilityRail,
  ValidationStamp,
} from "./components/evidence-map";
import styles from "./components/evidence-map/evidence-map.module.css";

/**
 * The Dark Evidence Map orchestrator (Track B, Pass 1 — implementation
 * plan §5's `InvestigationMap`). Builds the existing, unmodified
 * `buildInvestigationViewModel(data)` exactly once per response — the
 * same root view model `InvestigationDossier` (the live Folio) already
 * consumes — and owns the one piece of interaction state UX spec §12
 * requires: which satisfied characteristic is currently selected.
 *
 * Not wired into any route in this pass (`AlertInvestigationPage.tsx` is
 * untouched): this component is built and tested in isolation, per the
 * task's Pass 1 scope. It exists so the presentational component tree can
 * be reviewed and verified before any live swap, exactly mirroring the
 * discipline `InvestigationDossier` itself was built under.
 *
 * Selecting a pin toggles it off on re-selection, matching the isolated
 * prototype's own toggle behavior (UX spec §12; implementation plan §6).
 */
export function InvestigationMap({ data }: { data: AlertInvestigationResponse }) {
  const viewModel = useMemo(() => buildInvestigationViewModel(data), [data]);
  const [selectedConditionKey, setSelectedConditionKey] = useState<string | null>(null);

  const selectedRow = selectedConditionKey
    ? viewModel.concordance.rows.filter(isCharacteristicRow).find((r) => r.id === selectedConditionKey)
    : undefined;

  const highlightedPath =
    selectedRow?.provenance.kind === "verified" || selectedRow?.provenance.kind === "partial"
      ? selectedRow.provenance.normalizedPath
      : undefined;

  function handleSelectCondition(id: string) {
    setSelectedConditionKey((current) => (current === id ? null : id));
  }

  return (
    <div className={styles.mapRoot}>
      <EvidenceCanvas
        header={
          <AlertIdentityHeader docketHeader={viewModel.docketHeader} finding={viewModel.finding} />
        }
        source={<SourceSubmissionSpecimen inspection={viewModel.artifacts.sourceEvent} />}
        validation={<ValidationStamp validation={viewModel.artifacts.validationOutcome} />}
        normalized={
          <NormalizedEventSurface
            inspection={viewModel.artifacts.normalizedEvent}
            highlightedPath={highlightedPath}
          />
        }
        definition={
          <>
            <DetectionDefinitionFrame
              detectionDefinition={viewModel.artifacts.detectionDefinition}
              concordance={viewModel.concordance}
              selectedConditionKey={selectedConditionKey}
              onSelectCondition={handleSelectCondition}
            />
            {selectedRow && <ProvenanceAnnotation provenance={selectedRow.provenance} />}
          </>
        }
        result={
          <DetectionResultResolution
            detectionResult={viewModel.artifacts.detectionResult}
            concordance={viewModel.concordance}
          />
        }
        alert={<GeneratedAlertMarker alertId={viewModel.alertId} alert={viewModel.artifacts.alert} />}
        rail={<TraceabilityRail traceability={viewModel.traceability} />}
      />
    </div>
  );
}
