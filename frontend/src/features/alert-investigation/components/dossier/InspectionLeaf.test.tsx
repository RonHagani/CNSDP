import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { InspectionLeaf } from "./InspectionLeaf";
import { buildArtifactInspection } from "@/features/alert-investigation/lib/artifactInspection";
import { buildEvidenceRegister, resolveDefaultSelection } from "@/features/alert-investigation/lib/evidenceRegister";
import { fixtureIntact, fixturePartial, fixtureScenario2Intact } from "@/fixtures/alert-investigation/v1";
import type { ActiveArtifactInspection } from "./types";

describe("InspectionLeaf — all six typed inspection variants", () => {
  const model = buildArtifactInspection(fixtureIntact);

  it("source-event: delegates to the raw-event view", () => {
    render(<InspectionLeaf active={{ key: "source-event", inspection: model.sourceEvent }} />);
    expect(screen.getByText(/Source submission — raw event/)).toBeInTheDocument();
  });

  it("validation-outcome: delegates to the outcome/reason view", () => {
    render(<InspectionLeaf active={{ key: "validation-outcome", inspection: model.validationOutcome }} />);
    expect(screen.getByText("valid")).toBeInTheDocument();
  });

  it("normalized-event: delegates to the normalized-event view", () => {
    render(<InspectionLeaf active={{ key: "normalized-event", inspection: model.normalizedEvent }} />);
    expect(screen.getByText("kubernetes-admin")).toBeInTheDocument();
  });

  it("detection-definition: delegates to the definition view", () => {
    render(
      <InspectionLeaf active={{ key: "detection-definition", inspection: model.detectionDefinition }} />,
    );
    expect(screen.getByText(/Interactive container exec request/)).toBeInTheDocument();
  });

  it("detection-result: delegates to the match-reason view", () => {
    render(<InspectionLeaf active={{ key: "detection-result", inspection: model.detectionResult }} />);
    expect(screen.getByText("scenario-1")).toBeInTheDocument();
  });

  it("alert: delegates to the alert-summary view", () => {
    render(<InspectionLeaf active={{ key: "alert", inspection: model.alert }} />);
    expect(screen.getByText("kubernetes-admin")).toBeInTheDocument();
  });
});

describe("InspectionLeaf — only one artifact inspection appears at a time", () => {
  it("renders exactly one section heading naming the active artifact", () => {
    const model = buildArtifactInspection(fixtureIntact);
    render(<InspectionLeaf active={{ key: "alert", inspection: model.alert }} />);
    expect(screen.getByRole("heading", { name: /Inspection: Alert/i })).toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: /Inspection: Source event/i })).not.toBeInTheDocument();
  });
});

describe("InspectionLeaf — unavailable variant renders truthfully", () => {
  it("states the detection definition is unavailable for fixturePartial, not a blank", () => {
    const model = buildArtifactInspection(fixturePartial);
    render(
      <InspectionLeaf active={{ key: "detection-definition", inspection: model.detectionDefinition }} />,
    );
    expect(screen.getByText(/Detection definition is not available/i)).toBeInTheDocument();
  });
});

describe("InspectionLeaf — Source Event uses the existing JsonTree", () => {
  it("renders the raw event's real field values via JsonTree's own dl/dt/dd structure", () => {
    const model = buildArtifactInspection(fixtureIntact);
    render(<InspectionLeaf active={{ key: "source-event", inspection: model.sourceEvent }} />);
    // JsonTree renders scalar values quoted, as its own formatScalar does.
    expect(screen.getByText('"kubernetes-admin"')).toBeInTheDocument();
    expect(screen.getByText("auditID")).toBeInTheDocument();
  });
});

describe("InspectionLeaf — Detection Definition distinguishes declared from satisfied", () => {
  it("scenario 2: all five declared characteristics show, each marked Satisfied", () => {
    const model = buildArtifactInspection(fixtureScenario2Intact);
    render(
      <InspectionLeaf active={{ key: "detection-definition", inspection: model.detectionDefinition }} />,
    );
    for (const id of ["privileged_container", "host_network", "host_pid", "host_ipc", "host_path_volume"]) {
      expect(screen.getByText(id)).toBeInTheDocument();
    }
    expect(screen.getAllByText("Satisfied")).toHaveLength(5);
    expect(screen.queryByText("Not satisfied")).not.toBeInTheDocument();
  });

  it("an unsatisfied declared characteristic is shown as definition data, marked distinctly, not silently dropped", () => {
    const partiallyMatched = {
      ...fixtureScenario2Intact,
      detectionResult: {
        available: true,
        matchReason: {
          ...fixtureScenario2Intact.detectionResult.matchReason!,
          satisfiedCharacteristics: fixtureScenario2Intact.detectionResult.matchReason!.satisfiedCharacteristics.slice(
            0,
            3,
          ),
        },
      },
    };
    const model = buildArtifactInspection(partiallyMatched);
    render(
      <InspectionLeaf active={{ key: "detection-definition", inspection: model.detectionDefinition }} />,
    );
    expect(screen.getAllByText("Satisfied")).toHaveLength(3);
    expect(screen.getAllByText("Not satisfied")).toHaveLength(2);
  });
});

describe("InspectionLeaf — no scenario-1-only assumption", () => {
  it("normalized-event view for scenario 2 renders podCreation, not exec", () => {
    const model = buildArtifactInspection(fixtureScenario2Intact);
    render(<InspectionLeaf active={{ key: "normalized-event", inspection: model.normalizedEvent }} />);
    expect(screen.getByText("podCreation")).toBeInTheDocument();
    expect(screen.queryByText("exec")).not.toBeInTheDocument();
  });
});

describe("InspectionLeaf — related-artifact actions", () => {
  it("emits the exact typed artifact key of the clicked related-evidence action", async () => {
    const user = userEvent.setup();
    const model = buildArtifactInspection(fixtureIntact);
    const register = buildEvidenceRegister(fixtureIntact, resolveDefaultSelection(fixtureIntact));
    const detectionResultEntry = register.find((e) => e.key === "detection-result")!;
    const onSelectRelatedArtifact = vi.fn();

    const active: ActiveArtifactInspection = { key: "detection-result", inspection: model.detectionResult };
    render(
      <InspectionLeaf
        active={active}
        relatedArtifacts={detectionResultEntry.relatedArtifacts.map(
          (key) => register.find((e) => e.key === key)!,
        )}
        onSelectRelatedArtifact={onSelectRelatedArtifact}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Normalized event" }));
    expect(onSelectRelatedArtifact).toHaveBeenCalledWith("normalized-event");
  });
});
