import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DetectionDefinitionFrame } from "./DetectionDefinitionFrame";
import { buildInvestigationViewModel } from "@/features/alert-investigation/lib/investigationViewModel";
import {
  fixtureIntact,
  fixturePartial,
  fixtureScenario2Intact,
  fixtureScenario3Intact,
} from "@/fixtures/alert-investigation/v1";
import type { AlertInvestigationResponse } from "@/types/contract";

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

describe("DetectionDefinitionFrame — bus grouping (real fixtures, integration level)", () => {
  it("scenario 2: renders exactly the Host access / Privilege split, in bus document order, over the real declared list", () => {
    const vm = buildInvestigationViewModel(fixtureScenario2Intact);
    render(
      <DetectionDefinitionFrame
        detectionDefinition={vm.artifacts.detectionDefinition}
        concordance={vm.concordance}
        selectedConditionKey={null}
        onSelectCondition={vi.fn()}
      />,
    );
    const bus = screen.getByRole("group", { name: "Declared characteristics" });
    // Direct children in DOM order: the spine, then each label/pin-slot item
    // `buildBusLayout` produces. Checking real rendered order — not just the
    // pure layout function in isolation — confirms concordance.ts's real
    // declared-list order, characteristicGroups.ts's real lookup, and this
    // component's rendering all agree with each other end to end.
    const childTexts = Array.from(bus.children).map((el) => el.textContent?.trim() ?? "");
    expect(childTexts.filter((t) => t === "Host access" || t === "Privilege")).toEqual([
      "Host access",
      "Privilege",
    ]);

    const indexOf = (needle: string) => childTexts.findIndex((t) => t.includes(needle));
    const hostAccessLabel = indexOf("Host access");
    const privilegeLabel = indexOf("Privilege");
    for (const id of ["host_ipc", "host_network", "host_path_volume", "host_pid"]) {
      expect(indexOf(id)).toBeGreaterThan(hostAccessLabel);
      expect(indexOf(id)).toBeLessThan(privilegeLabel);
    }
    expect(indexOf("privileged_container")).toBeGreaterThan(privilegeLabel);
  });

  it("scenario 1: renders both satisfied pins with no group label at all (single shared group, below clustering threshold)", () => {
    const vm = buildInvestigationViewModel(fixtureIntact);
    render(
      <DetectionDefinitionFrame
        detectionDefinition={vm.artifacts.detectionDefinition}
        concordance={vm.concordance}
        selectedConditionKey={null}
        onSelectCondition={vi.fn()}
      />,
    );
    expect(screen.getByRole("button", { name: /stdin_streaming/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /tty_allocation/ })).toBeInTheDocument();
    expect(screen.queryByText("Host access")).not.toBeInTheDocument();
    expect(screen.queryByText("Privilege")).not.toBeInTheDocument();
    expect(screen.queryByText("Declared characteristics")).not.toBeInTheDocument();
  });

  it("scenario 3: renders its lone characteristic with no group label — the honest single-group/unknown-group fallback", () => {
    const vm = buildInvestigationViewModel(fixtureScenario3Intact);
    render(
      <DetectionDefinitionFrame
        detectionDefinition={vm.artifacts.detectionDefinition}
        concordance={vm.concordance}
        selectedConditionKey={null}
        onSelectCondition={vi.fn()}
      />,
    );
    expect(screen.getByRole("button", { name: /role_ref_cluster_admin/ })).toBeInTheDocument();
    expect(screen.queryByText("Host access")).not.toBeInTheDocument();
    expect(screen.queryByText("Privilege")).not.toBeInTheDocument();
    expect(screen.queryByText("Declared characteristics")).not.toBeInTheDocument();
  });

  it("carries no residual content across fixtures on re-render — scenario 2's pins and group labels are gone once scenario 1's concordance is rendered in the same tree", () => {
    const vm2 = buildInvestigationViewModel(fixtureScenario2Intact);
    const { rerender } = render(
      <DetectionDefinitionFrame
        detectionDefinition={vm2.artifacts.detectionDefinition}
        concordance={vm2.concordance}
        selectedConditionKey={null}
        onSelectCondition={vi.fn()}
      />,
    );
    expect(screen.getByText("Host access")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /privileged_container/ })).toBeInTheDocument();

    const vm1 = buildInvestigationViewModel(fixtureIntact);
    rerender(
      <DetectionDefinitionFrame
        detectionDefinition={vm1.artifacts.detectionDefinition}
        concordance={vm1.concordance}
        selectedConditionKey={null}
        onSelectCondition={vi.fn()}
      />,
    );
    expect(screen.queryByText("Host access")).not.toBeInTheDocument();
    expect(screen.queryByText("Privilege")).not.toBeInTheDocument();
    for (const id of ["privileged_container", "host_network", "host_pid", "host_ipc", "host_path_volume"]) {
      expect(screen.queryByRole("button", { name: new RegExp(id) })).not.toBeInTheDocument();
    }
    expect(screen.getByRole("button", { name: /stdin_streaming/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /tty_allocation/ })).toBeInTheDocument();
  });
});

