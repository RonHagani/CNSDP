import { afterEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { InvestigationMap } from "./InvestigationMap";
import * as investigationViewModelModule from "./lib/investigationViewModel";
import {
  fixtureBrokenTraceability,
  fixtureBrokenTraceabilitySourceKey,
  fixtureIntact,
  fixturePartial,
  fixtureScenario2Intact,
  fixtureScenario3Intact,
} from "@/fixtures/alert-investigation/v1";
import type { AlertInvestigationResponse } from "@/types/contract";

/**
 * A `requires_any` group only needs "at least one" declared option
 * satisfied for a match to exist (UX spec §2) — a response naming fewer
 * than all five of scenario 2's declared characteristics as satisfied is
 * therefore a legitimate, contract-representable shape. No real captured
 * backend testdata exercises this today (implementation plan §8), so —
 * per the plan's own documented fallback — this is a scoped, hand-built
 * response object for this test file alone (see concordance.test.ts's own
 * copy of this same object for the domain-level tests).
 */
const fixtureScenario2MixedSatisfaction: AlertInvestigationResponse = {
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

afterEach(() => {
  vi.restoreAllMocks();
});

describe("InvestigationMap — view model construction", () => {
  it("builds the investigation view model exactly once per response, even across state-only re-renders", async () => {
    const user = userEvent.setup();
    const spy = vi.spyOn(investigationViewModelModule, "buildInvestigationViewModel");
    render(<InvestigationMap data={fixtureIntact} />);
    expect(spy).toHaveBeenCalledTimes(1);

    await user.click(screen.getByRole("button", { name: /stdin_streaming/ }));
    expect(spy).toHaveBeenCalledTimes(1);
  });
});

describe("InvestigationMap — neutral state on load", () => {
  it("renders the header, all six artifacts, and the rail, with no pin preselected", () => {
    render(<InvestigationMap data={fixtureIntact} />);
    expect(screen.getAllByText(/Alert #1/).length).toBeGreaterThan(0);
    expect(screen.getByRole("button", { name: /stdin_streaming/ })).toHaveAttribute(
      "aria-pressed",
      "false",
    );
    expect(screen.getByRole("button", { name: /tty_allocation/ })).toHaveAttribute(
      "aria-pressed",
      "false",
    );
    expect(screen.queryByRole("group", { name: /Provenance record/i })).not.toBeInTheDocument();
  });
});

describe("InvestigationMap — Verified provenance selection (scenario 1)", () => {
  it("selecting a requestURI-derived pin reveals a Verified annotation with the real raw path", async () => {
    const user = userEvent.setup();
    render(<InvestigationMap data={fixtureIntact} />);
    await user.click(screen.getByRole("button", { name: /tty_allocation/ }));
    const record = screen.getByRole("group", { name: /Provenance record/i });
    expect(within(record).getByText("requestURI")).toBeInTheDocument();
    expect(within(record).getByText("exec.tty")).toBeInTheDocument();
  });

  it("highlights the corresponding Normalized Event row while selected", async () => {
    const user = userEvent.setup();
    const { container } = render(<InvestigationMap data={fixtureIntact} />);
    await user.click(screen.getByRole("button", { name: /tty_allocation/ }));
    const highlighted = container.querySelectorAll('[data-highlighted="true"]');
    expect(highlighted).toHaveLength(1);
    expect(highlighted[0].textContent).toContain("exec.tty");
  });

  it("re-selecting the same pin toggles the annotation closed", async () => {
    const user = userEvent.setup();
    render(<InvestigationMap data={fixtureIntact} />);
    const pin = screen.getByRole("button", { name: /tty_allocation/ });
    await user.click(pin);
    expect(screen.getByRole("group", { name: /Provenance record/i })).toBeInTheDocument();
    await user.click(pin);
    expect(screen.queryByRole("group", { name: /Provenance record/i })).not.toBeInTheDocument();
  });
});

describe("InvestigationMap — Partial provenance (scenario 2)", () => {
  it("selecting a requestObject-derived pin shows a Partial record with no raw path claimed", async () => {
    const user = userEvent.setup();
    render(<InvestigationMap data={fixtureScenario2Intact} />);
    await user.click(screen.getByRole("button", { name: /privileged_container/ }));
    const record = screen.getByRole("group", { name: /Provenance record/i });
    expect(within(record).getByText("podCreation.privileged")).toBeInTheDocument();
    expect(within(record).queryByText("Raw path")).not.toBeInTheDocument();
  });

  it("renders all five satisfied pins simultaneously visible on the bus", () => {
    render(<InvestigationMap data={fixtureScenario2Intact} />);
    for (const id of ["privileged_container", "host_network", "host_pid", "host_ipc", "host_path_volume"]) {
      expect(screen.getByRole("button", { name: new RegExp(id) })).toBeInTheDocument();
    }
  });
});

describe("InvestigationMap — scenario 3", () => {
  it("renders the real cluster-admin grant identity, not scenario 1's", () => {
    render(<InvestigationMap data={fixtureScenario3Intact} />);
    expect(screen.getByRole("button", { name: /role_ref_cluster_admin/ })).toBeInTheDocument();
    expect(screen.queryByText(/stdin_streaming/)).not.toBeInTheDocument();
  });

  it("selecting its one declared characteristic works correctly with only a single declared characteristic present", async () => {
    const user = userEvent.setup();
    render(<InvestigationMap data={fixtureScenario3Intact} />);
    const pin = screen.getByRole("button", { name: /role_ref_cluster_admin/ });
    expect(pin).toHaveAttribute("aria-pressed", "false");

    await user.click(pin);
    expect(pin).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByRole("group", { name: /Provenance record/i })).toBeInTheDocument();

    await user.click(pin);
    expect(pin).toHaveAttribute("aria-pressed", "false");
    expect(screen.queryByRole("group", { name: /Provenance record/i })).not.toBeInTheDocument();
  });
});

describe("InvestigationMap — declared but unsatisfied selection (satisfied and unsatisfied coexisting)", () => {
  it("renders both satisfied and declared-only pins simultaneously, distinguishably", () => {
    render(<InvestigationMap data={fixtureScenario2MixedSatisfaction} />);
    expect(screen.getByRole("button", { name: /privileged_container/ })).toHaveAttribute(
      "data-satisfied",
      "true",
    );
    expect(screen.getByRole("button", { name: /host_pid/ })).toHaveAttribute("data-satisfied", "false");
  });

  it("selecting a declared-but-unsatisfied pin reveals the plain declared-only statement, never a fabricated provenance record", async () => {
    const user = userEvent.setup();
    render(<InvestigationMap data={fixtureScenario2MixedSatisfaction} />);
    await user.click(screen.getByRole("button", { name: /host_pid/ }));
    const record = screen.getByRole("group", { name: /Provenance record/i });
    expect(within(record).getByText("Declared, not satisfied")).toBeInTheDocument();
    expect(within(record).queryByText(/Verified/)).not.toBeInTheDocument();
    expect(within(record).queryByText(/Partial/)).not.toBeInTheDocument();
  });

  it("re-selecting the same declared-only pin toggles the annotation closed", async () => {
    const user = userEvent.setup();
    render(<InvestigationMap data={fixtureScenario2MixedSatisfaction} />);
    const pin = screen.getByRole("button", { name: /host_pid/ });
    await user.click(pin);
    expect(screen.getByRole("group", { name: /Provenance record/i })).toBeInTheDocument();
    await user.click(pin);
    expect(screen.queryByRole("group", { name: /Provenance record/i })).not.toBeInTheDocument();
  });

  it("does not carry a stale declared-only selection when the same instance receives a different alert response", async () => {
    const user = userEvent.setup();
    const { rerender } = render(<InvestigationMap data={fixtureScenario2MixedSatisfaction} />);
    await user.click(screen.getByRole("button", { name: /host_pid/ }));
    expect(screen.getByRole("group", { name: /Provenance record/i })).toBeInTheDocument();

    rerender(<InvestigationMap data={fixtureScenario3Intact} />);

    expect(screen.queryByRole("group", { name: /Provenance record/i })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: /role_ref_cluster_admin/ })).toHaveAttribute(
      "aria-pressed",
      "false",
    );
  });
});

describe("InvestigationMap — unavailable artifact", () => {
  it("renders a truthful unavailable statement for the detection definition (fixturePartial)", () => {
    render(<InvestigationMap data={fixturePartial} />);
    expect(screen.getByText("Detection definition unavailable")).toBeInTheDocument();
  });
});

describe("InvestigationMap — traceability", () => {
  it("intact: renders every rail node with no broken segment", () => {
    const { container } = render(<InvestigationMap data={fixtureIntact} />);
    expect(container.querySelector('[data-broken="true"]')).toBeNull();
  });

  it("broken (raw_event_sha256): renders the specific failed link and its localized segment", () => {
    render(<InvestigationMap data={fixtureBrokenTraceability} />);
    expect(screen.getByTestId("rail-segment-0")).toHaveAttribute("data-broken", "true");
    // The explanation now appears twice, by design (UX requirement: the
    // canvas itself localizes the break, not only the Traceability Rail
    // below it) — both occurrences carry the same real explanation text.
    expect(screen.getAllByText(/recorded integrity digest/).length).toBeGreaterThan(0);
    expect(screen.getAllByText("raw_event_sha256").length).toBeGreaterThan(0);
  });

  it("broken (source_key): renders the specific failed link localized to the same segment as raw_event_sha256", () => {
    render(<InvestigationMap data={fixtureBrokenTraceabilitySourceKey} />);
    expect(screen.getByTestId("rail-segment-0")).toHaveAttribute("data-broken", "true");
    expect(screen.getAllByText(/source identity/).length).toBeGreaterThan(0);
    expect(screen.getAllByText("source_key").length).toBeGreaterThan(0);
  });
});

describe("InvestigationMap — keyboard operation", () => {
  it("a pin is selectable via Tab focus and Enter, not only click", async () => {
    const user = userEvent.setup();
    render(<InvestigationMap data={fixtureIntact} />);
    const pin = screen.getByRole("button", { name: /stdin_streaming/ });
    pin.focus();
    await user.keyboard("{Enter}");
    expect(screen.getByRole("group", { name: /Provenance record/i })).toBeInTheDocument();
  });
});

describe("InvestigationMap — selection reset across response changes", () => {
  it("does not carry a stale selectedConditionKey when the same instance receives a different alert response", async () => {
    const user = userEvent.setup();
    const { rerender } = render(<InvestigationMap data={fixtureIntact} />);
    await user.click(screen.getByRole("button", { name: /tty_allocation/ }));
    expect(screen.getByRole("group", { name: /Provenance record/i })).toBeInTheDocument();

    rerender(<InvestigationMap data={fixtureScenario2Intact} />);

    expect(screen.queryByRole("group", { name: /Provenance record/i })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: /privileged_container/ })).toHaveAttribute(
      "aria-pressed",
      "false",
    );
  });
});

describe("InvestigationMap — contract discipline", () => {
  it("renders no fabricated severity, confidence, or timing content anywhere", () => {
    const { container } = render(<InvestigationMap data={fixtureScenario2Intact} />);
    const text = container.textContent!.toLowerCase();
    for (const forbidden of ["severity", "confidence", "elapsed", "duration"]) {
      expect(text).not.toContain(forbidden);
    }
  });
});
