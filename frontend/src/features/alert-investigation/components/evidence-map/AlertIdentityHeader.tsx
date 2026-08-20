import type { DocketHeaderViewModel } from "@/features/alert-investigation/lib/investigationViewModel";
import type { FindingViewModel } from "@/features/alert-investigation/lib/finding";
import { formatOutcome } from "@/lib/outcome";
import styles from "./evidence-map.module.css";

/**
 * The permanent alert identity header (UX spec §4): one composed sentence
 * plus quiet technical metadata — alert id, matched detection name, actor/
 * operation/target, recorded outcome, request time, pinned revision, and a
 * compact traceability-state indicator. Carries what v0.1's Docket Header
 * and Finding split across two sections; there is no separate CLAIM
 * section here (UX spec §21 retires that split).
 *
 * `docketHeader`/`finding` are the same two root view-model slices the
 * (unmodified) `lib/investigationViewModel.ts` and `lib/finding.ts`
 * already produce — this component invents no new field.
 *
 * `DocketHeaderViewModel`'s own name is scheduled to be renamed to match
 * this document's vocabulary (implementation plan §3, Pass 2) — not done
 * in this pass, since Pass 2 is out of scope; the type is consumed here
 * under its current, unchanged name.
 */
export function AlertIdentityHeader({
  docketHeader,
  finding,
}: {
  docketHeader: DocketHeaderViewModel;
  finding: FindingViewModel;
}) {
  const { alertId, matchedDefinitionName, pinnedRevision, traceabilityIntact, failedLink } = docketHeader;

  return (
    <header className={`${styles.artifactShape} ${styles.header}`} aria-label="Alert identity">
      <p className={`${styles.headerSentence} ${styles.wrapLongValue}`}>
        <strong>Alert #{alertId}</strong> · {matchedDefinitionName ?? "detection identity unavailable"}
        {finding.available && (
          <>
            {" — "}
            <span className={styles.technical}>{finding.subject?.username ?? "—"}</span>{" "}
            {finding.operation?.verb ?? "—"}{" "}
            <span className={styles.technical}>
              {[finding.target?.resource, finding.target?.subresource].filter(Boolean).join("/") || "—"}
            </span>
            {" · outcome "}
            <span className={styles.technical}>{finding.outcome ? formatOutcome(finding.outcome) : "—"}</span>
          </>
        )}
      </p>

      {!finding.available && <p className={styles.eyebrow}>Alert summary unavailable</p>}

      <div className={styles.headerMeta}>
        {finding.requestTime && <span>{finding.requestTime}</span>}
        {pinnedRevision && <span title={pinnedRevision}>rev. {pinnedRevision.slice(0, 12)}…</span>}
        <span
          className={styles.traceabilityChip}
          data-state={traceabilityIntact ? "intact" : "broken"}
        >
          {traceabilityIntact ? "Traceability intact" : "Traceability broken"}
          {!traceabilityIntact && failedLink && (
            <span className={styles.technical}>&nbsp;· {failedLink}</span>
          )}
        </span>
      </div>
    </header>
  );
}
