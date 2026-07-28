import { describe, expect, it } from "vitest";
import { ApiError } from "@/lib/api/client";
import { fixtureIntact } from "@/fixtures/alert-investigation/v1";
import { validateAlertInventoryResponse, validateAlertInvestigationResponse } from "./validate";

describe("validateAlertInvestigationResponse", () => {
  it("accepts a real, well-shaped response unchanged", () => {
    expect(validateAlertInvestigationResponse(fixtureIntact)).toEqual(fixtureIntact);
  });

  it.each([
    ["not an object", null],
    ["a bare array", []],
    ["missing alertId", { ...fixtureIntact, alertId: undefined }],
    ["alertId not a number", { ...fixtureIntact, alertId: "1" }],
    ["missing traceability.intact", { ...fixtureIntact, traceability: {} }],
    ["missing sourceEvent.available", { ...fixtureIntact, sourceEvent: {} }],
  ])("rejects %s as malformed-response", (_label, malformed) => {
    const error = (() => {
      try {
        validateAlertInvestigationResponse(malformed);
        return null;
      } catch (e) {
        return e;
      }
    })();
    expect(error).toBeInstanceOf(ApiError);
    expect((error as ApiError).kind).toBe("malformed-response");
  });
});

describe("validateAlertInventoryResponse", () => {
  const wellShaped = {
    alerts: [
      {
        alertId: 1,
        detectionName: "Interactive container exec request",
        subject: { username: "kubernetes-admin" },
        operation: { verb: "get", requestURI: "/x" },
        target: { resource: "pods", name: "high-risk-pod" },
        outcome: { code: 101 },
        requestTime: "2026-07-21T15:25:11.901891Z",
        traceability: { intact: true },
      },
    ],
    total: 1,
  };

  it("accepts a well-shaped response unchanged", () => {
    expect(validateAlertInventoryResponse(wellShaped)).toEqual(wellShaped);
  });

  it("accepts an empty inventory", () => {
    expect(validateAlertInventoryResponse({ alerts: [], total: 0 })).toEqual({ alerts: [], total: 0 });
  });

  it.each([
    ["not an object", null],
    ["missing alerts array", { total: 1 }],
    ["alerts not an array", { alerts: {}, total: 1 }],
    ["missing total", { alerts: [] }],
    [
      "a row missing detectionName",
      { alerts: [{ ...wellShaped.alerts[0], detectionName: undefined }], total: 1 },
    ],
    [
      "a row with non-boolean traceability.intact",
      { alerts: [{ ...wellShaped.alerts[0], traceability: { intact: "yes" } }], total: 1 },
    ],
  ])("rejects %s as malformed-response", (_label, malformed) => {
    const error = (() => {
      try {
        validateAlertInventoryResponse(malformed);
        return null;
      } catch (e) {
        return e;
      }
    })();
    expect(error).toBeInstanceOf(ApiError);
    expect((error as ApiError).kind).toBe("malformed-response");
  });
});
