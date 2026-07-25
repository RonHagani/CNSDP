/**
 * Characteristic-group labels (UX spec §8 "Subgroup clustering";
 * implementation plan §3's `lib/characteristicGroups.ts`): a literal
 * id-to-label table for the one real, already-documented semantic split
 * among today's declared characteristics — scenario 2's four host-access
 * characteristics (`host_ipc`, `host_network`, `host_pid`,
 * `host_path_volume`) versus its one privilege characteristic
 * (`privileged_container`). This restates a distinction already present in
 * each characteristic's own `description` text (definitions/scenario-2.yaml)
 * — it never introduces a domain category the definitions do not already
 * imply. Data, not logic: mirrors `provenance.ts`'s own
 * `CHARACTERISTIC_NORMALIZED_PATH` table exactly, so a future scenario's
 * characteristic needs a one-line addition here, never a structural change.
 *
 * Every characteristic id absent from this table — today, scenario 1's
 * `stdin_streaming`/`tty_allocation`, scenario 3's
 * `role_ref_cluster_admin`, and any future/unrecognized id — resolves to
 * `UNGROUPED_LABEL`, one shared, honest fallback, never a fabricated or
 * guessed category. A characteristic list where every member resolves to
 * the same label (whether that is `UNGROUPED_LABEL` or one single real
 * label) has exactly one distinct group; a consumer rendering that as one
 * plain, undivided bus — with no scenario-identity check anywhere —
 * correctly reproduces UX spec §8/§19 item 10's "None (below the
 * two-clustering-cases threshold)" behavior for scenario 1 and scenario 3.
 */

export const UNGROUPED_LABEL = "Declared characteristics";

const CHARACTERISTIC_GROUP: Record<string, string> = {
  host_ipc: "Host access",
  host_network: "Host access",
  host_path_volume: "Host access",
  host_pid: "Host access",
  privileged_container: "Privilege",
};

/**
 * Resolves one characteristic id to its group label. Deterministic and
 * side-effect-free; an id with no explicit entry above safely degrades to
 * `UNGROUPED_LABEL` rather than throwing or returning `undefined`.
 */
export function characteristicGroup(id: string): string {
  return CHARACTERISTIC_GROUP[id] ?? UNGROUPED_LABEL;
}
