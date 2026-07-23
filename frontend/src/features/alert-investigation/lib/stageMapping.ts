import type { EvidenceArtifactId, SignalPathStageId } from "@/types/contract";

/**
 * The Signal Path's eight-stage visual grouping and its mapping back to the
 * platform's three real architecture axes — product-experience-brief.md
 * §3.4. This is reference data, not a restatement of a persisted workflow
 * state: `isWorkflowState: false` marks the two stages (traceability,
 * investigation) that are verification/read operations, never a value a
 * submission's status column takes.
 */
export interface StageDescriptor {
  id: SignalPathStageId;
  label: string;
  module: string;
  workflowState: string | null;
  isWorkflowState: boolean;
  artifacts: EvidenceArtifactId[];
  description: string;
}

export const STAGE_ORDER: SignalPathStageId[] = [
  "intake",
  "validation",
  "normalization",
  "detection-evaluation",
  "alerting",
  "evidence",
  "traceability",
  "investigation",
];

export const STAGES: Record<SignalPathStageId, StageDescriptor> = {
  intake: {
    id: "intake",
    label: "Intake",
    module: "Telemetry admission (module 1)",
    workflowState: "admitted",
    isWorkflowState: true,
    artifacts: ["source-event"],
    description: "The submission is durably admitted at the defined intake.",
  },
  validation: {
    id: "validation",
    label: "Validation",
    module: "Validation and classification (module 2)",
    workflowState: "admitted → validated",
    isWorkflowState: true,
    artifacts: ["validation-outcome"],
    description: "The submission is classified valid, invalid, incomplete, or unsupported.",
  },
  normalization: {
    id: "normalization",
    label: "Normalization",
    module: "Normalization (module 3)",
    workflowState: "validated → normalized",
    isWorkflowState: true,
    artifacts: ["normalized-event"],
    description:
      "A valid submission's raw event is transformed into the normalized representation.",
  },
  "detection-evaluation": {
    id: "detection-evaluation",
    label: "Detection evaluation",
    module: "Detection evaluation (module 4)",
    workflowState: "normalized → evaluated",
    isWorkflowState: true,
    artifacts: ["detection-definition", "detection-result"],
    description: "The normalized event is evaluated against every active detection definition.",
  },
  alerting: {
    id: "alerting",
    label: "Alerting",
    module: "Alert generation (module 5)",
    workflowState: "evaluated → alerted",
    isWorkflowState: true,
    artifacts: ["alert"],
    description: "Exactly one alert is produced for a matching detection result.",
  },
  evidence: {
    id: "evidence",
    label: "Evidence",
    module: "Evidence inventory (module 6)",
    workflowState: "alerted → evidenced",
    isWorkflowState: true,
    artifacts: [
      "source-event",
      "validation-outcome",
      "normalized-event",
      "detection-definition",
      "detection-result",
      "alert",
    ],
    description:
      "The complete six-artifact inventory is verified as a set before the submission is marked evidenced.",
  },
  traceability: {
    id: "traceability",
    label: "Traceability",
    module: "Traceability (module 7)",
    workflowState: null,
    isWorkflowState: false,
    artifacts: [],
    description:
      "Not a persisted workflow state. A verification operation, recomputed live on every retrieval — never a stored flag read back unchanged.",
  },
  investigation: {
    id: "investigation",
    label: "Investigation",
    module: "Retrieval and investigation (module 8)",
    workflowState: null,
    isWorkflowState: false,
    artifacts: [],
    description:
      "Not a persisted workflow state. The authenticated read path that composes and presents every artifact above — this screen itself.",
  },
};

export const EVIDENCE_ARTIFACT_LABELS: Record<EvidenceArtifactId, string> = {
  "source-event": "Source submission (raw event)",
  "validation-outcome": "Validation outcome",
  "normalized-event": "Normalized event",
  "detection-definition": "Detection definition",
  "detection-result": "Detection result (match reason)",
  alert: "Generated alert",
};
