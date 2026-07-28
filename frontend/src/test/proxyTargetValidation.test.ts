// @vitest-environment node
//
// Importing vite.config.ts pulls in Vite's own Node-side tooling
// (esbuild via `loadEnv`), which breaks under jsdom's global
// TextEncoder/Uint8Array polyfills — this file needs the real Node
// environment, not the project-wide jsdom default (vitest.config.ts).
import fs from "node:fs";
import path from "node:path";
import { describe, expect, it } from "vitest";
import { DEFAULT_API_PROXY_TARGET, validateApiProxyTarget } from "../../vite.config";

/**
 * Guards the class of bug that actually broke local development once
 * already: a proxy target that looks plausible but doesn't point at the
 * backend the documented startup path (docs/reference-environment.md)
 * actually brings up on `docker-compose.yml`'s own default port.
 */
describe("validateApiProxyTarget", () => {
  it("accepts a well-formed http(s) origin", () => {
    expect(() => validateApiProxyTarget("http://127.0.0.1:8080")).not.toThrow();
    expect(() => validateApiProxyTarget("https://backend.example:8443")).not.toThrow();
  });

  it("rejects a malformed target with a clear, actionable error rather than a bare URL parse failure", () => {
    expect(() => validateApiProxyTarget("not-a-url")).toThrow(/API_PROXY_TARGET is not a valid URL/);
    expect(() => validateApiProxyTarget("")).toThrow(/API_PROXY_TARGET is not a valid URL/);
    expect(() => validateApiProxyTarget(":8080")).toThrow(/API_PROXY_TARGET is not a valid URL/);
  });
});

describe("DEFAULT_API_PROXY_TARGET stays in sync with the documented startup path", () => {
  const repoRoot = path.resolve(__dirname, "../../..");

  it("matches docker-compose.yml's own APP_PORT default", () => {
    const compose = fs.readFileSync(path.join(repoRoot, "docker-compose.yml"), "utf-8");
    const match = compose.match(/APP_PORT:-(\d+)/);
    expect(match, "docker-compose.yml should define an APP_PORT default as ${APP_PORT:-<port>}").not.toBeNull();
    const composeDefaultPort = match![1];

    expect(DEFAULT_API_PROXY_TARGET).toBe(`http://127.0.0.1:${composeDefaultPort}`);
  });

  it("is itself a valid target", () => {
    expect(() => validateApiProxyTarget(DEFAULT_API_PROXY_TARGET)).not.toThrow();
  });
});
