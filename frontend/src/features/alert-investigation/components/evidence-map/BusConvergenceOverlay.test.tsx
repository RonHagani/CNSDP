import { useRef } from "react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import { render } from "@testing-library/react";
import { BusConvergenceOverlay } from "./BusConvergenceOverlay";

/**
 * A tracking stand-in for the jsdom-stubbed `ResizeObserver` (see
 * `src/test/setup.ts` — the global stub is a pure no-op, so it can't tell
 * these tests whether an instance was ever constructed, observed, or torn
 * down). Local to this file only; nothing else needs instance-level
 * visibility into ResizeObserver lifecycle.
 */
class TrackingResizeObserver implements ResizeObserver {
  static instances: TrackingResizeObserver[] = [];
  observedTargets: Element[] = [];
  disconnected = false;
  constructor() {
    TrackingResizeObserver.instances.push(this);
  }
  observe(target: Element) {
    this.observedTargets.push(target);
  }
  unobserve(target: Element) {
    this.observedTargets = this.observedTargets.filter((t) => t !== target);
  }
  disconnect() {
    this.disconnected = true;
  }
}

const PIN_COUNT = 3;

/**
 * Mounts the real DOM shape `BusConvergenceOverlay` measures against — a
 * container, `pinCount` pin buttons registered into a stable `Map` the
 * same way `DetectionDefinitionFrame`'s real ref callbacks do, and a
 * target node — so these tests exercise the actual measurement effect
 * against real elements, never a stub of the component under test.
 */
function Harness({ pinIds }: { pinIds: string[] }) {
  const containerRef = useRef<HTMLDivElement>(null);
  const targetRef = useRef<HTMLDivElement>(null);
  const pinElsRef = useRef(new Map<string, HTMLElement>());

  return (
    <div ref={containerRef}>
      {Array.from({ length: PIN_COUNT }).map((_, i) => (
        <button
          key={i}
          ref={(el) => {
            if (el) pinElsRef.current.set(`pin-${i}`, el);
            else pinElsRef.current.delete(`pin-${i}`);
          }}
        >
          {`pin-${i}`}
        </button>
      ))}
      <div ref={targetRef}>result</div>
      <BusConvergenceOverlay
        containerRef={containerRef}
        pinEls={pinElsRef.current}
        pinIds={pinIds}
        targetRef={targetRef}
      />
    </div>
  );
}

describe("BusConvergenceOverlay", () => {
  beforeEach(() => {
    TrackingResizeObserver.instances = [];
    vi.stubGlobal("ResizeObserver", TrackingResizeObserver);
  });

  it("renders no SVG lines when pinIds is empty", () => {
    const { container } = render(<Harness pinIds={[]} />);
    expect(container.querySelector("svg")).toBeNull();
    expect(container.querySelectorAll("line")).toHaveLength(0);
  });

  it("renders exactly one convergence line per populated pin id", () => {
    const { container } = render(<Harness pinIds={["pin-0", "pin-1", "pin-2"]} />);
    expect(container.querySelectorAll("line")).toHaveLength(3);
  });

  it("ignores missing or unresolved refs without crashing", () => {
    // "pin-99" is never registered into the harness's own pin map (only
    // pin-0/1/2 exist) — a real, observed shape: a stale id briefly
    // present in `pinIds` before its own row unmounts.
    expect(() =>
      render(<Harness pinIds={["pin-0", "pin-99-unresolved", "pin-1"]} />),
    ).not.toThrow();
    const { container } = render(<Harness pinIds={["pin-0", "pin-99-unresolved", "pin-1"]} />);
    expect(container.querySelectorAll("line")).toHaveLength(2);
  });

  it("renders the overlay as purely decorative: aria-hidden and outside the accessible/tab order", () => {
    const { container } = render(<Harness pinIds={["pin-0", "pin-1"]} />);
    const svg = container.querySelector("svg");
    expect(svg).not.toBeNull();
    expect(svg).toHaveAttribute("aria-hidden", "true");
    expect(svg).not.toHaveAttribute("role");
    expect(svg).not.toHaveAttribute("tabindex");
  });

  it("disconnects its ResizeObserver on unmount", () => {
    const { unmount } = render(<Harness pinIds={["pin-0", "pin-1"]} />);
    expect(TrackingResizeObserver.instances).toHaveLength(1);
    const instance = TrackingResizeObserver.instances[0];
    expect(instance.disconnected).toBe(false);
    unmount();
    expect(instance.disconnected).toBe(true);
  });

  it("removes its window resize listener on unmount", () => {
    const addSpy = vi.spyOn(window, "addEventListener");
    const removeSpy = vi.spyOn(window, "removeEventListener");

    const { unmount } = render(<Harness pinIds={["pin-0", "pin-1"]} />);
    const resizeCall = addSpy.mock.calls.find(([type]) => type === "resize");
    expect(resizeCall).toBeDefined();
    const handler = resizeCall![1];

    unmount();
    expect(removeSpy).toHaveBeenCalledWith("resize", handler);

    addSpy.mockRestore();
    removeSpy.mockRestore();
  });

  it("re-measures when the actual set of pin ids changes", () => {
    const { container, rerender } = render(<Harness pinIds={["pin-0", "pin-1"]} />);
    const containerEl = container.firstElementChild as HTMLElement;
    const measureSpy = vi.spyOn(containerEl, "getBoundingClientRect");
    measureSpy.mockClear();

    rerender(<Harness pinIds={["pin-0", "pin-1", "pin-2"]} />);

    expect(measureSpy).toHaveBeenCalled();
    expect(container.querySelectorAll("line")).toHaveLength(3);
    measureSpy.mockRestore();
  });

  it("does not create a duplicate ResizeObserver or duplicate lines when rerendered with an equivalent (same contents, new array instance) pin id list", () => {
    const { container, rerender } = render(<Harness pinIds={["pin-0", "pin-1"]} />);
    expect(TrackingResizeObserver.instances).toHaveLength(1);
    expect(container.querySelectorAll("line")).toHaveLength(2);

    // A genuinely new array reference with the exact same string contents —
    // precisely what `characteristicRows.map((row) => row.id)` produces on
    // every render of the real orchestrator, regardless of whether the
    // underlying data actually changed.
    rerender(<Harness pinIds={["pin-0", "pin-1"]} />);

    expect(TrackingResizeObserver.instances).toHaveLength(1);
    expect(container.querySelectorAll("line")).toHaveLength(2);
  });
});

