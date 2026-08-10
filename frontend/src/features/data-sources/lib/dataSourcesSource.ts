/**
 * The single seam between the Data Sources feature and its data source:
 * `GET /v1/data-sources` (internal/datasources/datasources.go), through the
 * shared typed API layer (src/lib/api/client.ts). Production runtime path
 * only — see alertInventorySource.ts's doc comment for why fixture data
 * never substitutes for this module.
 */

import { apiGet, ApiError } from "@/lib/api/client";
import { validateDataSourcesResponse } from "@/types/validate";
import type { DataSourcesResponse } from "@/types/contract";

export type DataSourcesFetchErrorKind = "unauthorized" | "unavailable";

export class DataSourcesFetchError extends Error {
  readonly kind: DataSourcesFetchErrorKind;

  constructor(kind: DataSourcesFetchErrorKind, message: string) {
    super(message);
    this.name = "DataSourcesFetchError";
    this.kind = kind;
  }
}

export async function fetchDataSources(signal?: AbortSignal): Promise<DataSourcesResponse> {
  try {
    const json = await apiGet("/v1/data-sources", { signal });
    return validateDataSourcesResponse(json);
  } catch (err) {
    if (err instanceof ApiError) {
      const kind: DataSourcesFetchErrorKind = err.kind === "unauthorized" ? "unauthorized" : "unavailable";
      throw new DataSourcesFetchError(kind, err.message);
    }
    throw err; // AbortError — let the caller's cancellation handling see it untouched
  }
}
