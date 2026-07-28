import { describe, expect, it, vi } from "vitest";
import { mockJsonResponse, stubFetchRoutes } from "@/test/mockFetch";
import { InventoryFetchError, fetchAlertInventory } from "./alertInventorySource";

// No afterEach fetch reset is needed: every test below stubs `fetch`
// itself before use, so the previous test's stub is simply overwritten
// (unlike `vi.unstubAllGlobals()`, which would also revert
// src/test/setup.ts's file-scoped ResizeObserver/IntersectionObserver
// stubs after the first test in a file that renders React components).

const wellShapedRow = {
  alertId: 1,
  detectionName: "Interactive container exec request",
  subject: { username: "kubernetes-admin" },
  operation: { verb: "get", requestURI: "/api/v1/namespaces/default/pods/high-risk-pod/exec" },
  target: { resource: "pods", name: "high-risk-pod", subresource: "exec" },
  outcome: { code: 101 },
  requestTime: "2026-07-21T15:25:11.901891Z",
  traceability: { intact: true },
};

describe("fetchAlertInventory", () => {
  it("requests GET /v1/alerts through the same-origin /api proxy prefix", async () => {
    const fetchMock = stubFetchRoutes({
      "/v1/alerts": () => mockJsonResponse(200, { alerts: [wellShapedRow], total: 1 }),
    });
    await fetchAlertInventory();
    expect(fetchMock).toHaveBeenCalledWith("/api/v1/alerts", expect.anything());
  });

  it("resolves the backend's rows and total exactly as returned", async () => {
    stubFetchRoutes({
      "/v1/alerts": () => mockJsonResponse(200, { alerts: [wellShapedRow, { ...wellShapedRow, alertId: 2 }], total: 2 }),
    });
    const result = await fetchAlertInventory();
    expect(result.total).toBe(2);
    expect(result.alerts.map((a) => a.alertId)).toEqual([1, 2]);
  });

  it("resolves an empty inventory from a real empty API response", async () => {
    stubFetchRoutes({ "/v1/alerts": () => mockJsonResponse(200, { alerts: [], total: 0 }) });
    await expect(fetchAlertInventory()).resolves.toEqual({ alerts: [], total: 0 });
  });

  it("rejects with an unauthorized InventoryFetchError for a 401", async () => {
    stubFetchRoutes({ "/v1/alerts": () => mockJsonResponse(401, {}) });
    const error = await fetchAlertInventory().catch((e: unknown) => e);
    expect(error).toBeInstanceOf(InventoryFetchError);
    expect((error as InventoryFetchError).kind).toBe("unauthorized");
  });

  it("rejects with an unauthorized InventoryFetchError for a 403", async () => {
    stubFetchRoutes({ "/v1/alerts": () => mockJsonResponse(403, {}) });
    const error = await fetchAlertInventory().catch((e: unknown) => e);
    expect(error).toBeInstanceOf(InventoryFetchError);
    expect((error as InventoryFetchError).kind).toBe("unauthorized");
  });

  it("rejects with an unavailable InventoryFetchError for a 5xx", async () => {
    stubFetchRoutes({ "/v1/alerts": () => mockJsonResponse(503, {}) });
    const error = await fetchAlertInventory().catch((e: unknown) => e);
    expect(error).toBeInstanceOf(InventoryFetchError);
    expect((error as InventoryFetchError).kind).toBe("unavailable");
  });

  it("rejects with an unavailable InventoryFetchError on a genuine network failure", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new TypeError("Failed to fetch")));
    const error = await fetchAlertInventory().catch((e: unknown) => e);
    expect(error).toBeInstanceOf(InventoryFetchError);
    expect((error as InventoryFetchError).kind).toBe("unavailable");
  });

  it("rejects with an unavailable InventoryFetchError when the body is not valid JSON", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response("not json", { status: 200 })));
    const error = await fetchAlertInventory().catch((e: unknown) => e);
    expect(error).toBeInstanceOf(InventoryFetchError);
    expect((error as InventoryFetchError).kind).toBe("unavailable");
  });

  it("rejects with an unavailable InventoryFetchError when the body does not match the contract shape", async () => {
    stubFetchRoutes({ "/v1/alerts": () => mockJsonResponse(200, { unexpected: true }) });
    const error = await fetchAlertInventory().catch((e: unknown) => e);
    expect(error).toBeInstanceOf(InventoryFetchError);
    expect((error as InventoryFetchError).kind).toBe("unavailable");
  });

  it("propagates a genuine AbortError untouched, never wrapped as an InventoryFetchError", async () => {
    const abortError = new DOMException("Aborted", "AbortError");
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(abortError));
    await expect(fetchAlertInventory()).rejects.toBe(abortError);
  });
});
