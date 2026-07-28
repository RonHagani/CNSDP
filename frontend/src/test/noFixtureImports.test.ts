import { readFileSync } from "node:fs";
import path from "node:path";
import { describe, expect, it } from "vitest";

/**
 * Static proof that the production runtime path never imports fixture
 * data. Component/e2e tests are free to import `fixturesById` for
 * test-only dependency injection (contract.ts's own doc comments and
 * alertSource.ts/alertInventorySource.ts's headers explain why) — but the
 * modules that actually run in the shipped app must not, or a real
 * backend alert would silently coexist with a client-only fixture list
 * that never matches it.
 */

const srcDir = path.resolve(process.cwd(), "src");

const PRODUCTION_RUNTIME_FILES = [
  "features/alert-inventory/AlertInventoryPage.tsx",
  "features/alert-inventory/lib/alertInventorySource.ts",
  "features/alert-inventory/hooks/useAlertInventory.ts",
  "features/alert-investigation/AlertInvestigationPage.tsx",
  "features/alert-investigation/lib/alertSource.ts",
  "features/alert-investigation/hooks/useAlertInvestigation.ts",
  "app/router.tsx",
  "app/AppShell.tsx",
];

const FORBIDDEN_PATTERNS = [/fixturesById/, /from ["']@\/fixtures/, /from ["'].*fixtures\/alert-investigation/];

describe("production runtime modules never import fixture data", () => {
  it.each(PRODUCTION_RUNTIME_FILES)("%s contains no fixture import or reference", (relativePath) => {
    const source = readFileSync(path.join(srcDir, relativePath), "utf8");
    for (const pattern of FORBIDDEN_PATTERNS) {
      expect(source).not.toMatch(pattern);
    }
  });
});
