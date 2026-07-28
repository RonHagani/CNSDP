import { useLayoutEffect, useRef, useState } from "react";

/**
 * Reports whether `ref`'s element currently has real, measured horizontal
 * overflow (`scrollWidth > clientWidth`) — the signal the inventory table
 * uses to show its scroll affordance only when there genuinely is more
 * content off-screen, never as a permanently displayed instruction.
 * jsdom performs no real layout, so this always resolves `false` under
 * vitest; true behavior is verified in the browser via Playwright.
 */
export function useHorizontalOverflow<T extends HTMLElement>() {
  const ref = useRef<T | null>(null);
  const [hasOverflow, setHasOverflow] = useState(false);

  useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return;

    function measure() {
      if (!el) return;
      setHasOverflow(el.scrollWidth > el.clientWidth + 1);
    }

    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(el);
    return () => observer.disconnect();
  });

  return { ref, hasOverflow };
}
