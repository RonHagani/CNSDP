import { test, expect } from "@playwright/test";
import { mockAlertStatus, mockFixtureBackend } from "./support/mockApi";

/**
 * Visual and interaction verification for the Dark Evidence Map — the live
 * Alert Investigation presentation as of Track B Pass 5's atomic shell
 * swap (InvestigationMap replacing the retired Forensic Case Folio). Runs
 * against the production preview build (playwright.config.ts points
 * baseURL at `vite preview`). Captures the required review screenshots
 * into review-artifacts/ (gitignored — never committed) and exercises
 * condition selection, keyboard operation, every route-level state, and
 * structural integrity (no horizontal overflow) at the UX spec's own
 * desktop viewport floor (1024px) and principal target range
 * (1440–1600px).
 *
 * The production frontend now calls the real `GET /v1/alerts/{id}`
 * endpoint (through the same-origin `/api` proxy — see vite.config.ts and
 * src/lib/api/client.ts), not a bundled fixture. `mockFixtureBackend`
 * (e2e/support/mockApi.ts) installs a network-level stand-in backend
 * serving the same committed fixtures every test in this file already
 * depended on, so the scenario/content coverage below is unchanged —
 * only the demo-scenario query-param mechanism it used to rely on for
 * the unauthorized/unavailable states is gone, replaced by a per-test
 * route status override (`mockAlertStatus`). The one real,
 * non-intercepted backend journey lives in e2e/real-backend.spec.ts.
 */

test.beforeEach(async ({ page }) => {
  await mockFixtureBackend(page);
});

const DESKTOP_VIEWPORTS = [
  ["1024px (viewport floor)", { width: 1024, height: 900 }],
  ["1440px (principal)", { width: 1440, height: 960 }],
  ["1600px (principal)", { width: 1600, height: 1000 }],
] as const;

test.describe("Alert Investigation — screenshots", () => {
  test("01 — scenario 1, wide desktop, default state", async ({ page }) => {
    await page.setViewportSize({ width: 1600, height: 1000 });
    await page.goto("/alerts/1");

    await expect(page.getByText("Alert #1", { exact: false }).first()).toBeVisible();
    await expect(page.getByText("Traceability intact").first()).toBeVisible();
    await expect(page.getByRole("heading", { name: "Source submission" })).toBeVisible();
    await expect(page.getByRole("heading", { name: "Detection result" })).toBeVisible();
    // No pin preselected on load.
    await expect(page.getByRole("group", { name: /Provenance record/i })).toHaveCount(0);

    await page.screenshot({
      path: "review-artifacts/01-scenario1-wide-desktop-default.png",
      fullPage: true,
    });
  });

  test("02 — scenario 1, wide desktop, Verified-provenance condition selected", async ({ page }) => {
    await page.setViewportSize({ width: 1600, height: 1000 });
    await page.goto("/alerts/1");

    await page.getByRole("button", { name: /tty_allocation/i }).click();
    const record = page.getByRole("group", { name: /Provenance record/i });
    await expect(record).toBeVisible();
    await expect(record.getByText("requestURI", { exact: true })).toBeVisible();

    await page.screenshot({
      path: "review-artifacts/02-scenario1-verified-provenance-selected.png",
      fullPage: true,
    });
  });

  test("03 — scenario 2, wide desktop, multiple Partial-provenance conditions", async ({ page }) => {
    await page.setViewportSize({ width: 1600, height: 1000 });
    await page.goto("/alerts/4");

    await expect(page.getByText("Alert #4", { exact: false }).first()).toBeVisible();
    // All five declared characteristics are satisfied and simultaneously
    // visible on the characteristic bus without any selection.
    for (const id of ["privileged_container", "host_network", "host_pid", "host_ipc", "host_path_volume"]) {
      await expect(page.getByRole("button", { name: new RegExp(id) })).toBeVisible();
    }

    await page.getByRole("button", { name: /privileged_container/i }).click();
    const record = page.getByRole("group", { name: /Provenance record/i });
    await expect(record).toBeVisible();
    await expect(record.getByText(/Partial/)).toBeVisible();

    await page.screenshot({
      path: "review-artifacts/03-scenario2-partial-provenance.png",
      fullPage: true,
    });
  });

  test("04 — scenario 3, wide desktop", async ({ page }) => {
    await page.setViewportSize({ width: 1600, height: 1000 });
    await page.goto("/alerts/5");

    await expect(page.getByText("Alert #5", { exact: false }).first()).toBeVisible();
    await expect(page.getByRole("button", { name: /role_ref_cluster_admin/i })).toBeVisible();

    await page.screenshot({
      path: "review-artifacts/04-scenario3-wide-desktop.png",
      fullPage: true,
    });
  });

  test("05 — scenario 1, below-1024px readable fallback", async ({ page }) => {
    await page.setViewportSize({ width: 768, height: 1024 });
    await page.goto("/alerts/1");

    await expect(page.getByText("Alert #1", { exact: false }).first()).toBeVisible();
    await expect(page.getByRole("heading", { name: "Source submission" })).toBeVisible();
    await expect(page.getByText("Traceability intact").first()).toBeVisible();

    const hasHorizontalOverflow = await page.evaluate(
      () => document.documentElement.scrollWidth > document.documentElement.clientWidth,
    );
    expect(hasHorizontalOverflow).toBe(false);

    await page.screenshot({
      path: "review-artifacts/05-scenario1-below-1024-fallback.png",
      fullPage: true,
    });
  });

  test("06 — broken traceability (raw_event_sha256), wide desktop", async ({ page }) => {
    await page.setViewportSize({ width: 1600, height: 1000 });
    await page.goto("/alerts/3");

    await expect(page.getByText("Alert #3", { exact: false }).first()).toBeVisible();
    await expect(page.getByText("Traceability broken").first()).toBeVisible();
    // Named twice by design: the canvas itself localizes the break (the
    // Source -> Normalized connector plus an anchored callout), not only
    // the Traceability Rail below it — both carry the same real text.
    await expect(page.getByText(/recorded integrity digest/).first()).toBeVisible();
    await expect(page.getByText("raw_event_sha256").first()).toBeVisible();

    await page.screenshot({
      path: "review-artifacts/06-broken-traceability-raw-event-sha256.png",
      fullPage: true,
    });
  });

  test("07 — broken traceability (source_key), wide desktop", async ({ page }) => {
    await page.setViewportSize({ width: 1600, height: 1000 });
    await page.goto("/alerts/6");

    await expect(page.getByText("Alert #6", { exact: false }).first()).toBeVisible();
    await expect(page.getByText("Traceability broken").first()).toBeVisible();
    await expect(page.getByText(/source identity/).first()).toBeVisible();
    await expect(page.getByText("source_key").first()).toBeVisible();

    await page.screenshot({
      path: "review-artifacts/07-broken-traceability-source-key.png",
      fullPage: true,
    });
  });
});

