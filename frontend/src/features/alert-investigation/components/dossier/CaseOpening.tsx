import type { DocketHeaderViewModel } from "@/features/alert-investigation/lib/investigationViewModel";
import type { FindingViewModel } from "@/features/alert-investigation/lib/finding";
import styles from "./dossier.module.css";

/**
 * The Case Opening (Composition Reset §2): one connected reading that
 * replaces the former Docket Header and Finding sections outright. There
 * is no rule, container, or heading break between "who this alert is" and
 * "what happened" here — both come from the same case-identity artifact
 * (`alert.summary` / `matchReason`), so they read as one identifying edge,
 * not two stacked sections.
 *
 * The investigation title is the strongest identification text and the
 * page's one `<h1>` — it already carries the matched-definition identity,
 * so the observed act below it states only what happened (subject,
 * operation, target, outcome, time), never repeating "matched detection"
 * prose the title and citation line already convey. Nothing here infers
 * severity, intent, confidence, or maliciousness; nothing is fabricated
 * beyond what the two view models carry.
 */
export function CaseOpening({
  docketHeader,
  finding,
  onOpenCommandPalette,
}: {
  docketHeader: DocketHeaderViewModel;
  finding: FindingViewModel;
  onOpenCommandPalette?: () => void;
}) {
  const { alertId, matchedScenario, matchedDefinitionName, pinnedRevision, traceabilityIntact } =
    docketHeader;

  const targetLabel = finding.target
    ? [finding.target.resource, finding.target.subresource].filter(Boolean).join("/")
    : "—";

  return (
    <header className={`${styles.dossierRoot} ${styles.caseOpening}`} aria-label="Alert identity">
      <p className={`${styles.citationLine} ${styles.wrapLongValue}`}>
        Alert #{alertId}
        {matchedScenario ? ` · ${matchedScenario}` : ""}
        {pinnedRevision ? (
          <>
            {" · rev. "}
            <span className={styles.citationRevision} title={pinnedRevision}>
              {pinnedRevision}
            </span>
          </>
        ) : (
          ""
        )}
      </p>

      <h1 className={styles.caseTitle}>{matchedDefinitionName ?? "Detection identity unavailable"}</h1>

      <div className={styles.caseMeta}>
        <p className={styles.traceabilityStat}>
          <span className={styles.eyebrow}>Traceability</span>{" "}
          <span className={`${styles.technical} ${traceabilityIntact ? "" : styles.rustMark}`}>
            {traceabilityIntact ? "Intact" : "Broken"}
          </span>
        </p>

        {onOpenCommandPalette && (
          <button
            type="button"
            className={`${styles.plainButton} ${styles.commandTrigger}`}
            onClick={onOpenCommandPalette}
          >
            <span aria-hidden="true">⌘K</span> Command palette
          </button>
        )}
      </div>

      {finding.available ? (
        <>
          <p className={`${styles.prose} ${styles.observedAct}`}>
            <span className={styles.technical}>{finding.subject?.username ?? "—"}</span> performed{" "}
            <span className={`${styles.technical} ${styles.wrapLongValue}`}>
              {finding.operation?.verb ?? "—"}
            </span>{" "}
            against{" "}
            <span className={`${styles.technical} ${styles.wrapLongValue}`}>
              {targetLabel}
              {finding.target?.name ? ` / ${finding.target.name}` : ""}
            </span>
            .
          </p>

          <p className={`${styles.proseSecondary} ${styles.outcomeAnnotation}`}>
            Outcome <span className={styles.technical}>{finding.outcome?.code ?? "—"}</span> recorded at{" "}
            <span className={`${styles.technical} ${styles.wrapLongValue}`}>
              {finding.requestTime ?? "—"}
            </span>
            .
          </p>
        </>
      ) : (
        <p className={`${styles.proseSecondary} ${styles.observedAct}`}>
          Alert summary is not available for this alert.
        </p>
      )}
    </header>
  );
}
