import { Link, useParams } from "react-router-dom";
import { useAlertInvestigation } from "./hooks/useAlertInvestigation";
import { AlertFetchError } from "./lib/alertSource";
import { InvestigationMap } from "./InvestigationMap";
import {
  LoadingScreen,
  NotFoundScreen,
  UnauthorizedScreen,
  UnavailableScreen,
} from "./components/StateScreens";
import mapStyles from "./components/evidence-map/evidence-map.module.css";
import styles from "./AlertInvestigationPage.module.css";

/**
 * The route component for `/alerts/:alertId`. Owns route params and the
 * query itself — data comes from the real `GET /v1/alerts/{id}` endpoint
 * via `useAlertInvestigation` (src/features/alert-investigation/lib/alertSource.ts),
 * never a fixture. The success branch renders exactly one thing, the Dark
 * Evidence Map (`InvestigationMap`). `key={query.data.alertId}` forces a
 * fresh `InvestigationMap` mount, and therefore fresh interaction
 * (selection) state, whenever the alert identity itself changes — so a
 * stale selection or provenance annotation from a previously viewed alert
 * can never survive a navigation to a different one.
 */
export function AlertInvestigationPage() {
  const { alertId = "1" } = useParams<{ alertId: string }>();
  const query = useAlertInvestigation(alertId);

  return (
    <div className={`${mapStyles.mapRoot} ${styles.page}`}>
      <a href="#investigation-main" className={styles.skipLink}>
        Skip to investigation content
      </a>

      <div className={styles.utilityBar}>
        <div className={styles.utilityBarIdentity}>
          <span className={styles.wordmark}>CNSDP</span>
          <span className={styles.wordmarkSub}>Alert Investigation</span>
        </div>
        <Link to="/alerts" className={styles.backLink}>
          ← Back to alerts
        </Link>
      </div>

      {query.isSuccess ? (
        <main id="investigation-main">
          <InvestigationMap data={query.data} key={query.data.alertId} />
        </main>
      ) : (
        <>
          {query.isPending && <LoadingScreen />}

          {query.isError &&
            (query.error instanceof AlertFetchError ? (
              query.error.kind === "unauthorized" ? (
                <UnauthorizedScreen />
              ) : query.error.kind === "not-found" ? (
                <NotFoundScreen alertId={alertId} />
              ) : (
                <UnavailableScreen onRetry={() => query.refetch()} />
              )
            ) : (
              <UnavailableScreen onRetry={() => query.refetch()} />
            ))}
        </>
      )}
    </div>
  );
}
