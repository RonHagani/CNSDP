/**
 * The single seam between the Alert Inventory feature and its data
 * source: `GET /v1/alerts` (internal/retrieval/list.go), through the
 * shared typed API layer (src/lib/api/client.ts). This is the production
 * runtime path — it imports no fixture, derives no id list of its own,
 * and accepts no synthetic scenario override. Fixture data
 * (src/fixtures/alert-investigation/v1.ts) still exists, but only as
 * test-time dependency injection: component tests and e2e specs mock
 * `fetch` and hand back fixture-shaped bodies, never by swapping this
 * module's own data source.
 */

import { apiGet, ApiError } from "@/lib/api/client";
import { validateAlertInventoryResponse } from "@/types/validate";
import type { AlertInventoryResponse } from "@/types/contract";

export type InventoryFetchErrorKind = "unauthorized" | "unavailable";

export class InventoryFetchError extends Error {
  readonly kind: InventoryFetchErrorKind;

  constructor(kind: InventoryFetchErrorKind, message: string) {
    super(message);
    this.name = "InventoryFetchError";
    this.kind = kind;
  }
}

export async function fetchAlertInventory(signal?: AbortSignal): Promise<AlertInventoryResponse> {
  try {
    const json = await apiGet("/v1/alerts", { signal });
    return validateAlertInventoryResponse(json);
  } catch (err) {
    if (err instanceof ApiError) {
      const kind: InventoryFetchErrorKind = err.kind === "unauthorized" ? "unauthorized" : "unavailable";
      throw new InventoryFetchError(kind, err.message);
    }
    throw err; // AbortError — let the caller's cancellation handling see it untouched
  }
}
