import {
  useMemo,
  useState,
  useEffect,
  useRef,
  type KeyboardEvent as ReactKeyboardEvent,
} from "react";
import * as Dialog from "@radix-ui/react-dialog";
import { VisuallyHidden } from "@radix-ui/react-visually-hidden";
import { AnimatePresence, motion, useReducedMotion } from "framer-motion";
import styles from "./CommandPalette.module.css";

export interface Command {
  id: string;
  label: string;
  hint?: string;
  run: () => void;
}

/**
 * Keyboard-accessible command palette. Radix's unstyled Dialog primitive
 * provides real value here specifically: portal rendering, focus trap, and
 * Escape/overlay-dismiss handling for free, without adopting any Radix or
 * shadcn visual styling — every visible pixel below is CNSDP's own tokens
 * (product-experience-brief.md §9). Command list is restrained and
 * hand-authored (lib/commands.ts) — no command references a capability the
 * product does not actually have.
 */
export function CommandPalette({
  open,
  onOpenChange,
  commands,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  commands: Command[];
}) {
  const [query, setQuery] = useState("");
  const [activeIndex, setActiveIndex] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);
  const reduceMotion = useReducedMotion();

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return commands;
    return commands.filter((c) => c.label.toLowerCase().includes(q));
  }, [commands, query]);

  useEffect(() => {
    if (open) {
      setQuery("");
      setActiveIndex(0);
    }
  }, [open]);

  useEffect(() => {
    setActiveIndex(0);
  }, [filtered.length]);

  function handleKeyDown(event: ReactKeyboardEvent) {
    if (event.key === "ArrowDown") {
      event.preventDefault();
      setActiveIndex((i) => Math.min(i + 1, filtered.length - 1));
    } else if (event.key === "ArrowUp") {
      event.preventDefault();
      setActiveIndex((i) => Math.max(i - 1, 0));
    } else if (event.key === "Enter") {
      event.preventDefault();
      const command = filtered[activeIndex];
      if (command) {
        command.run();
        onOpenChange(false);
      }
    }
  }

  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <AnimatePresence>
        {open && (
          <Dialog.Portal forceMount>
            <Dialog.Overlay asChild forceMount>
              <motion.div
                className={styles.overlay}
                initial={reduceMotion ? false : { opacity: 0 }}
                animate={{ opacity: 1 }}
                exit={{ opacity: 0 }}
                transition={{ duration: reduceMotion ? 0 : 0.15 }}
              />
            </Dialog.Overlay>
            <Dialog.Content asChild forceMount onOpenAutoFocus={(e) => e.preventDefault()}>
              <motion.div
                className={styles.panel}
                role="combobox"
                aria-expanded="true"
                aria-owns="command-palette-list"
                initial={reduceMotion ? false : { opacity: 0, scale: 0.98, y: -8 }}
                animate={{ opacity: 1, scale: 1, y: 0 }}
                exit={{ opacity: 0, scale: 0.98, y: -8 }}
                transition={{ duration: reduceMotion ? 0 : 0.16, ease: [0.16, 1, 0.3, 1] }}
              >
                <VisuallyHidden asChild>
                  <Dialog.Title>Investigation command palette</Dialog.Title>
                </VisuallyHidden>
                <VisuallyHidden asChild>
                  <Dialog.Description>
                    Search and run investigation commands for the current alert.
                  </Dialog.Description>
                </VisuallyHidden>
                <input
                  ref={inputRef}
                  autoFocus
                  className={styles.input}
                  placeholder="Jump to a stage, artifact, or action…"
                  value={query}
                  onChange={(e) => setQuery(e.target.value)}
                  onKeyDown={handleKeyDown}
                  aria-controls="command-palette-list"
                  aria-activedescendant={
                    filtered[activeIndex] ? `command-${filtered[activeIndex].id}` : undefined
                  }
                />
                <ul className={styles.list} id="command-palette-list" role="listbox">
                  {filtered.length === 0 && <li className={styles.empty}>No matching commands.</li>}
                  {filtered.map((command, index) => (
                    <li
                      key={command.id}
                      id={`command-${command.id}`}
                      role="option"
                      aria-selected={index === activeIndex}
                      className={styles.item}
                      data-active={index === activeIndex || undefined}
                      onMouseEnter={() => setActiveIndex(index)}
                      onClick={() => {
                        command.run();
                        onOpenChange(false);
                      }}
                    >
                      <span>{command.label}</span>
                      {command.hint && <span className={styles.hint}>{command.hint}</span>}
                    </li>
                  ))}
                </ul>
              </motion.div>
            </Dialog.Content>
          </Dialog.Portal>
        )}
      </AnimatePresence>
    </Dialog.Root>
  );
}
