import { isRouteErrorResponse, useRouteError } from "react-router-dom";
import styles from "./RootErrorBoundary.module.css";

/**
 * Router-level error boundary — catches routing/render faults distinct
 * from an investigation-data fetch failure (which is handled inline by
 * AlertInvestigationPage as an "unavailable backend" presentation state,
 * not a route crash).
 */
export function RootErrorBoundary() {
  const error = useRouteError();
  const message = isRouteErrorResponse(error)
    ? `${error.status} ${error.statusText}`
    : error instanceof Error
      ? error.message
      : "An unexpected error occurred.";

  return (
    <div className={styles.root} role="alert">
      <p className={styles.eyebrow}>CNSDP</p>
      <h1 className={styles.heading}>The investigation console could not render.</h1>
      <p className={styles.message}>{message}</p>
    </div>
  );
}