describe("BusConvergenceOverlay — pinIdsKey content sensitivity", () => {
  beforeEach(() => {
    TrackingResizeObserver.instances = [];
    vi.stubGlobal("ResizeObserver", TrackingResizeObserver);
  });

  /**
   * A harness whose pins carry real, distinct, per-id mocked geometry (via
   * a `getBoundingClientRect` override keyed by DOM element) — every prior
   * test in this file leaves geometry at jsdom's default all-zero rect,
   * which cannot distinguish "measured pin A's real position" from
   * "measured pin B's real position," and so cannot prove a re-measurement
   * used *correct*, non-stale geometry rather than merely having run at
   * all. `positions` maps an id to a unique x-coordinate; the container
   * and target stay at the jsdom default (0,0,0,0), so each ray's own
   * `x1` resolves to exactly its pin's own mocked `left` value.
   */
  function GeometryHarness({
    pinIds,
    positions,
  }: {
    pinIds: string[];
    positions: Record<string, number>;
  }) {
    const containerRef = useRef<HTMLDivElement>(null);
    const targetRef = useRef<HTMLDivElement>(null);
    const pinElsRef = useRef(new Map<string, HTMLElement>());

    return (
      <div ref={containerRef}>
        {pinIds.map((id) => (
          <button
            key={id}
            ref={(el) => {
              if (el) pinElsRef.current.set(id, el);
              else pinElsRef.current.delete(id);
            }}
            data-mock-x={positions[id]}
          >
            {id}
          </button>
        ))}
        <div ref={targetRef}>result</div>
        <BusConvergenceOverlay
          containerRef={containerRef}
          pinEls={pinElsRef.current}
          pinIds={pinIds}
          targetRef={targetRef}
        />
      </div>
    );
  }

  it("re-measures with correct, non-stale geometry when the pin count stays the same but the actual ids, their order, and their resolved elements all change", () => {
    // Every button carries its own intended x position via `data-mock-x`;
    // this override reads that attribute back so each element reports its
    // *own* distinct, real rect — never a shared default that would mask
    // a stale-vs-fresh mix-up. Container/target are untouched (0,0,0,0),
    // so `fromRight` is always false and each ray's `x1` resolves to
    // exactly that pin's own mocked `left`.
    const rectSpy = vi
      .spyOn(HTMLElement.prototype, "getBoundingClientRect")
      .mockImplementation(function (this: HTMLElement) {
        const raw = this.getAttribute("data-mock-x");
        const x = raw !== null ? Number(raw) : 0;
        return {
          x,
          y: 0,
          left: x,
          right: x + 40,
          top: 0,
          bottom: 20,
          width: 40,
          height: 20,
          toJSON() {
            return this;
          },
        } as DOMRect;
      });

    try {
      const linesOf = (container: HTMLElement) =>
        Array.from(container.querySelectorAll("line[data-pin-id]"));

      const { container, rerender } = render(
        <GeometryHarness pinIds={["a", "b"]} positions={{ a: 100, b: 200 }} />,
      );

      // Baseline: two rays, in declared order, each at its own real position.
      expect(linesOf(container).map((l) => l.getAttribute("data-pin-id"))).toEqual(["a", "b"]);
      expect(Number(linesOf(container)[0].getAttribute("x1"))).toBe(100);
      expect(Number(linesOf(container)[1].getAttribute("x1"))).toBe(200);

      // Same two ids, same positions, order alone reversed. Ray endpoint
      // *values* are order-independent (each is looked up by id), so the
      // only observable proof that a real re-measurement ran — as opposed
      // to the previous render simply being left in place — is that the
      // rendered `<line>` order now follows the new declared order.
      rerender(<GeometryHarness pinIds={["b", "a"]} positions={{ a: 100, b: 200 }} />);
      expect(linesOf(container).map((l) => l.getAttribute("data-pin-id"))).toEqual(["b", "a"]);

      // Same count (2), but "a" is replaced by a brand-new id "c" resolving
      // to a brand-new DOM element at a brand-new position. A stale
      // geometry object would either still show "a" at x=100 or show
      // nothing at all for "c" — real recalculation must show "c" at its
      // own real position and "a" gone entirely.
      rerender(<GeometryHarness pinIds={["b", "c"]} positions={{ b: 200, c: 300 }} />);
      const idsNow = linesOf(container).map((l) => l.getAttribute("data-pin-id"));
      expect(idsNow).toEqual(["b", "c"]);
      expect(idsNow).not.toContain("a");
      expect(Number(linesOf(container)[0].getAttribute("x1"))).toBe(200);
      expect(Number(linesOf(container)[1].getAttribute("x1"))).toBe(300);
    } finally {
      rectSpy.mockRestore();
    }
  });
});
