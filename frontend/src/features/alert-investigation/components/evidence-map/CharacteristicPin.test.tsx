import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { CharacteristicPin } from "./CharacteristicPin";
import type { CharacteristicConcordanceRow } from "@/features/alert-investigation/lib/concordance";

function row(kind: "verified" | "partial" | "unavailable"): CharacteristicConcordanceRow {
  const base = { kind: "requires_any" as const, id: "tty_allocation", description: "TTY allocation." };
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
});
