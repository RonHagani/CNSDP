import { describe, expect, it } from "vitest";
import { render, screen, within } from "@testing-library/react";
import { EvidenceCanvas } from "./EvidenceCanvas";

const SLOT_CONTENT = {
  header: "the-header",
  source: "the-source",
  validation: "the-validation",
  normalized: "the-normalized",
  definition: "the-definition",
  result: "the-result",
  alert: "the-alert",
  rail: "the-rail",
} as const;

function renderCanvas() {
  return render(
    <EvidenceCanvas
      header={<span>{SLOT_CONTENT.header}</span>}
      source={<span>{SLOT_CONTENT.source}</span>}
      validation={<span>{SLOT_CONTENT.validation}</span>}
      normalized={<span>{SLOT_CONTENT.normalized}</span>}
      definition={<span>{SLOT_CONTENT.definition}</span>}
      result={<span>{SLOT_CONTENT.result}</span>}
      alert={<span>{SLOT_CONTENT.alert}</span>}
      rail={<span>{SLOT_CONTENT.rail}</span>}
    />,
  );
}

describe("EvidenceCanvas", () => {
  it("renders every provided slot's content", () => {
    renderCanvas();
    for (const text of Object.values(SLOT_CONTENT)) {
      expect(screen.getByText(text)).toBeInTheDocument();
    }
  });

  it("renders exactly the five artifact-level connector relationships, each once", () => {
    const { container } = renderCanvas();
    const wires = container.querySelectorAll("[data-wire]");
    expect(wires).toHaveLength(5);

    const wireIds = Array.from(wires).map((el) => el.getAttribute("data-wire"));
    expect(wireIds).toEqual(
      expect.arrayContaining([
        "source-validation-stub",
        "source-normalized",
        "normalized-definition",
        "definition-result",
        "result-alert",
      ]),
    );
  });

  it("never renders a sixth, invented connector relationship", () => {
    const { container } = renderCanvas();
    expect(container.querySelectorAll("[data-wire]")).toHaveLength(5);
  });

  it("renders every connector as purely decorative: aria-hidden and outside the accessible/tab order", () => {
    const { container } = renderCanvas();
    const wires = container.querySelectorAll("[data-wire]");
    wires.forEach((wire) => {
      expect(wire.getAttribute("aria-hidden")).toBe("true");
      expect(wire).not.toHaveAttribute("role");
      expect(wire).not.toHaveAttribute("tabindex");
    });
  });

  it("positions each connector as a true DOM neighbor of the two artifacts it claims to connect", () => {
    // A set-membership check alone can't tell a correctly-placed connector
    // from one whose label was swapped with another's — this asserts each
    // `data-wire` element's actual DOM neighbors match the two slots its
    // own name claims, so a mislabeled or misassigned relationship fails.
    renderCanvas();

    const sourceNormalized = document.querySelector('[data-wire="source-normalized"]');
    expect(sourceNormalized?.previousElementSibling?.textContent).toContain(SLOT_CONTENT.source);
    expect(sourceNormalized?.nextElementSibling?.textContent).toContain(SLOT_CONTENT.normalized);

    const normalizedDefinition = document.querySelector('[data-wire="normalized-definition"]');
    expect(normalizedDefinition?.previousElementSibling?.textContent).toContain(SLOT_CONTENT.normalized);
    expect(normalizedDefinition?.nextElementSibling?.textContent).toContain(SLOT_CONTENT.definition);

    const definitionResult = document.querySelector('[data-wire="definition-result"]');
    expect(definitionResult?.previousElementSibling?.textContent).toContain(SLOT_CONTENT.definition);
    expect(definitionResult?.nextElementSibling?.textContent).toContain(SLOT_CONTENT.result);

    const resultAlert = document.querySelector('[data-wire="result-alert"]');
    expect(resultAlert?.previousElementSibling?.textContent).toContain(SLOT_CONTENT.result);
    expect(resultAlert?.nextElementSibling?.textContent).toContain(SLOT_CONTENT.alert);
  });

  it("keeps Validation Outcome and its stub nested inside Source Submission's own group, not promoted to an independent sibling", () => {
    // Walking up to "some shared ancestor" is too weak a check — the grid
    // root is a shared ancestor of literally every slot, so that approach
    // would still pass even if Validation were moved out to its own
    // top-level sibling. Asserting containment within one specific, named
    // group (identified by data-testid, not by tree-walking or CSS class)
    // fails correctly if that promotion ever happens.
    renderCanvas();
    const group = screen.getByTestId("source-validation-group");

    expect(within(group).getByText(SLOT_CONTENT.validation)).toBeInTheDocument();
    expect(within(group).getByText(SLOT_CONTENT.source)).toBeInTheDocument();
    expect(group.querySelector('[data-wire="source-validation-stub"]')).not.toBeNull();
  });
});
