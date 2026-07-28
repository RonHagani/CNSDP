/**
 * Minimal runtime shape validation for the two live API responses
 * (contract.ts's own doc comments name the exact backend types each
 * interface mirrors). Fixture data is authored in TypeScript and
 * type-checked at compile time, so it never needed this; a real network
 * response carries no such guarantee, so each production data source
 * validates its response through these functions before trusting it as
 * the typed contract — an unrecognized shape becomes an `ApiError`
 * ("malformed-response"), never a value silently cast and passed on.
 *
 * Deliberately shallow: this checks the fields a consumer actually reads
 * (AlertIdentityHeader, the evidence-map artifacts, the inventory table)
 * are present with the right primitive types, not a full recursive
 * schema of every optional nested field — consistent with this project's
 * "smallest honest" validation, not a generic schema-validation library.
 */

import { ApiError } from "@/lib/api/client";
import type { AlertInventoryResponse, AlertInvestigationResponse } from "./contract";

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function hasAvailableFlag(value: unknown): value is { available: boolean } {
  return isRecord(value) && typeof value.available === "boolean";
}

export function validateAlertInvestigationResponse(json: unknown): AlertInvestigationResponse {
  if (
    !isRecord(json) ||
    typeof json.alertId !== "number" ||
    !hasAvailableFlag(json.sourceEvent) ||
    !hasAvailableFlag(json.validationOutcome) ||
    !hasAvailableFlag(json.normalizedEvent) ||
    !hasAvailableFlag(json.detectionDefinition) ||
    !hasAvailableFlag(json.detectionResult) ||
    !hasAvailableFlag(json.alert) ||
    !isRecord(json.traceability) ||
    typeof json.traceability.intact !== "boolean"
  ) {
    throw new ApiError(
      "malformed-response",
      "The alert investigation response did not match the expected AlertInvestigationResponse shape.",
    );
  }
  return json as unknown as AlertInvestigationResponse;
}

export function validateAlertInventoryResponse(json: unknown): AlertInventoryResponse {
  if (!isRecord(json) || !Array.isArray(json.alerts) || typeof json.total !== "number") {
    throw new ApiError(
      "malformed-response",
      "The alert inventory response did not match the expected AlertInventoryResponse shape.",
    );
  }
  for (const item of json.alerts) {
    if (
      !isRecord(item) ||
      typeof item.alertId !== "number" ||
      typeof item.detectionName !== "string" ||
      !isRecord(item.subject) ||
      !isRecord(item.operation) ||
      !isRecord(item.target) ||
      !isRecord(item.outcome) ||
      typeof item.requestTime !== "string" ||
      !isRecord(item.traceability) ||
      typeof item.traceability.intact !== "boolean"
    ) {
      throw new ApiError(
        "malformed-response",
        "An alert inventory row did not match the expected AlertSummaryListItem shape.",
      );
    }
  }
  return json as unknown as AlertInventoryResponse;
}
