import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { CommandPalette, type Command } from "./CommandPalette";

function buildCommands(run: (id: string) => void): Command[] {
  return [
    { id: "focus-intake", label: "Focus stage: Intake", run: () => run("focus-intake") },
    {
      id: "focus-validation",
      label: "Focus stage: Validation",
      run: () => run("focus-validation"),
    },
    {
      id: "inspect-traceability",
      label: "Inspect traceability integrity",
      run: () => run("inspect-traceability"),
    },
  ];
}

describe("CommandPalette", () => {
  it("does not render its list when closed", () => {
    render(
      <CommandPalette open={false} onOpenChange={() => {}} commands={buildCommands(() => {})} />,
    );
    expect(screen.queryByRole("combobox")).not.toBeInTheDocument();
  });

  it("filters commands as the query changes", async () => {
    const user = userEvent.setup();
    render(<CommandPalette open onOpenChange={() => {}} commands={buildCommands(() => {})} />);

    const input = screen.getByPlaceholderText(/Jump to a stage/);
    await user.type(input, "traceability");

    expect(screen.getByText("Inspect traceability integrity")).toBeInTheDocument();
    expect(screen.queryByText("Focus stage: Intake")).not.toBeInTheDocument();
  });

  it("runs the highlighted command and closes on Enter — no command for a capability the product lacks", async () => {
    const user = userEvent.setup();
    const ran: string[] = [];
    const onOpenChange = vi.fn();
    render(
      <CommandPalette
        open
        onOpenChange={onOpenChange}
        commands={buildCommands((id) => ran.push(id))}
      />,
    );

    const input = screen.getByPlaceholderText(/Jump to a stage/);
    await user.type(input, "validation{Enter}");

    expect(ran).toEqual(["focus-validation"]);
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("closes on Escape without running a command", async () => {
    const user = userEvent.setup();
    const ran: string[] = [];
    const onOpenChange = vi.fn();
    render(
      <CommandPalette
        open
        onOpenChange={onOpenChange}
        commands={buildCommands((id) => ran.push(id))}
      />,
    );

    await user.keyboard("{Escape}");
    expect(ran).toEqual([]);
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });
});
