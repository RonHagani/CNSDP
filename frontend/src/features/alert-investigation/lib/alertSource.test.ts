import { describe, expect, it, vi } from "vitest";
import { fixtureBrokenTraceability, fixtureIntact } from "@/fixtures/alert-investigation/v1";
import { mockJsonResponse, stubFetchRoutes } from "@/test/mockFetch";
import { AlertFetchError, fetchAlertInvestigation } from "./alertSource";

describe("fetchAlertInvestigation", () => {
  it("requests GET /v1/alerts/{id} through the same-origin /api proxy prefix", async () => {
    const fetchMock = stubFetchRoutes({ "/v1/alerts/1": () => mockJsonResponse(200, fixtureIntact) });
    await fetchAlertInvestigation("1");
    expect(fetchMock).toHaveBeenCalledWith("/api/v1/alerts/1", expect.anything());
  });

  it("resolves the backend's response body for a matched, intact alert", async () => {
    stubFetchRoutes({ "/v1/alerts/1": () => mockJsonResponse(200, fixtureIntact) });
    const result = await fetchAlertInvestigation("1");
    expect(result.alertId).toBe(1);
    expect(result.traceability.intact).toBe(true);
  });

  it("resolves a broken-traceability response exactly as the backend reports it", async () => {
    stubFetchRoutes({ "/v1/alerts/3": () => mockJsonResponse(200, fixtureBrokenTraceability) });
    const result = await fetchAlertInvestigation("3");
    expect(result.traceability.intact).toBe(false);
    expect(result.traceability.failedLink).toBe("raw_event_sha256");
  });

  it("rejects with a not-found AlertFetchError for a 404", async () => {
    stubFetchRoutes({ "/v1/alerts/999": () => mockJsonResponse(404, {}) });
    await expect(fetchAlertInvestigation("999")).rejects.toMatchObject({
      name: "AlertFetchError",
      kind: "not-found",
    });
  });

  it("rejects with an unauthorized AlertFetchError for a 401", async () => {
    stubFetchRoutes({ "/v1/alerts/1": () => mockJsonResponse(401, {}) });
    const error = await fetchAlertInvestigation("1").catch((e: unknown) => e);
    expect(error).toBeInstanceOf(AlertFetchError);
    expect((error as AlertFetchError).kind).toBe("unauthorized");
  });

  it("rejects with an unauthorized AlertFetchError for a 403", async () => {
    stubFetchRoutes({ "/v1/alerts/1": () => mockJsonResponse(403, {}) });
    const error = await fetchAlertInvestigation("1").catch((e: unknown) => e);
    expect(error).toBeInstanceOf(AlertFetchError);
    expect((error as AlertFetchError).kind).toBe("unauthorized");
  });

  it("rejects with an unavailable AlertFetchError for a 5xx", async () => {
    stubFetchRoutes({ "/v1/alerts/1": () => mockJsonResponse(500, {}) });
    const error = await fetchAlertInvestigation("1").catch((e: unknown) => e);
    expect(error).toBeInstanceOf(AlertFetchError);
    expect((error as AlertFetchError).kind).toBe("unavailable");
  });

  it("rejects with an unavailable AlertFetchError on a genuine network failure", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new TypeError("Failed to fetch")));
    const error = await fetchAlertInvestigation("1").catch((e: unknown) => e);
    expect(error).toBeInstanceOf(AlertFetchError);
    expect((error as AlertFetchError).kind).toBe("unavailable");
  });

  it("rejects with an unavailable AlertFetchError when the response body is not valid JSON", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response("not json{{{", { status: 200 })));
    const error = await fetchAlertInvestigation("1").catch((e: unknown) => e);
    expect(error).toBeInstanceOf(AlertFetchError);
    expect((error as AlertFetchError).kind).toBe("unavailable");
  });

  it("rejects with an unavailable AlertFetchError when the response body does not match the contract shape", async () => {
    stubFetchRoutes({ "/v1/alerts/1": () => mockJsonResponse(200, { unexpected: true }) });
    const error = await fetchAlertInvestigation("1").catch((e: unknown) => e);
    expect(error).toBeInstanceOf(AlertFetchError);
    expect((error as AlertFetchError).kind).toBe("unavailable");
  });

  it("propagates a genuine AbortError untouched, never wrapped as an AlertFetchError", async () => {
    const abortError = new DOMException("Aborted", "AbortError");
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(abortError));
    await expect(fetchAlertInvestigation("1")).rejects.toBe(abortError);
  });
});
