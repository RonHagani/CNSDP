import { useEffect, useRef, useState } from "react";
import { motion, useReducedMotion } from "framer-motion";
import type { AlertInvestigationResponse } from "@/types/contract";
import { STAGE_ORDER, STAGES } from "../lib/stageMapping";
import { deriveStageStatus } from "../lib/deriveStageStatus";
import { useInvestigationUI } from "../lib/InvestigationUIContext";
import styles from "./ConduitField.module.css";

/**
 * The dominant spatial system this screen is organized around — an
 * organic, winding infrastructural conduit running the full height of the
 * investigation, not a list of nav buttons in a sidebar column. The eight
 * real pipeline stages (product-experience-brief.md §3.4) are junctions
 * threaded onto it; the active junction illuminates and extends a tendril
 * into the content it governs. Node positions are computed from the
 * authored SVG path itself (getPointAtLength), so the curve is the single
 * source of truth for where every junction sits — not independently laid
 * out and coincidentally aligned.
 */
const CURVE_PATH =
  "M 140,0 C 140,100 70,200 70,300 C 70,400 170,500 170,600 C 170,700 60,800 60,900 " +
  "C 60,1000 180,1100 180,1200 C 180,1300 70,1400 70,1500 C 70,1600 160,1700 160,1800 " +
  "C 160,1900 120,2000 120,2100";
const VIEWBOX_HEIGHT = 2100;

interface NodePoint {
  x: number;
  y: number;
}

export function ConduitField({ data }: { data: AlertInvestigationResponse }) {
  const { selectedStage, selectStage } = useInvestigationUI();
  const statuses = deriveStageStatus(data);
  const reduceMotion = useReducedMotion();
  const pathRef = useRef<SVGPathElement>(null);
  const [points, setPoints] = useState<NodePoint[]>([]);

  useEffect(() => {
    const pathEl = pathRef.current;
    if (!pathEl || typeof pathEl.getPointAtLength !== "function") return;
    const total = pathEl.getTotalLength();
    const next: NodePoint[] = STAGE_ORDER.map((_, i) => {
      const point = pathEl.getPointAtLength((total * i) / (STAGE_ORDER.length - 1));
      return { x: point.x, y: point.y };
    });
    setPoints(next);
  }, []);

  const activeIndex = STAGE_ORDER.indexOf(selectedStage);

  return (
    <div className={styles.field} aria-hidden="false">
      <svg
        className={styles.svg}
        viewBox={`0 0 240 ${VIEWBOX_HEIGHT}`}
        preserveAspectRatio="none"
        role="img"
        aria-label="Signal Path — the pipeline conduit connecting every investigation stage for this alert"
      >
        <defs>
          <filter id="conduit-glow" x="-100%" y="-100%" width="300%" height="300%">
            <feGaussianBlur stdDeviation="6" result="blur" />
            <feMerge>
              <feMergeNode in="blur" />
              <feMergeNode in="SourceGraphic" />
            </feMerge>
          </filter>
        </defs>

        <path ref={pathRef} d={CURVE_PATH} className={styles.pathGhost} />
        <motion.path
          d={CURVE_PATH}
          className={styles.pathLive}
          initial={reduceMotion ? false : { pathLength: 0 }}
          animate={{ pathLength: 1 }}
          transition={{ duration: reduceMotion ? 0 : 1.4, ease: [0.16, 1, 0.3, 1] }}
        />
      </svg>

      <ol className={styles.nodes}>
        {STAGE_ORDER.map((stageId, i) => {
          const stage = STAGES[stageId];
          const status = statuses[stageId];
          const point = points[i];
          const isActive = i === activeIndex;
          if (!point) return null;
          return (
            <li
              key={stageId}
              className={styles.nodeWrap}
              style={{
                top: `${(point.y / VIEWBOX_HEIGHT) * 100}%`,
                left: `${(point.x / 240) * 100}%`,
              }}
            >
              <button
                type="button"
                className={styles.node}
                data-tone={status.tone}
                data-active={isActive || undefined}
                aria-current={isActive ? "step" : undefined}
                aria-label={`${stage.label} — ${stage.module}${stage.isWorkflowState ? `, workflow state ${stage.workflowState}` : ", not a persisted workflow state"}`}
                onClick={() => selectStage(stageId)}
              >
                <span className={styles.nodeCore} data-tone={status.tone} />
                <span className={styles.nodeIndex}>{String(i + 1).padStart(2, "0")}</span>
              </button>
              <span className={styles.nodeTag} data-active={isActive || undefined}>
                {stage.label}
              </span>
              {isActive && <span className={styles.tendril} aria-hidden="true" />}
            </li>
          );
        })}
      </ol>
    </div>
  );
}
