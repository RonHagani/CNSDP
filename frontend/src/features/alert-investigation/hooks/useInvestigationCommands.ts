import { useMemo } from "react";
import type { Command } from "@/components/CommandPalette/CommandPalette";
import { STAGE_ORDER, STAGES } from "../lib/stageMapping";
import { useInvestigationUI } from "../lib/InvestigationUIContext";

/**
 * A restrained, hand-authored command set. Every command maps to a real
 * capability already implemented on this screen — no command references a
 * route, filter, or action the product does not have (Milestone 1 brief:
 * "Do not include commands for product capabilities that do not exist.").
 */
export function useInvestigationCommands(): Command[] {
  const { selectStage, selectLineage } = useInvestigationUI();

  return useMemo<Command[]>(() => {
    const stageCommands: Command[] = STAGE_ORDER.map((stageId) => ({
      id: `stage-${stageId}`,
      label: `Focus stage: ${STAGES[stageId].label}`,
      hint: STAGES[stageId].module,
      run: () => {
        selectStage(stageId);
        window.scrollTo({ top: 0, behavior: "smooth" });
      },
    }));

    return [
      ...stageCommands,
      {
        id: "open-lineage",
        label: "Open raw-to-normalized field lineage",
        hint: "Intake / Normalization",
        run: () => {
          selectStage("normalization");
          document
            .getElementById("lineage-stage")
            ?.scrollIntoView({ behavior: "smooth", block: "start" });
        },
      },
      {
        id: "focus-evidence-chain",
        label: "Focus the exhibit procession",
        run: () =>
          document
            .getElementById("evidence-ribbon-title")
            ?.scrollIntoView({ behavior: "smooth", block: "center" }),
      },
      {
        id: "inspect-traceability",
        label: "Inspect traceability integrity",
        run: () => {
          selectStage("traceability");
          document
            .getElementById("traceability-verdict")
            ?.scrollIntoView({ behavior: "smooth", block: "center" });
        },
      },
      {
        id: "clear-lineage",
        label: "Clear raw/normalized lineage highlight",
        run: () => selectLineage(null),
      },
      {
        id: "return-to-summary",
        label: "Return to investigation summary",
        run: () => {
          selectStage("investigation");
          window.scrollTo({ top: 0, behavior: "smooth" });
        },
      },
    ];
  }, [selectStage, selectLineage]);
}
