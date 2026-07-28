import { defineConfig, loadEnv, type Plugin, type ProxyOptions } from "vite";
import react from "@vitejs/plugin-react";
import path from "node:path";
import { fileURLToPath } from "node:url";

const dirname = path.dirname(fileURLToPath(import.meta.url));

/**
 * Default backend target for the local reference environment
 * (docker-compose.yml's published `APP_PORT`, default 8080) —
 * overridable via `API_PROXY_TARGET` in `frontend/.env` (see
 * `.env.example`). Deliberately not `VITE_`-prefixed: Vite only inlines
 * `VITE_`-prefixed variables into the shipped client bundle, so this
 * name is what keeps the proxy's destination a Node-side, build-time
 * setting the browser can never read. The proxy always forwards to this
 * one explicitly configured origin — never a target derived from a
 * request, header, or query string. Kept in sync with
 * `docker-compose.yml`'s own `APP_PORT` default by
 * `src/test/proxyTargetValidation.test.ts`, which reads that file's text
 * directly rather than trusting this comment alone.
 */
export const DEFAULT_API_PROXY_TARGET = "http://127.0.0.1:8080";

// `mode` doesn't need to be the Vite-supplied one here: there are no
// mode-specific env files (.env.development, .env.production, ...), only
// a single local `.env` (see .env.example) that applies regardless of
// mode. A plain object config (rather than defineConfig's function form)
// keeps this file mergeable by vitest.config.ts's `mergeConfig`.
const env = loadEnv("development", dirname, "");
const apiProxyTarget = env.API_PROXY_TARGET || DEFAULT_API_PROXY_TARGET;
const apiProxyToken = env.API_PROXY_TOKEN ?? "";

/**
 * Rejects a malformed `API_PROXY_TARGET` with a clear, actionable error
 * at startup rather than letting a typo surface later as an opaque
 * `ECONNREFUSED`/`ENOTFOUND` on the first proxied request. Exported so
 * `src/test/proxyTargetValidation.test.ts` can exercise it directly with
 * both valid and invalid inputs.
 */
export function validateApiProxyTarget(target: string): void {
  try {
    void new URL(target);
  } catch {
    throw new Error(`vite.config.ts: API_PROXY_TARGET is not a valid URL: ${JSON.stringify(target)}`);
  }
}

validateApiProxyTarget(apiProxyTarget);

/**
 * Builds the `/api` proxy's own options — including the one place a
 * bearer credential exists anywhere in this frontend, entirely
 * server-side. Exported so `src/test/devProxyAuth.test.ts` can exercise
 * the real `proxyReq` handler in isolation, without starting an actual
 * HTTP server.
 *
 * Security boundary enforced here (never in browser code):
 *   - every proxied request gets `token` attached as its own
 *     `Authorization` header, added after the request already left the
 *     browser;
 *   - any `Authorization` header the browser itself supplied is removed
 *     first and unconditionally — this proxy never blindly forwards a
 *     client-presented credential, forged or otherwise;
 *   - proxy errors are logged by failure kind only, never with headers
 *     or the token value.
 */
export function createApiProxy(target: string, token: string): ProxyOptions {
  return {
    target,
    changeOrigin: true,
    rewrite: (requestPath) => requestPath.replace(/^\/api/, ""),
    configure(proxy) {
      proxy.on("proxyReq", (proxyReq) => {
        proxyReq.removeHeader("authorization");
        if (token) {
          proxyReq.setHeader("Authorization", `Bearer ${token}`);
        }
      });
      proxy.on("error", (err) => {
        console.error(`[dev proxy] /api request failed: ${err.name}: ${err.message}`);
      });
    },
  };
}

function requireApiProxyToken() {
  if (!apiProxyToken) {
    throw new Error(
      "vite.config.ts: API_PROXY_TOKEN is not set. The /api dev proxy requires a server-side " +
        "credential to reach the real backend — copy frontend/.env.example to frontend/.env and set " +
        "API_PROXY_TOKEN (never VITE_API_PROXY_TOKEN — see .env.example).",
    );
  }
}

/**
 * Fails server startup clearly when required secure configuration is
 * missing. Scoped to `configureServer`/`configurePreviewServer`, which
 * only run for `vite dev` / `vite preview` — never for `vite build`, and
 * never when vitest.config.ts's `mergeConfig` merely reads this file's
 * exported config object for the test runner. `npm run build`,
 * `npm run typecheck`, and `npm run test` are therefore unaffected by
 * whether a backend credential is configured at all.
 */
const requireApiProxyTokenPlugin: Plugin = {
  name: "cnsdp-require-api-proxy-token",
  configureServer() {
    requireApiProxyToken();
  },
  configurePreviewServer() {
    requireApiProxyToken();
  },
};

const apiProxy: Record<string, ProxyOptions> = {
  "/api": createApiProxy(apiProxyTarget, apiProxyToken),
};

export default defineConfig({
  plugins: [react(), requireApiProxyTokenPlugin],
  resolve: {
    alias: {
      "@": path.resolve(dirname, "src"),
    },
  },
  server: {
    port: 5173,
    strictPort: true,
    proxy: apiProxy,
  },
  preview: {
    host: "127.0.0.1",
    port: 4173,
    strictPort: true,
    proxy: apiProxy,
  },
});
