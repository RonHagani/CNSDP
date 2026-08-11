import { useState } from "react";
import { AppShell } from "@/app/AppShell";
import { StatusBadge, type StatusTone } from "@/components/StatusBadge/StatusBadge";
import { useHorizontalOverflow } from "@/features/alert-inventory/hooks/useHorizontalOverflow";
import type { SubmissionListItem, SubmissionOutcomeFilter } from "@/types/contract";
import { useSubmissions } from "./hooks/useSubmissions";
import { SubmissionsFetchError } from "./lib/submissionsSource";
import styles from "./SubmissionsPage.module.css";

const FILTERS: Array<{ label: string; value: SubmissionOutcomeFilter | undefined }> = [
  { label: "All", value: undefined },
  { label: "Pending", value: "pending" },
  { label: "Valid", value: "valid" },
  { label: "Invalid", value: "invalid" },
  { label: "Incomplete", value: "incomplete" },
  { label: "Unsupported", value: "unsupported" },
];

function outcomeTone(outcome: SubmissionListItem["validationOutcome"]): StatusTone {
  if (!outcome.available) return "neutral";
  switch (outcome.outcome) {
    case "valid":
      return "intact";
    case "incomplete":
      return "warning";
    case "invalid":
    case "unsupported":
      return "broken";
    default:
      return "neutral";
  }
}

function outcomeLabel(outcome: SubmissionListItem["validationOutcome"]): string {
  if (!outcome.available) return "Pending";
  switch (outcome.outcome) {
    case "valid":
      return "Valid";
    case "invalid":
      return "Invalid";
    case "incomplete":
      return "Incomplete";
    case "unsupported":
      return "Unsupported";
    default:
      return "—";
  }
}

function formatTimestamp(iso: string): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso;
  return date.toISOString().replace("T", " ").replace("Z", " UTC");
}

function FilterBar({
  active,
  onChange,
}: {
  active: SubmissionOutcomeFilter | undefined;
  onChange: (value: SubmissionOutcomeFilter | undefined) => void;
}) {
  return (
    <div className={styles.filterBar} role="group" aria-label="Filter by validation outcome">
      {FILTERS.map((f) => (
        <button
          key={f.label}
          type="button"
          className={styles.filterButton}
          aria-pressed={active === f.value}
          onClick={() => onChange(f.value)}
        >
          {f.label}
        </button>
      ))}
    </div>
  );
}

function SubmissionRow({ item }: { item: SubmissionListItem }) {
  const { validationOutcome } = item;
  const hasAuditId = item.auditId.length > 0;
  const hasReason = Boolean(validationOutcome.reason);
  return (
    <tr className={styles.row}>
      <td className={styles.cellId}>#{item.submissionId}</td>
      <td className={styles.cellTechnical}>{item.status}</td>
      <td
        className={hasAuditId ? `${styles.cellTechnical} ${styles.hasTooltip}` : styles.cellTechnical}
        title={hasAuditId ? item.auditId : undefined}
      >
        {item.auditId}
      </td>
      <td className={styles.cellTechnical}>{item.auditStage}</td>
      <td className={styles.cellCreatedAt}>{formatTimestamp(item.createdAt)}</td>
      <td className={styles.cellOutcome}>
        <StatusBadge tone={outcomeTone(validationOutcome)}>{outcomeLabel(validationOutcome)}</StatusBadge>
      </td>
      <td
        className={hasReason ? `${styles.cellReason} ${styles.hasTooltip}` : styles.cellReason}
        title={validationOutcome.reason}
      >
        {validationOutcome.reason || "—"}
      </td>
    </tr>
  );
}