test.describe("Alert Investigation — interaction", () => {
  test("selecting a satisfied characteristic pin reveals its provenance record", async ({ page }) => {
    await page.goto("/alerts/1");
    await page.getByRole("button", { name: /stdin_streaming/i }).click();
    await expect(page.getByRole("group", { name: /Provenance record/i })).toBeVisible();
  });

  test("keyboard selection: focusing a pin and activating with Enter", async ({ page }) => {
    await page.goto("/alerts/1");
    const pin = page.getByRole("button", { name: /tty_allocation/i });
    await pin.focus();
    await page.keyboard.press("Enter");
    await expect(page.getByRole("group", { name: /Provenance record/i })).toBeVisible();
  });

  test("re-selecting the same pin closes its provenance record", async ({ page }) => {
    await page.goto("/alerts/1");
    const pin = page.getByRole("button", { name: /tty_allocation/i });
    await pin.click();
    await expect(page.getByRole("group", { name: /Provenance record/i })).toBeVisible();
    await pin.click();
    await expect(page.getByRole("group", { name: /Provenance record/i })).toHaveCount(0);
  });
});

test.describe("Alert Investigation — route-level states", () => {
  test("not-found, unauthorized, and unavailable states each render distinctly", async ({
    page,
  }) => {
    await page.goto("/alerts/999");
    await expect(page.getByText(/No alert exists with id 999/)).toBeVisible();

    await mockAlertStatus(page, 1, 401);
    await page.goto("/alerts/1");
    await expect(page.getByText("Authentication required")).toBeVisible();

    await mockAlertStatus(page, 1, 500);
    await page.goto("/alerts/1");
    await expect(page.getByText("The investigation backend could not be reached")).toBeVisible();
    await expect(page.getByRole("button", { name: "Retry" })).toBeVisible();
  });

  test("partial artifact availability renders the gap explicitly, other artifacts unaffected", async ({
    page,
  }) => {
    await page.goto("/alerts/2");
    await expect(page.getByText("Alert #2", { exact: false }).first()).toBeVisible();
    await expect(page.getByText("Detection definition unavailable")).toBeVisible();
    await expect(page.getByRole("heading", { name: "Source submission" })).toBeVisible();
  });
});

test.describe("Alert Investigation — structural integrity", () => {
  for (const [label, viewport] of DESKTOP_VIEWPORTS) {
    test(`functions correctly at ${label}: no overflow, key regions visible, selection traces correctly`, async ({
      page,
    }) => {
      await page.setViewportSize(viewport);
      await page.goto("/alerts/1");

      // All six evidence artifacts remain reachable and visible, not just
      // a sample of them, at this exact viewport.
      await expect(page.getByRole("heading", { name: "Source submission" })).toBeVisible();
      await expect(page.getByRole("group", { name: "Validation outcome" })).toBeVisible();
      await expect(page.getByRole("heading", { name: "Normalized event" })).toBeVisible();
      await expect(page.getByRole("heading", { name: "Detection definition" })).toBeVisible();
      await expect(page.getByRole("heading", { name: "Detection result" })).toBeVisible();
      await expect(page.getByRole("heading", { name: "Generated alert" })).toBeVisible();
      await expect(page.getByText("Traceability intact").first()).toBeVisible();

      const hasHorizontalOverflow = await page.evaluate(
        () => document.documentElement.scrollWidth > document.documentElement.clientWidth,
      );
      expect(hasHorizontalOverflow).toBe(false);

      // The selected condition's trace lands on the correct normalized
      // field and its provenance annotation, at this exact viewport.
      await page.getByRole("button", { name: /tty_allocation/i }).click();
      const record = page.getByRole("group", { name: /Provenance record/i });
      await expect(record).toBeVisible();
      await expect(record.getByText("exec.tty", { exact: true })).toBeVisible();
      const highlightedRow = page.locator('[data-highlighted="true"]');
      await expect(highlightedRow).toHaveCount(1);
      await expect(highlightedRow).toContainText("exec.tty");
    });
  }

  test("no page-level horizontal overflow below 1024px (readable fallback, not a dedicated phone experience)", async ({
    page,
  }) => {
    await page.setViewportSize({ width: 768, height: 1024 });
    await page.goto("/alerts/1");
    await expect(page.getByRole("heading", { name: "Source submission" })).toBeVisible();

    const hasHorizontalOverflow = await page.evaluate(
      () => document.documentElement.scrollWidth > document.documentElement.clientWidth,
    );
    expect(hasHorizontalOverflow).toBe(false);
  });

  test("no console errors or warnings during the primary investigation flow", async ({ page }) => {
    const messages: string[] = [];
    page.on("console", (msg) => {
      if (msg.type() === "error" || msg.type() === "warning") {
        messages.push(`${msg.type()}: ${msg.text()}`);
      }
    });

    await page.goto("/alerts/1");
    await page.getByRole("button", { name: /tty_allocation/i }).click();
    await page.getByRole("button", { name: /tty_allocation/i }).click();

    expect(messages).toEqual([]);
  });
});

