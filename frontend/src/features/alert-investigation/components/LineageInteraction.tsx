import { useMemo, useState } from "react";
import { motion, useReducedMotion } from "framer-motion";
import type { AlertInvestigationResponse, NormalizedEvent, RawAuditEvent } from "@/types/contract";
import { JsonTree } from "./JsonTree";
import { buildLineageLinks, resolveHighlights, type LineageLink } from "../lib/lineage";
import { useInvestigationUI } from "../lib/InvestigationUIContext";
import styles from "./LineageInteraction.module.css";

const NORMALIZED_FIELD_ROWS: Array<{ path: string; label: string }> = [
  { path: "subject.username", label: "Requesting subject" },
  { path: "operation.verb", label: "Verb" },
  { path: "operation.requestURI", label: "Request URI" },
  { path: "target.resource", label: "Target resource" },
  { path: "target.name", label: "Target name" },
  { path: "target.namespace", label: "Target namespace" },
  { path: "target.subresource", label: "Target subresource" },
  { path: "outcome.code", label: "Outcome code" },
  { path: "requestTime", label: "Request time" },
  { path: "exec.stdin", label: "Exec: stdin streaming" },
  { path: "exec.tty", label: "Exec: TTY allocation" },
];

function readPath(obj: unknown, path: string): unknown {
  return path.split(".").reduce<unknown>((acc, key) => {
    if (acc && typeof acc === "object" && key in (acc as Record<string, unknown>)) {
      return (acc as Record<string, unknown>)[key];
    }
    return undefined;
  }, obj);
}

function formatValue(value: unknown): string {
  if (typeof value === "boolean") return value ? "true" : "false";
  return String(value);
}

type ExpandedRecord = "raw" | "normalized" | null;

/**
 * The hero interaction and magic moment (product-experience-brief.md §4,
 * this milestone's §C). The section opens with the Decoder Strip — a
 * compact summary instrument (docs/design-reference/alert-investigation/
 * flagship-assembly.html) making the one genuine raw→normalized
 * transformation in this event readable at a glance.
 *
 * Below it, exactly one thing is primary: the selected field's provenance
 * record. Nothing is preselected on load. The complete raw and normalized
 * records are real, full-detail inspectors (the original JsonTree /
 * normalized field list, unmodified internally) but are secondary —
 * reachable through two restrained text actions, at most one open at a
 * time, never both dominating the page by default the way they used to.
 */
