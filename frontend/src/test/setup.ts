import "@testing-library/jest-dom/vitest";
import { afterEach, vi } from "vitest";
import { cleanup } from "@testing-library/react";

// A handful of test files (e.g. devProxyAuth.test.ts) opt into the real
// Node environment via `// @vitest-environment node` instead of the
// project-wide jsdom default (vitest.config.ts), because they exercise
// Node-only tooling that breaks under jsdom's global polyfills. This
// setup file still runs for those too (vitest applies `setupFiles`
// regardless of per-file environment), so every DOM-dependent stub below
// is guarded — a plain no-op when there is no `document` to patch.
if (typeof document !== "undefined") {
  // jsdom does not implement IntersectionObserver, which some Framer Motion
  // features probe for at mount time regardless of whether a given
  // component uses `whileInView` — a minimal no-op stand-in is sufficient
  // here since tests assert on final rendered state, not scroll timing.
  class MockIntersectionObserver implements IntersectionObserver {
    readonly root: Element | Document | null = null;
    readonly rootMargin: string = "";
    readonly thresholds: ReadonlyArray<number> = [];
    observe() {}
    unobserve() {}
    disconnect() {}
    takeRecords(): IntersectionObserverEntry[] {
      return [];
    }
  }
  vi.stubGlobal("IntersectionObserver", MockIntersectionObserver);

  // jsdom also does not implement ResizeObserver; stub it out in case a
  // future component observes element size (nothing currently does).
  class MockResizeObserver implements ResizeObserver {
    observe() {}
    unobserve() {}
    disconnect() {}
  }
  vi.stubGlobal("ResizeObserver", MockResizeObserver);

  // jsdom does not implement Element.scrollIntoView either, used by the
  // command palette's focus-and-scroll commands (useInvestigationCommands).
  Element.prototype.scrollIntoView = vi.fn();

  // vitest.config.ts runs with `globals: false`, so @testing-library/react's
  // automatic afterEach cleanup (which detects a global test API) never
  // registers on its own — without this, DOM from one test leaks into the
  // next and produces false "multiple elements found" failures.
  afterEach(() => {
    cleanup();
  });
}
