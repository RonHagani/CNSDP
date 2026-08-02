import { describe, expect, it, vi } from "vitest";
import { mockJsonResponse, stubFetchRoutes } from "@/test/mockFetch";
import { DataSourcesFetchError, fetchDataSources } from "./dataSourcesSource";

const noDataSource = {
  id: "audit-events",
  displayName: "Audit Events API",
  endpoint: "POST /v1/audit-events",
  eventCount: 0,
  lastEventAt: null,
};

const observedSource = {
  ...noDataSource,
  eventCount: 42,
  lastEventAt: "2026-08-02T12:00:00Z",
};

describe("fetchDataSources", () => {
  it("requests GET /v1/data-sources through the same-origin /api proxy prefix", async () => {
    const fetchMock = stubFetchRoutes({
      "/v1/data-sources": () => mockJsonResponse(200, { dataSources: [noDataSource], total: 1 }),
    });
    await fetchDataSources();
    expect(fetchMock).toHaveBeenCalledWith("/api/v1/data-sources", expect.anything());
  });

  it("resolves the backend's sources and total exactly as returned, including the no-data case", async () => {
    stubFetchRoutes({
      "/v1/data-sources": () => mockJsonResponse(200, { dataSources: [noDataSource], total: 1 }),
    });
    const result = await fetchDataSources();
    expect(result).toEqual({ dataSources: [noDataSource], total: 1 });
  });

  it("resolves the observed case with a non-null lastEventAt and positive eventCount", async () => {
    stubFetchRoutes({
      "/v1/data-sources": () => mockJsonResponse(200, { dataSources: [observedSource], total: 1 }),
    });
    const result = await fetchDataSources();
    expect(result.dataSources[0].eventCount).toBe(42);
    expect(result.dataSources[0].lastEventAt).toBe("2026-08-02T12:00:00Z");
  });

  it("rejects with an unauthorized DataSourcesFetchError for a 401", async () => {
    stubFetchRoutes({ "/v1/data-sources": () => mockJsonResponse(401, {}) });
    const error = await fetchDataSources().catch((e: unknown) => e);
    expect(error).toBeInstanceOf(DataSourcesFetchError);
    expect((error as DataSourcesFetchError).kind).toBe("unauthorized");
  });

  it("rejects with an unavailable DataSourcesFetchError for a 5xx", async () => {
    stubFetchRoutes({ "/v1/data-sources": () => mockJsonResponse(503, {}) });
    const error = await fetchDataSources().catch((e: unknown) => e);
    expect(error).toBeInstanceOf(DataSourcesFetchError);
    expect((error as DataSourcesFetchError).kind).toBe("unavailable");
  });

  it("rejects with an unavailable DataSourcesFetchError on a genuine network failure", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new TypeError("Failed to fetch")));
    const error = await fetchDataSources().catch((e: unknown) => e);
    expect(error).toBeInstanceOf(DataSourcesFetchError);
    expect((error as DataSourcesFetchError).kind).toBe("unavailable");
  });

  it("rejects with an unavailable DataSourcesFetchError when the body does not match the contract shape", async () => {
    stubFetchRoutes({ "/v1/data-sources": () => mockJsonResponse(200, { unexpected: true }) });
    const error = await fetchDataSources().catch((e: unknown) => e);
    expect(error).toBeInstanceOf(DataSourcesFetchError);
    expect((error as DataSourcesFetchError).kind).toBe("unavailable");
  });

  it("propagates a genuine AbortError untouched, never wrapped as a DataSourcesFetchError", async () => {
    const abortError = new DOMException("Aborted", "AbortError");
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(abortError));
    await expect(fetchDataSources()).rejects.toBe(abortError);
  });
});
