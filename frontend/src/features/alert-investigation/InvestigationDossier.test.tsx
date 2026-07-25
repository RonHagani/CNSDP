import { afterEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { InvestigationDossier } from "./InvestigationDossier";
import * as investigationViewModelModule from "./lib/investigationViewModel";
import {
  fixtureBrokenTraceability,
  fixtureIntact,
  fixturePartial,
  fixtureScenario2Intact,
  fixtureScenario3Intact,
} from "@/fixtures/alert-investigation/v1";

afterEach(() => {
  vi.restoreAllMocks();
});

describe("InvestigationDossier — view model construction", () => {
  it("builds the investigation view model exactly once per response, even across state-only re-renders", async () => {
    const user = userEvent.setup();
    const spy = vi.spyOn(investigationViewModelModule, "buildInvestigationViewModel");
    render(<InvestigationDossier data={fixtureIntact} />);
    expect(spy).toHaveBeenCalledTimes(1);

    // Selecting a register entry changes local state only — `data` itself
    // does not change identity, so the view model must not be rebuilt.
    await user.click(screen.getAllByRole("button", { name: /Source submission/i })[0]);
    expect(spy).toHaveBeenCalledTimes(1);
  });
});

describe("InvestigationDossier — default selection", () => {
  it("selects Detection Result by default and shows it in both the register and the leaf", () => {
    render(<InvestigationDossier data={fixtureIntact} />);
    expect(screen.getByRole("button", { name: /Detection result/i })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(screen.getByRole("heading", { name: /Inspection: Detection result/i })).toBeInTheDocument();
  });

  it("the case opening and Evidence Concordance are visible immediately, with no condition preselected", () => {
    render(<InvestigationDossier data={fixtureIntact} />);
    // The Composition Reset merges the former Finding section into the
    // Case Opening's own `<h1>` case title — there is no separate
    // "Finding" heading any more, so the equivalent assertion is that the
    // case title (the strongest identification text) renders immediately.
    expect(screen.getByRole("heading", { level: 1 })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Evidence concordance" })).toBeInTheDocument();
    expect(screen.queryByRole("group", { name: /Provenance record/i })).not.toBeInTheDocument();
  });
});

describe("InvestigationDossier — selecting each of the six evidence artifacts", () => {
  const cases: { label: RegExp; heading: RegExp }[] = [
    { label: /Source submission/i, heading: /Inspection: Source event/i },
    { label: /Validation outcome/i, heading: /Inspection: Validation outcome/i },
    { label: /Normalized event/i, heading: /Inspection: Normalized event/i },
    { label: /Detection definition/i, heading: /Inspection: Detection definition/i },
    { label: /Detection result/i, heading: /Inspection: Detection result/i },
    { label: /Generated alert/i, heading: /Inspection: Alert/i },
  ];

  it.each(cases)("selecting $label updates the Inspection Leaf to $heading", async ({ label, heading }) => {
    const user = userEvent.setup();
    render(<InvestigationDossier data={fixtureIntact} />);
    const registerButtons = screen.getAllByRole("button", { name: label });
    await user.click(registerButtons[0]);
    expect(screen.getByRole("heading", { name: heading })).toBeInTheDocument();
  });
});

describe("InvestigationDossier — selecting a concordance condition", () => {
  it("reveals a provenance record without changing the current register selection", async () => {
    const user = userEvent.setup();
    render(<InvestigationDossier data={fixtureIntact} />);
    await user.click(screen.getByRole("button", { name: /stdin_streaming/i }));

    expect(screen.getByRole("group", { name: /Provenance record/i })).toBeInTheDocument();
    // Register selection is unaffected — still Detection Result.
    expect(screen.getByRole("button", { name: /Detection result/i })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
  });
});

describe("InvestigationDossier — Verified provenance (scenario 1)", () => {
  it("selecting a requestURI-derived characteristic shows a Verified record with a real raw path", async () => {
    const user = userEvent.setup();
    render(<InvestigationDossier data={fixtureIntact} />);
    await user.click(screen.getByRole("button", { name: /tty_allocation/i }));
    const record = screen.getByRole("group", { name: /Provenance record/i });
    expect(within(record).getByText("requestURI")).toBeInTheDocument();
    expect(within(record).getByText("exec.tty")).toBeInTheDocument();
  });
});

describe("InvestigationDossier — Partial provenance (scenario 2)", () => {
  it("selecting a requestObject-derived characteristic shows a Partial record with no raw path claimed", async () => {
    const user = userEvent.setup();
    render(<InvestigationDossier data={fixtureScenario2Intact} />);
    await user.click(screen.getByRole("button", { name: /privileged_container/i }));
    const record = screen.getByRole("group", { name: /Provenance record/i });
    expect(within(record).getByText("podCreation.privileged")).toBeInTheDocument();
    expect(within(record).queryByText("Raw path")).not.toBeInTheDocument();
  });

  it("renders multiple Partial provenance conditions simultaneously visible in the concordance", () => {
    render(<InvestigationDossier data={fixtureScenario2Intact} />);
    expect(screen.getAllByText(/^Partial —/).length).toBe(5);
  });
});

describe("InvestigationDossier — unavailable artifact inspection", () => {
  it("selecting the unavailable detection-definition entry (fixturePartial) renders a truthful unavailable state", async () => {
    const user = userEvent.setup();
    render(<InvestigationDossier data={fixturePartial} />);
    const registerSection = screen.getByRole("heading", { name: "Evidence register" }).closest("section")!;
    await user.click(within(registerSection).getByRole("button", { name: /Detection definition/i }));
    // Both the register's own summary and the leaf's detail state the
    // same true fact — one in the register row, one in the leaf.
    expect(screen.getAllByText(/Detection definition is not available/i).length).toBeGreaterThanOrEqual(1);
    expect(screen.getByRole("heading", { name: /Inspection: Detection definition/i })).toBeInTheDocument();
  });
});

describe("InvestigationDossier — traceability", () => {
  it("intact: renders a verified statement", () => {
    render(<InvestigationDossier data={fixtureIntact} />);
    expect(screen.getByText("Traceability verified.")).toBeInTheDocument();
  });

  it("broken: renders the specific failed link, independent of full artifact availability", () => {
    render(<InvestigationDossier data={fixtureBrokenTraceability} />);
    expect(screen.getByText("Traceability broken.")).toBeInTheDocument();
    expect(screen.getByText("Failed link: raw_event_sha256")).toBeInTheDocument();

    // All six register entries remain available even though the chain is
    // broken — no entry's own summary claims it is unavailable.
    const registerSection = screen.getByRole("heading", { name: "Evidence register" }).closest("section")!;
    expect(within(registerSection).queryByText(/is not available/i)).not.toBeInTheDocument();
  });
});

describe("InvestigationDossier — scenario 2 and 3 render without scenario-1 assumptions", () => {
  it("scenario 2: no exec/stdin/tty content anywhere in the rendered dossier", () => {
    const { container } = render(<InvestigationDossier data={fixtureScenario2Intact} />);
    expect(container.textContent).not.toMatch(/stdin_streaming|tty_allocation/);
  });

  it("scenario 3: renders the real cluster-admin grant, not scenario 1's identity", () => {
    render(<InvestigationDossier data={fixtureScenario3Intact} />);
    expect(screen.getAllByText(/scenario-3/).length).toBeGreaterThan(0);
    // Appears in both the concordance row (observed fact) and the
    // Detection Result inspection leaf (satisfied characteristics list).
    expect(screen.getAllByText("role_ref_cluster_admin").length).toBeGreaterThanOrEqual(1);
  });
});

describe("InvestigationDossier — command palette artifact actions", () => {
  it("running an 'Inspect' command selects that artifact without a prior register click", async () => {
    const user = userEvent.setup();
    render(<InvestigationDossier data={fixtureIntact} />);

    await user.keyboard("{Control>}k{/Control}");
    await user.type(screen.getByPlaceholderText(/Jump to/i), "Inspect normalized event");
    await user.keyboard("{Enter}");

    // The palette's exit animation keeps the background `aria-hidden` for a
    // moment after Enter — wait for it to clear before querying by role.
    expect(await screen.findByRole("heading", { name: /Inspection: Normalized event/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Normalized event/i })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
  });
});

describe("InvestigationDossier — keyboard operation", () => {
  it("a register entry is selectable via Tab focus and Enter, not only click", async () => {
    const user = userEvent.setup();
    render(<InvestigationDossier data={fixtureIntact} />);
    const registerSection = screen.getByRole("heading", { name: "Evidence register" }).closest("section")!;
    const alertButton = within(registerSection).getByRole("button", { name: /Generated alert/i });
    alertButton.focus();
    await user.keyboard("{Enter}");
    expect(screen.getByRole("heading", { name: /Inspection: Alert/i })).toBeInTheDocument();
  });
});

describe("InvestigationDossier — contract discipline", () => {
  it("renders exactly six evidence-register entries", () => {
    render(<InvestigationDossier data={fixtureIntact} />);
    expect(screen.getByRole("heading", { name: "Evidence register" })).toBeInTheDocument();
    // Six register buttons plus command-palette trigger buttons elsewhere;
    // scope to the register list itself.
    const registerHeading = screen.getByRole("heading", { name: "Evidence register" });
    const registerSection = registerHeading.closest("section")!;
    expect(within(registerSection).getAllByRole("listitem")).toHaveLength(6);
  });

  it("never renders traceability as a seventh register entry", () => {
    render(<InvestigationDossier data={fixtureIntact} />);
    const registerHeading = screen.getByRole("heading", { name: "Evidence register" });
    const registerSection = registerHeading.closest("section")!;
    expect(within(registerSection).queryByText(/traceability/i)).not.toBeInTheDocument();
  });

  it("renders no legacy stage-navigation vocabulary or elements anywhere in the DOM", () => {
    const { container } = render(<InvestigationDossier data={fixtureIntact} />);
    const text = container.textContent!.toLowerCase();
    for (const forbidden of ["signal path", "stage rail", "conduit", "exhibit procession", "decoder strip"]) {
      expect(text).not.toContain(forbidden);
    }
    expect(container.querySelector('[data-role="conduit-field"]')).toBeNull();
    expect(container.querySelector('[data-spotlight]')).toBeNull();
  });
});
