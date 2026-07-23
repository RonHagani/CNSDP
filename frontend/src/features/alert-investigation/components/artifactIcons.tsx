/**
 * Six small hand-authored SVG glyphs, one per real evidence artifact
 * (FR-031) — an authored icon system, not a generic icon-library import,
 * so each of the six tiles in EvidenceRibbon is differentiated by role,
 * not only by text (this milestone's requirement D).
 *
 * This is a data/icon module, not a page component — every export below
 * is intentionally a small constant, never intended to hot-reload as its
 * own route, so fast-refresh's component-export convention doesn't apply.
 */
/* eslint-disable react-refresh/only-export-components */
import type { ReactElement, ReactNode } from "react";
import type { EvidenceArtifactId } from "@/types/contract";

function IconBase({ children }: { children: ReactNode }) {
  return (
    <svg viewBox="0 0 32 32" width="22" height="22" fill="none" aria-hidden="true">
      {children}
    </svg>
  );
}

const SourceEventIcon = () => (
  <IconBase>
    <path d="M9 4h10l6 6v18H9z" stroke="currentColor" strokeWidth="1.4" strokeLinejoin="round" />
    <path d="M19 4v6h6" stroke="currentColor" strokeWidth="1.4" strokeLinejoin="round" />
    <path d="M12.5 17h9M12.5 21h9M12.5 25h5.5" stroke="currentColor" strokeWidth="1.2" />
  </IconBase>
);

const ValidationIcon = () => (
  <IconBase>
    <circle cx="16" cy="16" r="12" stroke="currentColor" strokeWidth="1.4" />
    <path
      d="M10.5 16.5l3.8 3.8L22 12"
      stroke="currentColor"
      strokeWidth="1.6"
      strokeLinecap="round"
      strokeLinejoin="round"
    />
  </IconBase>
);

const NormalizedEventIcon = () => (
  <IconBase>
    <path d="M5 10h9M5 16h9M5 22h9" stroke="currentColor" strokeWidth="1.3" />
    <path d="M16 16h5" stroke="currentColor" strokeWidth="1.3" />
    <path d="M18 10l8 6-8 6z" stroke="currentColor" strokeWidth="1.4" strokeLinejoin="round" />
  </IconBase>
);

const DetectionDefinitionIcon = () => (
  <IconBase>
    <path
      d="M16 8c-2.5-2-6-2.5-10-1.5v18c4-1 7.5-.5 10 1.5 2.5-2 6-2.5 10-1.5v-18c-4-1-7.5-.5-10 1.5z"
      stroke="currentColor"
      strokeWidth="1.3"
      strokeLinejoin="round"
    />
    <path d="M16 8v18" stroke="currentColor" strokeWidth="1.3" />
  </IconBase>
);

const DetectionResultIcon = () => (
  <IconBase>
    <circle cx="14" cy="14" r="8.5" stroke="currentColor" strokeWidth="1.4" />
    <circle cx="14" cy="14" r="3" stroke="currentColor" strokeWidth="1.2" />
    <path d="M20 20l6 6" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" />
  </IconBase>
);

const AlertIcon = () => (
  <IconBase>
    <path
      d="M16 6l3.5 6.2 7 1-5 4.9 1.2 6.9-6.7-3.5-6.7 3.5 1.2-6.9-5-4.9 7-1z"
      stroke="currentColor"
      strokeWidth="1.3"
      strokeLinejoin="round"
    />
  </IconBase>
);

export const ARTIFACT_ICONS: Record<EvidenceArtifactId, () => ReactElement> = {
  "source-event": SourceEventIcon,
  "validation-outcome": ValidationIcon,
  "normalized-event": NormalizedEventIcon,
  "detection-definition": DetectionDefinitionIcon,
  "detection-result": DetectionResultIcon,
  alert: AlertIcon,
};
