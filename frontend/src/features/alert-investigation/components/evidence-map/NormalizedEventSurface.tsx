import type { NormalizedEventInspection } from "@/features/alert-investigation/lib/artifactInspection";
import styles from "./evidence-map.module.css";

/**
 * The Normalized Event field surface (UX spec §7): the central artifact,
 * one row per field, generic across whichever single scenario block
 * (`exec` | `podCreation` | `clusterRoleBinding`) is populated — no
 * scenario-specific branch in the rendering mechanism itself beyond
 * checking which block is present (a real, contract-typed fact, not an
 * invented distinction).
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
}: {
  inspection: NormalizedEventInspection;
  highlightedPath?: string;
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
    ["outcome.code", String(event.outcome.code ?? "—")],
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

  return (
    <section className={`${styles.artifactShape} ${styles.fieldSurface}`} aria-labelledby="normalized-heading">
      <h2 id="normalized-heading" className={`${styles.eyebrow} ${styles.fieldSurfaceHeading}`}>
        Normalized event
      </h2>
      {rows.map(([key, value]) => (
        <div key={key} className={styles.fieldRow} data-highlighted={key === highlightedPath || undefined}>
          <span className={styles.fieldRowKey}>{key}</span>
          <span className={`${styles.fieldRowValue} ${styles.wrapLongValue}`}>{value}</span>
        </div>
      ))}
    </section>
  );
}
