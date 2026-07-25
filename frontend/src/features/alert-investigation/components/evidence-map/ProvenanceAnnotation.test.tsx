import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { ProvenanceAnnotation } from "./ProvenanceAnnotation";
import type { CharacteristicConcordanceRow } from "@/features/alert-investigation/lib/concordance";

describe("ProvenanceAnnotation — Verified", () => {
  it("renders the real raw path and value, never a fabricated one", () => {
    const row: CharacteristicConcordanceRow = {
      kind: "requires_any",
      id: "tty_allocation",
      description: "The exec request requests interactive terminal (TTY) allocation.",
      satisfied: true,
      group: "Declared characteristics",
      provenance: {
        kind: "verified",
        rawPath: "requestURI",
        rawValue: "/api/v1/.../exec?tty=true",
        behavior: 'Parsed from the requestURI\'s "tty" query parameter.',
        normalizedPath: "exec.tty",
        normalizedValue: "true",
        condition: "The exec request requests interactive terminal (TTY) allocation.",
      },
    };
    render(<ProvenanceAnnotation row={row} />);
    expect(screen.getByText("requestURI")).toBeInTheDocument();
    expect(screen.getByText("exec.tty")).toBeInTheDocument();
    expect(screen.getByText(/Verified/)).toBeInTheDocument();
  });
});

describe("ProvenanceAnnotation — Partial", () => {
  it("states the real limitation and never claims a raw path", () => {
    const row: CharacteristicConcordanceRow = {
      kind: "requires_any",
      id: "privileged_container",
      description: "A container in the Pod requests privileged mode.",
      satisfied: true,
      group: "Privilege",
      provenance: {
        kind: "partial",
        normalizedPath: "podCreation.privileged",
        normalizedValue: "true",
        condition: "A container in the Pod requests privileged mode.",
        limitation:
          "This field's source location cannot be structurally identified in the current raw-event contract.",
      },
    };
    render(<ProvenanceAnnotation row={row} />);
    expect(screen.getByText(/cannot be structurally identified/)).toBeInTheDocument();
    expect(screen.queryByText("Raw path")).not.toBeInTheDocument();
    expect(screen.getByText(/Partial/)).toBeInTheDocument();
  });
});

describe("ProvenanceAnnotation — Unavailable", () => {
  it("names the specific missing artifact", () => {
    const row: CharacteristicConcordanceRow = {
      kind: "requires_any",
      id: "stdin_streaming",
      description: "The exec request enables standard-input streaming.",
      satisfied: true,
      group: "Declared characteristics",
      provenance: {
        kind: "unavailable",
        missingArtifact: "source-event",
        condition: "The exec request enables standard-input streaming.",
        explanation: "The source event is unavailable, so this fact's raw origin cannot be inspected.",
      },
    };
    render(<ProvenanceAnnotation row={row} />);
    expect(screen.getByText("Source event")).toBeInTheDocument();
    expect(screen.getByText(/Unavailable/)).toBeInTheDocument();
  });
});

describe("ProvenanceAnnotation — declared but unsatisfied", () => {
  it("renders the plain declared-only statement, never a fabricated provenance record", () => {
    const row: CharacteristicConcordanceRow = {
      kind: "requires_any",
      id: "host_pid",
      description: "The Pod uses the host process-identifier (PID) namespace.",
      satisfied: false,
      group: "Host access",
    };
    render(<ProvenanceAnnotation row={row} />);
    expect(screen.getByText("Declared, not satisfied")).toBeInTheDocument();
    expect(screen.getByText(row.description)).toBeInTheDocument();
    expect(
      screen.getByText("Declared in this definition; not satisfied by this alert's recorded event."),
    ).toBeInTheDocument();
    expect(screen.queryByText(/Verified/)).not.toBeInTheDocument();
    expect(screen.queryByText(/Partial/)).not.toBeInTheDocument();
  });
});
