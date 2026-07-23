import { motion, useReducedMotion } from "framer-motion";
import { StatusBadge } from "@/components/StatusBadge/StatusBadge";
import styles from "./StateScreens.module.css";

function Screen({
  eyebrow,
  title,
  message,
  tone,
  action,
}: {
  eyebrow: string;
  title: string;
  message: string;
  tone: "severed" | "branch" | "neutral";
  action?: { label: string; onClick: () => void };
}) {
  const reduceMotion = useReducedMotion();
  return (
    <motion.div
      className={styles.screen}
      role="status"
      initial={reduceMotion ? false : { opacity: 0 }}
      animate={{ opacity: 1 }}
      transition={{ duration: reduceMotion ? 0 : 0.22 }}
    >
      <StatusBadge tone={tone}>{eyebrow}</StatusBadge>
      <h1 className={styles.title}>{title}</h1>
      <p className={styles.message}>{message}</p>
      {action && (
        <button type="button" className={styles.action} onClick={action.onClick}>
          {action.label}
        </button>
      )}
    </motion.div>
  );
}

export function LoadingScreen() {
  return (
    <div className={styles.loading} role="status" aria-live="polite">
      <div className={styles.loadingRail}>
        {Array.from({ length: 8 }).map((_, i) => (
          <span key={i} className={styles.loadingDot} />
        ))}
      </div>
      <p className={styles.loadingLabel}>Retrieving alert investigation record…</p>
    </div>
  );
}

export function UnauthorizedScreen() {
  return (
    <Screen
      eyebrow="401 Unauthorized"
      tone="severed"
      title="Authentication required"
      message="The bearer token was missing or did not match the platform's configured API_TOKEN (NFR-012, NFR-013). Every product-exposed path — including this one — denies unauthenticated or unauthorized access."
    />
  );
}

export function UnavailableScreen({ onRetry }: { onRetry: () => void }) {
  return (
    <Screen
      eyebrow="Backend unavailable"
      tone="severed"
      title="The investigation backend could not be reached"
      message="No internal error detail is available for display — a platform fault is reported without leaking implementation detail (NFR-022)."
      action={{ label: "Retry", onClick: onRetry }}
    />
  );
}

export function NotFoundScreen({ alertId }: { alertId: string }) {
  return (
    <Screen
      eyebrow="404 Not found"
      tone="branch"
      title={`No alert exists with id ${alertId}`}
      message="This mirrors the real backend's documented behavior for a nonexistent alert id — a distinct, visible outcome, never a silently empty investigation."
    />
  );
}