/*
 * Real-browser geometry coverage for the characteristic bus (Track B Pass
 * 7B follow-up): the DESKTOP_VIEWPORTS loop above only ever visits
 * /alerts/1 (scenario 1 — two ungrouped characteristics, a single row),
 * which cannot exercise the multi-row, grouped bus scenario 2 renders.
 * jsdom-based component tests can't verify this either — jsdom never
 * computes real layout, so a completely disconnected or zero-height bus
 * spine would pass every unit test unnoticed, exactly as it did in this
 * feature's first pass (confirmed by direct measurement against a
 * running build: `.busSpine`'s `grid-row: 1 / -1` resolved to zero
 * height, because `.bus` declares no explicit `grid-template-rows` for
 * `-1` to span against). These tests assert real, browser-computed
 * bounding boxes — never a screenshot's mere existence, never a
 * viewport-specific hardcoded coordinate — against the real Scenario 2
 * fixture (/alerts/4), at every required desktop width plus the
 * below-1024px fallback.
 */
test.describe("Alert Investigation — characteristic bus geometry (scenario 2)", () => {
  const SCENARIO2_DESKTOP_VIEWPORTS = [
    ["1600px", { width: 1600, height: 1000 }],
    ["1440px", { width: 1440, height: 960 }],
    ["1280px", { width: 1280, height: 960 }],
    ["1024px (viewport floor)", { width: 1024, height: 960 }],
  ] as const;

  const SCENARIO2_CHARACTERISTIC_IDS = [
    "host_ipc",
    "host_network",
    "host_path_volume",
    "host_pid",
    "privileged_container",
  ];

  for (const [label, viewport] of SCENARIO2_DESKTOP_VIEWPORTS) {
    test(`renders a genuinely continuous branching bus at ${label}: real spine geometry, both groups, all five characteristics, no overflow`, async ({
      page,
    }) => {
      await page.setViewportSize(viewport);
      await page.goto("/alerts/4");
      await expect(page.getByText("Alert #4", { exact: false }).first()).toBeVisible();

      const bus = page.getByRole("group", { name: "Declared characteristics" });
      await expect(bus).toBeVisible();

      // All five real Scenario 2 characteristics, present exactly once —
      // never dropped, never duplicated.
      for (const id of SCENARIO2_CHARACTERISTIC_IDS) {
        await expect(page.getByRole("button", { name: new RegExp(id) })).toHaveCount(1);
      }

      // Both real declared groups are visible, derived from the row's own
      // `group` field — never a scenario/fixture-id check. Exact text
      // match: "Privilege" is otherwise a substring match inside the
      // privileged_container pin's own accessible name.
      await expect(bus.getByText("Host access", { exact: true })).toBeVisible();
      await expect(bus.getByText("Privilege", { exact: true })).toBeVisible();

      // The spine's real, browser-computed geometry: visible, materially
      // tall (relative to the real measured pin span — never a hardcoded
      // viewport-specific pixel value), and covering every row from the
      // first pin through the last.
      const geometry = await page.evaluate(() => {
        const busEl = document.querySelector('[aria-label="Declared characteristics"]') as HTMLElement;
        const spine = busEl.firstElementChild as HTMLElement;
        const pins = Array.from(busEl.querySelectorAll("button"));
        const labels = Array.from(busEl.querySelectorAll("p"));
        const rectOf = (el: Element) => {
          const r = el.getBoundingClientRect();
          return { top: r.top, bottom: r.bottom, left: r.left, right: r.right, height: r.height };
        };
        // The stem itself: a `::after` pseudo-element on the pin's slot
        // wrapper, not a real DOM node — `getBoundingClientRect` can't see
        // it at all, so proving it is actually drawn (not merely that the
        // pin sits near the spine, which stays true even if the stem were
        // hidden entirely) requires reading its own computed style
        // directly via `getComputedStyle(element, "::after")`.
        const stemInfoFor = (characteristicId: string) => {
          const pin = pins.find((p) => p.textContent?.includes(characteristicId));
          const slot = pin?.parentElement;
          if (!slot) return null;
          const cs = getComputedStyle(slot, "::after");
          return {
            slotSide: slot.className.includes("pinSlotLeft") ? "left" : "right",
            display: cs.display,
            width: cs.width,
            backgroundColor: cs.backgroundColor,
          };
        };
        return {
          spineDisplay: getComputedStyle(spine).display,
          spineRect: rectOf(spine),
          pinRects: pins.map(rectOf),
          labelRects: labels.map(rectOf),
          // host_ipc (declared first, index 0) is always the left side;
          // host_network (declared second, index 1) is always the right
          // side — a structural fact of `buildBusLayout`'s alternation,
          // not a coincidence of current styling.
          leftStem: stemInfoFor("host_ipc"),
          rightStem: stemInfoFor("host_network"),
        };
      });

      expect(geometry.spineDisplay).not.toBe("none");

      const firstPinTop = Math.min(...geometry.pinRects.map((r) => r.top));
      const lastPinBottom = Math.max(...geometry.pinRects.map((r) => r.bottom));
      const realPinSpan = lastPinBottom - firstPinTop;
      const TOLERANCE = 6; // px — documented rounding tolerance, not a fixture-specific measurement

      // Materially greater than zero: within a fraction of the real
      // measured content span, not a token sliver (the exact zero-height
      // regression this test exists to catch).
      expect(geometry.spineRect.height).toBeGreaterThan(realPinSpan * 0.8);
      // Spans from the first pin row through the final pin row.
      expect(geometry.spineRect.top).toBeLessThanOrEqual(firstPinTop + TOLERANCE);
      expect(geometry.spineRect.bottom).toBeGreaterThanOrEqual(lastPinBottom - TOLERANCE);

      // Every pin sits within a small, bounded gap of the spine — close
      // enough that a real connecting stem bridges it, never detached.
      const STEM_MAX_GAP = 32; // px — a generous upper bound on the fixed stem-gutter token; a much larger gap means a detached stem
      for (const r of geometry.pinRects) {
        const gap = r.right <= geometry.spineRect.left ? geometry.spineRect.left - r.right : r.left - geometry.spineRect.right;
        expect(gap).toBeGreaterThanOrEqual(0);
        expect(gap).toBeLessThanOrEqual(STEM_MAX_GAP);
      }

      // The stem itself — not just pin-to-spine proximity — is genuinely
      // rendered on both sides. A pin sitting close to the spine (checked
      // above) stays true even if its connecting stem were hidden or
      // removed entirely, since that check only measures the pin's own
      // position; this is the one assertion that would fail if a real
      // stem stopped being drawn, on either side.
      for (const stem of [geometry.leftStem, geometry.rightStem]) {
        expect(stem).not.toBeNull();
        expect(stem!.display).not.toBe("none");
        expect(parseFloat(stem!.width)).toBeGreaterThan(0);
        expect(stem!.backgroundColor).not.toBe("rgba(0, 0, 0, 0)");
        expect(stem!.backgroundColor).not.toBe("transparent");
      }
      expect(geometry.leftStem?.slotSide).toBe("left");
      expect(geometry.rightStem?.slotSide).toBe("right");

      // No group label vertically overlaps a characteristic pin.
      for (const label of geometry.labelRects) {
        for (const pin of geometry.pinRects) {
          const overlaps = !(
            label.right <= pin.left ||
            label.left >= pin.right ||
            label.bottom <= pin.top ||
            label.top >= pin.bottom
          );
          expect(overlaps).toBe(false);
        }
      }

      const hasHorizontalOverflow = await page.evaluate(
        () => document.documentElement.scrollWidth > document.documentElement.clientWidth,
      );
      expect(hasHorizontalOverflow).toBe(false);
    });
  }

  test("below 1024px: the desktop spine is hidden, the readable single-column fallback remains, all five characteristics and both groups stay reachable, no overflow", async ({
    page,
  }) => {
    await page.setViewportSize({ width: 768, height: 1000 });
    await page.goto("/alerts/4");
    await expect(page.getByText("Alert #4", { exact: false }).first()).toBeVisible();

    const bus = page.getByRole("group", { name: "Declared characteristics" });
    await expect(bus).toBeVisible();

    const layout = await page.evaluate(() => {
      const busEl = document.querySelector('[aria-label="Declared characteristics"]') as HTMLElement;
      const spine = busEl.firstElementChild as HTMLElement;
      return {
        spineDisplay: getComputedStyle(spine).display,
        busDisplay: getComputedStyle(busEl).display,
      };
    });
    expect(layout.spineDisplay).toBe("none");
    expect(layout.busDisplay).toBe("flex");

    for (const id of SCENARIO2_CHARACTERISTIC_IDS) {
      await expect(page.getByRole("button", { name: new RegExp(id) })).toBeVisible();
    }
    await expect(bus.getByText("Host access", { exact: true })).toBeVisible();
    await expect(bus.getByText("Privilege", { exact: true })).toBeVisible();

    const hasHorizontalOverflow = await page.evaluate(
      () => document.documentElement.scrollWidth > document.documentElement.clientWidth,
    );
    expect(hasHorizontalOverflow).toBe(false);
  });
});

