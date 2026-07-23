import type { AlertInvestigationResponse } from "@/types/contract";
import { STAGES } from "../lib/stageMapping";
import { useInvestigationUI } from "../lib/InvestigationUIContext";
import { LineageInteraction } from "./LineageInteraction";
import styles from "./StageContent.module.css";

const OUTCOME_TONE: Record<string, string> = {
  valid: styles.toneSignal,
  invalid: styles.toneSevered,
  incomplete: styles.toneBranch,
  unsupported: styles.toneBranch,
};

/**
 * Routes the selected Signal Path stage to its content, all sharing one
 * editorial register (a display-serif section opener + generous single-
 * focus canvas) instead of the six identical ArtifactPanel cards this
 * replaces. Intake and Normalization both resolve to the lineage
 * interaction — the screen's centerpiece — since that is the one
 * interaction spanning both artifacts.
 */
export function StageContent({ data }: { data: AlertInvestigationResponse }) {
  const { selectedStage } = useInvestigationUI();

  switch (selectedStage) {
    case "intake":
    case "normalization":
      return (
        <LineageInteraction sourceEvent={data.sourceEvent} normalizedEvent={data.normalizedEvent} />
      );
    case "validation":
      return <ValidationStage validationOutcome={data.validationOutcome} />;
    case "detection-evaluation":
      return (
        <DetectionStage
          detectionDefinition={data.detectionDefinition}
          detectionResult={data.detectionResult}
        />
      );
    case "alerting":
      return <AlertingStage alert={data.alert} />;
    case "evidence":
    case "traceability":
    case "investigation":
      return <MetaStage stageId={selectedStage} />;
  }
}

function ValidationStage({
  validationOutcome,
}: {
  validationOutcome: AlertInvestigationResponse["validationOutcome"];
}) {
  const outcome = validationOutcome.outcome;
  return (
    <div className={styles.stage}>
      <StageHead
        eyebrow="Evidence artifact 2 of 6"
        title="Validation outcome"
        hint={
          validationOutcome.available
            ? "Every submission receives exactly one of four outcomes."
            : undefined
        }
      />
      {!validationOutcome.available && <UnavailableNote label="Validation outcome" />}
      {outcome && (
        <div className={styles.singleFocus}>
          <span className={`${styles.outcomeBadge} ${OUTCOME_TONE[outcome]}`}>{outcome}</span>
          {validationOutcome.reason ? (
            <p className={styles.prose}>{validationOutcome.reason}</p>
          ) : (
            <p className={styles.proseMuted}>
              No stated reason — a valid outcome carries none by definition (FR-011, FR-012).
            </p>
          )}
        </div>
      )}
    </div>
  );
}

function DetectionStage({
  detectionDefinition,
  detectionResult,
}: {
  detectionDefinition: AlertInvestigationResponse["detectionDefinition"];
  detectionResult: AlertInvestigationResponse["detectionResult"];
}) {
  const def = detectionDefinition.definition;
  const satisfiedIds = new Set(
    detectionResult.matchReason?.satisfiedCharacteristics.map((c) => c.id) ?? [],
  );
  const declared = [
    ...(def?.conditions.requires_any ?? []),
    ...(def?.conditions.requires_all ?? []),
  ];

  return (
    <div className={styles.stage}>
      <StageHead
        eyebrow="Evidence artifacts 4–5 of 6"
        title={def?.name ?? "Detection definition"}
        hint={
          detectionDefinition.revision
            ? `Pinned revision ${detectionDefinition.revision.slice(0, 24)}…`
            : undefined
        }
      />
      {(!detectionDefinition.available || !detectionResult.available) && (
        <UnavailableNote label="Detection definition or result" />
      )}
      {def && (
        <div className={styles.singleFocus}>
          <p className={styles.prose}>{def.description}</p>
          <ul className={styles.characteristicList}>
            {declared.map((c) => (
              <li
                key={c.id}
                className={styles.characteristic}
                data-satisfied={satisfiedIds.has(c.id)}
              >
                <span className={styles.characteristicGlyph}>
                  {satisfiedIds.has(c.id) ? "✓" : "·"}
                </span>
                <span className={styles.characteristicId}>{c.id}</span>
                <p className={styles.characteristicText}>{c.description}</p>
              </li>
            ))}
          </ul>
          <p className={styles.proseMuted}>
            {detectionResult.matchReason?.satisfiedCharacteristics.length ?? 0} of{" "}
            {declared.length || 1} declared characteristics satisfied — an honest checklist against
            the definition, never an invented confidence score.
          </p>
        </div>
      )}
    </div>
  );
}

function AlertingStage({ alert }: { alert: AlertInvestigationResponse["alert"] }) {
  const summary = alert.summary;
  return (
    <div className={styles.stage}>
      <StageHead eyebrow="Evidence artifact 6 of 6" title="Generated alert" />
      {!alert.available && <UnavailableNote label="Alert summary" />}
      {summary && (
        <dl className={styles.kv}>
          <div className={styles.kvRow}>
            <dt>Matched definition</dt>
            <dd>{summary.matchReason.definitionName}</dd>
          </div>
          <div className={styles.kvRow}>
            <dt>Subject</dt>
            <dd>{summary.subject.username}</dd>
          </div>
          <div className={styles.kvRow}>
            <dt>Operation</dt>
            <dd>{summary.operation.verb}</dd>
          </div>
          <div className={styles.kvRow}>
            <dt>Target</dt>
            <dd>
              {summary.target.resource}/{summary.target.name}
              {summary.target.subresource ? `/${summary.target.subresource}` : ""}
            </dd>
          </div>
          <div className={styles.kvRow}>
            <dt>Outcome code</dt>
            <dd>{summary.outcome.code ?? "—"}</dd>
          </div>
          <div className={styles.kvRow}>
            <dt>Request time</dt>
            <dd>{summary.requestTime}</dd>
          </div>
        </dl>
      )}
    </div>
  );
}

function MetaStage({ stageId }: { stageId: "evidence" | "traceability" | "investigation" }) {
  const stage = STAGES[stageId];
  return (
    <div className={styles.stage}>
      <StageHead eyebrow={stage.module} title={stage.label} />
      <div className={styles.singleFocus}>
        <p className={styles.proseLarge}>{stage.description}</p>
        {!stage.isWorkflowState && (
          <p className={styles.proseMuted}>
            Not a persisted workflow state — traceability and investigation are verification and
            read operations, not values a submission's status column ever takes (product-experience-
            brief.md §3.4).
          </p>
        )}
        <p className={styles.proseMuted}>The evidence chain below is this stage made concrete.</p>
      </div>
    </div>
  );
}

function StageHead({ eyebrow, title, hint }: { eyebrow: string; title: string; hint?: string }) {
  return (
    <div className={styles.stageHead}>
      <p className={styles.eyebrow}>{eyebrow}</p>
      <h2 className={styles.stageTitle}>{title}</h2>
      {hint && <p className={styles.stageHint}>{hint}</p>}
    </div>
  );
}

function UnavailableNote({ label }: { label: string }) {
  return (
    <p className={styles.unavailable} role="note">
      {label} is not available for this alert. Per FR-035, this gap is shown explicitly rather than
      omitted.
    </p>
  );
}
