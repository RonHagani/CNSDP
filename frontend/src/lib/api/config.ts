/**
 * Runtime API configuration for the production frontend.
 *
 * The browser always calls the same-origin path prefix `/api/*`, never
 * an absolute cross-origin backend URL and never with any credential
 * attached. Vite's dev server and preview server (vite.config.ts) proxy
 * that prefix to the real backend (`cmd/platform`), stripping the `/api`
 * prefix and — server-side only — attaching the service credential the
 * real backend requires. Neither the proxy's target nor its credential
 * is readable from this file or anywhere else in client code: they live
 * in two environment variables read by `vite.config.ts` via `loadEnv`,
 * named without Vite's client-exposure prefix on purpose — that prefix
 * is the one thing that determines whether Vite inlines a value into the
 * shipped browser bundle. See frontend/.env.example for the exact names
 * and why.
 */

export const API_BASE_PATH = "/api";
