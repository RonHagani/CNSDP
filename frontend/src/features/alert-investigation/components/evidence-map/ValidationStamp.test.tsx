import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { ValidationStamp } from "./ValidationStamp";

describe("ValidationStamp", () => {
  it("renders the real outcome value generically for valid", () => {
    render(<ValidationStamp validation={{ available: true, outcome: "valid" }} />);
    expect(screen.getByText("valid")).toBeInTheDocument();
  });

  it("renders generically for a non-valid outcome with its reason, never hardcoded to valid", () => {
    render(
      <ValidationStamp
        validation={{ available: true, outcome: "incomplete", reason: "Missing required field." }}
      />,
    );
    expect(screen.getByText("incomplete")).toBeInTheDocument();
    expect(screen.getByText("Missing required field.")).toBeInTheDocument();
  });

  it("renders no reason line when a valid outcome legitimately carries none", () => {
    render(<ValidationStamp validation={{ available: true, outcome: "valid" }} />);
    expect(screen.queryByText(/Missing/)).not.toBeInTheDocument();
  });

  it("renders an explicit unavailable statement when the artifact is unavailable", () => {
    render(<ValidationStamp validation={{ available: false }} />);
    expect(screen.getByText("Validation outcome unavailable")).toBeInTheDocument();
  });
});
