import type { AlertInvestigationResponse } from "@/types/contract";
import styles from "./InvestigationHero.module.css";

/**
 * The docket header — a compact case-identity record, not a masthead.
 * Structure and register follow the approved flagship reference
 * (docs/design-reference/alert-investigation/flagship-assembly.html): a
 * citation line, a restrained sans-serif title, one metadata line, and a
 * narrow traceability stub. No serif headline, no seal, no glow — every
 * value below is a real response field, never invented.
 */
export function InvestigationHero({
  data,
  onOpenPalette,
}: {
  data: AlertInvestigationResponse;
  onOpenPalette: () => void;
}) {
  const summary = data.alert.summary;
  const matchReason = data.detectionResult.matchReason;

  const target = summary
    ? `${summary.target.resource}/${summary.target.name}` +
      (summary.target.namespace || summary.target.subresource
        ? ` (${summary.target.namespace ?? "—"}·${summary.target.subresource ?? "—"})`
        : "")
    : "—";

  const source = data.sourceEvent.rawEvent
    ? `source ${truncateMiddle(data.sourceEvent.rawEvent.auditID, 8, 6)}`
    : "source unavailable";

  return (
    <header className={styles.hero}>
      <div className={styles.topline}>
        <span className={styles.wordmark}>CNSDP</span>
        <button type="button" className={styles.capsule} onClick={onOpenPalette}>
          <kbd className={styles.kbd}>⌘K</kbd>
          <span>Command</span>
        </button>
      </div>

      <div className={styles.docket}>
        <div className={styles.docketMain}>
          <p className={styles.citation}>
            Alert no. <b>{String(data.alertId).padStart(4, "0")}</b>
            {matchReason?.scenario && (
              <>
                <span className={styles.dot}>·</span>
                {matchReason.scenario}
              </>
            )}
            {data.detectionDefinition.revision && (
              <>
                <span className={styles.dot}>·</span>
                rev. <b>{truncateMiddle(data.detectionDefinition.revision, 8, 8)}</b>
              </>
            )}
          </p>

          <h1 className={styles.title}>
            {matchReason?.definitionName ?? "Detection identity unavailable"}
          </h1>

          <p className={styles.meta}>
            <b>{summary?.subject.username ?? "—"}</b>
            <span className={styles.sep}>—</span>
            <b>{summary?.operation.verb ?? "—"}</b>
            <span className={styles.sep}>—</span>
            {target}
            <span className={styles.sep}>—</span>
            {source}
          </p>
        </div>

        <div className={styles.docketTrace}>
          <p className={styles.traceLabel}>Traceability</p>
          <p className={styles.traceValue} data-intact={data.traceability.intact}>
            {data.traceability.intact ? "Intact" : "Broken"}
          </p>
          <p className={styles.traceSub}>
            {data.traceability.intact
              ? "alert ↔ evidence chain verified"
              : `link failed — ${data.traceability.failedLink ?? "unresolved"}`}
          </p>
        </div>
      </div>
    </header>
  );
}

function truncateMiddle(value: string, headLength: number, tailLength: number): string {
  if (value.length <= headLength + tailLength) return value;
  return `${value.slice(0, headLength)}…${value.slice(-tailLength)}`;
}