/*
 * Real-browser geometry coverage for `BusConvergenceOverlay` — the
 * measured SVG that draws one ray from every declared characteristic pin
 * to Detection Result. jsdom-based component tests (`BusConvergenceOverlay.test.tsx`)
 * can only verify structural presence and lifecycle (line count, aria
 * attributes, ResizeObserver/listener cleanup) — jsdom never computes
 * real layout, so a ray anchored to the wrong element, or one that stops
 * updating after a resize, would pass every unit test unnoticed. These
 * tests assert real, browser-computed bounding boxes against the real
 * Scenario 2 fixture (/alerts/4, five declared characteristics, two
 * groups) — never a screenshot's mere existence, never a hardcoded
 * viewport-specific coordinate.
 *
 * A ray's `x1/y1/x2/y2` are SVG attributes in the shared canvas
 * container's own local coordinate space (the overlay's `viewBox` and
 * pixel size match that container exactly), not viewport-absolute — every
 * check below converts them to absolute page coordinates via the
 * container's own `getBoundingClientRect()` before comparing against
 * pin/result rects, which `getBoundingClientRect()` already reports in
 * absolute page coordinates.
 */
test.describe("Alert Investigation — bus convergence rays (scenario 2)", () => {
  const CHARACTERISTIC_IDS = [
    "host_ipc",
    "host_network",
    "host_path_volume",
    "host_pid",
    "privileged_container",
  ];
  const CONVERGENCE_VIEWPORTS = [
    ["1600px", { width: 1600, height: 1000 }],
    ["1440px", { width: 1440, height: 960 }],
    ["1280px", { width: 1280, height: 960 }],
    ["1024px (viewport floor)", { width: 1024, height: 960 }],
  ] as const;

  /** Reads every convergence ray's absolute-page-coordinate endpoints,
   *  plus the real bounding boxes of every pin, Detection Result, and the
   *  two other text-bearing artifacts a ray must never cross. */
  async function readConvergenceGeometry(page: import("@playwright/test").Page) {
    return page.evaluate((ids: string[]) => {
      const lines = Array.from(document.querySelectorAll("line[data-pin-id]")) as SVGLineElement[];
      const container = lines[0]?.closest("svg")?.parentElement;
      if (!container) return null;
      const containerRect = container.getBoundingClientRect();

      const toAbsolute = (line: SVGLineElement) => ({
        x1: containerRect.left + line.x1.baseVal.value,
        y1: containerRect.top + line.y1.baseVal.value,
        x2: containerRect.left + line.x2.baseVal.value,
        y2: containerRect.top + line.y2.baseVal.value,
      });

      const rects = ids.map((id) => {
        const pin = Array.from(document.querySelectorAll("button")).find((b) =>
          b.textContent?.includes(id),
        );
        return pin?.getBoundingClientRect() ?? null;
      });

      const sectionRectFor = (headingText: string) => {
        const heading = Array.from(document.querySelectorAll("h2")).find(
          (h) => h.textContent?.trim() === headingText,
        );
        const section = heading?.closest("section");
        return section?.getBoundingClientRect() ?? null;
      };

      const rectToPlain = (r: DOMRect | null) =>
        r && { top: r.top, bottom: r.bottom, left: r.left, right: r.right };

      return {
        rayCount: lines.length,
        rays: lines.map((l) => ({ id: l.getAttribute("data-pin-id"), ...toAbsolute(l) })),
        pinRects: rects.map(rectToPlain),
        resultRect: rectToPlain(sectionRectFor("Detection result")),
        normalizedRect: rectToPlain(sectionRectFor("Normalized event")),
        sourceRect: rectToPlain(sectionRectFor("Source submission")),
      };
    }, CHARACTERISTIC_IDS);
  }

  const TOLERANCE = 8; // px — rounding/border tolerance, not a fixture-specific measurement

  function assertGeometryIsSound(
    geometry: NonNullable<Awaited<ReturnType<typeof readConvergenceGeometry>>>,
  ) {
    // Exactly one ray per declared characteristic — never dropped, never
    // duplicated, regardless of how many other overlays share the canvas.
    expect(geometry.rayCount).toBe(CHARACTERISTIC_IDS.length);

    for (let i = 0; i < CHARACTERISTIC_IDS.length; i++) {
      const id = CHARACTERISTIC_IDS[i];
      const pinRect = geometry.pinRects[i];
      const ray = geometry.rays.find((r) => r.id === id);
      expect(pinRect, `pin rect for ${id}`).not.toBeNull();
      expect(ray, `ray for ${id}`).toBeDefined();

      // The ray's own endpoints are real, finite, non-degenerate numbers —
      // never stuck at an unmeasured (0,0) origin.
      expect(Number.isFinite(ray!.x1)).toBe(true);
      expect(Number.isFinite(ray!.y1)).toBe(true);
      expect(ray!.x1 + ray!.y1).toBeGreaterThan(0);

      // Begins adjacent to its own pin: the (x1,y1) endpoint's vertical
      // position falls within the pin's own vertical span, and its
      // horizontal position sits at (or within a small tolerance of) the
      // pin's own left or right edge — never floating far from the pin
      // that supposedly emits it.
      expect(ray!.y1).toBeGreaterThanOrEqual(pinRect!.top - TOLERANCE);
      expect(ray!.y1).toBeLessThanOrEqual(pinRect!.bottom + TOLERANCE);
      const distanceFromPinEdge = Math.min(
        Math.abs(ray!.x1 - pinRect!.left),
        Math.abs(ray!.x1 - pinRect!.right),
      );
      expect(distanceFromPinEdge).toBeLessThanOrEqual(TOLERANCE);

      // Terminates inside or adjacent to Detection Result's own box —
      // never in empty canvas space, never inside some other artifact.
      expect(ray!.x2).toBeGreaterThanOrEqual(geometry.resultRect!.left - TOLERANCE);
      expect(ray!.x2).toBeLessThanOrEqual(geometry.resultRect!.right + TOLERANCE);
      expect(ray!.y2).toBeGreaterThanOrEqual(geometry.resultRect!.top - TOLERANCE);
      expect(ray!.y2).toBeLessThanOrEqual(geometry.resultRect!.bottom + TOLERANCE);

      // Never crosses another artifact's readable text: a ray only ever
      // runs from a Definition pin to Result (both strictly to the right
      // of Normalized/Source), so neither endpoint may fall inside
      // Normalized Event's or Source Submission's own box.
      for (const textRect of [geometry.normalizedRect, geometry.sourceRect]) {
        for (const point of [
          { x: ray!.x1, y: ray!.y1 },
          { x: ray!.x2, y: ray!.y2 },
        ]) {
          const insideTextRegion =
            point.x >= textRect!.left &&
            point.x <= textRect!.right &&
            point.y >= textRect!.top &&
            point.y <= textRect!.bottom;
          expect(insideTextRegion).toBe(false);
        }
      }
    }
  }

  for (const [label, viewport] of CONVERGENCE_VIEWPORTS) {
    test(`renders exactly five rays, each attached to its pin and terminating at Detection Result, at ${label}`, async ({
      page,
    }) => {
      await page.setViewportSize(viewport);
      await page.goto("/alerts/4");
      await expect(page.getByText("Alert #4", { exact: false }).first()).toBeVisible();
      // No pin selected — isolates the always-on convergence overlay from
      // the separate, selection-driven trace overlay, which would add its
      // own `<line>` elements to the same canvas.
      await expect(page.getByRole("group", { name: /Provenance record/i })).toHaveCount(0);

      const geometry = await readConvergenceGeometry(page);
      expect(geometry).not.toBeNull();
      assertGeometryIsSound(geometry!);
    });
  }

  test("live resize (1600 -> 1440 -> 1280 -> 1024) keeps ray geometry attached, with no duplicated lines or overlays", async ({
    page,
  }) => {
    await page.setViewportSize({ width: 1600, height: 1000 });
    await page.goto("/alerts/4");
    await expect(page.getByText("Alert #4", { exact: false }).first()).toBeVisible();

    let previousGeometry = await readConvergenceGeometry(page);
    expect(previousGeometry).not.toBeNull();
    assertGeometryIsSound(previousGeometry!);

    for (const width of [1440, 1280, 1024]) {
      await page.setViewportSize({ width, height: 960 });
      // Real ResizeObserver callbacks fire on a later animation frame, not
      // synchronously with the viewport change itself.
      await page.waitForTimeout(150);

      // Never more than one canvas overlay SVG (the convergence overlay;
      // no selection trace overlay exists here since nothing is selected)
      // and never more than one `<line>` per declared characteristic —
      // the regression a leaked/duplicated ResizeObserver would produce.
      const svgCount = await page.evaluate(() => document.querySelectorAll("svg").length);
      expect(svgCount).toBe(1);

      const geometry = await readConvergenceGeometry(page);
      expect(geometry).not.toBeNull();
      assertGeometryIsSound(geometry!);

      // The geometry actually changed to match the new, narrower layout —
      // not merely "still present," but genuinely re-measured. At least
      // one ray's start or end point must have moved by more than a
      // rounding-level amount.
      const moved = geometry!.rays.some((ray) => {
        const prior = previousGeometry!.rays.find((r) => r.id === ray.id)!;
        return (
          Math.abs(ray.x1 - prior.x1) > TOLERANCE ||
          Math.abs(ray.y1 - prior.y1) > TOLERANCE ||
          Math.abs(ray.x2 - prior.x2) > TOLERANCE ||
          Math.abs(ray.y2 - prior.y2) > TOLERANCE
        );
      });
      expect(moved).toBe(true);

      previousGeometry = geometry;
    }

    const hasHorizontalOverflow = await page.evaluate(
      () => document.documentElement.scrollWidth > document.documentElement.clientWidth,
    );
    expect(hasHorizontalOverflow).toBe(false);
  });
});