function LoadingState() {
  return (
    <div className={styles.tableScroll} role="status" aria-live="polite">
      <span className="cnsdp-visually-hidden">Loading submissions…</span>
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
      <StatusBadge tone="broken">Submissions unavailable</StatusBadge>
      <h2 className={styles.stateTitle}>The submissions backend could not be reached</h2>
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

function EmptyState({ filter }: { filter: SubmissionOutcomeFilter | undefined }) {
  const label = FILTERS.find((f) => f.value === filter)?.label ?? "All";
  return (
    <div className={styles.state} role="status">
      <StatusBadge tone="neutral">No submissions</StatusBadge>
      <h2 className={styles.stateTitle}>
        {filter === undefined ? "No submissions have been received yet." : `No submissions currently match "${label}".`}
      </h2>
      <p className={styles.stateMessage}>
        Once telemetry reaches the defined intake, every submission — including invalid, incomplete,
        and unsupported ones, and valid submissions that don't match a detection — is reviewable here.
      </p>
    </div>
  );
}

/** The route component for `/submissions`: a keyset-paginated, read-only
 *  review of every submission received at the defined intake, from the
 *  real `GET /v1/submissions` endpoint
 *  (src/features/submissions/lib/submissionsSource.ts). Closes the
 *  FR-011/FR-012/FR-013 reviewability gap for submissions that never
 *  produce an alert — invalid, incomplete, and unsupported submissions,
 *  and valid submissions that don't match any detection scenario — none of
 *  which the Alert Inventory ever surfaces, since it is keyed off alerts.
 *  Carries no submission-detail or raw-event inspection view and no
 *  navigation into one: this is the list-only foundation slice. */
export function SubmissionsPage() {
  const [filter, setFilter] = useState<SubmissionOutcomeFilter | undefined>(undefined);
  const query = useSubmissions(filter);
  const { ref: scrollRef, hasOverflow } = useHorizontalOverflow<HTMLDivElement>();

  const rows = query.data?.pages.flatMap((page) => page.submissions) ?? [];
  // The newest fetched page's total, not the first: each page computes its
  // own total independently at fetch time (no shared transaction across
  // pages), so the latest page is the most current snapshot available --
  // using pages[0] would freeze the displayed count to what it was before
  // any "Load more" click, even as later pages observe newer submissions.
  const total = query.data?.pages.at(-1)?.total ?? 0;

  return (
    <AppShell>
      <div className={styles.wrap}>
        <div className={styles.toolbar}>
          <h1 className={styles.title}>Submissions</h1>
          <div className={styles.toolbarMeta}>
            {hasOverflow && (
              <span className={styles.scrollHint} role="note">
                ⇄ Scroll for more columns
              </span>
            )}
            {query.isSuccess && (
              <span className={styles.count}>
                {rows.length} of {total} submission{total === 1 ? "" : "s"}
              </span>
            )}
          </div>
        </div>

        <FilterBar active={filter} onChange={setFilter} />

        {query.isPending && <LoadingState />}

        {query.isError &&
          (query.error instanceof SubmissionsFetchError && query.error.kind === "unauthorized" ? (
            <UnauthorizedState />
          ) : (
            <ErrorState onRetry={() => query.refetch()} />
          ))}

        {query.isSuccess &&
          (rows.length === 0 ? (
            <EmptyState filter={filter} />
          ) : (
            <>
              <div
                className={`${styles.tableScroll} cnsdp-scrollbar`}
                ref={scrollRef}
                tabIndex={hasOverflow ? 0 : undefined}
                role={hasOverflow ? "region" : undefined}
                aria-label={hasOverflow ? "Submissions table, scrollable horizontally" : undefined}
              >
                <table className={styles.table}>
                  <thead>
                    <tr>
                      <th scope="col">Submission</th>
                      <th scope="col">Status</th>
                      <th scope="col">Audit ID</th>
                      <th scope="col">Audit stage</th>
                      <th scope="col">Received</th>
                      <th scope="col">Outcome</th>
                      <th scope="col">Reason</th>
                    </tr>
                  </thead>
                  <tbody>
                    {rows.map((item) => (
                      <SubmissionRow key={item.submissionId} item={item} />
                    ))}
                  </tbody>
                </table>
              </div>

              {query.hasNextPage && (
                <div className={styles.loadMoreWrap}>
                  <button
                    type="button"
                    className={styles.loadMoreButton}
                    onClick={() => query.fetchNextPage()}
                    disabled={query.isFetchingNextPage}
                  >
                    {query.isFetchingNextPage ? "Loading…" : "Load more"}
                  </button>
                </div>
              )}
            </>
          ))}
      </div>
    </AppShell>
  );
}
