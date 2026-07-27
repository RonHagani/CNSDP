import { useLayoutEffect, useState, type RefObject } from "react";

export interface ConnectorPoint {
  x: number;
  y: number;
}

export interface ConnectorPath {
  from: ConnectorPoint;
  to: ConnectorPoint;
  width: number;
  height: number;
}

/**
 * Measures two elements' positions relative to a shared ancestor and
 * returns a two-point connector (exit edge of `fromRef`, facing edge of
 * `toRef`), recalculated whenever `deps` changes and on resize.
 *
 * This exists only for the one genuinely dynamic relationship the Dark
 * Evidence Map canvas has to draw — the selection trace between whichever
 * Normalized Event row and whichever characteristic pin happen to be
 * active. Every *structural* relationship (the six artifacts' connectors,
 * the characteristic bus's own pin geometry) stays pure CSS Grid/data-
 * driven and never uses this hook — a fixed CSS track can express "the
 * gutter between Normalized and Definition," but not "the specific row a
 * user just selected," so a small measured overlay (the same technique any
 * interactive tooltip/connector library uses) is the honest way to draw
 * this one line without hardcoding pixel geometry anywhere.
 */
export function useConnectorPath(
  containerRef: RefObject<HTMLElement | null>,
  fromRef: RefObject<HTMLElement | null>,
  toRef: RefObject<HTMLElement | null>,
  deps: readonly unknown[],
): ConnectorPath | null {
  const [path, setPath] = useState<ConnectorPath | null>(null);

  useLayoutEffect(() => {
    const container = containerRef.current;
    const fromEl = fromRef.current;
    const toEl = toRef.current;
    if (!container || !fromEl || !toEl) {
      setPath(null);
      return;
    }

    function measure() {
      const containerRect = container!.getBoundingClientRect();
      const fromRect = fromEl!.getBoundingClientRect();
      const toRect = toEl!.getBoundingClientRect();
      const fromExitsRight = fromRect.left <= toRect.left;

      setPath({
        from: {
          x: (fromExitsRight ? fromRect.right : fromRect.left) - containerRect.left,
          y: fromRect.top + fromRect.height / 2 - containerRect.top,
        },
        to: {
          x: (fromExitsRight ? toRect.left : toRect.right) - containerRect.left,
          y: toRect.top + toRect.height / 2 - containerRect.top,
        },
        width: containerRect.width,
        height: containerRect.height,
      });
    }

    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(container);
    window.addEventListener("resize", measure);
    return () => {
      observer.disconnect();
      window.removeEventListener("resize", measure);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps);

  return path;
}
