import type { CharacteristicConcordanceRow } from "@/features/alert-investigation/lib/concordance";

/**
 * Pure layout segmentation for the characteristic bus (UX spec §8): turns
 * the flat `CharacteristicConcordanceRow[]` `concordance.ts` already
 * produces into an ordered list of bus items — a branching pin alternating
 * left/right off the bus, optionally preceded by a subgroup label whenever
 * the row's existing `group` field changes from the previous row's.
 *
 * Consumes only `row.group` (`lib/characteristicGroups.ts`, already
 * computed) — never scenario/fixture identity, never `row.description` or
 * any other presentation text. A response where every row shares one
 * `group` value (today: `UNGROUPED_LABEL` for scenarios 1 and 3, but this
 * rule is generic — it would behave identically for any single real named
 * group too) renders with zero labels, matching UX spec §8's "None, below
 * the two-clustering-cases threshold." Declared-but-unsatisfied rows are
 * laid out exactly like satisfied ones — `satisfied` never affects bus
 * geometry, only `CharacteristicPin`'s own rendered state.
 */

export type BusItem =
  | { kind: "label"; key: string; group: string }
  | { kind: "pin"; key: string; row: CharacteristicConcordanceRow; side: "left" | "right" };

export function buildBusLayout(rows: CharacteristicConcordanceRow[]): BusItem[] {
  const distinctGroupCount = new Set(rows.map((r) => r.group)).size;
  const showLabels = distinctGroupCount > 1;

  const items: BusItem[] = [];
  rows.forEach((row, i) => {
    if (showLabels && (i === 0 || rows[i - 1].group !== row.group)) {
      items.push({ kind: "label", key: `label-${row.group}-${i}`, group: row.group });
    }
    items.push({ kind: "pin", key: row.id, row, side: i % 2 === 0 ? "left" : "right" });
  });
  return items;
}
