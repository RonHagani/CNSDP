import type { ReactNode } from "react";
import { NavLink } from "react-router-dom";
import styles from "./AppShell.module.css";

/** Destinations that exist but have no implemented route yet. Rendered as
 *  visibly disabled, non-navigating chrome — never a fake functional page
 *  (product requirement: no placeholder page for a feature that doesn't
 *  exist). */
const PLANNED_DESTINATIONS = ["System Health"];

/**
 * The restrained production application shell: product identity plus a
 * persistent primary navigation area. Wraps the Alert Inventory (and any
 * other future top-level screen) — deliberately not the Dark Evidence Map
 * investigation canvas, which keeps its own distraction-free utility bar
 * (AlertInvestigationPage) rather than being wrapped a second time.
 */
export function AppShell({ children }: { children: ReactNode }) {
  return (
    <div className={styles.shell}>
      <header className={styles.header}>
        <div className={styles.identity}>
          <span className={styles.wordmark}>CNSDP</span>
          <span className={styles.wordmarkSub}>Security Telemetry &amp; Detection Platform</span>
        </div>
        <nav className={styles.nav} aria-label="Primary">
          <NavLink to="/alerts" className={styles.navLink}>
            Alerts
          </NavLink>
          <NavLink to="/data-sources" className={styles.navLink}>
            Data Sources
          </NavLink>
          <NavLink to="/detections" className={styles.navLink}>
            Detections
          </NavLink>
          {PLANNED_DESTINATIONS.map((label) => (
            <span key={label} className={styles.navDisabled} aria-disabled="true">
              {label}
              <span className={styles.navDisabledTag}>Soon</span>
            </span>
          ))}
        </nav>
      </header>
      <main className={styles.main}>{children}</main>
    </div>
  );
}
