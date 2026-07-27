import { useLayoutEffect, useRef, useState } from "react";
import type { SourceEventInspection } from "@/features/alert-investigation/lib/artifactInspection";
import { JsonTree } from "../JsonTree";
import styles from "./evidence-map.module.css";

const EMPTY_PATHS = new Set<string>();
const RAW_HIGHLIGHT_SELECTOR = "[data-raw-highlight='true']";

/**
 * Splits `text` around the first occurrence of `highlight` and wraps it in
 * a `<mark>` — used only for a Verified characteristic's real, located
 * substring (`ProvenanceState.rawHighlight`, `lib/provenance.ts`). Renders
 * the plain text unchanged when `highlight` is absent or genuinely not
 * found, never approximating or fabricating a match. `data-raw-highlight`
 * is a plain attribute, not the CSS-module class, so the scroll-into-view
 * effect below can find this exact node without depending on the module's
 * generated class name.
 */
function renderHighlightedText(text: string, highlight: string | undefined) {
  if (!highlight) return text;
  const index = text.indexOf(highlight);
  if (index < 0) return text;
  return (
    <>
      {text.slice(0, index)}
      <mark className={styles.specimenRawHighlight} data-raw-highlight="true">
        {text.slice(index, index + highlight.length)}
      </mark>
      {text.slice(index + highlight.length)}
    </>
  );
}

/**
 * The Source Submission specimen (UX spec §5): the origin object, in
 * three visual tiers — identity/request essentials, secondary ingestion
 * metadata, and the raw payload (last, quietest, never hidden). The raw
 * payload is `requestURI` for scenario 1 or a real excerpt of
 * `requestObject` for scenarios 2/3 (both real, contract-typed fields —
 * this distinction is not invented, it follows directly from which field
 * the response actually carries). The on-demand "view full raw record"
 * affordance reuses `JsonTree` unchanged for the complete `RawAuditEvent`
 * (UX spec §5's `annotations`/full `sourceIPs`/complete `requestObject`
 * requirement) — explicit and never auto-expanded.
 *
 * Verified-only raw substring marking: when the selected characteristic is
 * Verified and its exact locator is present verbatim in `requestURI`
 * (`ProvenanceState.rawHighlight`, computed in `lib/provenance.ts`),
 * `renderHighlightedText` wraps that literal substring in a `<mark>` — the
 * one honest case where the raw field's own text, not just the containing
 * field, can be pointed to. Partial provenance never gets this treatment:
 * `provenance.ts` has no located substring for a `requestObject`-derived
 * characteristic (its own annotation says so explicitly), so nothing here
 * invents one — the `requestObject` branch below passes `JsonTree` an
 * always-empty `highlightedPaths`, on purpose, not as a deferred gap.
 * Because the raw-payload box scrolls its own overflow (`.specimenRawBox`),
 * a newly-marked substring can render outside its visible area without
 * intervention; the layout effect below scrolls it into that box's own
 * view, never the page's.
 */
