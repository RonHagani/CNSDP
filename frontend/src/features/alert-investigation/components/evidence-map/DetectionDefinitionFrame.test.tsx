import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { DetectionDefinitionFrame } from "./DetectionDefinitionFrame";
import { buildInvestigationViewModel } from "@/features/alert-investigation/lib/investigationViewModel";
import { fixtureIntact, fixturePartial, fixtureScenario2Intact } from "@/fixtures/alert-investigation/v1";

describe("DetectionDefinitionFrame — scenario 1", () => {
  it("renders the real definition name, description, operation clause, and both satisfied pins", () => {
    const vm = buildInvestigationViewModel(fixtureIntact);
    render(
      <DetectionDefinitionFrame
        detectionDefinition={vm.artifacts.detectionDefinition}
        concordance={vm.concordance}
        selectedConditionKey={null}
        onSelectCondition={vi.fn()}
      />,
    );
    expect(screen.getByText("Interactive container exec request")).toBeInTheDocument();
    expect(screen.getAllByText(/pods\/exec/).length).toBeGreaterThan(0);
    expect(screen.getByRole("button", { name: /stdin_streaming/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /tty_allocation/ })).toBeInTheDocument();
  });

  it("renders no requires_outcome clause when none is declared (scenario 1 has none)", () => {
    const vm = buildInvestigationViewModel(fixtureIntact);
    render(
      <DetectionDefinitionFrame
        detectionDefinition={vm.artifacts.detectionDefinition}
        concordance={vm.concordance}
        selectedConditionKey={null}
        onSelectCondition={vi.fn()}
      />,
    );
    expect(screen.queryByText(/requires_outcome/)).not.toBeInTheDocument();
  });
});

describe("DetectionDefinitionFrame — scenario 2", () => {
  it("renders the declared requires_outcome clause and all five satisfied pins", () => {
    const vm = buildInvestigationViewModel(fixtureScenario2Intact);
    render(
      <DetectionDefinitionFrame
        detectionDefinition={vm.artifacts.detectionDefinition}
        concordance={vm.concordance}
        selectedConditionKey={null}
        onSelectCondition={vi.fn()}
      />,
    );
    expect(screen.getByText(/requires_outcome: success/)).toBeInTheDocument();
    for (const id of ["privileged_container", "host_network", "host_pid", "host_ipc", "host_path_volume"]) {
      expect(screen.getByRole("button", { name: new RegExp(id) })).toBeInTheDocument();
    }
  });
});

describe("DetectionDefinitionFrame — unavailable", () => {
  it("renders an explicit unavailable statement", () => {
    const vm = buildInvestigationViewModel(fixturePartial);
    render(
      <DetectionDefinitionFrame
        detectionDefinition={vm.artifacts.detectionDefinition}
        concordance={vm.concordance}
        selectedConditionKey={null}
        onSelectCondition={vi.fn()}
      />,
    );
    expect(screen.getByText("Detection definition unavailable")).toBeInTheDocument();
  });
});

describe("DetectionDefinitionFrame — selection", () => {
  it("marks the selected pin's aria-pressed true and others false", () => {
    const vm = buildInvestigationViewModel(fixtureIntact);
    render(
      <DetectionDefinitionFrame
        detectionDefinition={vm.artifacts.detectionDefinition}
        concordance={vm.concordance}
        selectedConditionKey="tty_allocation"
        onSelectCondition={vi.fn()}
      />,
    );
    expect(screen.getByRole("button", { name: /tty_allocation/ })).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByRole("button", { name: /stdin_streaming/ })).toHaveAttribute(
      "aria-pressed",
      "false",
    );
  });
});
