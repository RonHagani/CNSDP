/**
 * The single seam between the Submissions feature and its data source:
 * `GET /v1/submissions` (internal/retrieval/submissions.go), through the
 * shared typed API layer (src/lib/api/client.ts). Production runtime path
 * only — see alertInventorySource.ts's doc comment for why fixture data
 * never substitutes for this module.
 */

import { apiGet, ApiError } from "@/lib/api/client";
import { validateSubmissionsResponse } from "@/types/validate";
import type { SubmissionOutcomeFilter, SubmissionsResponse } from "@/types/contract";

export type SubmissionsFetchErrorKind = "unauthorized" | "unavailable";

export class SubmissionsFetchError extends Error {
  readonly kind: SubmissionsFetchErrorKind;

  constructor(kind: SubmissionsFetchErrorKind, message: string) {
    super(message);
    this.name = "SubmissionsFetchError";
    this.kind = kind;
  }
}

export interface FetchSubmissionsParams {
  /** Omitted (or `undefined`) requests every submission regardless of
   *  validation outcome, including pending ones. */
  outcome?: SubmissionOutcomeFilter;
  /** The `submissionId` of the last row of the previous page — omitted for
   *  the first page. Backend keyset cursor (see submissions.go). */
  cursor?: number | null;
  signal?: AbortSignal;
}

export async function fetchSubmissions(params: FetchSubmissionsParams = {}): Promise<SubmissionsResponse> {
  const query = new URLSearchParams();
  if (params.outcome) query.set("outcome", params.outcome);
  if (params.cursor != null) query.set("cursor", String(params.cursor));
  const qs = query.toString();

  try {
    const json = await apiGet(`/v1/submissions${qs ? `?${qs}` : ""}`, { signal: params.signal });
    return validateSubmissionsResponse(json);
  } catch (err) {
    if (err instanceof ApiError) {
      const kind: SubmissionsFetchErrorKind = err.kind === "unauthorized" ? "unauthorized" : "unavailable";
      throw new SubmissionsFetchError(kind, err.message);
    }
    throw err; // AbortError — let the caller's cancellation handling see it untouched
  }
}
