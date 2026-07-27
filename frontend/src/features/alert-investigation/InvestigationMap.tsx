import { useMemo, useRef, useState } from "react";
import type { AlertInvestigationResponse } from "@/types/contract";
import { isCharacteristicRow } from "./lib/concordance";
import { buildInvestigationViewModel } from "./lib/investigationViewModel";
import { buildBusLayout } from "./components/evidence-map/characteristicBusLayout";
import {
  AlertIdentityHeader,
  BusConvergenceOverlay,
  DetectionDefinitionFrame,
  DetectionResultResolution,
  EvidenceCanvas,
  GeneratedAlertMarker,
  NormalizedEventSurface,
  ProvenanceAnnotation,
  SelectionTraceOverlay,
  SourceSubmissionSpecimen,
  TraceabilityRail,
  ValidationStamp,
} from "./components/evidence-map";
import styles from "./components/evidence-map/evidence-map.module.css";

/**
 * The Dark Evidence Map orchestrator (Track B §5; Track C — Dark Evidence
 * Map Visual Fidelity Reset — wires up the selection interaction the
 * approved screenshots require: dimming unrelated content, an anchored
 * provenance callout, a routed row-to-pin trace, honest raw-substring
 * emphasis for Verified, an honest gap treatment for Partial, and a
 * canvas-localized broken-traceability segment). Builds
 * `buildInvestigationViewModel(data)` exactly once per response and owns
 * the one piece of interaction state UX spec §12 requires: which declared
 * characteristic (satisfied or declared-only) is currently selected.
 *
 * The remaining refs exist purely to let the two measured SVG overlays
 * route their lines: the canvas root anchors both; `rowRef`/`pinRef` back
 * `SelectionTraceOverlay`'s one dynamic row-to-pin connector; `pinElsRef`
 * (every currently-rendered pin, keyed by id) and `resultRef` back
 * `BusConvergenceOverlay`'s permanent, always-on rays from each declared
 * characteristic to Detection Result. None of them participate in the
 * characteristic bus's own structural layout (`characteristicBusLayout.ts`
 * stays pure CSS/data-driven) — they only ever back these two connector
 * overlays.
 *
 * Selecting a pin toggles it off on re-selection, matching the isolated
 * prototype's own toggle behavior (UX spec §12; implementation plan §6).
 */
