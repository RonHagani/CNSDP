import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { NormalizedEventSurface } from "./NormalizedEventSurface";
import { buildArtifactInspection } from "@/features/alert-investigation/lib/artifactInspection";
import {
  fixtureIntact,
  fixtureScenario2Intact,
  fixtureScenario3Intact,
} from "@/fixtures/alert-investigation/v1";

describe("NormalizedEventSurface — scenario 1 (exec)", () => {
  it("renders the core fields and the exec scenario block generically", () => {
    const { normalizedEvent } = buildArtifactInspection(fixtureIntact);
    render(<NormalizedEventSurface inspection={normalizedEvent} />);
    expect(screen.getByText("kubernetes-admin")).toBeInTheDocument();
    expect(screen.getByText("exec.stdin")).toBeInTheDocument();
    expect(screen.getByText("exec.tty")).toBeInTheDocument();
  });

  it("highlights exactly the row matching highlightedPath", () => {
    const { normalizedEvent } = buildArtifactInspection(fixtureIntact);
    const { container } = render(
      <NormalizedEventSurface inspection={normalizedEvent} highlightedPath="exec.tty" />,
    );
    const highlighted = container.querySelectorAll('[data-highlighted="true"]');
    expect(highlighted).toHaveLength(1);
    expect(highlighted[0].textContent).toContain("exec.tty");
  });
});

describe("NormalizedEventSurface — scenario 2 (podCreation)", () => {
  it("renders all five podCreation booleans generically, no scenario-1 assumption", () => {
    const { normalizedEvent } = buildArtifactInspection(fixtureScenario2Intact);
    render(<NormalizedEventSurface inspection={normalizedEvent} />);
    expect(screen.getByText("podCreation.privileged")).toBeInTheDocument();
    expect(screen.getByText("podCreation.hostPathVolume")).toBeInTheDocument();
    expect(screen.queryByText("exec.stdin")).not.toBeInTheDocument();
  });
});

describe("NormalizedEventSurface — scenario 3 (clusterRoleBinding)", () => {
  it("renders roleRef as kind/name and subjects as a flat comma-joined list", () => {
    const { normalizedEvent } = buildArtifactInspection(fixtureScenario3Intact);
    render(<NormalizedEventSurface inspection={normalizedEvent} />);
    expect(screen.getByText("ClusterRole/cluster-admin")).toBeInTheDocument();
    expect(screen.getByText("User:alice@example.com")).toBeInTheDocument();
  });
});

describe("NormalizedEventSurface — unavailable", () => {
  it("renders an explicit unavailable statement", () => {
    render(<NormalizedEventSurface inspection={{ available: false }} />);
    expect(screen.getByText("Normalized event unavailable")).toBeInTheDocument();
  });
});