/*
 * Result -> Alert connector geometry (regression coverage for a real,
 * empirically-confirmed cascade bug: `.wireHorizontal`'s `align-self: start`
 * won a same-specificity cascade tie over `.wireResultAlert`'s own
 * `align-self: center`, because the compound override at
 * `.wireDefinitionResult.wireHorizontal, .wireResultAlert.wireHorizontal`
 * only ever set `margin-top`. The connector rendered at the very top of the
 * canvas row — measured ~370px away from the actual Detection Result /
 * Generated Alert boxes on scenario 2 at 1600px before the fix — while the
 * two boxes it is supposed to join centered correctly against each other.
 * These tests assert the connector's real, browser-computed position
 * relative to the real boxes, never a screenshot's mere existence and
 * never a hardcoded absolute page coordinate.
 */
test.describe("Alert Investigation — Result -> Alert connector geometry", () => {
  const WIRE_VIEWPORTS = [
    ["1600px", { width: 1600, height: 1000 }],
    ["1440px", { width: 1440, height: 960 }],
    ["1280px", { width: 1280, height: 960 }],
    ["1024px (viewport floor)", { width: 1024, height: 960 }],
  ] as const;

  async function readWireGeometry(page: import("@playwright/test").Page) {
    return page.evaluate(() => {
      const wire = document.querySelector('[data-wire="result-alert"]');
      const resultSection = Array.from(document.querySelectorAll("section")).find((s) =>
        s.querySelector("#result-heading"),
      );
      const alertSection = Array.from(document.querySelectorAll("section")).find((s) =>
        s.querySelector("#alert-heading"),
      );
      if (!wire || !resultSection || !alertSection) return null;
      const w = wire.getBoundingClientRect();
      const r = resultSection.getBoundingClientRect();
      const a = alertSection.getBoundingClientRect();
      return {
        wire: { top: w.top, bottom: w.bottom, left: w.left, right: w.right },
        result: { top: r.top, bottom: r.bottom, left: r.left, right: r.right, height: r.height },
        alert: { top: a.top, bottom: a.bottom, left: a.left, right: a.right, height: a.height },
      };
    });
  }

  for (const [label, viewport] of WIRE_VIEWPORTS) {
    test(`Result -> Alert connector stays centered on the Result/Alert row at ${label}, even though the two boxes render at different heights`, async ({
      page,
    }) => {
      await page.setViewportSize(viewport);
      // Scenario 2 (five declared characteristics) stretches Detection
      // Definition the tallest of any fixture, so Result/Alert center
      // against the widest row-height range this connector has to track.
      await page.goto("/alerts/4");
      await expect(page.getByText("Alert #4", { exact: false }).first()).toBeVisible();

      const geometry = await readWireGeometry(page);
      expect(geometry).not.toBeNull();
      const { wire, result, alert } = geometry!;

      // A genuine precondition, not an assumption: Result and Alert really
      // do render at different heights (each box's own internal content —
      // the tally/wedge vs. the alert summary line — differs), so a
      // connector that merely happened to line up when both boxes were
      // exactly the same height would not be exercised by this assertion.
      expect(Math.abs(result.height - alert.height)).toBeGreaterThan(1);

      // Terminates between the two real boxes: the connector's own
      // horizontal span sits in the gutter separating them, never inside
      // either box and never off in empty space.
      expect(wire.left).toBeGreaterThanOrEqual(result.right - 2);
      expect(wire.right).toBeLessThanOrEqual(alert.left + 2);

      // Vertically centered on the row the two boxes actually share —
      // within a small tolerance *relative to the row's own height*
      // (never an absolute page coordinate, since that shifts with
      // viewport width and content height). Before the fix this measured
      // ~370px off; this tolerance would not have let that pass.
      const resultCenter = (result.top + result.bottom) / 2;
      const alertCenter = (alert.top + alert.bottom) / 2;
      const rowCenter = (resultCenter + alertCenter) / 2;
      const wireCenter = (wire.top + wire.bottom) / 2;
      const rowSpan = Math.max(result.height, alert.height);
      expect(Math.abs(wireCenter - rowCenter)).toBeLessThanOrEqual(rowSpan * 0.05);
    });
  }
});

