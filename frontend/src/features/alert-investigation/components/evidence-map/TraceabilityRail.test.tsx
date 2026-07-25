import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { TraceabilityRail } from "./TraceabilityRail";
import { buildTraceabilityProof } from "@/features/alert-investigation/lib/traceability";
import { allArtifactsAvailable } from "@/features/alert-investigation/lib/evidenceRegister";
import { fixtureBrokenTraceabilitySourceKey } from "@/fixtures/alert-investigation/v1";

describe("TraceabilityRail — intact", () => {
  it("renders all four real chain node labels and no broken segment", () => {
    const model = buildTraceabilityProof({ traceability: { intact: true } } as never, true);
    const { container } = render(<TraceabilityRail traceability={model} />);
    expect(screen.getByText("Source submission")).toBeInTheDocument();
    expect(screen.getByText("Normalized event")).toBeInTheDocument();
    expect(screen.getByText("Detection result")).toBeInTheDocument();
    expect(screen.getByText("Generated alert")).toBeInTheDocument();
    expect(container.querySelector('[data-broken="true"]')).toBeNull();
  });
});

describe("TraceabilityRail — broken, raw_event_sha256", () => {
  it("localizes the break to the submission-to-normalized segment only", () => {
    const model = buildTraceabilityProof(
      { traceability: { intact: false, failedLink: "raw_event_sha256" } } as never,
      true,
    );
    render(<TraceabilityRail traceability={model} />);
    expect(screen.getByTestId("rail-segment-0")).toHaveAttribute("data-broken", "true");
    expect(screen.getByTestId("rail-segment-1")).not.toHaveAttribute("data-broken");
    expect(screen.getByTestId("rail-segment-2")).not.toHaveAttribute("data-broken");
    expect(screen.getByText(/recorded integrity digest/)).toBeInTheDocument();
  });
});

describe("TraceabilityRail — broken, alert", () => {
  it("localizes the break to the result-to-alert segment only", () => {
    const model = buildTraceabilityProof(
      { traceability: { intact: false, failedLink: "alert" } } as never,
      true,
    );
    render(<TraceabilityRail traceability={model} />);
    expect(screen.getByTestId("rail-segment-0")).not.toHaveAttribute("data-broken");
    expect(screen.getByTestId("rail-segment-1")).not.toHaveAttribute("data-broken");
    expect(screen.getByTestId("rail-segment-2")).toHaveAttribute("data-broken", "true");
  });
});

describe("TraceabilityRail — broken, source_key (real fixture)", () => {
  it("localizes the break to the submission-to-normalized segment only, same as raw_event_sha256", () => {
    const model = buildTraceabilityProof(
      fixtureBrokenTraceabilitySourceKey,
      allArtifactsAvailable(fixtureBrokenTraceabilitySourceKey),
    );
    render(<TraceabilityRail traceability={model} />);
    expect(screen.getByTestId("rail-segment-0")).toHaveAttribute("data-broken", "true");
    expect(screen.getByTestId("rail-segment-1")).not.toHaveAttribute("data-broken");
    expect(screen.getByTestId("rail-segment-2")).not.toHaveAttribute("data-broken");
    expect(screen.getByText(/source identity/)).toBeInTheDocument();
  });
});
