import { describe, expect, it } from "vitest";
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { LineageInteraction } from "./LineageInteraction";
import { renderWithInvestigationUI } from "@/test/renderWithProviders";
import { fixtureIntact } from "@/fixtures/alert-investigation/v1";

function renderLineage() {
  return renderWithInvestigationUI(
    <LineageInteraction
      sourceEvent={fixtureIntact.sourceEvent}
      normalizedEvent={fixtureIntact.normalizedEvent}
    />,
  );
}

describe("LineageInteraction — the hero interaction", () => {
  it("neutral initial state — no field preselected, no full record open", () => {
    renderLineage();

    expect(
      screen.getByText(/Open the source or normalized record below, then select a field/),
    ).toBeInTheDocument();
    expect(screen.queryByRole("group", { name: /Raw event content/ })).not.toBeInTheDocument();
    expect(
      screen.queryByRole("group", { name: /Normalized event content/ }),
    ).not.toBeInTheDocument();
    expect(screen.queryByText("user.username")).not.toBeInTheDocument();
  });

  it("opening the full source record reveals the raw inspector", async () => {
    const user = userEvent.setup();
    renderLineage();

    await user.click(screen.getByRole("button", { name: "Inspect full source record" }));

    expect(screen.getByRole("group", { name: /Raw event content/ })).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Hide full source record" }),
    ).toHaveAttribute("aria-expanded", "true");
  });

  it("opening the full normalized record reveals the normalized inspector and closes the source one", async () => {
    const user = userEvent.setup();
    renderLineage();

    await user.click(screen.getByRole("button", { name: "Inspect full source record" }));
    expect(screen.getByRole("group", { name: /Raw event content/ })).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Inspect full normalized record" }));

    expect(screen.queryByRole("group", { name: /Raw event content/ })).not.toBeInTheDocument();
    expect(screen.getByRole("group", { name: /Normalized event content/ })).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Inspect full source record" }),
    ).toHaveAttribute("aria-expanded", "false");
  });

  it("selecting a raw field opens a focused provenance record", async () => {
    const user = userEvent.setup();
    renderLineage();

    await user.click(screen.getByRole("button", { name: "Inspect full source record" }));
    const rawField = screen.getByRole("button", { name: '"kubernetes-admin"' });
    await user.click(rawField);

    expect(rawField).toHaveAttribute("data-highlighted", "true");
    expect(screen.getByText("user.username")).toBeInTheDocument();
    expect(
      screen.getByText("The requesting subject is carried verbatim from user.username."),
    ).toBeInTheDocument();
    expect(screen.getByText("subject.username")).toBeInTheDocument();
  });

  it("selecting a normalized field opens a focused provenance record", async () => {
    const user = userEvent.setup();
    renderLineage();

    await user.click(screen.getByRole("button", { name: "Inspect full normalized record" }));
    const normalizedField = screen.getByRole("button", { name: "kubernetes-admin" });
    await user.click(normalizedField);

    expect(normalizedField).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByText("user.username")).toBeInTheDocument();
    expect(screen.getByText("subject.username")).toBeInTheDocument();
  });

  it("selecting the shared requestURI raw field surfaces every normalized fact it feeds", async () => {
    const user = userEvent.setup();
    renderLineage();

    await user.click(screen.getByRole("button", { name: "Inspect full source record" }));
    const rawRequestUri = screen.getByRole("button", {
      name: `"${fixtureIntact.sourceEvent.rawEvent!.requestURI}"`,
    });
    await user.click(rawRequestUri);

    expect(rawRequestUri).toHaveAttribute("data-highlighted", "true");
    expect(screen.getByText("exec.stdin")).toBeInTheDocument();
    expect(screen.getByText("exec.tty")).toBeInTheDocument();
  });

  it("is keyboard operable: opening a record and selecting a field", async () => {
    const user = userEvent.setup();
    renderLineage();

    const sourceToggle = screen.getByRole("button", { name: "Inspect full source record" });
    sourceToggle.focus();
    await user.keyboard("{Enter}");
    expect(screen.getByRole("group", { name: /Raw event content/ })).toBeInTheDocument();

    const rawField = screen.getByRole("button", { name: '"kubernetes-admin"' });
    rawField.focus();
    await user.keyboard("{Enter}");

    expect(rawField).toHaveAttribute("data-highlighted", "true");
  });

  it("renders a visible, non-crashing state when the source or normalized artifact is unavailable", () => {
    renderWithInvestigationUI(
      <LineageInteraction
        sourceEvent={{ available: false }}
        normalizedEvent={fixtureIntact.normalizedEvent}
      />,
    );
    expect(screen.getByText(/Source submission unavailable/)).toBeInTheDocument();
  });
});
