import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { ProvenanceRecord } from "./ProvenanceRecord";
import { deriveProvenanceState } from "@/features/alert-investigation/lib/provenance";
import { fixtureIntact, fixtureScenario2Intact } from "@/fixtures/alert-investigation/v1";
import type { AlertInvestigationResponse } from "@/types/contract";

describe("ProvenanceRecord — Verified", () => {
  it("renders documented condition, normalized path/value, transformation behavior, and raw path/value", () => {
    const provenance = deriveProvenanceState(
      { id: "stdin_streaming", description: "The exec request enables standard-input streaming." },
      fixtureIntact,
    );
    render(<ProvenanceRecord provenance={provenance} />);

    expect(screen.getByText("The exec request enables standard-input streaming.")).toBeInTheDocument();
    expect(screen.getByText("exec.stdin")).toBeInTheDocument();
    expect(screen.getByText("true")).toBeInTheDocument();
    expect(screen.getByText("requestURI")).toBeInTheDocument();
    expect(screen.getByText(/stdin=true/)).toBeInTheDocument();
  });
});

describe("ProvenanceRecord — Partial", () => {
  it("renders documented condition, normalized fact, and a restrained limitation explanation, with no raw path claimed", () => {
    const provenance = deriveProvenanceState(
      { id: "privileged_container", description: "A container in the Pod requests privileged mode." },
      fixtureScenario2Intact,
    );
    render(<ProvenanceRecord provenance={provenance} />);

    expect(screen.getByText("A container in the Pod requests privileged mode.")).toBeInTheDocument();
    expect(screen.getByText("podCreation.privileged")).toBeInTheDocument();
    expect(screen.queryByText("Raw path")).not.toBeInTheDocument();
    expect(screen.queryByText("requestObject")).not.toBeInTheDocument();
  });
});

describe("ProvenanceRecord — Unavailable", () => {
  it("renders the unavailable artifact and explanation, with no fabricated source path", () => {
    const data: AlertInvestigationResponse = { ...fixtureIntact, sourceEvent: { available: false } };
    const provenance = deriveProvenanceState(
      { id: "stdin_streaming", description: "The exec request enables standard-input streaming." },
      data,
    );
    render(<ProvenanceRecord provenance={provenance} />);

    expect(screen.getByText("Source event")).toBeInTheDocument();
    expect(screen.getByText(/raw origin cannot be inspected/i)).toBeInTheDocument();
    expect(screen.queryByText("requestURI")).not.toBeInTheDocument();
  });
});

describe("ProvenanceRecord — no generic warning language", () => {
  it("never renders a severity word for any of the three states", () => {
    const states = [
      deriveProvenanceState({ id: "stdin_streaming", description: "d" }, fixtureIntact),
      deriveProvenanceState({ id: "privileged_container", description: "d" }, fixtureScenario2Intact),
      deriveProvenanceState({ id: "stdin_streaming", description: "d" }, {
        ...fixtureIntact,
        sourceEvent: { available: false },
      }),
    ];
    for (const provenance of states) {
      const { container, unmount } = render(<ProvenanceRecord provenance={provenance} />);
      const text = container.textContent!.toLowerCase();
      for (const forbidden of ["warning", "severity", "critical", "danger"]) {
        expect(text).not.toContain(forbidden);
      }
      unmount();
    }
  });
});
