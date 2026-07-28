import { execFileSync } from "node:child_process";
import { randomUUID } from "node:crypto";
import { readFileSync, readdirSync, statSync } from "node:fs";
import path from "node:path";
import { beforeAll, describe, expect, it } from "vitest";

/**
 * Proves — against a real production build, not a unit-level assertion —
 * that a service credential present in the build environment as
 * `API_PROXY_TOKEN` (server-side, no `VITE_` prefix) never ends up in
 * the shipped client output. A unique, freshly generated sentinel value
 * stands in for a real token, so a false pass can't happen just because
 * the value already appeared in the repository for some unrelated
 * reason.
 *
 * Deliberately never logs the sentinel itself: every assertion below
 * compares a derived boolean (`includes(...)`), never the raw string, so
 * a failure's diff output cannot leak it either.
 */

const frontendRoot = process.cwd();
const distDir = path.join(frontendRoot, "dist");
const sentinel = `cnsdp-build-leak-sentinel-${randomUUID()}`;

function collectFiles(dir: string): string[] {
  const out: string[] = [];
  for (const entry of readdirSync(dir)) {
    const full = path.join(dir, entry);
    const stat = statSync(full);
    if (stat.isDirectory()) out.push(...collectFiles(full));
    else out.push(full);
  }
  return out;
}

beforeAll(() => {
  // Invokes Vite's CLI entry point directly via `node`, rather than
  // `npx vite build` — `execFileSync` doesn't go through a shell, and on
  // Windows `npx`/`vite` are `.cmd` shims that shell-less spawning can't
  // resolve.
  const viteBin = path.join(frontendRoot, "node_modules/vite/bin/vite.js");
  execFileSync(process.execPath, [viteBin, "build"], {
    cwd: frontendRoot,
    env: {
      ...process.env,
      API_PROXY_TARGET: "http://127.0.0.1:8080",
      API_PROXY_TOKEN: sentinel,
    },
    stdio: "pipe",
  });
}, 60_000);

describe("production build output never contains a service credential", () => {
  it("the build actually produced output to scan (the scan itself is not vacuous)", () => {
    const files = collectFiles(distDir);
    expect(files.length).toBeGreaterThan(0);
  });

  it("no built file contains the unique build-time sentinel credential", () => {
    const files = collectFiles(distDir);
    const offenders = files.filter((f) => readFileSync(f, "utf8").includes(sentinel));
    expect(offenders).toEqual([]);
  });

  it("no built file contains the literal variable name VITE_API_TOKEN", () => {
    const files = collectFiles(distDir);
    const offenders = files.filter((f) => readFileSync(f, "utf8").includes("VITE_API_TOKEN"));
    expect(offenders).toEqual([]);
  });

  it("no built file contains the literal variable name API_PROXY_TOKEN", () => {
    const files = collectFiles(distDir);
    const offenders = files.filter((f) => readFileSync(f, "utf8").includes("API_PROXY_TOKEN"));
    expect(offenders).toEqual([]);
  });

  it("scans every asset kind the build produces, including source maps if any exist", () => {
    const files = collectFiles(distDir);
    const extensions = new Set(files.map((f) => path.extname(f)));
    // A coarse sanity check that the scan covers real, varied output —
    // not just, say, index.html alone.
    expect(extensions.size).toBeGreaterThan(1);
  });
});
