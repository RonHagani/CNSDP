import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { CharacteristicPin } from "./CharacteristicPin";
import type { CharacteristicConcordanceRow } from "@/features/alert-investigation/lib/concordance";

function row(kind: "verified" | "partial" | "unavailable"): CharacteristicConcordanceRow {
  const base = {
    kind: "requires_any" as const,
    id: "tty_allocation",
    description: "TTY allocation.",
    satisfied: true,
    group: "Declared characteristics",
  };
  switch (kind) {
    case "verified":
      return {
        ...base,
        provenance: {
          kind: "verified",
          rawPath: "requestURI",
          rawValue: "...tty=true...",
          behavior: "Parsed from the requestURI's tty query parameter.",
          normalizedPath: "exec.tty",
          normalizedValue: "true",
          condition: base.description,
        },
      };
    case "partial":
      return {
        ...base,
        provenance: {
          kind: "partial",
          normalizedPath: "podCreation.privileged",
          normalizedValue: "true",
          condition: base.description,
          limitation: "This field's source location cannot be structurally identified.",
        },
      };
    case "unavailable":
      return {
        ...base,
        provenance: {
          kind: "unavailable",
          missingArtifact: "source-event",
          condition: base.description,
          explanation: "The source event is unavailable.",
        },
      };
  }
}

function unsatisfiedRow(): CharacteristicConcordanceRow {
  return {
    kind: "requires_any",
    id: "host_pid",
    description: "The Pod uses the host process-identifier (PID) namespace.",
    satisfied: false,
    group: "Host access",
  };
}

describe("CharacteristicPin", () => {
  it("renders the real characteristic id", () => {
    render(<CharacteristicPin row={row("verified")} selected={false} onSelect={vi.fn()} />);
    expect(screen.getByText("tty_allocation")).toBeInTheDocument();
  });

  it.each([
    ["verified", "Verified"],
    ["partial", "Partial"],
    ["unavailable", "Unavailable"],
  ] as const)("labels %s provenance with visible text, not color alone", (kind, label) => {
    render(<CharacteristicPin row={row(kind)} selected={false} onSelect={vi.fn()} />);
    expect(screen.getByText(label)).toBeInTheDocument();
  });

  it("is a native button reflecting selection via aria-pressed", () => {
    render(<CharacteristicPin row={row("verified")} selected={true} onSelect={vi.fn()} />);
    expect(screen.getByRole("button")).toHaveAttribute("aria-pressed", "true");
  });

  it("calls onSelect with the real id on click", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    render(<CharacteristicPin row={row("verified")} selected={false} onSelect={onSelect} />);
    await user.click(screen.getByRole("button"));
    expect(onSelect).toHaveBeenCalledWith("tty_allocation");
  });

  it("is keyboard-operable via Tab focus and Enter", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    render(<CharacteristicPin row={row("verified")} selected={false} onSelect={onSelect} />);
    screen.getByRole("button").focus();
    await user.keyboard("{Enter}");
    expect(onSelect).toHaveBeenCalledWith("tty_allocation");
  });

  it("marks a satisfied pin with data-satisfied=\"true\"", () => {
    render(<CharacteristicPin row={row("verified")} selected={false} onSelect={vi.fn()} />);
    expect(screen.getByRole("button")).toHaveAttribute("data-satisfied", "true");
  });
});

describe("CharacteristicPin — declared but unsatisfied", () => {
  it("renders a plain declared-only label instead of a provenance kind, never fabricating one", () => {
    render(<CharacteristicPin row={unsatisfiedRow()} selected={false} onSelect={vi.fn()} />);
    expect(screen.getByText("host_pid")).toBeInTheDocument();
    expect(screen.getByText("Declared, not satisfied")).toBeInTheDocument();
    expect(screen.queryByText("Verified")).not.toBeInTheDocument();
    expect(screen.queryByText("Partial")).not.toBeInTheDocument();
    expect(screen.queryByText("Unavailable")).not.toBeInTheDocument();
  });

  it("exposes both true and false data-satisfied states, distinguishable structurally, not by color alone", () => {
    const { unmount } = render(
      <CharacteristicPin row={unsatisfiedRow()} selected={false} onSelect={vi.fn()} />,
    );
    expect(screen.getByRole("button")).toHaveAttribute("data-satisfied", "false");
    unmount();

    render(<CharacteristicPin row={row("verified")} selected={false} onSelect={vi.fn()} />);
    expect(screen.getByRole("button")).toHaveAttribute("data-satisfied", "true");
  });

  it("remains independently selectable", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    render(<CharacteristicPin row={unsatisfiedRow()} selected={false} onSelect={onSelect} />);
    await user.click(screen.getByRole("button"));
    expect(onSelect).toHaveBeenCalledWith("host_pid");
  });
});
