import { test, expect } from "@playwright/test";

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
 */

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
    await expect(page.getByText(/recorded integrity digest/)).toBeVisible();

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
    await expect(page.getByText(/source identity/)).toBeVisible();

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

    await page.goto("/alerts/1?demo=unauthorized");
    await expect(page.getByText("Authentication required")).toBeVisible();

    await page.goto("/alerts/1?demo=unavailable");
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