export function InvestigationMap({ data }: { data: AlertInvestigationResponse }) {
  const viewModel = useMemo(() => buildInvestigationViewModel(data), [data]);
  const [selectedConditionKey, setSelectedConditionKey] = useState<string | null>(null);

  const canvasRef = useRef<HTMLDivElement>(null);
  const rowRef = useRef<HTMLDivElement | null>(null);
  const pinRef = useRef<HTMLButtonElement | null>(null);
  const pinElsRef = useRef(new Map<string, HTMLButtonElement>());
  const resultRef = useRef<HTMLElement | null>(null);

  const characteristicRows = viewModel.concordance.available
    ? viewModel.concordance.rows.filter(isCharacteristicRow)
    : [];

  const selectedRow = selectedConditionKey
    ? characteristicRows.find((r) => r.id === selectedConditionKey)
    : undefined;

  const selectedProvenance = selectedRow && selectedRow.satisfied ? selectedRow.provenance : undefined;

  const selectedProvenanceKind: "verified" | "partial" | null =
    selectedProvenance && (selectedProvenance.kind === "verified" || selectedProvenance.kind === "partial")
      ? selectedProvenance.kind
      : null;

  const highlightedPath =
    selectedProvenance && (selectedProvenance.kind === "verified" || selectedProvenance.kind === "partial")
      ? selectedProvenance.normalizedPath
      : undefined;

  const rawHighlight =
    selectedProvenance && selectedProvenance.kind === "verified" ? selectedProvenance.rawHighlight : undefined;

  const selectedSide = selectedConditionKey
    ? buildBusLayout(characteristicRows).find(
        (item) => item.kind === "pin" && item.row.id === selectedConditionKey,
      )
    : undefined;
  const annotationSide = selectedSide && selectedSide.kind === "pin" ? selectedSide.side : "left";

  const failedLink = viewModel.traceability.failedLink;
  const sourceSegmentBroken =
    !viewModel.traceability.intact && (failedLink === "raw_event_sha256" || failedLink === "source_key");
  const resultAlertBroken = !viewModel.traceability.intact && failedLink === "alert";

  const sourceNormalizedState = sourceSegmentBroken ? "broken" : selectedProvenanceKind ?? "default";

  function handleSelectCondition(id: string) {
    setSelectedConditionKey((current) => (current === id ? null : id));
  }

  /** Backs both overlays from one registration path: every pin's node goes
   *  into the shared map the convergence overlay measures in bulk, and the
   *  one currently selected also lands in `pinRef` for the (unrelated,
   *  selection-only) trace overlay to read back. */
  function registerPinRef(id: string, el: HTMLButtonElement | null) {
    if (el) pinElsRef.current.set(id, el);
    else pinElsRef.current.delete(id);
    if (id === selectedConditionKey) pinRef.current = el;
  }

  return (
    <div className={styles.mapRoot}>
      <EvidenceCanvas
        canvasRef={canvasRef}
        dimmed={selectedConditionKey !== null}
        sourceNormalizedState={sourceNormalizedState}
        resultAlertBroken={resultAlertBroken}
        header={
          <AlertIdentityHeader docketHeader={viewModel.docketHeader} finding={viewModel.finding} />
        }
        source={
          <SourceSubmissionSpecimen inspection={viewModel.artifacts.sourceEvent} rawHighlight={rawHighlight} />
        }
        validation={<ValidationStamp validation={viewModel.artifacts.validationOutcome} />}
        normalized={
          <NormalizedEventSurface
            inspection={viewModel.artifacts.normalizedEvent}
            highlightedPath={highlightedPath}
            highlightKind={selectedProvenanceKind ?? undefined}
            registerHighlightedRowRef={(el) => {
              rowRef.current = el;
            }}
          />
        }
        definition={
          <>
            <DetectionDefinitionFrame
              detectionDefinition={viewModel.artifacts.detectionDefinition}
              concordance={viewModel.concordance}
              selectedConditionKey={selectedConditionKey}
              onSelectCondition={handleSelectCondition}
              registerPinRef={registerPinRef}
            />
            {selectedRow && <ProvenanceAnnotation row={selectedRow} side={annotationSide} />}
          </>
        }
        result={
          <DetectionResultResolution
            detectionResult={viewModel.artifacts.detectionResult}
            concordance={viewModel.concordance}
            sectionRef={resultRef}
          />
        }
        alert={<GeneratedAlertMarker alertId={viewModel.alertId} alert={viewModel.artifacts.alert} />}
        rail={<TraceabilityRail traceability={viewModel.traceability} />}
        brokenCallout={
          sourceSegmentBroken && failedLink ? (
            <div className={styles.brokenCallout}>
              <p className={styles.brokenCalloutKind}>Traceability — broken link</p>
              <p className={styles.brokenCalloutBody}>
                <code className={styles.technical}>{failedLink}</code>
                <br />
                {viewModel.traceability.explanation}
              </p>
            </div>
          ) : undefined
        }
        overlay={
          <>
            <BusConvergenceOverlay
              containerRef={canvasRef}
              pinEls={pinElsRef.current}
              pinIds={characteristicRows.map((row) => row.id)}
              targetRef={resultRef}
              dimmed={selectedConditionKey !== null}
            />
            {highlightedPath && (
              <SelectionTraceOverlay
                containerRef={canvasRef}
                rowRef={rowRef}
                pinRef={pinRef}
                kind={selectedProvenanceKind}
                selectionKey={selectedConditionKey}
              />
            )}
          </>
        }
      />
    </div>
  );
}