describe("DetectionDefinitionFrame — bus spine (structural presence only; real geometry is verified by e2e, not jsdom)", () => {
  it("renders exactly one spine element for a multi-row grouped bus (scenario 2), aria-hidden and non-interactive", () => {
    const vm = buildInvestigationViewModel(fixtureScenario2Intact);
    const { container } = render(
      <DetectionDefinitionFrame
        detectionDefinition={vm.artifacts.detectionDefinition}
        concordance={vm.concordance}
        selectedConditionKey={null}
        onSelectCondition={vi.fn()}
      />,
    );
    const bus = screen.getByRole("group", { name: "Declared characteristics" });
    const spines = container.querySelectorAll('[aria-hidden="true"]');
    // Exactly one aria-hidden decorative element inside the bus (the
    // spine) — never zero (missing), never more than one (duplicated).
    const spinesInBus = Array.from(spines).filter((el) => bus.contains(el));
    expect(spinesInBus).toHaveLength(1);
    const spine = spinesInBus[0];
    expect(spine.tagName).not.toBe("BUTTON");
    expect(spine).not.toHaveAttribute("role", "button");
    expect(spine).not.toHaveAttribute("tabindex");
  });

  it("places the spine as the bus's first child — a plain sibling of the pin slots, never nested inside a pin", () => {
    const vm = buildInvestigationViewModel(fixtureScenario2Intact);
    render(
      <DetectionDefinitionFrame
        detectionDefinition={vm.artifacts.detectionDefinition}
        concordance={vm.concordance}
        selectedConditionKey={null}
        onSelectCondition={vi.fn()}
      />,
    );
    const bus = screen.getByRole("group", { name: "Declared characteristics" });
    const firstChild = bus.firstElementChild!;
    expect(firstChild).toHaveAttribute("aria-hidden", "true");
    // Every button (pin) is a sibling of the spine, not an ancestor or
    // descendant of it — confirms the spine can never visually or
    // structurally end up "inside" a pin.
    for (const button of Array.from(bus.querySelectorAll("button"))) {
      expect(button.contains(firstChild)).toBe(false);
      expect(firstChild.contains(button)).toBe(false);
    }
  });

  it("wraps every pin in a left/right slot class, the structural hook stems and the spine rely on — one slot per declared row, none shared or skipped", () => {
    const vm = buildInvestigationViewModel(fixtureScenario2Intact);
    render(
      <DetectionDefinitionFrame
        detectionDefinition={vm.artifacts.detectionDefinition}
        concordance={vm.concordance}
        selectedConditionKey={null}
        onSelectCondition={vi.fn()}
      />,
    );
    const bus = screen.getByRole("group", { name: "Declared characteristics" });
    const buttons = Array.from(bus.querySelectorAll("button"));
    expect(buttons).toHaveLength(5);
    for (const button of buttons) {
      const slot = button.parentElement!;
      // A slot class exists (left or right) — jsdom can't confirm which
      // *side* is visually correct (that's `characteristicBusLayout.test.ts`'s
      // job, on the pure function, plus the e2e geometry suite on the real
      // render), only that every pin is wrapped in exactly one such slot.
      expect(slot.className).toMatch(/pinSlot(Left|Right)/);
    }
  });

  it("does not expose the spine to assistive technology regardless of viewport — aria-hidden is unconditional, not viewport-gated", () => {
    // jsdom never evaluates the `@media (max-width: 1023px)` rule that
    // hides the spine visually below 1024px (jsdom does no real layout or
    // media-query-driven rendering at all) — so this test cannot and does
    // not claim to verify the fallback's visual hiding. What it can
    // truthfully verify: the spine's accessibility posture (aria-hidden,
    // non-interactive) is set unconditionally in the markup itself, so
    // there is no viewport at which it could ever become exposed to a
    // screen reader — the CSS-only hiding below 1024px is strictly an
    // additional visual affordance on top of an already-inert element.
    const vm = buildInvestigationViewModel(fixtureScenario2Intact);
    render(
      <DetectionDefinitionFrame
        detectionDefinition={vm.artifacts.detectionDefinition}
        concordance={vm.concordance}
        selectedConditionKey={null}
        onSelectCondition={vi.fn()}
      />,
    );
    const bus = screen.getByRole("group", { name: "Declared characteristics" });
    expect(bus.firstElementChild).toHaveAttribute("aria-hidden", "true");
  });
});

