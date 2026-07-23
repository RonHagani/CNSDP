import { motion, useReducedMotion } from "framer-motion";
import type { AlertInvestigationResponse, EvidenceArtifactId } from "@/types/contract";
import { useInvestigationUI } from "../lib/InvestigationUIContext";
import { ARTIFACT_ICONS } from "./artifactIcons";
import styles from "./EvidenceRibbon.module.css";

interface Exhibit {
  id: EvidenceArtifactId;
  index: number;
  name: string;
  caption: string;
  available: boolean;
  stage: Parameters<ReturnType<typeof useInvestigationUI>["selectStage"]>[0];
}

/**
 * The six-artifact minimum evidence set (FR-031) as a designed procession
 * of exhibits — sculpted, angled-badge tiles on a rail, each carrying its
 * own hand-authored icon (artifactIcons.tsx), not six repeated rectangular
 * cards distinguished only by their text. Traceability verification is
 * deliberately not rendered here — see ProofChain, a separate crafted
 * object, per product-experience-brief.md §3.3: it is not a seventh
 * artifact and must not visually read as one.
 */
export function EvidenceRibbon({ data }: { data: AlertInvestigationResponse }) {
  const { selectStage } = useInvestigationUI();
  const reduceMotion = useReducedMotion();

  const exhibits: Exhibit[] = [
    {
      id: "source-event",
      index: 1,
      name: "Source submission",
      caption: data.sourceEvent.rawEvent
        ? `auditID ${data.sourceEvent.rawEvent.auditID.slice(0, 8)}…`
        : "—",
      available: data.sourceEvent.available,
      stage: "intake",
    },
    {
      id: "validation-outcome",
      index: 2,
      name: "Validation outcome",
      caption: data.validationOutcome.outcome ?? "—",
      available: data.validationOutcome.available,
      stage: "validation",
    },
    {
      id: "normalized-event",
      index: 3,
      name: "Normalized event",
      caption: data.normalizedEvent.event
        ? `${data.normalizedEvent.event.operation.verb} ${data.normalizedEvent.event.target.resource}`
        : "—",
      available: data.normalizedEvent.available,
      stage: "normalization",
    },
    {
      id: "detection-definition",
      index: 4,
      name: "Detection definition",
      caption: data.detectionDefinition.revision
        ? `rev ${data.detectionDefinition.revision.slice(0, 8)}…`
        : "—",
      available: data.detectionDefinition.available,
      stage: "detection-evaluation",
    },
    {
      id: "detection-result",
      index: 5,
      name: "Detection result",
      caption: data.detectionResult.matchReason?.scenario ?? "—",
      available: data.detectionResult.available,
      stage: "detection-evaluation",
    },
    {
      id: "alert",
      index: 6,
      name: "Generated alert",
      caption: `#${data.alertId}`,
      available: data.alert.available,
      stage: "alerting",
    },
  ];

  const availableCount = exhibits.filter((e) => e.available).length;

  return (
    <section className={styles.ribbon} aria-labelledby="evidence-ribbon-title">
      <header className={styles.head}>
        <div>
          <p className={styles.eyebrow}>Minimum evidence set · FR-031</p>
          <h2 className={styles.title} id="evidence-ribbon-title">
            The exhibit procession
          </h2>
        </div>
        <p className={styles.count}>
          {availableCount} <span className={styles.countDivider}>/</span> {exhibits.length} present
        </p>
      </header>

      <div className={styles.rail} aria-hidden="true" />

      <ol className={styles.exhibits}>
        {exhibits.map((exhibit, i) => {
          const Icon = ARTIFACT_ICONS[exhibit.id];
          return (
            <li key={exhibit.id} className={styles.slot} data-parity={i % 2 === 0 ? "even" : "odd"}>
              <motion.button
                type="button"
                className={styles.exhibitCard}
                data-available={exhibit.available}
                onClick={() => selectStage(exhibit.stage)}
                initial={reduceMotion ? false : { opacity: 0, y: 16 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{
                  duration: reduceMotion ? 0 : 0.45,
                  delay: reduceMotion ? 0 : i * 0.06,
                }}
              >
                <span className={styles.exhibitIcon} data-available={exhibit.available}>
                  <Icon />
                </span>
                <span className={styles.exhibitIndex}>
                  {String(exhibit.index).padStart(2, "0")}
                </span>
                <span className={styles.exhibitName}>{exhibit.name}</span>
                <span className={styles.exhibitCaption}>{exhibit.caption}</span>
                <span className={styles.exhibitStatus} data-available={exhibit.available}>
                  {exhibit.available ? "present" : "missing"}
                </span>
              </motion.button>
            </li>
          );
        })}
      </ol>
    </section>
  );
}
