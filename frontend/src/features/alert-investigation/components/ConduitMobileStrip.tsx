import type { AlertInvestigationResponse } from "@/types/contract";
import { STAGE_ORDER, STAGES } from "../lib/stageMapping";
import { deriveStageStatus } from "../lib/deriveStageStatus";
import { useInvestigationUI } from "../lib/InvestigationUIContext";
import styles from "./ConduitMobileStrip.module.css";

/**
 * The conduit's mobile interpretation — a horizontal transit rail with a
 * large "now showing" readout, not the vertical field shrunk sideways.
 * Authored separately (product-experience-brief.md §9's requirement for a
 * deliberate breakpoint transformation, not naive stacking) — hidden above
 * 767px, where ConduitField takes over.
 */
export function ConduitMobileStrip({ data }: { data: AlertInvestigationResponse }) {
  const { selectedStage, selectStage } = useInvestigationUI();
  const statuses = deriveStageStatus(data);
  const current = STAGES[selectedStage];
  const index = STAGE_ORDER.indexOf(selectedStage);

  return (
    <div className={styles.strip}>
      <p className={styles.readout}>
        <span className={styles.readoutIndex}>{String(index + 1).padStart(2, "0")} / 08</span>
        {current.label}
      </p>
      <div className={styles.rail}>
        <div className={styles.railLine} aria-hidden="true" />
        <ol className={styles.dots}>
          {STAGE_ORDER.map((stageId, i) => {
            const status = statuses[stageId];
            const isActive = stageId === selectedStage;
            return (
              <li key={stageId}>
                <button
                  type="button"
                  className={styles.dot}
                  data-tone={status.tone}
                  data-active={isActive || undefined}
                  aria-current={isActive ? "step" : undefined}
                  aria-label={STAGES[stageId].label}
                  onClick={() => selectStage(stageId)}
                >
                  <span className={styles.dotIndex}>{i + 1}</span>
                </button>
              </li>
            );
          })}
        </ol>
      </div>
    </div>
  );
}
