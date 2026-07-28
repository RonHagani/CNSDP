import { Link } from "react-router-dom";
import { AppShell } from "./AppShell";
import styles from "./NotFoundPage.module.css";

/**
 * Catch-all route for a path that matches no defined route (distinct from
 * the alert-investigation feature's own "no alert exists with this id"
 * state, which is a data-layer 404 for a route that does exist). Wrapped
 * in AppShell so an unmatched URL still visibly reads as the platform,
 * not a bare error page.
 */
export function NotFoundPage() {
  return (
    <AppShell>
      <div className={styles.wrap} role="status">
        <p className={styles.eyebrow}>404 Not found</p>
        <h1 className={styles.heading}>This page does not exist.</h1>
        <p className={styles.message}>
          There is no route at this address. Return to the alert inventory to continue.
        </p>
        <Link to="/alerts" className={styles.link}>
          Go to alert inventory →
        </Link>
      </div>
    </AppShell>
  );
}
