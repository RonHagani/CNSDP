import { useInfiniteQuery } from "@tanstack/react-query";
import type { SubmissionOutcomeFilter } from "@/types/contract";
import { fetchSubmissions } from "../lib/submissionsSource";

/**
 * Cursor-forward pagination over `GET /v1/submissions`: each call to
 * `fetchNextPage` requests the next keyset page using the previous page's
 * own `nextCursor`, and TanStack Query accumulates the pages — the client
 * counterpart to the backend's keyset (not offset) pagination design (see
 * internal/retrieval/submissions.go's package doc): a page boundary is
 * always expressed as "after this submission id," which stays correct
 * while the worker concurrently admits and advances submissions, unlike an
 * OFFSET-based page number would.
 *
 * `outcome` is part of the query key so switching the filter starts a
 * fresh, independently cached page sequence rather than mixing pages
 * fetched under a different filter.
 */
export function useSubmissions(outcome: SubmissionOutcomeFilter | undefined) {
  return useInfiniteQuery({
    queryKey: ["submissions", outcome ?? "all"],
    queryFn: ({ pageParam, signal }) => fetchSubmissions({ outcome, cursor: pageParam, signal }),
    initialPageParam: null as number | null,
    getNextPageParam: (lastPage) => lastPage.nextCursor,
    retry: false,
    staleTime: 30_000,
  });
}
