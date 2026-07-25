import { describe, expect, it } from "vitest";
import { characteristicGroup, UNGROUPED_LABEL } from "./characteristicGroups";

describe("characteristicGroup — scenario 2's real documented split", () => {
  it("groups the four host-namespace/access characteristics together", () => {
    expect(characteristicGroup("host_ipc")).toBe("Host access");
    expect(characteristicGroup("host_network")).toBe("Host access");
    expect(characteristicGroup("host_pid")).toBe("Host access");
    expect(characteristicGroup("host_path_volume")).toBe("Host access");
  });

  it("groups the one privilege characteristic separately", () => {
    expect(characteristicGroup("privileged_container")).toBe("Privilege");
  });

  it("produces exactly two distinct group labels across scenario 2's five real characteristic ids", () => {
    const ids = ["host_ipc", "host_network", "host_path_volume", "host_pid", "privileged_container"];
    const labels = new Set(ids.map(characteristicGroup));
    expect(labels).toEqual(new Set(["Host access", "Privilege"]));
  });
});

describe("characteristicGroup — safe fallback", () => {
  it("resolves scenario 1's characteristics to the shared ungrouped label, not a fabricated category", () => {
    expect(characteristicGroup("stdin_streaming")).toBe(UNGROUPED_LABEL);
    expect(characteristicGroup("tty_allocation")).toBe(UNGROUPED_LABEL);
  });

  it("resolves scenario 3's single characteristic to the shared ungrouped label", () => {
    expect(characteristicGroup("role_ref_cluster_admin")).toBe(UNGROUPED_LABEL);
  });

  it("degrades an entirely unknown, future characteristic id to the same safe fallback rather than throwing", () => {
    expect(() => characteristicGroup("some_future_characteristic_not_yet_declared")).not.toThrow();
    expect(characteristicGroup("some_future_characteristic_not_yet_declared")).toBe(UNGROUPED_LABEL);
  });

  it("scenario 1's two characteristics never collapse into scenario 2's real group labels", () => {
    expect(characteristicGroup("stdin_streaming")).not.toBe("Host access");
    expect(characteristicGroup("stdin_streaming")).not.toBe("Privilege");
    expect(characteristicGroup("tty_allocation")).not.toBe("Host access");
    expect(characteristicGroup("tty_allocation")).not.toBe("Privilege");
  });
});

describe("characteristicGroup — exact mapping, falsifiable against every known id", () => {
  it("maps every currently-declared characteristic id to its exact expected group in one pass", () => {
    const expected: Record<string, string> = {
      host_ipc: "Host access",
      host_network: "Host access",
      host_path_volume: "Host access",
      host_pid: "Host access",
      privileged_container: "Privilege",
      stdin_streaming: UNGROUPED_LABEL,
      tty_allocation: UNGROUPED_LABEL,
      role_ref_cluster_admin: UNGROUPED_LABEL,
    };
    const actual = Object.fromEntries(Object.keys(expected).map((id) => [id, characteristicGroup(id)]));
    expect(actual).toEqual(expected);
  });

  it("the unknown-id fallback is exactly UNGROUPED_LABEL, not an empty string or a different sentinel", () => {
    expect(characteristicGroup("totally_unrecognized_id")).toBe(UNGROUPED_LABEL);
    expect(characteristicGroup("totally_unrecognized_id")).not.toBe("");
    expect(characteristicGroup("totally_unrecognized_id")).not.toBe("Host access");
    expect(characteristicGroup("totally_unrecognized_id")).not.toBe("Privilege");
  });
});
