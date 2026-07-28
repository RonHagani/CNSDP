import { Link } from "react-router-dom";
import { AppShell } from "@/app/AppShell";
import { StatusBadge } from "@/components/StatusBadge/StatusBadge";
import type { AlertSummaryListItem } from "@/types/contract";
import { useAlertInventory } from "./hooks/useAlertInventory";
import { useHorizontalOverflow } from "./hooks/useHorizontalOverflow";
import { InventoryFetchError } from "./lib/alertInventorySource";
import styles from "./AlertInventoryPage.module.css";

function formatOperation(item: AlertSummaryListItem): string {
  return `${item.operation.verb} ${item.operation.requestURI}`;
}

function formatTarget(item: AlertSummaryListItem): string {
  const kind = [item.target.resource, item.target.subresource].filter(Boolean).join("/");
  return item.target.name ? `${kind} · ${item.target.name}` : kind;
}

function formatRequestTime(iso: string): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso;
  return date.toISOString().replace("T", " ").replace("Z", " UTC");
}

function AlertRow({ item }: { item: AlertSummaryListItem }) {
  return (
    <tr className={styles.row}>
      <td className={styles.cellAlertId}>
        <Link className={styles.rowLink} to={`/alerts/${item.alertId}`}>
          #{item.alertId}
        </Link>
      </td>
      <td className={styles.cellDetection} title={item.detectionName}>
        {item.detectionName}
      </td>
      <td className={styles.cellTechnical} title={item.subject.username}>
        {item.subject.username}
      </td>
      <td className={styles.cellTechnical} title={formatOperation(item)}>
        {formatOperation(item)}
      </td>
      <td className={styles.cellTechnical} title={formatTarget(item)}>
        {formatTarget(item)}
      </td>
      <td className={styles.cellTechnical}>{item.outcome.code ?? "—"}</td>
      <td className={styles.cellRequestTime}>{formatRequestTime(item.requestTime)}</td>
      <td className={styles.cellTraceability}>
        <StatusBadge tone={item.traceability.intact ? "intact" : "broken"}>
          {item.traceability.intact ? "Intact" : (item.traceability.failedLink ?? "Broken")}
        </StatusBadge>
      </td>
    </tr>
  );
}

function LoadingState() {
  return (
    <div className={styles.tableScroll} role="status" aria-live="polite">
      <span className="cnsdp-visually-hidden">Loading alert inventory…</span>
      {Array.from({ length: 6 }).map((_, i) => (
        <div key={i} className={styles.loadingRow} aria-hidden="true" />
      ))}
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
      <StatusBadge tone="broken">Inventory unavailable</StatusBadge>
      <h2 className={styles.stateTitle}>The alert inventory backend could not be reached</h2>
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
    <div className={styles.state} role="status">
      <StatusBadge tone="neutral">No alerts</StatusBadge>
      <h2 className={styles.stateTitle}>No alerts have been generated yet.</h2>
      <p className={styles.stateMessage}>
        Once telemetry is admitted, validated, normalized, and matched against a detection
        definition, generated alerts will appear here.
      </p>
    </div>
  );
}

/** The route component for `/alerts` — the product's primary entry point:
 *  a dense, deterministic inventory of every alert currently available
 *  from the real `GET /v1/alerts` endpoint (src/features/alert-inventory/lib/alertInventorySource.ts),
 *  each row navigating to its full Dark Evidence Map investigation at
 *  `/alerts/:alertId`. */
export function AlertInventoryPage() {
  const query = useAlertInventory();
  const { ref: scrollRef, hasOverflow } = useHorizontalOverflow<HTMLDivElement>();

  return (
    <AppShell>
      <div className={styles.wrap}>
        <div className={styles.toolbar}>
          <h1 className={styles.title}>Alerts</h1>
          <div className={styles.toolbarMeta}>
            {hasOverflow && (
              <span className={styles.scrollHint} role="note">
                ⇄ Scroll for more columns
              </span>
            )}
            {query.isSuccess && (
              <span className={styles.count}>
                {query.data.total} alert{query.data.total === 1 ? "" : "s"}
              </span>
            )}
          </div>
        </div>

        {query.isPending && <LoadingState />}

        {query.isError &&
          (query.error instanceof InventoryFetchError && query.error.kind === "unauthorized" ? (
            <UnauthorizedState />
          ) : (
            <ErrorState onRetry={() => query.refetch()} />
          ))}

        {query.isSuccess &&
          (query.data.alerts.length === 0 ? (
            <EmptyState />
          ) : (
            <div
              className={`${styles.tableScroll} cnsdp-scrollbar`}
              ref={scrollRef}
              tabIndex={hasOverflow ? 0 : undefined}
              role={hasOverflow ? "region" : undefined}
              aria-label={hasOverflow ? "Alert inventory table, scrollable horizontally" : undefined}
            >
              <table className={styles.table}>
                <thead>
                  <tr>
                    <th scope="col">Alert</th>
                    <th scope="col">Detection</th>
                    <th scope="col">Subject</th>
                    <th scope="col">Operation</th>
                    <th scope="col">Target</th>
                    <th scope="col">Outcome</th>
                    <th scope="col">Request time</th>
                    <th scope="col">Traceability</th>
                  </tr>
                </thead>
                <tbody>
                  {query.data.alerts.map((item) => (
                    <AlertRow key={item.alertId} item={item} />
                  ))}
                </tbody>
              </table>
            </div>
          ))}
      </div>
    </AppShell>
  );
}
