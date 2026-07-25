import { describe, expect, it, vi } from "vitest";
import { renderHook } from "@testing-library/react";
import { useInvestigationCommands } from "./useInvestigationCommands";

describe("useInvestigationCommands", () => {
  it("produces one command per real evidence artifact, plus focus-concordance and focus-traceability, and no stage command", () => {
    const { result } = renderHook(() => useInvestigationCommands({ onSelectArtifact: vi.fn() }));
    const labels = result.current.map((c) => c.label);

    expect(labels).toEqual([
      "Focus matched conditions",
      "Inspect source submission",
      "Inspect validation outcome",
      "Inspect normalized event",
      "Inspect detection definition",
      "Inspect detection result",
      "Inspect generated alert",
      "Focus traceability proof",
    ]);
    expect(labels.some((l) => /stage/i.test(l))).toBe(false);
  });

  it("running an artifact command calls onSelectArtifact with the exact real artifact key", () => {
    const onSelectArtifact = vi.fn();
    const { result } = renderHook(() => useInvestigationCommands({ onSelectArtifact }));

    const inspectAlert = result.current.find((c) => c.id === "inspect-alert")!;
    inspectAlert.run();
    expect(onSelectArtifact).toHaveBeenCalledWith("alert");

    const inspectDefinition = result.current.find((c) => c.id === "inspect-detection-definition")!;
    inspectDefinition.run();
    expect(onSelectArtifact).toHaveBeenCalledWith("detection-definition");
  });

  it("focus commands do not call onSelectArtifact", () => {
    const onSelectArtifact = vi.fn();
    const { result } = renderHook(() => useInvestigationCommands({ onSelectArtifact }));

    result.current.find((c) => c.id === "focus-matched-conditions")!.run();
    result.current.find((c) => c.id === "focus-traceability-proof")!.run();
    expect(onSelectArtifact).not.toHaveBeenCalled();
  });
});
