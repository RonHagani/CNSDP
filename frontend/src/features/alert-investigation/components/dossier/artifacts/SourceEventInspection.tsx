import type { SourceEventInspection as SourceEventInspectionModel } from "@/features/alert-investigation/lib/artifactInspection";
import { JsonTree } from "../../JsonTree";
import styles from "../dossier.module.css";

const EMPTY_PATHS = new Set<string>();

/**
 * Source Event Inspection Leaf content. Reuses the existing `JsonTree`
 * primitive unchanged to render the complete raw event — this component
 * does not duplicate or reinterpret any raw field itself; every value
 * shown is exactly what the response carried. Field-level lineage
 * highlighting is not wired up here (that is Step 3/4 orchestration
 * state, out of scope for this presentational component) — `onSelectPath`
 * is optional and forwarded as a no-op when the caller does not supply
 * one, so `JsonTree` remains fully functional in isolation.
 */
export function SourceEventInspection({
  inspection,
  onSelectPath,
}: {
  inspection: SourceEventInspectionModel;
  onSelectPath?: (path: string) => void;
}) {
  if (!inspection.available) {
    return <p className={styles.proseSecondary}>Source event is not available for this alert.</p>;
  }

  return (
    <div>
      <p className={styles.eyebrow}>Source submission — raw event</p>
      <div className={styles.stack}>
        <JsonTree
          data={inspection.rawEvent}
          linkablePaths={EMPTY_PATHS}
          highlightedPaths={EMPTY_PATHS}
          dimOthers={false}
          onSelectPath={onSelectPath ?? (() => {})}
        />
      </div>
    </div>
  );
}
