import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiError, apiGet } from "./client";

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

afterEach(() => {
  vi.unstubAllEnvs();
});

describe("apiGet", () => {
  it("requests the same-origin /api-prefixed path", async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, { ok: true }));
    vi.stubGlobal("fetch", fetchMock);

    await apiGet("/v1/alerts");

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url] = fetchMock.mock.calls[0];
    expect(url).toBe("/api/v1/alerts");
  });

  it("never constructs or attaches an Authorization header — the browser holds no credential", async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, { ok: true }));
    vi.stubGlobal("fetch", fetchMock);

    await apiGet("/v1/alerts");

    const [, init] = fetchMock.mock.calls[0];
    const headers = init.headers as Record<string, string> | Headers;
    const hasAuthHeader =
      headers instanceof Headers ? headers.has("Authorization") : "Authorization" in headers;
    expect(hasAuthHeader).toBe(false);
  });

  it("resolves with the parsed JSON body on success", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(jsonResponse(200, { hello: "world" })));
    await expect(apiGet("/v1/alerts")).resolves.toEqual({ hello: "world" });
  });

  it("passes the caller's AbortSignal through to fetch", async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, {}));
    vi.stubGlobal("fetch", fetchMock);
    const controller = new AbortController();

    await apiGet("/v1/alerts", { signal: controller.signal });

    expect(fetchMock.mock.calls[0][1].signal).toBe(controller.signal);
  });

  it.each([
    [401, "unauthorized"],
    [403, "unauthorized"],
    [404, "not-found"],
    [418, "client-error"],
    [429, "client-error"],
    [500, "server-error"],
    [503, "server-error"],
  ] as const)("maps HTTP %i to ApiError kind %s", async (status, kind) => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(jsonResponse(status, {})));

    const error = await apiGet("/v1/alerts").catch((e: unknown) => e);
    expect(error).toBeInstanceOf(ApiError);
    expect((error as ApiError).kind).toBe(kind);
    expect((error as ApiError).status).toBe(status);
  });

  it("throws a network-error ApiError when fetch itself rejects", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new TypeError("Failed to fetch")));

    const error = await apiGet("/v1/alerts").catch((e: unknown) => e);
    expect(error).toBeInstanceOf(ApiError);
    expect((error as ApiError).kind).toBe("network-error");
  });

  it("throws a malformed-response ApiError when the body is not valid JSON", async () => {
    const response = new Response("not json{{{", { status: 200 });
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(response));

    const error = await apiGet("/v1/alerts").catch((e: unknown) => e);
    expect(error).toBeInstanceOf(ApiError);
    expect((error as ApiError).kind).toBe("malformed-response");
  });

  it("rethrows a genuine AbortError untouched, not wrapped as an ApiError", async () => {
    const abortError = new DOMException("Aborted", "AbortError");
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(abortError));

    const error = await apiGet("/v1/alerts").catch((e: unknown) => e);
    expect(error).toBe(abortError);
    expect(error).not.toBeInstanceOf(ApiError);
  });
});
