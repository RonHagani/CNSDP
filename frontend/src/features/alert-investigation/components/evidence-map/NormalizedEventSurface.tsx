import type { NormalizedEventInspection } from "@/features/alert-investigation/lib/artifactInspection";
import { formatOutcome } from "@/lib/outcome";
import styles from "./evidence-map.module.css";

/**
 * The Normalized Event field surface (UX spec §7): the central artifact,
 * one row per field, generic across whichever single scenario block
 * (`exec` | `podCreation` | `clusterRoleBinding` | `cloudTrailIAMAction`)
 * is populated — no scenario-specific branch in the rendering mechanism
 * itself beyond checking which block is present (a real, contract-typed
 * fact, not an invented distinction).
 *
 * `highlightedPath` is the selected characteristic's own
 * `ProvenanceState.normalizedPath` (Verified/Partial only — Unavailable
 * carries no path) — an already-computed value passed down from
 * `InvestigationMap`, never re-derived here. Pass 1 renders the highlight
 * as a static row treatment; the literal routed connector line from this
 * row into the selected pin (UX spec §12 step 2) is deferred to a later
 * refinement pass.
 */
export function NormalizedEventSurface({
  inspection,
  highlightedPath,
  highlightKind,
  registerHighlightedRowRef,
}: {
  inspection: NormalizedEventInspection;
  highlightedPath?: string;
  /** The selected characteristic's provenance kind, when it carries one
   *  (Verified/Partial only — Unavailable and declared-only selections
   *  have no field to highlight at all) — distinguishes the teal/amber
   *  row treatment without introducing a second highlighting mechanism. */
  highlightKind?: "verified" | "partial";
  /** Attaches the highlighted row's own DOM node so the selection trace
   *  overlay can measure where to route its line — only ever invoked for
   *  the one row matching `highlightedPath`. */
  registerHighlightedRowRef?: (el: HTMLDivElement | null) => void;
}) {
  if (!inspection.available) {
    return (
      <section className={`${styles.artifactShape} ${styles.fieldSurface}`} aria-labelledby="normalized-heading">
        <h2 id="normalized-heading" className={styles.eyebrow}>
          Normalized event
        </h2>
        <p className={styles.statusUnavailable}>Normalized event unavailable</p>
      </section>
    );
  }

  const event = inspection.event;
  const target = [event.target.resource, event.target.subresource].filter(Boolean).join("/");

  const rows: [string, string][] = [
    ["subject.username", event.subject.username],
    ["operation.verb", event.operation.verb],
    ["operation.requestURI", event.operation.requestURI],
    ["target.resource", target || "—"],
    ...(event.target.name ? ([["target.name", event.target.name]] as [string, string][]) : []),
    ...(event.target.namespace
      ? ([["target.namespace", event.target.namespace]] as [string, string][])
      : []),
    ["outcome.code", formatOutcome(event.outcome)],
    ["requestTime", event.requestTime],
  ];

  if (event.exec) {
    rows.push(["exec.stdin", String(event.exec.stdin)], ["exec.tty", String(event.exec.tty)]);
  }
  if (event.podCreation) {
    rows.push(
      ["podCreation.privileged", String(event.podCreation.privileged)],
      ["podCreation.hostNetwork", String(event.podCreation.hostNetwork)],
      ["podCreation.hostPID", String(event.podCreation.hostPID)],
      ["podCreation.hostIPC", String(event.podCreation.hostIPC)],
      ["podCreation.hostPathVolume", String(event.podCreation.hostPathVolume)],
    );
  }
  if (event.clusterRoleBinding) {
    rows.push(
      ["clusterRoleBinding.bindingName", event.clusterRoleBinding.bindingName],
      [
        "clusterRoleBinding.roleRef",
        `${event.clusterRoleBinding.roleRef.kind}/${event.clusterRoleBinding.roleRef.name}`,
      ],
    );
    if (event.clusterRoleBinding.subjects && event.clusterRoleBinding.subjects.length > 0) {
      rows.push([
        "clusterRoleBinding.subjects",
        event.clusterRoleBinding.subjects.map((s) => `${s.kind}:${s.name}`).join(", "),
      ]);
    }
  }
  if (event.cloudTrailIAMAction) {
    const { affectedUser, affectedDevice, policyName } = event.cloudTrailIAMAction;
    if (affectedUser) rows.push(["cloudTrailIAMAction.affectedUser", affectedUser]);
    if (affectedDevice) rows.push(["cloudTrailIAMAction.affectedDevice", affectedDevice]);
    if (policyName) rows.push(["cloudTrailIAMAction.policyName", policyName]);
  }

  return (
    <section className={`${styles.artifactShape} ${styles.fieldSurface}`} aria-labelledby="normalized-heading">
      <h2 id="normalized-heading" className={`${styles.eyebrow} ${styles.fieldSurfaceHeading}`}>
        Normalized event
      </h2>
      {rows.map(([key, value]) => {
        const isHighlighted = key === highlightedPath;
        return (
          <div
            key={key}
            ref={isHighlighted ? registerHighlightedRowRef : undefined}
            className={styles.fieldRow}
            data-highlighted={isHighlighted || undefined}
            data-highlight-kind={isHighlighted ? highlightKind : undefined}
          >
            <span className={`${styles.fieldRowKey} ${styles.truncate}`} title={key}>
              {key}
            </span>
            <span className={`${styles.fieldRowValue} ${styles.truncate}`} title={value}>
              {value}
            </span>
          </div>
        );
      })}
    </section>
  );
}
