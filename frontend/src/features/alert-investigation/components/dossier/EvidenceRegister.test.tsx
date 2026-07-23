import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { EvidenceRegister } from "./EvidenceRegister";
import {
  buildEvidenceRegister,
  resolveDefaultSelection,
} from "@/features/alert-investigation/lib/evidenceRegister";
import { fixtureIntact, fixturePartial } from "@/fixtures/alert-investigation/v1";

function renderRegister(fixture = fixtureIntact) {
  const defaultKey = resolveDefaultSelection(fixture);
  const entries = buildEvidenceRegister(fixture, defaultKey);
  const onSelectArtifact = vi.fn();
  const utils = render(
    <EvidenceRegister entries={entries} selectedArtifact={defaultKey} onSelectArtifact={onSelectArtifact} />,
  );
  return { ...utils, entries, defaultKey, onSelectArtifact };
}

describe("EvidenceRegister — exactly six entries in canonical order", () => {
  it("renders exactly six register rows", () => {
    renderRegister();
    expect(screen.getAllByRole("button")).toHaveLength(6);
  });

  it("renders every artifact label in the fixed canonical order", () => {
    renderRegister();
    const labels = ["Source submission", "Validation outcome", "Normalized event", "Detection definition", "Detection result", "Generated alert"];
    const buttons = screen.getAllByRole("button");
    labels.forEach((label, i) => {
      expect(buttons[i]).toHaveTextContent(label);
    });
  });

  it("never renders a seventh traceability row", () => {
    renderRegister();
    expect(screen.queryByText(/traceability/i)).not.toBeInTheDocument();
  });
});

describe("EvidenceRegister — unavailable entries remain visible", () => {
  it("keeps the unavailable detection-definition entry present with its own real summary", () => {
    renderRegister(fixturePartial);
    expect(screen.getAllByRole("button")).toHaveLength(6);
    expect(screen.getByText(/Detection definition is not available/i)).toBeInTheDocument();
  });
});

describe("EvidenceRegister — selected state", () => {
  it("marks the default-selected entry (Detection Result) as pressed", () => {
    renderRegister();
    const detectionResultButton = screen.getByRole("button", { name: /Detection result/i });
    expect(detectionResultButton).toHaveAttribute("aria-pressed", "true");
  });

  it("marks non-selected entries as not pressed", () => {
    renderRegister();
    const sourceEventButton = screen.getByRole("button", { name: /Source submission/i });
    expect(sourceEventButton).toHaveAttribute("aria-pressed", "false");
  });
});

describe("EvidenceRegister — callback behavior", () => {
  it("clicking an entry emits its exact artifact key", async () => {
    const user = userEvent.setup();
    const { onSelectArtifact } = renderRegister();
    await user.click(screen.getByRole("button", { name: /Source submission/i }));
    expect(onSelectArtifact).toHaveBeenCalledWith("source-event");
  });

  it("keyboard activation works via Enter on a focused entry", async () => {
    const user = userEvent.setup();
    const { onSelectArtifact } = renderRegister();
    const button = screen.getByRole("button", { name: /Generated alert/i });
    button.focus();
    await user.keyboard("{Enter}");
    expect(onSelectArtifact).toHaveBeenCalledWith("alert");
  });

  it("selecting an unavailable entry still emits its key (selection is not blocked by availability)", async () => {
    const user = userEvent.setup();
    const { onSelectArtifact } = renderRegister(fixturePartial);
    await user.click(screen.getByRole("button", { name: /Detection definition/i }));
    expect(onSelectArtifact).toHaveBeenCalledWith("detection-definition");
  });
});
