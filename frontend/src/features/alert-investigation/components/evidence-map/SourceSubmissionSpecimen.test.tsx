import { describe, expect, it } from "vitest";
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