describe("DetectionDefinitionFrame — declared but unsatisfied", () => {
  /**
   * See concordance.test.ts's own doc comment for why this hand-built
   * response object (varying only `satisfiedCharacteristics`, per the
   * implementation plan §8's documented fallback) is a legitimate,
   * contract-representable shape.
   */
  const withUnsatisfiedOption: AlertInvestigationResponse = {
    ...fixtureScenario2Intact,
    detectionResult: {
      available: true,
      matchReason: {
        ...fixtureScenario2Intact.detectionResult.matchReason!,
        satisfiedCharacteristics: fixtureScenario2Intact.detectionResult.matchReason!.satisfiedCharacteristics.slice(
          0,
          2,
        ),
      },
    },
  };

  it("renders both satisfied and declared-only pins, distinguishably, none discarded", () => {
    const vm = buildInvestigationViewModel(withUnsatisfiedOption);
    render(
      <DetectionDefinitionFrame
        detectionDefinition={vm.artifacts.detectionDefinition}
        concordance={vm.concordance}
        selectedConditionKey={null}
        onSelectCondition={vi.fn()}
      />,
    );
    for (const id of ["privileged_container", "host_network", "host_pid", "host_ipc", "host_path_volume"]) {
      expect(screen.getByRole("button", { name: new RegExp(id) })).toBeInTheDocument();
    }
    expect(screen.getAllByText("Declared, not satisfied").length).toBe(3);
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

  it("selects a pin on the branching bus by Tab + Enter, across a grouped scenario", async () => {
    const user = userEvent.setup();
    const onSelectCondition = vi.fn();
    const vm = buildInvestigationViewModel(fixtureScenario2Intact);
    render(
      <DetectionDefinitionFrame
        detectionDefinition={vm.artifacts.detectionDefinition}
        concordance={vm.concordance}
        selectedConditionKey={null}
        onSelectCondition={onSelectCondition}
      />,
    );
    const pin = screen.getByRole("button", { name: /privileged_container/ });
    pin.focus();
    expect(pin).toHaveFocus();
    await user.keyboard("{Enter}");
    expect(onSelectCondition).toHaveBeenCalledWith("privileged_container");
  });
});
