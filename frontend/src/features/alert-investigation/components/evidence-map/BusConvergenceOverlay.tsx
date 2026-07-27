import { useEffect, useState, type RefObject } from "react";
import styles from "./evidence-map.module.css";

interface ConvergenceRay {
  id: string;
  fromX: number;
  fromY: number;
  toX: number;
  toY: number;
}

interface ConvergenceGeometry {
  rays: ConvergenceRay[];
  width: number;
  height: number;
}

/**
 * Draws the Detection Definition -> Detection Result relationship as the
 * approved screenshots actually show it: not one flat connector in the
 * grid gutter, but a faint ray from *every* declared characteristic pin
 * converging on Detection Result's own facing edge — the bus's branching
 * pins visibly funneling into the single resolved-match object
 * (`DetectionResultResolution`'s own "convergence" naming, made literal).
 *
 * Always present (not selection-gated) — it renders the same permanent,
 * structural fact regardless of which characteristic, if any, is
 * currently selected; the selection-driven trace (`SelectionTraceOverlay`)
 * is a separate, independently-rendered overlay layer for the one dynamic
 * row-to-pin relationship.
 *
 * Measures real DOM positions (`pinEls`/`targetRef`) rather than any fixed
 * or viewport-specific coordinate — the same honest, industry-standard
 * technique `useConnectorPath` already uses for the selection trace, now
 * applied to a fixed (not user-driven) set of endpoints.
 */
export function BusConvergenceOverlay({
  containerRef,
  pinEls,
  pinIds,
  targetRef,
  dimmed = false,
}: {
  containerRef: RefObject<HTMLElement | null>;
  /** Every currently-rendered pin's own DOM node, keyed by row id — a
   *  stable Map instance mutated in place by `DetectionDefinitionFrame`'s
   *  ref callbacks; read here only inside an effect, never during render,
   *  so it always reflects the commit that just finished. */
  pinEls: Map<string, HTMLElement>;
  /** The declared characteristic ids to draw a ray for, in the real
   *  concordance's own order — the only reactive trigger this overlay
   *  needs (a new alert response, or a definition with a different
   *  declared list, both change this array's identity). */
  pinIds: readonly string[];
  targetRef: RefObject<HTMLElement | null>;
  dimmed?: boolean;
}) {
  const [geometry, setGeometry] = useState<ConvergenceGeometry | null>(null);
  // `JSON.stringify`, not a delimiter-joined string: a characteristic id is
  // validated only as non-empty and unique-within-its-own-definition
  // (internal/detection/detection.go's `validate` — no character-set
  // restriction), so a plain `.join(",")` cannot be shown collision-free —
  // e.g. `["a,b", "c"]` and `["a", "b,c"]` would join to the identical
  // string. `JSON.stringify` escapes embedded quotes/delimiters, so two
  // distinct string arrays can never serialize to the same value.
  const pinIdsKey = JSON.stringify(pinIds);

  // A regular effect, not `useLayoutEffect`: this component is mounted
  // unconditionally from the very first render (unlike `SelectionTraceOverlay`,
  // which only ever mounts after a selection exists, by which point its
  // container ref is already attached from an earlier commit). React
  // attaches a host node's own ref only after all of its descendants'
  // layout effects have run in that same commit — so on this component's
  // first pass, `containerRef.current` (an *ancestor* DOM node) is still
  // null inside a layout effect, and the very first measurement silently
  // no-ops. Confirmed with a controlled real-browser A/B check on a fresh,
  // uninteracted page load: the `useLayoutEffect` version never draws a
  // single ray within 2s; a plain `useEffect` — which runs only after the
  // whole commit, including the ancestor's own ref attachment, has
  // finished — draws every ray within ~350ms with zero interaction.
  useEffect(() => {
    const container = containerRef.current;
    const target = targetRef.current;
    if (!container || !target || pinIds.length === 0) {
      setGeometry(null);
      return;
    }

    function measure() {
      const containerRect = container!.getBoundingClientRect();
      const targetRect = target!.getBoundingClientRect();
      const toX = targetRect.left - containerRect.left;
      const toY = targetRect.top + targetRect.height / 2 - containerRect.top;

      const rays: ConvergenceRay[] = [];
      for (const id of pinIds) {
        const pinEl = pinEls.get(id);
        if (!pinEl) continue;
        const pinRect = pinEl.getBoundingClientRect();
        const fromRight = pinRect.left <= targetRect.left;
        rays.push({
          id,
          fromX: (fromRight ? pinRect.right : pinRect.left) - containerRect.left,
          fromY: pinRect.top + pinRect.height / 2 - containerRect.top,
          toX,
          toY,
        });
      }
      setGeometry({ rays, width: containerRect.width, height: containerRect.height });
    }

    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(container);
    window.addEventListener("resize", measure);
    return () => {
      observer.disconnect();
      window.removeEventListener("resize", measure);
    };
    // `pinIds` itself is deliberately not a dependency here — the real
    // orchestrator (`InvestigationMap`) recomputes it as a brand-new array
    // via `characteristicRows.map(...)` on every single render, regardless
    // of whether the declared list actually changed. Depending on the
    // array reference directly was confirmed (by a focused test) to tear
    // down and recreate the ResizeObserver on every unrelated re-render
    // (e.g. toggling `dimmed`) even when no pin was added or removed.
    // `pinIdsKey` is a content-derived primitive, so the effect only
    // re-runs when the declared set of ids genuinely changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pinIdsKey, pinEls, containerRef, targetRef]);

  if (!geometry || geometry.rays.length === 0) return null;

  return (
    <svg
      className={styles.convergenceOverlay}
      width={geometry.width}
      height={geometry.height}
      viewBox={`0 0 ${geometry.width} ${geometry.height}`}
      aria-hidden="true"
      data-dim={dimmed || undefined}
    >
      {geometry.rays.map((ray) => (
        <line
          key={ray.id}
          className={styles.convergenceLine}
          data-pin-id={ray.id}
          x1={ray.fromX}
          y1={ray.fromY}
          x2={ray.toX}
          y2={ray.toY}
        />
      ))}
    </svg>
  );
}
