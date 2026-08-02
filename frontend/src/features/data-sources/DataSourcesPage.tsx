import { AppShell } from "@/app/AppShell";
import { StatusBadge } from "@/components/StatusBadge/StatusBadge";
import type { DataSourceItem } from "@/types/contract";
import { useDataSources } from "./hooks/useDataSources";
import { DataSourcesFetchError } from "./lib/dataSourcesSource";
import styles from "./DataSourcesPage.module.css";

function formatLastEventAt(iso: string | null): string {
  if (iso === null) return "—";
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso;
  return date.toISOString().replace("T", " ").replace("Z", " UTC");
}

function DataSourceCard({ item }: { item: DataSourceItem }) {
  const hasEvents = item.eventCount > 0;
  return (
    <div className={styles.card}>
      <div className={styles.cardHeader}>
        <h2 className={styles.cardTitle}>{item.displayName}</h2>
        <StatusBadge tone={hasEvents ? "intact" : "neutral"}>
          {hasEvents ? "Events observed" : "No events observed"}
        </StatusBadge>
      </div>
      <p className={styles.cardEndpoint}>{item.endpoint}</p>
      <dl className={styles.cardStats}>
        <div className={styles.stat}>
          <dt className={styles.statLabel}>Events accepted</dt>
          <dd className={styles.statValue}>{item.eventCount}</dd>
        </div>
        <div className={styles.stat}>
          <dt className={styles.statLabel}>Last event</dt>
          <dd className={styles.statValue}>{formatLastEventAt(item.lastEventAt)}</dd>
        </div>
      </dl>
    </div>
  );
}

function LoadingState() {
  return (
    <div className={styles.cardGrid} role="status" aria-live="polite">
      <span className="cnsdp-visually-hidden">Loading data sources…</span>
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
      <StatusBadge tone="broken">Data sources unavailable</StatusBadge>
      <h2 className={styles.stateTitle}>The data sources backend could not be reached</h2>
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

/** The route component for `/data-sources`: a read-only, retrospective
 *  account of telemetry already accepted through the platform's defined
 *  intake (FR-036), from the real `GET /v1/data-sources` endpoint
 *  (src/features/data-sources/lib/dataSourcesSource.ts). Carries no health,
 *  staleness, or missing-delivery judgment (docs/scope.md scope decision 6). */
export function DataSourcesPage() {
  const query = useDataSources();

  return (
    <AppShell>
      <div className={styles.wrap}>
        <div className={styles.toolbar}>
          <h1 className={styles.title}>Data Sources</h1>
          {query.isSuccess && (
            <span className={styles.count}>
              {query.data.total} source{query.data.total === 1 ? "" : "s"}
            </span>
          )}
        </div>

        {query.isPending && <LoadingState />}

        {query.isError &&
          (query.error instanceof DataSourcesFetchError && query.error.kind === "unauthorized" ? (
            <UnauthorizedState />
          ) : (
            <ErrorState onRetry={() => query.refetch()} />
          ))}

        {query.isSuccess && (
          <div className={styles.cardGrid}>
            {query.data.dataSources.map((item) => (
              <DataSourceCard key={item.id} item={item} />
            ))}
          </div>
        )}
      </div>
    </AppShell>
  );
}
