import { createContext, useContext, useMemo, useState, type ReactNode } from "react";
import type { SignalPathStageId } from "@/types/contract";
import type { LineageSelection } from "./lineage";

interface InvestigationUIValue {
  selectedStage: SignalPathStageId;
  selectStage: (stage: SignalPathStageId) => void;
  lineage: LineageSelection | null;
  selectLineage: (selection: LineageSelection | null) => void;
  paletteOpen: boolean;
  setPaletteOpen: (open: boolean) => void;
}

const InvestigationUIContext = createContext<InvestigationUIValue | null>(null);

/**
 * Local, feature-scoped UI state shared by the Signal Path rail, the
 * evidence inspector, the field-lineage view, the evidence graph, and the
 * command palette. Plain React context — not Zustand or another external
 * store — deliberately: this is the one genuine cross-component sharing
 * need in Milestone 1 (product-experience-brief.md §9 / Milestone 1 brief:
 * "do not introduce Zustand unless the implemented interactions genuinely
 * require shared client state"), and context is sufficient for it.
 */
export function InvestigationUIProvider({ children }: { children: ReactNode }) {
  // Lands on "normalization" — the stage that renders the raw/normalized
  // field-lineage view (StageContent.tsx) — so the flagship interaction is
  // immediately visible, not one extra click away from first paint.
  const [selectedStage, setSelectedStage] = useState<SignalPathStageId>("normalization");
  const [lineage, setLineage] = useState<LineageSelection | null>(null);
  const [paletteOpen, setPaletteOpen] = useState(false);

  const value = useMemo<InvestigationUIValue>(
    () => ({
      selectedStage,
      selectStage: setSelectedStage,
      lineage,
      selectLineage: setLineage,
      paletteOpen,
      setPaletteOpen,
    }),
    [selectedStage, lineage, paletteOpen],
  );

  return (
    <InvestigationUIContext.Provider value={value}>{children}</InvestigationUIContext.Provider>
  );
}

// This hook and its provider are one small, cohesive context module;
// splitting them into two files purely to satisfy fast-refresh linting
// would be unjustified structure for a single piece of shared UI state.
// eslint-disable-next-line react-refresh/only-export-components
export function useInvestigationUI(): InvestigationUIValue {
  const ctx = useContext(InvestigationUIContext);
  if (!ctx) {
    throw new Error("useInvestigationUI must be used within an InvestigationUIProvider.");
  }
  return ctx;
}