/*
 * Broken-callout grid row collapse (regression coverage for a real,
 * empirically-confirmed layout bug: an always-declared `brokenCallout` grid
 * row consumed its own two flanking `gap`s even at 0px content height,
 * silently doubling the space between the main artifact row and the
 * Traceability Rail — measured 48px instead of 24px — on every
 * intact-traceability alert).
 */
test.describe("Alert Investigation — broken-callout grid row", () => {
  test("an intact alert reserves no empty broken-callout row: its Source -> Rail gap equals a broken alert's own Callout -> Rail gap (one real row-gap, not two)", async ({
    page,
  }) => {
    await page.setViewportSize({ width: 1600, height: 1000 });

    await page.goto("/alerts/1");
    await expect(page.getByText("Traceability intact").first()).toBeVisible();
    expect(await page.locator('[class*="canvasBrokenCallout"]').count()).toBe(0);
    const intactGap = await page.evaluate(() => {
      const rail = document.querySelector('[class*="canvasRail"]')!;
      const source = document.querySelector('[data-testid="source-validation-group"]')!;
      return rail.getBoundingClientRect().top - source.getBoundingClientRect().bottom;
    });

    await page.goto("/alerts/3");
    await expect(page.getByText("Traceability broken").first()).toBeVisible();
    const brokenGap = await page.evaluate(() => {
      const rail = document.querySelector('[class*="canvasRail"]')!;
      const callout = document.querySelector('[class*="canvasBrokenCallout"]')!;
      return rail.getBoundingClientRect().top - callout.getBoundingClientRect().bottom;
    });

    // Both gaps are the same single real grid `row-gap` — if the intact
    // case were still silently reserving the empty row (the regression
    // this test exists to catch), its own gap would measure roughly
    // double the broken case's.
    expect(Math.abs(intactGap - brokenGap)).toBeLessThanOrEqual(4);
  });

  test("both broken-traceability variants still render their callout with no overlap or clipping against the rail above it", async ({
    page,
  }) => {
    for (const url of ["/alerts/3", "/alerts/6"]) {
      await page.goto(url);
      await expect(page.getByText("Traceability broken").first()).toBeVisible();

      const geometry = await page.evaluate(() => {
        const callout = document.querySelector('[class*="canvasBrokenCallout"]')!;
        const rail = document.querySelector('[class*="canvasRail"]')!;
        const c = callout.getBoundingClientRect();
        const r = rail.getBoundingClientRect();
        return { calloutHeight: c.height, calloutBottom: c.bottom, railTop: r.top };
      });

      expect(geometry.calloutHeight).toBeGreaterThan(0);
      // No overlap: the callout's own bottom edge never reaches into the
      // rail's own top edge.
      expect(geometry.calloutBottom).toBeLessThanOrEqual(geometry.railTop);
    }
  });

  test("the 768px single-column fallback is unaffected: both intact and broken alerts still stack correctly with no horizontal overflow", async ({
    page,
  }) => {
    await page.setViewportSize({ width: 768, height: 1024 });

    await page.goto("/alerts/1");
    await expect(page.getByText("Alert #1", { exact: false }).first()).toBeVisible();
    await expect(page.getByRole("heading", { name: "Source submission" })).toBeVisible();
    expect(
      await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth),
    ).toBe(false);

    await page.goto("/alerts/3");
    await expect(page.getByText("Traceability broken").first()).toBeVisible();
    await expect(page.locator('[class*="canvasBrokenCallout"]')).toBeVisible();
    expect(
      await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth),
    ).toBe(false);
  });
});

