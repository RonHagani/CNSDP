import type { DocketHeaderViewModel } from "@/features/alert-investigation/lib/investigationViewModel";
import styles from "./dossier.module.css";

/**
 * The compact identity strip (UX spec §3.1) — not a hero, not a masthead.
 * Renders exactly the fields `DocketHeaderViewModel` carries: alert
 * identity, matched scenario/definition identity, pinned revision, and
 * the traceability indicator. This component constructs no domain
 * summary of its own — every value is either passed straight through or
 * omitted with an honest "unavailable" statement when the model does not
 * carry it.
 *
 * `DocketHeaderViewModel` does not carry subject/operation/target/outcome
 * /request time — those are the Finding's content (UX spec §3.2,
 * rendered by the separate `InvestigationFinding` component), not the
 * Docket Header's. This component does not duplicate them here; doing so
 * would mean either reopening `AlertInvestigationResponse` directly or
 * inventing a value not present on this view model, both of which are
 * excluded by the presentation boundary.
 */
export function DocketHeader({
  docketHeader,
  onOpenCommandPalette,
}: {
  docketHeader: DocketHeaderViewModel;
  onOpenCommandPalette?: () => void;
}) {
  const { alertId, matchedScenario, matchedDefinitionName, pinnedRevision, traceabilityIntact } =
    docketHeader;

  return (
    <header className={`${styles.dossierRoot} ${styles.section}`} aria-label="Alert identity">
      <div className={styles.stack}>
        <p className={`${styles.eyebrow} ${styles.wrapLongValue}`}>
          Alert #{alertId}
          {matchedScenario ? ` · ${matchedScenario}` : ""}
          {pinnedRevision ? ` · rev. ${pinnedRevision}` : ""}
        </p>

        <p className={styles.heading}>
          {matchedDefinitionName ?? "Detection identity unavailable"}
        </p>

        <dl className={styles.fieldList}>
          <div className={styles.fieldRow}>
            <dt>Traceability</dt>
            <dd className={styles.technical}>{traceabilityIntact ? "Intact" : "Broken"}</dd>
          </div>
        </dl>

        {onOpenCommandPalette && (
          <button type="button" className={styles.plainButton} onClick={onOpenCommandPalette}>
            Command palette
          </button>
        )}
      </div>
    </header>
  );
}
