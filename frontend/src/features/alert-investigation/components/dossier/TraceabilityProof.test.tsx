import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { TraceabilityProof } from "./TraceabilityProof";
import { buildTraceabilityProof } from "@/features/alert-investigation/lib/traceability";
import { allArtifactsAvailable } from "@/features/alert-investigation/lib/evidenceRegister";
import { deriveProvenanceState } from "@/features/alert-investigation/lib/provenance";
import {
  fixtureBrokenTraceability,
  fixtureIntact,
  fixtureScenario2Intact,
} from "@/fixtures/alert-investigation/v1";
import type { AlertInvestigationResponse } from "@/types/contract";

function renderProof(fixture: AlertInvestigationResponse) {
  const proof = buildTraceabilityProof(fixture, allArtifactsAvailable(fixture));
  render(<TraceabilityProof traceability={proof} />);
  return proof;
}

describe("TraceabilityProof — intact", () => {
  it("renders a verified statement and the real four-link chain", () => {
    renderProof(fixtureIntact);
    expect(screen.getByText("Traceability verified.")).toBeInTheDocument();
    expect(screen.queryByText(/Failed link/)).not.toBeInTheDocument();
    for (const label of ["Alert", "Detection result", "Normalized event", "Submission"]) {
      expect(screen.getByText(label)).toBeInTheDocument();
    }
  });
});

describe("TraceabilityProof — each failedLink value", () => {
  it("renders the specific, named alert failure", () => {
    const data: AlertInvestigationResponse = {
      ...fixtureIntact,
      traceability: { intact: false, failedLink: "alert" },
    };
    renderProof(data);
    expect(screen.getByText("Traceability broken.")).toBeInTheDocument();
    expect(screen.getByText("Failed link: alert")).toBeInTheDocument();
    expect(screen.getByText(/alert itself does not resolve/i)).toBeInTheDocument();
  });

  it("renders the specific, named source_key failure", () => {
    const data: AlertInvestigationResponse = {
      ...fixtureIntact,
      traceability: { intact: false, failedLink: "source_key" },
    };
    renderProof(data);
    expect(screen.getByText("Failed link: source_key")).toBeInTheDocument();
    expect(screen.getByText(/source identity/i)).toBeInTheDocument();
  });

  it("renders the specific, named raw_event_sha256 failure (fixtureBrokenTraceability)", () => {
    renderProof(fixtureBrokenTraceability);
    expect(screen.getByText("Failed link: raw_event_sha256")).toBeInTheDocument();
    expect(screen.getByText(/integrity digest/i)).toBeInTheDocument();
  });
});

describe("TraceabilityProof — does not treat traceability as an evidence artifact", () => {
  it("renders no accession index or 'artifact' language", () => {
    const { container } = render(
      <TraceabilityProof traceability={buildTraceabilityProof(fixtureIntact, true)} />,
    );
    expect(container.textContent).not.toMatch(/artifact/i);
  });
});

describe("TraceabilityProof — partial provenance does not alter traceability output", () => {
  it("an alert with Partial provenance on a matched characteristic still renders full traceability intact", () => {
    // scenario 2's characteristics are all Partial provenance by real
    // design (requestObject-derived) — confirm that fact has zero effect
    // on this component's output for an intact-traceability fixture.
    const provenance = deriveProvenanceState(
      { id: "privileged_container", description: "d" },
      fixtureScenario2Intact,
    );
    expect(provenance.kind).toBe("partial");

    renderProof(fixtureScenario2Intact);
    expect(screen.getByText("Traceability verified.")).toBeInTheDocument();
  });
});
