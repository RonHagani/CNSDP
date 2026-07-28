/**
 * The single seam between the Alert Investigation feature and its data
 * source: `GET /v1/alerts/{id}` (internal/retrieval/retrieval.go), through
 * the shared typed API layer (src/lib/api/client.ts). This is the
 * production runtime path — it imports no fixture and accepts no
 * synthetic scenario override. Fixture data (src/fixtures/alert-investigation/v1.ts)
 * still exists, but only as test-time dependency injection: component
 * tests and e2e specs mock `fetch` and hand back fixture-shaped bodies,
 * never by swapping this module's own data source.
 */

import { apiGet, ApiError, type ApiErrorKind } from "@/lib/api/client";
import { validateAlertInvestigationResponse } from "@/types/validate";
import type { AlertInvestigationResponse } from "@/types/contract";

export type AlertFetchErrorKind = "unauthorized" | "unavailable" | "not-found";

export class AlertFetchError extends Error {
  readonly kind: AlertFetchErrorKind;

  constructor(kind: AlertFetchErrorKind, message: string) {
    super(message);
    this.name = "AlertFetchError";
    this.kind = kind;
  }
}

/**
 * Maps the API layer's transport-level error vocabulary onto the
 * page-level presentation states the UI actually distinguishes (401/403 ->
 * "unauthorized", 404 -> "not-found", everything else -> "unavailable" --
 * consistent with NFR-022: a platform fault is reported without leaking
 * which specific transport failure produced it).
 */
function toAlertFetchErrorKind(kind: ApiErrorKind): AlertFetchErrorKind {
  switch (kind) {
    case "unauthorized":
      return "unauthorized";
    case "not-found":
      return "not-found";
    default:
      return "unavailable";
  }
}

export async function fetchAlertInvestigation(
  alertId: string,
  signal?: AbortSignal,
): Promise<AlertInvestigationResponse> {
  try {
    const json = await apiGet(`/v1/alerts/${encodeURIComponent(alertId)}`, { signal });
    return validateAlertInvestigationResponse(json);
  } catch (err) {
    if (err instanceof ApiError) {
      throw new AlertFetchError(toAlertFetchErrorKind(err.kind), err.message);
    }
    throw err; // AbortError — let the caller's cancellation handling see it untouched
  }
}
