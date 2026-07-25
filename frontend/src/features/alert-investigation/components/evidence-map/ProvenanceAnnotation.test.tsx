import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { ProvenanceAnnotation } from "./ProvenanceAnnotation";
import type { ProvenanceState } from "@/features/alert-investigation/lib/provenance";

describe("ProvenanceAnnotation — Verified", () => {
  it("renders the real raw path and value, never a fabricated one", () => {
    const provenance: ProvenanceState = {
      kind: "verified",
      rawPath: "requestURI",
      rawValue: "/api/v1/.../exec?tty=true",
      behavior: 'Parsed from the requestURI\'s "tty" query parameter.',
      normalizedPath: "exec.tty",
      normalizedValue: "true",
      condition: "The exec request requests interactive terminal (TTY) allocation.",
    };
    render(<ProvenanceAnnotation provenance={provenance} />);
    expect(screen.getByText("requestURI")).toBeInTheDocument();
    expect(screen.getByText("exec.tty")).toBeInTheDocument();
    expect(screen.getByText(/Verified/)).toBeInTheDocument();
  });
});

describe("ProvenanceAnnotation — Partial", () => {
  it("states the real limitation and never claims a raw path", () => {
    const provenance: ProvenanceState = {
      kind: "partial",
      normalizedPath: "podCreation.privileged",
      normalizedValue: "true",
      condition: "A container in the Pod requests privileged mode.",
      limitation: "This field's source location cannot be structurally identified in the current raw-event contract.",
    };
    render(<ProvenanceAnnotation provenance={provenance} />);
    expect(screen.getByText(/cannot be structurally identified/)).toBeInTheDocument();
    expect(screen.queryByText("Raw path")).not.toBeInTheDocument();
    expect(screen.getByText(/Partial/)).toBeInTheDocument();
  });
});

describe("ProvenanceAnnotation — Unavailable", () => {
  it("names the specific missing artifact", () => {
    const provenance: ProvenanceState = {
      kind: "unavailable",
      missingArtifact: "source-event",
      condition: "The exec request enables standard-input streaming.",
      explanation: "The source event is unavailable, so this fact's raw origin cannot be inspected.",
    };
    render(<ProvenanceAnnotation provenance={provenance} />);
    expect(screen.getByText("Source event")).toBeInTheDocument();
    expect(screen.getByText(/Unavailable/)).toBeInTheDocument();
  });
});
