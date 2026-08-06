/**
 * The single seam between the Detections feature and its data source:
 * `GET /v1/detections` (internal/retrieval/detections.go), through the
 * shared typed API layer (src/lib/api/client.ts). Production runtime path
 * only — see alertInventorySource.ts's doc comment for why fixture data
 * never substitutes for this module.
 */

import { apiGet, ApiError } from "@/lib/api/client";
import { validateDetectionsResponse } from "@/types/validate";
import type { DetectionsResponse } from "@/types/contract";

export type DetectionsFetchErrorKind = "unauthorized" | "unavailable";

export class DetectionsFetchError extends Error {
  readonly kind: DetectionsFetchErrorKind;

  constructor(kind: DetectionsFetchErrorKind, message: string) {
    super(message);
    this.name = "DetectionsFetchError";
    this.kind = kind;
  }
}

export async function fetchDetections(signal?: AbortSignal): Promise<DetectionsResponse> {
  try {
    const json = await apiGet("/v1/detections", { signal });
    return validateDetectionsResponse(json);
  } catch (err) {
    if (err instanceof ApiError) {
      const kind: DetectionsFetchErrorKind = err.kind === "unauthorized" ? "unauthorized" : "unavailable";
      throw new DetectionsFetchError(kind, err.message);
    }
    throw err; // AbortError — let the caller's cancellation handling see it untouched
  }
}
