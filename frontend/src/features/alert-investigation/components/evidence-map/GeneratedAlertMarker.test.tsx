import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { GeneratedAlertMarker } from "./GeneratedAlertMarker";
import { buildArtifactInspection } from "@/features/alert-investigation/lib/artifactInspection";
import { fixtureIntact, fixtureScenario3Intact } from "@/fixtures/alert-investigation/v1";

describe("GeneratedAlertMarker", () => {
  it("renders the real alert id and a one-line summary from alert.summary", () => {
    const { alert } = buildArtifactInspection(fixtureIntact);
    render(<GeneratedAlertMarker alertId={fixtureIntact.alertId} alert={alert} />);
    expect(screen.getByText(/Alert #1/)).toBeInTheDocument();
    expect(screen.getByText(/kubernetes-admin/)).toBeInTheDocument();
    expect(screen.getByText(/outcome 101/)).toBeInTheDocument();
  });

  it("renders scenario 3's real target, not scenario 1's", () => {
    const { alert } = buildArtifactInspection(fixtureScenario3Intact);
    render(<GeneratedAlertMarker alertId={fixtureScenario3Intact.alertId} alert={alert} />);
    expect(screen.getByText(/spike-crb-jsonpatch/)).toBeInTheDocument();
  });

  it("renders an explicit unavailable statement when the alert summary is unavailable", () => {
    render(<GeneratedAlertMarker alertId={7} alert={{ available: false }} />);
    expect(screen.getByText("Alert summary unavailable")).toBeInTheDocument();
  });
});
