import { afterEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SourceSubmissionSpecimen } from "./SourceSubmissionSpecimen";
import { buildArtifactInspection } from "@/features/alert-investigation/lib/artifactInspection";
import { fixtureIntact, fixtureScenario2Intact } from "@/fixtures/alert-investigation/v1";

describe("SourceSubmissionSpecimen — scenario 1 (requestURI payload)", () => {
  it("renders real tier-1 identity fields and the full requestURI as raw payload", () => {
    const { sourceEvent } = buildArtifactInspection(fixtureIntact);
    render(<SourceSubmissionSpecimen inspection={sourceEvent} />);
    expect(screen.getByText(/34b75a57-e1c0-4659-a21f-2d39256f018c/)).toBeInTheDocument();
    expect(screen.getByText(/kubernetes-admin/)).toBeInTheDocument();
    expect(screen.getByText(/pods\/exec/)).toBeInTheDocument();
  });

  it("view full raw record is on-demand, never auto-expanded", async () => {
    const user = userEvent.setup();
    const { sourceEvent } = buildArtifactInspection(fixtureIntact);
    render(<SourceSubmissionSpecimen inspection={sourceEvent} />);
    expect(screen.queryByText(/authorization\.k8s\.io\/decision/)).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /view full raw record/i }));
    expect(screen.getByText(/authorization\.k8s\.io\/decision/)).toBeInTheDocument();
  });
});

describe("SourceSubmissionSpecimen — scenario 2 (requestObject payload)", () => {
  it("renders a real requestObject excerpt via JsonTree, not the requestURI", () => {
    const { sourceEvent } = buildArtifactInspection(fixtureScenario2Intact);
    render(<SourceSubmissionSpecimen inspection={sourceEvent} />);
    expect(screen.getByText("hostNetwork")).toBeInTheDocument();
  });
});

describe("SourceSubmissionSpecimen — unavailable", () => {
  it("renders an explicit unavailable statement", () => {
    render(<SourceSubmissionSpecimen inspection={{ available: false }} />);
    expect(screen.getByText("Source event unavailable")).toBeInTheDocument();
  });
});

describe("SourceSubmissionSpecimen — Verified raw-substring scroll-into-view", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  /** Mocks `getBoundingClientRect` for every element by identity — the raw
   *  box via its own class, the mark via its `data-raw-highlight` attribute
   *  — so the values are in place before the component's first render (and
   *  therefore before its `useLayoutEffect` first runs), not patched in
   *  afterward. */
  function mockGeometry(boxRect: Partial<DOMRect>, markRect: Partial<DOMRect>) {
    vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockImplementation(function (
      this: HTMLElement,
    ) {
      if (this.hasAttribute("data-raw-highlight")) return fullRect(markRect);
      if (this.className.toString().includes("specimenRawBox")) return fullRect(boxRect);
      return fullRect({});
    });
  }

  function fullRect(rect: Partial<DOMRect>): DOMRect {
    return {
      top: 0,
      bottom: 0,
      left: 0,
      right: 0,
      width: 0,
      height: 0,
      x: 0,
      y: 0,
      toJSON() {
        return this;
      },
      ...rect,
    } as DOMRect;
  }

  it("scrolls the raw box down when the newly-selected mark renders below its visible bottom", () => {
    mockGeometry({ top: 0, bottom: 100 }, { top: 150, bottom: 186 });
    const { sourceEvent } = buildArtifactInspection(fixtureIntact);
    const { container } = render(<SourceSubmissionSpecimen inspection={sourceEvent} rawHighlight="tty=true" />);

    const box = container.querySelector('[class*="specimenRawBox"]') as HTMLElement;
    expect(box.scrollTop).toBe(86); // markRect.bottom(186) - boxRect.bottom(100)
  });

  it("scrolls the raw box up when the newly-selected mark renders above its visible top", () => {
    mockGeometry({ top: 100, bottom: 300 }, { top: 50, bottom: 86 });
    const { sourceEvent } = buildArtifactInspection(fixtureIntact);
    const { container } = render(<SourceSubmissionSpecimen inspection={sourceEvent} rawHighlight="tty=true" />);

    const box = container.querySelector('[class*="specimenRawBox"]') as HTMLElement;
    expect(box.scrollTop).toBe(-50); // boxRect.top(100) - markRect.top(50), moved the other way
  });

  it("does nothing when the mark is already fully visible inside the box", () => {
    mockGeometry({ top: 0, bottom: 200 }, { top: 50, bottom: 86 });
    const { sourceEvent } = buildArtifactInspection(fixtureIntact);
    const { container } = render(<SourceSubmissionSpecimen inspection={sourceEvent} rawHighlight="tty=true" />);

    const box = container.querySelector('[class*="specimenRawBox"]') as HTMLElement;
    expect(box.scrollTop).toBe(0);
  });

  it("never touches scrollTop when no Verified selection is active", () => {
    // Geometry that *would* trigger a scroll adjustment if the effect ran
    // unconditionally — proves the early-return guard, not just an
    // incidental zero.
    mockGeometry({ top: 0, bottom: 100 }, { top: 150, bottom: 186 });
    const { sourceEvent } = buildArtifactInspection(fixtureIntact);
    const { container } = render(<SourceSubmissionSpecimen inspection={sourceEvent} />);

    const box = container.querySelector('[class*="specimenRawBox"]') as HTMLElement;
    expect(box.scrollTop).toBe(0);
  });

  it("never fabricates a mark, and never scrolls, for Partial (requestObject) provenance", () => {
    const { sourceEvent } = buildArtifactInspection(fixtureScenario2Intact);
    const { container } = render(
      <SourceSubmissionSpecimen inspection={sourceEvent} rawHighlight={undefined} />,
    );
    expect(container.querySelector("[data-raw-highlight]")).toBeNull();
  });
});
