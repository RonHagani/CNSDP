import { useMemo } from "react";
import type { Command } from "@/components/CommandPalette/CommandPalette";
import type { ArtifactKey } from "../components/dossier";

/**
 * A restrained, hand-authored command set grounded entirely in the
 * approved Causal Evidence Dossier IA — every command maps to a real
 * capability the screen actually has, and none references a processing
 * stage, a route, or a capability this product does not support (UX spec
 * §10). Replaces the legacy stage-focus command list; no longer reads
 * from a global context — every action is a plain callback the caller
 * (`InvestigationDossier`) supplies, operating on the same local
 * selection state the visible Evidence Register and Evidence Concordance
 * controls already use.
 */
const ARTIFACT_COMMANDS: { key: ArtifactKey; label: string }[] = [
  { key: "source-event", label: "Inspect source submission" },
  { key: "validation-outcome", label: "Inspect validation outcome" },
  { key: "normalized-event", label: "Inspect normalized event" },
  { key: "detection-definition", label: "Inspect detection definition" },
  { key: "detection-result", label: "Inspect detection result" },
  { key: "alert", label: "Inspect generated alert" },
];

export function useInvestigationCommands({
  onSelectArtifact,
}: {
  onSelectArtifact: (key: ArtifactKey) => void;
}): Command[] {
  return useMemo<Command[]>(() => {
    const focusConcordance: Command = {
      id: "focus-matched-conditions",
      label: "Focus matched conditions",
      run: () => {
        document
          .getElementById("evidence-concordance-heading")
          ?.scrollIntoView({ behavior: "smooth", block: "start" });
      },
    };

    const inspectCommands: Command[] = ARTIFACT_COMMANDS.map(({ key, label }) => ({
      id: `inspect-${key}`,
      label,
      run: () => {
        onSelectArtifact(key);
        document
          .getElementById("inspection-leaf-heading")
          ?.scrollIntoView({ behavior: "smooth", block: "start" });
      },
    }));

    const focusTraceability: Command = {
      id: "focus-traceability-proof",
      label: "Focus traceability proof",
      run: () => {
        document
          .getElementById("traceability-proof-heading")
          ?.scrollIntoView({ behavior: "smooth", block: "start" });
      },
    };

    return [focusConcordance, ...inspectCommands, focusTraceability];
  }, [onSelectArtifact]);
}
