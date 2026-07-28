// @vitest-environment node
//
// Importing vite.config.ts pulls in Vite's own Node-side tooling
// (esbuild via `loadEnv`), which breaks under jsdom's global
// TextEncoder/Uint8Array polyfills — this file needs the real Node
// environment, not the project-wide jsdom default (vitest.config.ts).
import { EventEmitter } from "node:events";
import { describe, expect, it, vi } from "vitest";
import type { HttpProxy } from "vite";
import { createApiProxy } from "../../vite.config";

/**
 * Exercises the dev/preview proxy's own server-side credential boundary
 * (vite.config.ts's `createApiProxy`) directly, without starting a real
 * HTTP server: `configure(proxy, options)` is called exactly the way
 * Vite itself calls it, with a real `EventEmitter` standing in for
 * `http-proxy`'s `Server` (which is itself just an `EventEmitter` with
 * extra methods this code never calls) and a minimal fake `proxyReq`
 * exposing only the header-mutation surface node-http-proxy's real
 * `proxyReq` event handler receives.
 */

function fakeProxyReq(initialHeaders: Record<string, string> = {}) {
  const headers = new Map(Object.entries(initialHeaders).map(([k, v]) => [k.toLowerCase(), v]));
  return {
    setHeader(name: string, value: string) {
      headers.set(name.toLowerCase(), value);
    },
    getHeader(name: string) {
      return headers.get(name.toLowerCase());
    },
    removeHeader(name: string) {
      headers.delete(name.toLowerCase());
    },
  };
}

function wireProxy(target: string, token: string) {
  const options = createApiProxy(target, token);
  const proxy = new EventEmitter() as unknown as HttpProxy.Server;
  options.configure!(proxy, options);
  return proxy;
}

describe("createApiProxy — server-side credential boundary", () => {
  it("attaches the configured server-side token as the outbound Authorization header", () => {
    const proxy = wireProxy("http://backend.invalid", "server-side-secret");
    const proxyReq = fakeProxyReq();

    proxy.emit("proxyReq", proxyReq, {}, {}, {});

    expect(proxyReq.getHeader("Authorization")).toBe("Bearer server-side-secret");
  });

  it("overwrites a browser-forged Authorization header rather than forwarding it unchanged", () => {
    const proxy = wireProxy("http://backend.invalid", "server-side-secret");
    // node-http-proxy has already copied the inbound request's headers
    // onto proxyReq by the time the `proxyReq` event fires — simulate a
    // client that sent its own forged credential.
    const proxyReq = fakeProxyReq({ Authorization: "Bearer forged-client-value" });

    proxy.emit("proxyReq", proxyReq, {}, {}, {});

    const finalHeader = proxyReq.getHeader("Authorization");
    expect(finalHeader).toBe("Bearer server-side-secret");
    expect(finalHeader).not.toContain("forged-client-value");
  });

  it("removes an inbound Authorization header outright when no server-side token is configured", () => {
    const proxy = wireProxy("http://backend.invalid", "");
    const proxyReq = fakeProxyReq({ Authorization: "Bearer forged-client-value" });

    proxy.emit("proxyReq", proxyReq, {}, {}, {});

    expect(proxyReq.getHeader("Authorization")).toBeUndefined();
  });

  it("logs a proxy error by failure kind only, never a credential value", () => {
    const proxy = wireProxy("http://backend.invalid", "server-side-secret");
    const logSpy = vi.spyOn(console, "error").mockImplementation(() => {});

    proxy.emit("error", new Error("connect ECONNREFUSED"), {}, {});

    expect(logSpy).toHaveBeenCalledTimes(1);
    const loggedText = logSpy.mock.calls[0].join(" ");
    expect(loggedText).not.toContain("server-side-secret");
    expect(loggedText).toContain("ECONNREFUSED");
    logSpy.mockRestore();
  });
});
