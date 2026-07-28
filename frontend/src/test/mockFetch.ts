import { vi } from "vitest";

/** Builds a real `Response` carrying a JSON body — the same shape
 *  `apiGet` (src/lib/api/client.ts) expects to parse. */
export function mockJsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

/**
 * Installs a global `fetch` stub for tests exercising a real production
 * data-source module (alertSource.ts, alertInventorySource.ts) without a
 * real backend. `routes` maps a URL suffix (e.g. `"/v1/alerts/1"`) to a
 * handler producing the `Response` (or a pending/rejecting promise, for
 * loading/network-failure cases) — matched by suffix so callers never
 * need to hardcode the `/api` prefix `client.ts` prepends. An unmapped
 * URL resolves 404, surfacing a clear test failure rather than a hang.
 */
export function stubFetchRoutes(routes: Record<string, () => Response | Promise<Response>>) {
  const fn = vi.fn((input: RequestInfo | URL) => {
    const url = typeof input === "string" ? input : input instanceof URL ? input.toString() : input.url;
    for (const [suffix, handler] of Object.entries(routes)) {
      if (url.endsWith(suffix)) return handler();
    }
    return Promise.resolve(mockJsonResponse(404, { error: `unmapped test route: ${url}` }));
  });
  vi.stubGlobal("fetch", fn);
  return fn;
}
