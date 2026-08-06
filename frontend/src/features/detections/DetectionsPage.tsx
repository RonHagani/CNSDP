import { AppShell } from "@/app/AppShell";
import { StatusBadge } from "@/components/StatusBadge/StatusBadge";
import type { DetectionItem, DetectionOperation } from "@/types/contract";
import { useDetections } from "./hooks/useDetections";
import { DetectionsFetchError } from "./lib/detectionsSource";
import styles from "./DetectionsPage.module.css";

/** A faithful, read-only rendering of the operation clause a definition
 *  declares — no characteristic invented beyond what conditions.operation
 *  actually carries. */
function formatOperation(op: DetectionOperation): string {
  const target = op.subresource ? `${op.resource}/${op.subresource}` : op.resource;
  return op.verb ? `${target} · ${op.verb}` : target;
}

function DetectionCard({ item }: { item: DetectionItem }) {
  const { conditions } = item;
  const requiresAll = conditions.requires_all ?? [];
  const requiresAny = conditions.requires_any ?? [];

  return (
    <div className={styles.card}>
      <div className={styles.cardHeader}>
        <h2 className={styles.cardTitle}>{item.name}</h2>
        <span className={styles.scenarioTag}>{item.scenario}</span>
      </div>
      <p className={styles.cardDescription}>{item.description}</p>
      <dl className={styles.conditions}>
        <div className={styles.conditionRow}>
          <dt className={styles.conditionLabel}>Operation</dt>
          <dd className={styles.conditionValue}>{formatOperation(conditions.operation)}</dd>
        </div>
        {conditions.requires_outcome && (
          <div className={styles.conditionRow}>
            <dt className={styles.conditionLabel}>Requires outcome</dt>
            <dd className={styles.conditionValue}>{conditions.requires_outcome}</dd>
          </div>
        )}
        {requiresAll.length > 0 && (
          <div className={styles.conditionRow}>
            <dt className={styles.conditionLabel}>All of</dt>
            <dd className={styles.conditionValue}>
              <ul className={styles.characteristicList}>
                {requiresAll.map((c) => (
                  <li key={c.id}>{c.description}</li>
                ))}
              </ul>
            </dd>
          </div>
        )}
        {requiresAny.length > 0 && (
          <div className={styles.conditionRow}>
            <dt className={styles.conditionLabel}>Any of</dt>
            <dd className={styles.conditionValue}>
              <ul className={styles.characteristicList}>
                {requiresAny.map((c) => (
                  <li key={c.id}>{c.description}</li>
                ))}
              </ul>
            </dd>
          </div>
        )}
      </dl>
      <p className={styles.revision} title={item.revision}>
        rev. {item.revision.slice(0, 12)}…
      </p>
    </div>
  );
}

function LoadingState() {
  return (
    <div className={styles.cardGrid} role="status" aria-live="polite">
      <span className="cnsdp-visually-hidden">Loading detections…</span>
      <div className={styles.loadingCard} aria-hidden="true" />
      <div className={styles.loadingCard} aria-hidden="true" />
      <div className={styles.loadingCard} aria-hidden="true" />
    </div>
  );
}

function UnauthorizedState() {
  return (
    <div className={styles.state} role="alert">
      <StatusBadge tone="broken">401 Unauthorized</StatusBadge>
      <h2 className={styles.stateTitle}>Authentication required</h2>
      <p className={styles.stateMessage}>
        The bearer token was missing or did not match the platform's configured API_TOKEN. Every
        product-exposed path — including this one — denies unauthenticated or unauthorized access.
      </p>
    </div>
  );
}

function ErrorState({ onRetry }: { onRetry: () => void }) {
  return (
    <div className={styles.state} role="alert">
      <StatusBadge tone="broken">Detections unavailable</StatusBadge>
      <h2 className={styles.stateTitle}>The detections backend could not be reached</h2>
      <p className={styles.stateMessage}>
        No internal error detail is available for display — a platform fault is reported without
        leaking implementation detail.
      </p>
      <button type="button" className={styles.retryButton} onClick={onRetry}>
        Retry
      </button>
    </div>
  );
}

function EmptyState() {
  return (
    <div className={styles.state}>
      <h2 className={styles.stateTitle}>No active detections</h2>
      <p className={styles.stateMessage}>
        No detection definitions are currently active on this deployment.
      </p>
    </div>
  );
}

/** The route component for `/detections`: a read-only catalog of the
 *  platform's currently active detection definitions (FR-020, FR-021,
 *  FR-022), from the real `GET /v1/detections` endpoint
 *  (src/features/detections/lib/detectionsSource.ts). Carries no editing,
 *  enable/disable, authoring, or lifecycle capability — detection
 *  definitions remain version-controlled and immutable (ADR-0004). */
export function DetectionsPage() {
  const query = useDetections();

  return (
    <AppShell>
      <div className={styles.wrap}>
        <div className={styles.toolbar}>
          <h1 className={styles.title}>Detections</h1>
          {query.isSuccess && (
            <span className={styles.count}>
              {query.data.total} detection{query.data.total === 1 ? "" : "s"}
            </span>
          )}
        </div>

        {query.isPending && <LoadingState />}

        {query.isError &&
          (query.error instanceof DetectionsFetchError && query.error.kind === "unauthorized" ? (
            <UnauthorizedState />
          ) : (
            <ErrorState onRetry={() => query.refetch()} />
          ))}

        {query.isSuccess && query.data.detections.length === 0 && <EmptyState />}

        {query.isSuccess && query.data.detections.length > 0 && (
          <div className={styles.cardGrid}>
            {query.data.detections.map((item) => (
              <DetectionCard key={item.scenario} item={item} />
            ))}
          </div>
        )}
      </div>
    </AppShell>
  );
}
