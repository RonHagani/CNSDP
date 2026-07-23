/**
 * The single seam between the Alert Investigation feature and its data
 * source. Today this resolves versioned fixtures (src/fixtures); replacing
 * it with a real `fetch("/v1/alerts/" + id, { headers: { Authorization } })`
 * call is the only change required to connect this screen to the live
 * backend (product-experience-brief.md §10, delivery step 4) — every
 * consumer of `fetchAlertInvestigation` only ever sees
 * `AlertInvestigationResponse | AlertFetchError`, exactly the shape a real
 * `fetch` against `GET /v1/alerts/{id}` would produce.
 *
 * The demo-only `scenario` override exists purely to make the seven
 * required presentation states (product-experience-brief milestone spec)
 * reachable without a live backend. It has no equivalent in the real API —
 * a production build wired to the real endpoint would delete it, not keep
 * it as a hidden capability.
 */

import { fixturesById } from "@/fixtures/alert-investigation/v1";
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
 * Demo-only fetch scenario override, read from the `?demo=` query
 * parameter. Mirrors real HTTP failure modes the backend already documents
 * and tests (401 for a missing/invalid bearer token — NFR-012/NFR-013;
 * 404 for a nonexistent alert id — retrieval_integration_test.go
 * `TestServeHTTP_NonexistentAlert_Returns404`; a 5xx / network failure —
 * `TestServeHTTP_CancelledContext_Returns500ButNeverLeaksInternalDetails`,
 * whose response body never leaks internal detail, mirrored below by a
 * deliberately generic message) plus an artificially extended latency to
 * make the loading state directly observable.
 */
export type DemoScenario = "unauthorized" | "unavailable" | "slow";

const SIMULATED_LATENCY_MS = 260;
const SIMULATED_SLOW_LATENCY_MS = 4000;

function delay(ms: number, signal?: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(resolve, ms);
    signal?.addEventListener("abort", () => {
      clearTimeout(timer);
      reject(new DOMException("Aborted", "AbortError"));
    });
  });
}

export async function fetchAlertInvestigation(
  alertId: string,
  demoScenario?: DemoScenario,
  signal?: AbortSignal,
): Promise<AlertInvestigationResponse> {
  await delay(demoScenario === "slow" ? SIMULATED_SLOW_LATENCY_MS : SIMULATED_LATENCY_MS, signal);

  if (demoScenario === "unauthorized") {
    throw new AlertFetchError(
      "unauthorized",
      "401 Unauthorized — the bearer token was missing or did not match the platform's configured API_TOKEN.",
    );
  }
  if (demoScenario === "unavailable") {
    throw new AlertFetchError(
      "unavailable",
      "The investigation backend could not be reached. No internal error detail is available for display.",
    );
  }

  const fixture = fixturesById[alertId];
  if (!fixture) {
    throw new AlertFetchError("not-found", `No alert exists with id ${alertId}.`);
  }
  return fixture;
}
