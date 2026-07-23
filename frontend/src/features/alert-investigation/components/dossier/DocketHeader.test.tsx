import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DocketHeader } from "./DocketHeader";
import { buildInvestigationViewModel } from "@/features/alert-investigation/lib/investigationViewModel";
import { fixtureIntact, fixtureScenario2Intact } from "@/fixtures/alert-investigation/v1";

describe("DocketHeader — real identity fields", () => {
  it("renders the real alert id, scenario, definition name, and revision from scenario 1", () => {
    const { docketHeader } = buildInvestigationViewModel(fixtureIntact);
    render(<DocketHeader docketHeader={docketHeader} />);

    expect(screen.getByText(/Alert #1/)).toBeInTheDocument();
    expect(screen.getByText(/scenario-1/)).toBeInTheDocument();
    expect(screen.getByText(docketHeader.matchedDefinitionName!)).toBeInTheDocument();
    expect(screen.getByText(new RegExp(docketHeader.pinnedRevision!))).toBeInTheDocument();
  });

  it("renders the real alert id and scenario for scenario 2, not scenario 1's identity", () => {
    const { docketHeader } = buildInvestigationViewModel(fixtureScenario2Intact);
    render(<DocketHeader docketHeader={docketHeader} />);

    expect(screen.getByText(/Alert #4/)).toBeInTheDocument();
    expect(screen.getByText(/scenario-2/)).toBeInTheDocument();
    expect(screen.queryByText(/scenario-1/)).not.toBeInTheDocument();
  });
});

describe("DocketHeader — no invented content", () => {
  it("never renders a severity, confidence, or invented value", () => {
    const { docketHeader } = buildInvestigationViewModel(fixtureIntact);
    const { container } = render(<DocketHeader docketHeader={docketHeader} />);
    const text = container.textContent!.toLowerCase();
    for (const forbidden of ["severity", "confidence", "critical", "high-risk"]) {
      expect(text).not.toContain(forbidden);
    }
  });

  it("states detection identity as unavailable, honestly, when the model carries no definition name", () => {
    render(
      <DocketHeader
        docketHeader={{
          alertId: 7,
          matchedScenario: undefined,
          matchedDefinitionName: undefined,
          pinnedRevision: undefined,
          traceabilityIntact: true,
        }}
      />,
    );
    expect(screen.getByText("Detection identity unavailable")).toBeInTheDocument();
  });
});

describe("DocketHeader — traceability status is readable", () => {
  it("reads Intact when traceabilityIntact is true", () => {
    render(
      <DocketHeader
        docketHeader={{ alertId: 1, traceabilityIntact: true }}
      />,
    );
    expect(screen.getByText("Intact")).toBeInTheDocument();
  });

  it("reads Broken when traceabilityIntact is false", () => {
    render(
      <DocketHeader
        docketHeader={{ alertId: 1, traceabilityIntact: false }}
      />,
    );
    expect(screen.getByText("Broken")).toBeInTheDocument();
  });
});

describe("DocketHeader — long values remain fully accessible", () => {
  it("renders the complete, untruncated revision string as visible text", () => {
    const { docketHeader } = buildInvestigationViewModel(fixtureIntact);
    render(<DocketHeader docketHeader={docketHeader} />);
    // The full 64-character revision must be present verbatim — never
    // shortened with an ellipsis, which would remove it from the
    // accessible name.
    expect(screen.getByText(new RegExp(docketHeader.pinnedRevision!))).toBeInTheDocument();
    expect(docketHeader.pinnedRevision!.length).toBeGreaterThan(40);
  });
});

describe("DocketHeader — optional command-palette trigger", () => {
  it("does not render a trigger when no callback is supplied", () => {
    const { docketHeader } = buildInvestigationViewModel(fixtureIntact);
    render(<DocketHeader docketHeader={docketHeader} />);
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });

  it("renders a real, keyboard-operable button that calls the supplied callback", async () => {
    const user = userEvent.setup();
    const onOpenCommandPalette = vi.fn();
    const { docketHeader } = buildInvestigationViewModel(fixtureIntact);
    render(<DocketHeader docketHeader={docketHeader} onOpenCommandPalette={onOpenCommandPalette} />);

    const trigger = screen.getByRole("button", { name: /command palette/i });
    trigger.focus();
    await user.keyboard("{Enter}");
    expect(onOpenCommandPalette).toHaveBeenCalledTimes(1);
  });
});