/*
 * Regression coverage for the Track B visual pre-commit corrective pass.
 * jsdom cannot truthfully verify any of these — scroll-container geometry,
 * real text-wrapping/line-height, and computed `display` all require a
 * real layout engine — so each check here reads real, browser-computed
 * values, never a screenshot's mere existence.
 */
test.describe("Alert Investigation — Verified raw-substring visibility", () => {
  test("selecting tty_allocation scrolls its raw substring into the raw-payload box's own view, without moving the page", async ({
    page,
  }) => {
    await page.setViewportSize({ width: 1024, height: 900 });
    await page.goto("/alerts/1");
    await expect(page.getByText("Alert #1", { exact: false }).first()).toBeVisible();

    await page.getByRole("button", { name: /tty_allocation/i }).click();
    await expect(page.locator("[data-raw-highlight='true']")).toBeVisible();

    const geometry = await page.evaluate(() => {
      const mark = document.querySelector("[data-raw-highlight='true']")!;
      const box = document.querySelector('[class*="specimenRawBox"]')!;
      const m = mark.getBoundingClientRect();
      const b = box.getBoundingClientRect();
      return {
        markTop: m.top,
        markBottom: m.bottom,
        boxTop: b.top,
        boxBottom: b.bottom,
        pageScrollY: window.scrollY,
      };
    });

    // The mark's real, measured position sits within the raw box's own
    // visible bounds — never above or below it — immediately after
    // selection, with no manual scroll performed by this test.
    expect(geometry.markTop).toBeGreaterThanOrEqual(geometry.boxTop - 1);
    expect(geometry.markBottom).toBeLessThanOrEqual(geometry.boxBottom + 1);
    // Container-local only: the page itself never scrolled.
    expect(geometry.pageScrollY).toBe(0);
  });

  test("switching between two different Verified pins keeps the newly-selected substring visible, not the previous one's position", async ({
    page,
  }) => {
    await page.setViewportSize({ width: 1024, height: 900 });
    await page.goto("/alerts/1");
    await expect(page.getByText("Alert #1", { exact: false }).first()).toBeVisible();

    await page.getByRole("button", { name: /tty_allocation/i }).click();
    await expect(page.locator("[data-raw-highlight='true']")).toHaveText("tty=true");

    await page.getByRole("button", { name: /tty_allocation/i }).click(); // toggle off
    await page.getByRole("button", { name: /stdin_streaming/i }).click();
    await expect(page.locator("[data-raw-highlight='true']")).toHaveText("stdin=true");

    const geometry = await page.evaluate(() => {
      const mark = document.querySelector("[data-raw-highlight='true']")!;
      const box = document.querySelector('[class*="specimenRawBox"]')!;
      const m = mark.getBoundingClientRect();
      const b = box.getBoundingClientRect();
      return { markTop: m.top, markBottom: m.bottom, boxTop: b.top, boxBottom: b.bottom };
    });
    expect(geometry.markTop).toBeGreaterThanOrEqual(geometry.boxTop - 1);
    expect(geometry.markBottom).toBeLessThanOrEqual(geometry.boxBottom + 1);
  });
});

