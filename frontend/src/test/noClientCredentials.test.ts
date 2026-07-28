import { readdirSync, readFileSync, statSync } from "node:fs";
import path from "node:path";
import { describe, expect, it } from "vitest";

/**
 * Static proof that no production frontend module references a bearer
 * credential or its configuration. A real service credential must never
 * be constructed, held, or referenced by anything the browser executes
 * — see src/lib/api/client.ts and config.ts's own doc comments, and
 * frontend/.env.example's "two different kinds of variable" note.
 *
 * Scans every production `.ts`/`.tsx` file under `src/`, excluding:
 *   - `*.test.ts(x)` — test files legitimately assert on header behavior
 *     (src/lib/api/client.test.ts, e2e's mock backend);
 *   - `src/test/**` — test-only infrastructure (mockFetch.ts, this file
 *     itself), never imported by the shipped app;
 *   - `src/fixtures/**` — versioned fixture data, established test-only
 *     dependency injection (noFixtureImports.test.ts covers its own
 *     boundary already).
 *
 * `vite.config.ts` is not under `src/` at all, so it is naturally outside
 * this scan's scope — it is the one place server-side proxy code is
 * permitted to construct a credential-bearing header (vite.config.ts's
 * own doc comments explain why).
 */

const srcDir = path.resolve(process.cwd(), "src");

const EXCLUDED_DIR_PREFIXES = [path.join(srcDir, "test"), path.join(srcDir, "fixtures")];

function collectProductionFiles(dir: string): string[] {
  const files: string[] = [];
  for (const entry of readdirSync(dir)) {
    const full = path.join(dir, entry);
    if (EXCLUDED_DIR_PREFIXES.some((prefix) => full === prefix || full.startsWith(prefix + path.sep))) {
      continue;
    }
    const stat = statSync(full);
    if (stat.isDirectory()) {
      files.push(...collectProductionFiles(full));
    } else if (/\.(ts|tsx)$/.test(entry) && !/\.test\.(ts|tsx)$/.test(entry)) {
      files.push(full);
    }
  }
  return files;
}

const FORBIDDEN_LITERALS = ["VITE_API_TOKEN", "API_PROXY_TOKEN", "Authorization", "Bearer"];

describe("production frontend modules never reference a bearer credential", () => {
  const files = collectProductionFiles(srcDir);

  it("found a non-trivial set of production files to scan (the scan itself is not vacuous)", () => {
    expect(files.length).toBeGreaterThan(10);
  });

  it.each(files.map((f) => path.relative(srcDir, f)))("%s contains none of VITE_API_TOKEN, API_PROXY_TOKEN, Authorization, Bearer", (relativePath) => {
    const source = readFileSync(path.join(srcDir, relativePath), "utf8");
    for (const literal of FORBIDDEN_LITERALS) {
      expect(source.includes(literal)).toBe(false);
    }
  });
});