export function LineageInteraction({
  sourceEvent,
  normalizedEvent,
}: {
  sourceEvent: AlertInvestigationResponse["sourceEvent"];
  normalizedEvent: AlertInvestigationResponse["normalizedEvent"];
}) {
  const { lineage, selectLineage } = useInvestigationUI();
  const reduceMotion = useReducedMotion();
  const [expandedRecord, setExpandedRecord] = useState<ExpandedRecord>(null);

  const links = useMemo(
    () => buildLineageLinks(normalizedEvent.event, sourceEvent.rawEvent),
    [normalizedEvent.event, sourceEvent.rawEvent],
  );
  const linkableRawPaths = useMemo(() => new Set(links.map((l) => l.rawPath)), [links]);
  const linkableNormalizedPaths = useMemo(
    () => new Set(links.map((l) => l.normalizedPath)),
    [links],
  );
  const highlights = useMemo(() => resolveHighlights(links, lineage), [links, lineage]);
  const spotlightOn = lineage !== null;

  if (!sourceEvent.available || !normalizedEvent.available) {
    return (
      <div className={styles.unavailable} role="note">
        {!sourceEvent.available && <p>Source submission unavailable — lineage cannot be traced.</p>}
        {!normalizedEvent.available && (
          <p>Normalized event unavailable — lineage cannot be traced.</p>
        )}
      </div>
    );
  }

  function toggleRecord(which: Exclude<ExpandedRecord, null>) {
    setExpandedRecord((current) => (current === which ? null : which));
  }

  return (
    <div className={styles.theater} id="lineage-stage" data-spotlight={spotlightOn}>
      {sourceEvent.rawEvent && normalizedEvent.event && (
        <DecoderStrip rawEvent={sourceEvent.rawEvent} normalized={normalizedEvent.event} />
      )}

      <div className={styles.inspection}>
        {highlights.activeLinks.length > 0 ? (
          <ProvenanceRecord
            activeLinks={highlights.activeLinks}
            rawEvent={sourceEvent.rawEvent}
            normalized={normalizedEvent.event}
            reduceMotion={reduceMotion}
          />
        ) : (
          <p className={styles.instruction}>
            Open the source or normalized record below, then select a field to inspect its
            provenance.
          </p>
        )}

        <div className={styles.recordToggles}>
          <button
            type="button"
            className={styles.recordToggle}
            aria-expanded={expandedRecord === "raw"}
            onClick={() => toggleRecord("raw")}
          >
            {expandedRecord === "raw" ? "Hide full source record" : "Inspect full source record"}
          </button>
          <button
            type="button"
            className={styles.recordToggle}
            aria-expanded={expandedRecord === "normalized"}
            onClick={() => toggleRecord("normalized")}
          >
            {expandedRecord === "normalized"
              ? "Hide full normalized record"
              : "Inspect full normalized record"}
          </button>
        </div>

        {expandedRecord === "raw" && (
          <div className={styles.exhibit}>
            <div className={styles.paneLabel}>
              <span>Exhibit A</span>
              <span className={styles.paneLabelSub}>Source submission — raw event</span>
            </div>
            <div
              className={styles.scrollRegion}
              tabIndex={0}
              role="group"
              aria-label="Raw event content, scrollable"
            >
              <JsonTree
                data={sourceEvent.rawEvent}
                linkablePaths={linkableRawPaths}
                highlightedPaths={highlights.rawPaths}
                dimOthers={lineage !== null}
                onSelectPath={(path) => selectLineage({ side: "raw", path })}
              />
            </div>
          </div>
        )}

        {expandedRecord === "normalized" && (
          <div className={styles.manifest}>
            <div className={styles.paneLabel}>
              <span>Manifest</span>
              <span className={styles.paneLabelSub}>Normalized event — extracted facts</span>
            </div>
            <div
              className={styles.scrollRegion}
              tabIndex={0}
              role="group"
              aria-label="Normalized event content, scrollable"
            >
              <dl className={styles.fieldList}>
                {NORMALIZED_FIELD_ROWS.map(({ path, label }) => {
                  const value = readPath(normalizedEvent.event, path);
                  if (value === undefined) return null;
                  const linkable = linkableNormalizedPaths.has(path);
                  const highlighted = highlights.normalizedPaths.has(path);
                  const dimmed = lineage !== null && !highlighted;
                  return (
                    <div key={path} className={styles.fieldRow}>
                      <dt className={styles.fieldLabel}>{label}</dt>
                      <dd className={styles.fieldValue}>
                        {linkable ? (
                          <button
                            type="button"
                            className={styles.fieldButton}
                            data-highlighted={highlighted || undefined}
                            data-dimmed={dimmed || undefined}
                            data-lineage-path={path}
                            aria-pressed={highlighted}
                            onClick={() => selectLineage({ side: "normalized", path })}
                          >
                            {formatValue(value)}
                          </button>
                        ) : (
                          <span data-dimmed={dimmed || undefined}>{formatValue(value)}</span>
                        )}
                      </dd>
                    </div>
                  );
                })}
              </dl>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

/**
 * The focused selected-field record — one ruled forensic entry, not a
 * card: raw path/value, the real preservation-or-transformation behavior
 * already documented by the lineage mapping (lib/lineage.ts), and the
 * normalized path/value(s) it feeds. When a shared raw field feeds more
 * than one normalized fact (e.g. the requestURI's query string), the raw
 * side is shown once against every destination it maps to, rather than
 * repeating it.
 */
function ProvenanceRecord({
  activeLinks,
  rawEvent,
  normalized,
  reduceMotion,
}: {
  activeLinks: LineageLink[];
  rawEvent: RawAuditEvent | undefined;
  normalized: NormalizedEvent | undefined;
  reduceMotion: boolean | null;
}) {
  const first = activeLinks[0];
  if (!first) return null;

  return (
    <motion.div
      className={styles.record}
      initial={reduceMotion ? false : { opacity: 0 }}
      animate={{ opacity: 1 }}
      transition={{ duration: reduceMotion ? 0 : 0.15 }}
    >
      <div className={styles.recordRaw}>
        <p className={styles.recordKicker}>Raw</p>
        <p className={styles.recordPath}>{first.rawPath}</p>
        <p className={styles.recordValue}>{formatValue(readPath(rawEvent, first.rawPath))}</p>
      </div>

      <div className={styles.recordLeader} aria-hidden="true" />

      <div className={styles.recordDestinations}>
        {activeLinks.map((link) => (
          <div key={link.normalizedPath} className={styles.recordDestination}>
            <p className={styles.recordBehavior}>{link.label}</p>
            <p className={styles.recordKicker}>Normalized</p>
            <p className={styles.recordPath}>{link.normalizedPath}</p>
            <p className={styles.recordValue}>
              {formatValue(readPath(normalized, link.normalizedPath))}
            </p>
          </div>
        ))}
      </div>
    </motion.div>
  );
}

/**
 * The Decoder Strip — the section's summary/primary visual layer, matching
 * the approved flagship reference. Every value below is read directly from
 * the real raw and normalized events; nothing here is invented. It
 * dramatizes the one genuine parse in this pipeline (requestURI's stdin/tty
 * query parameters → normalized booleans, ADR-0003 Q1) — every other
 * lineage link is a verbatim copy, which is why it is the one link worth a
 * dedicated instrument rather than a table row.
 */
function DecoderStrip({
  rawEvent,
  normalized,
}: {
  rawEvent: RawAuditEvent;
  normalized: NormalizedEvent;
}) {
  const totalParams = countQueryParams(rawEvent.requestURI);
  const stdinRaw = readQueryParam(rawEvent.requestURI, "stdin");
  const ttyRaw = readQueryParam(rawEvent.requestURI, "tty");
  const hasExec = normalized.exec !== undefined;
  const relevantCount = [stdinRaw, ttyRaw].filter((v) => v !== undefined).length;

  const rawTarget = formatTarget(
    rawEvent.objectRef?.resource,
    rawEvent.objectRef?.name,
    rawEvent.objectRef?.namespace,
    rawEvent.objectRef?.subresource,
  );
  const normTarget = formatTarget(
    normalized.target.resource,
    normalized.target.name,
    normalized.target.namespace,
    normalized.target.subresource,
  );

  return (
    <div className={styles.instrument}>
      <p className={`${styles.instrumentRail} ${styles.instrumentRailRaw}`}>
        <b>Raw source event</b>
        {rawEvent.verb.toUpperCase()} {rawTarget} · requestURI query, {totalParams} parameter
        {totalParams === 1 ? "" : "s"} · auditID {truncateMiddle(rawEvent.auditID, 8, 6)}
      </p>

      <div className={styles.strip}>
        <div className={`${styles.stage} ${styles.stageOrigin}`}>
          <p className={styles.stageTag}>Origin</p>
          <p className={styles.stageIdx}>01</p>
          <p className={styles.stageLbl}>Extract query parameters</p>
          <p className={styles.stageContent}>
            {hasExec ? `stdin=${stdinRaw ?? "—"} · tty=${ttyRaw ?? "—"}` : "No boolean characteristics"}
          </p>
        </div>

        <div className={`${styles.stage} ${styles.stageParse}`}>
          <p className={styles.stageIdx}>02</p>
          <p className={styles.stageLbl}>Parse &amp; preserve booleans</p>
          <p className={styles.stageContent}>
            {hasExec
              ? `"${stdinRaw ?? ttyRaw ?? "true"}" → ${formatValue(normalized.exec?.stdin)} / ${formatValue(normalized.exec?.tty)}`
              : "—"}
          </p>
          <div className={styles.opLeader} />
          <p className={styles.rhTag}>Read Head</p>
          <div className={styles.rhLeader} />
        </div>

        <div className={`${styles.stage} ${styles.stageDest}`}>
          <p className={styles.stageTag}>Destination</p>
          <p className={styles.stageIdx}>03</p>
          <p className={styles.stageLbl}>Emit normalized facts</p>
          <p className={styles.stageContent}>
            {hasExec ? (
              <>
                exec.stdin={formatValue(normalized.exec?.stdin)}
                <br />
                exec.tty={formatValue(normalized.exec?.tty)}
              </>
            ) : (
              "—"
            )}
          </p>
        </div>

        {/* One continuous spanning row, not per-column segments — the
            datum line and the gate that interrupts it share this row so
            there is no grid-gap break in the line outside the gate. */}
        <div className={styles.datumRow}>
          <div className={`${styles.datumSeg} ${styles.datumSegLeft}`} />
          <div className={`${styles.datumSeg} ${styles.datumSegRight}`} />
          <div className={styles.gate}>
            <div className={styles.rhJaws} />
          </div>
        </div>

        <p className={`${styles.anchorNote} ${styles.anchorOrigin}`}>
          query.stdin, query.tty &middot; {relevantCount} of {totalParams} relevant
        </p>
        <p className={`${styles.anchorNote} ${styles.anchorParse}`}>
          string &#8594; bool &middot; Normalize(), single-pass
        </p>
        <p className={`${styles.anchorNote} ${styles.anchorDest}`}>
          event.exec.stdin, event.exec.tty
        </p>
      </div>

      <p className={`${styles.instrumentRail} ${styles.instrumentRailNorm}`}>
        <b>Normalized event</b>
        subject {normalized.subject.username} · target {normTarget} · outcome{" "}
        {normalized.outcome.code ?? "—"} · requestTime {normalized.requestTime}
      </p>
    </div>
  );
}

function formatTarget(
  resource: string | undefined,
  name: string | undefined,
  namespace: string | undefined,
  subresource: string | undefined,
): string {
  if (!resource || !name) return "—";
  const base = `${resource}/${name}`;
  if (!namespace && !subresource) return base;
  return `${base} (${namespace ?? "—"}·${subresource ?? "—"})`;
}

function countQueryParams(requestURI: string): number {
  const queryIndex = requestURI.indexOf("?");
  if (queryIndex === -1) return 0;
  return Array.from(new URLSearchParams(requestURI.slice(queryIndex + 1)).entries()).length;
}

function readQueryParam(requestURI: string, key: string): string | undefined {
  const queryIndex = requestURI.indexOf("?");
  if (queryIndex === -1) return undefined;
  return new URLSearchParams(requestURI.slice(queryIndex + 1)).get(key) ?? undefined;
}

function truncateMiddle(value: string, headLength: number, tailLength: number): string {
  if (value.length <= headLength + tailLength) return value;
  return `${value.slice(0, headLength)}…${value.slice(-tailLength)}`;
}