test.describe("Alert Investigation — Scenario 2 raw payload readability", () => {
  const READABILITY_WIDTHS = [1024, 1280, 1440, 1600];

  for (const width of READABILITY_WIDTHS) {
    test(`requestObject's "Pod" value renders on a single line, not one character per line, at ${width}px`, async ({
      page,
    }) => {
      await page.setViewportSize({ width, height: 900 });
      await page.goto("/alerts/4");
      await expect(page.getByText("Alert #4", { exact: false }).first()).toBeVisible();

      const info = await page.evaluate(() => {
        const box = document.querySelector('[class*="specimenRawBox"]')!;
        const podValue = Array.from(box.querySelectorAll("span")).find((s) =>
          s.textContent?.includes('"Pod"'),
        );
        if (!podValue) return null;
        const rect = podValue.getBoundingClientRect();
        return { boxWidth: box.getBoundingClientRect().width, valueHeight: rect.height };
      });

      expect(info).not.toBeNull();
      // A real minimum box width -- the old 130px-floor regression measured
      // 92px here and collapsed "Pod" into three separate lines.
      expect(info!.boxWidth).toBeGreaterThan(150);
      // Single-line proof: a value that wraps one character per line grows
      // roughly as tall as it is many characters; a real single mono-space
      // line at this font size is well under 24px regardless of viewport.
      expect(info!.valueHeight).toBeLessThan(24);
    });
  }
});

test.describe("Alert Investigation — overlays suppressed in the 768px fallback", () => {
  test("the bus-convergence overlay is not visually rendered at 768px, for either intact or broken-traceability alerts", async ({
    page,
  }) => {
    await page.setViewportSize({ width: 768, height: 1024 });
    for (const url of ["/alerts/1", "/alerts/3", "/alerts/6"]) {
      await page.goto(url);
      await expect(page.getByText(/Alert #/, { exact: false }).first()).toBeVisible();

      const info = await page.evaluate(() => {
        const overlay = document.querySelector('svg[class*="convergenceOverlay"]');
        if (!overlay) return { present: false, display: "absent", visualArea: 0 };
        const rect = overlay.getBoundingClientRect();
        return {
          present: true,
          display: getComputedStyle(overlay).display,
          // `display: none` collapses an element's own box to zero, even
          // though its <line> children still exist in the DOM (this
          // component stays mounted across breakpoints by design) — a
          // real, painted zero-area box is the authoritative proof no ray
          // is visually shown, not merely that some CSS class was applied.
          visualArea: rect.width * rect.height,
        };
      });
      expect(info.display).toBe("none");
      expect(info.visualArea).toBe(0);
    }
  });

  test("selecting a pin at 768px never visually renders a selection-trace line", async ({ page }) => {
    await page.setViewportSize({ width: 768, height: 1024 });
    await page.goto("/alerts/1");
    await expect(page.getByText("Alert #1", { exact: false }).first()).toBeVisible();

    await page.getByRole("button", { name: /tty_allocation/i }).click();
    await expect(page.getByRole("group", { name: /Provenance record/i })).toBeVisible();

    const info = await page.evaluate(() => {
      const overlay = document.querySelector('svg[class*="traceOverlay"]');
      if (!overlay) return { present: false, display: "absent", visualArea: 0 };
      const rect = overlay.getBoundingClientRect();
      return { present: true, display: getComputedStyle(overlay).display, visualArea: rect.width * rect.height };
    });
    expect(info.display).toBe("none");
    expect(info.visualArea).toBe(0);
  });
});

test.describe("Alert Investigation — desktop overlays remain after the 768px suppression rule", () => {
  test("bus convergence rays and the selection trace still render above the 1024px fallback", async ({
    page,
  }) => {
    await page.setViewportSize({ width: 1024, height: 900 });
    await page.goto("/alerts/4");
    await expect(page.getByText("Alert #4", { exact: false }).first()).toBeVisible();

    const rayCount = await page.evaluate(() => document.querySelectorAll("line[data-pin-id]").length);
    expect(rayCount).toBe(5);

    await page.getByRole("button", { name: /privileged_container/i }).click();
    const traceCount = await page.evaluate(() => document.querySelectorAll("path[data-kind]").length);
    expect(traceCount).toBe(1);
  });
});
