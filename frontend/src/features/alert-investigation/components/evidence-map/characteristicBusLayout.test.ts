import { describe, expect, it } from "vitest";
import { buildBusLayout } from "./characteristicBusLayout";
import type { CharacteristicConcordanceRow } from "@/features/alert-investigation/lib/concordance";

function satisfiedRow(id: string, group: string): CharacteristicConcordanceRow {
  return {
    kind: "requires_any",
    id,
    description: `${id} description.`,
    group,
    satisfied: true,
    provenance: {
      kind: "partial",
      normalizedPath: `podCreation.${id}`,
      normalizedValue: "true",
      condition: `${id} description.`,
      limitation: "This field's source location cannot be structurally identified.",
    },
  };
}

function unsatisfiedRow(id: string, group: string): CharacteristicConcordanceRow {
  return {
    kind: "requires_any",
    id,
    description: `${id} description.`,
    group,
    satisfied: false,
  };
}

describe("buildBusLayout — scenario 1 shape (two rows, one ungrouped bucket)", () => {
  const rows = [satisfiedRow("stdin_streaming", "Declared characteristics"), satisfiedRow("tty_allocation", "Declared characteristics")];

  it("renders zero labels — a single shared group never clusters", () => {
    const items = buildBusLayout(rows);
    expect(items.filter((i) => i.kind === "label")).toHaveLength(0);
  });

  it("alternates left/right by declaration order, both pins present", () => {
    const items = buildBusLayout(rows);
    const pins = items.filter((i) => i.kind === "pin");
    expect(pins).toHaveLength(2);
    expect(pins[0]).toMatchObject({ side: "left", row: { id: "stdin_streaming" } });
    expect(pins[1]).toMatchObject({ side: "right", row: { id: "tty_allocation" } });
  });
});

describe("buildBusLayout — scenario 2 shape (five rows, Host access / Privilege split)", () => {
  const rows = [
    satisfiedRow("host_ipc", "Host access"),
    satisfiedRow("host_network", "Host access"),
    satisfiedRow("host_path_volume", "Host access"),
    satisfiedRow("host_pid", "Host access"),
    satisfiedRow("privileged_container", "Privilege"),
  ];

  it("renders exactly two labels, one per distinct group, each before its group's first row", () => {
    const items = buildBusLayout(rows);
    const labels = items.filter((i): i is Extract<typeof i, { kind: "label" }> => i.kind === "label");
    expect(labels.map((l) => l.group)).toEqual(["Host access", "Privilege"]);
  });

  it("places the Privilege label immediately before privileged_container, not before any Host access row", () => {
    const items = buildBusLayout(rows);
    const privilegeLabelIndex = items.findIndex((i) => i.kind === "label" && i.group === "Privilege");
    const privilegedPinIndex = items.findIndex((i) => i.kind === "pin" && i.row.id === "privileged_container");
    expect(privilegeLabelIndex).toBe(privilegedPinIndex - 1);
  });

  it("keeps all five pins, alternating sides strictly by declaration order regardless of grouping", () => {
    const items = buildBusLayout(rows);
    const pins = items.filter((i) => i.kind === "pin");
    expect(pins.map((p) => p.row.id)).toEqual([
      "host_ipc",
      "host_network",
      "host_path_volume",
      "host_pid",
      "privileged_container",
    ]);
    expect(pins.map((p) => p.side)).toEqual(["left", "right", "left", "right", "left"]);
  });
});

describe("buildBusLayout — unknown/single-group fallback", () => {
  it("renders no labels for a lone ungrouped row (scenario 3 shape)", () => {
    const items = buildBusLayout([satisfiedRow("role_ref_cluster_admin", "Declared characteristics")]);
    expect(items.filter((i) => i.kind === "label")).toHaveLength(0);
    expect(items).toHaveLength(1);
  });

  it("clusters honestly under the generic fallback label when an unrecognized id and a real named group are mixed", () => {
    // Not a real current fixture shape, but a genuine contract-permitted one:
    // confirms the mechanism never special-cases UNGROUPED_LABEL by string value.
    const rows = [satisfiedRow("host_network", "Host access"), satisfiedRow("some_future_characteristic", "Declared characteristics")];
    const items = buildBusLayout(rows);
    const labels = items.filter((i): i is Extract<typeof i, { kind: "label" }> => i.kind === "label");
    expect(labels.map((l) => l.group)).toEqual(["Host access", "Declared characteristics"]);
  });
});

describe("buildBusLayout — declared-but-unsatisfied rows", () => {
  it("lays out unsatisfied rows identically to satisfied ones — satisfaction never affects bus geometry", () => {
    const rows = [
      satisfiedRow("host_ipc", "Host access"),
      unsatisfiedRow("host_pid", "Host access"),
      satisfiedRow("privileged_container", "Privilege"),
    ];
    const items = buildBusLayout(rows);
    const pins = items.filter((i) => i.kind === "pin");
    expect(pins.map((p) => p.row.id)).toEqual(["host_ipc", "host_pid", "privileged_container"]);
    expect(pins.map((p) => p.side)).toEqual(["left", "right", "left"]);
    expect(items.filter((i) => i.kind === "label").map((l) => (l as { group: string }).group)).toEqual([
      "Host access",
      "Privilege",
    ]);
  });
});

describe("buildBusLayout — empty input", () => {
  it("returns an empty layout without throwing", () => {
    expect(buildBusLayout([])).toEqual([]);
  });
});