export function SourceSubmissionSpecimen({
  inspection,
  rawHighlight,
}: {
  inspection: SourceEventInspection;
  /** The selected Verified characteristic's exact raw substring
   *  (`ProvenanceState.rawHighlight`), when one was honestly located —
   *  never present for Partial/Unavailable selections or a Verified fact
   *  whose locator couldn't be found verbatim in the raw text. */
  rawHighlight?: string;
}) {
  const [fullRecordExpanded, setFullRecordExpanded] = useState(false);
  const rawBoxRef = useRef<HTMLDivElement>(null);

  /**
   * Scrolls a newly-marked Verified substring into its own raw-payload
   * box's visible area — container-local only (`Element.scrollTop`, never
   * `window.scroll*` or `scrollIntoView`, which can walk up and move
   * ancestor/page scroll positions too). Keyed on `rawHighlight` itself,
   * not on every render, so it only runs when the selection actually
   * produces a new (or newly-absent) mark, never on the box's own ordinary
   * user-driven scrolling. Does nothing when the mark is already fully
   * within the box's visible bounds — the two branches below only ever
   * move `scrollTop` when the mark's real, measured position is above or
   * below that bound, never unconditionally.
   */
  useLayoutEffect(() => {
    if (!rawHighlight) return;
    const container = rawBoxRef.current;
    const mark = container?.querySelector<HTMLElement>(RAW_HIGHLIGHT_SELECTOR);
    if (!container || !mark) return;

    const containerRect = container.getBoundingClientRect();
    const markRect = mark.getBoundingClientRect();
    if (markRect.top < containerRect.top) {
      container.scrollTop -= containerRect.top - markRect.top;
    } else if (markRect.bottom > containerRect.bottom) {
      container.scrollTop += markRect.bottom - containerRect.bottom;
    }
  }, [rawHighlight]);

  if (!inspection.available) {
    return (
      <section className={`${styles.artifactShape} ${styles.specimen}`} aria-labelledby="specimen-heading">
        <h2 id="specimen-heading" className={styles.eyebrow}>
          Source submission
        </h2>
        <p className={styles.statusUnavailable}>Source event unavailable</p>
      </section>
    );
  }

  const raw = inspection.rawEvent;
  const target = [raw.objectRef?.resource, raw.objectRef?.subresource].filter(Boolean).join("/");

  return (
    <section className={`${styles.artifactShape} ${styles.specimen}`} aria-labelledby="specimen-heading">
      <h2 id="specimen-heading" className={styles.eyebrow}>
        Source submission
      </h2>

      <div className={styles.specimenTier1}>
        <p className={`${styles.technical} ${styles.wrapLongValue}`}>auditID {raw.auditID}</p>
        <p className={styles.technical}>verb {raw.verb}</p>
        <p className={`${styles.technical} ${styles.wrapLongValue}`}>user {raw.user.username}</p>
        <p className={`${styles.technical} ${styles.wrapLongValue}`}>
          objectRef {target || "—"}
          {raw.objectRef?.name ? ` (${raw.objectRef.name})` : ""}
          {raw.objectRef?.namespace ? ` ns:${raw.objectRef.namespace}` : ""}
        </p>
        <p className={styles.technical}>response.code {raw.responseStatus?.code ?? "—"}</p>
      </div>

      <div className={styles.specimenTier2}>
        <p>
          {raw.level ?? "—"} / {raw.stage}
        </p>
        {raw.sourceIPs && raw.sourceIPs.length > 0 && (
          <p className={styles.wrapLongValue}>sourceIPs {raw.sourceIPs.join(", ")}</p>
        )}
        {raw.userAgent && <p className={styles.wrapLongValue}>userAgent {raw.userAgent}</p>}
        <p className={styles.wrapLongValue}>received {raw.requestReceivedTimestamp}</p>
      </div>

      <div className={styles.specimenTier3}>
        <p className={styles.eyebrow}>Raw payload</p>
        <div className={styles.specimenRawBox} ref={rawBoxRef}>
          {raw.requestObject !== undefined ? (
            <JsonTree
              data={raw.requestObject}
              linkablePaths={EMPTY_PATHS}
              highlightedPaths={EMPTY_PATHS}
              dimOthers={false}
              onSelectPath={() => {}}
            />
          ) : (
            <p className={`${styles.technical} ${styles.wrapLongValue}`}>
              {renderHighlightedText(raw.requestURI, rawHighlight)}
            </p>
          )}
        </div>

        <button
          type="button"
          className={styles.plainButton}
          aria-expanded={fullRecordExpanded}
          onClick={() => setFullRecordExpanded((v) => !v)}
        >
          {fullRecordExpanded ? "Hide full raw record" : "View full raw record"}
        </button>
        {fullRecordExpanded && (
          <div className={styles.specimenRawBox}>
            <JsonTree
              data={raw}
              linkablePaths={EMPTY_PATHS}
              highlightedPaths={EMPTY_PATHS}
              dimOthers={false}
              onSelectPath={() => {}}
            />
          </div>
        )}
      </div>
    </section>
  );
}
