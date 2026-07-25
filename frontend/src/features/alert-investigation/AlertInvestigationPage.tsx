import { useParams, useSearchParams } from "react-router-dom";
import { useAlertInvestigation } from "./hooks/useAlertInvestigation";
import { AlertFetchError, type DemoScenario } from "./lib/alertSource";
import { InvestigationMap } from "./InvestigationMap";
import {
  LoadingScreen,
  NotFoundScreen,
  UnauthorizedScreen,
  UnavailableScreen,
} from "./components/StateScreens";
import mapStyles from "./components/evidence-map/evidence-map.module.css";
import styles from "./AlertInvestigationPage.module.css";

const VALID_DEMO_SCENARIOS: DemoScenario[] = ["unauthorized", "unavailable", "slow"];

/**
 * The route component for `/alerts/:alertId`. Owns route params, the
 * demo-scenario override, and the query itself — unchanged by this pass.
 * The success branch now renders exactly one thing, the Dark Evidence Map
 * (`InvestigationMap`), replacing the entire Forensic Case Folio
 * composition (`InvestigationDossier`) in one coordinated change (Track B
 * Pass 5, the atomic shell swap) — this route never renders a mix of the
 * two visual systems. `key={query.data.alertId}` forces a fresh
 * `InvestigationMap` mount, and therefore fresh interaction (selection)
 * state, whenever the alert identity itself changes — the same mechanism
 * `InvestigationDossier` relied on.
 *
 * `InvestigationDossier` and the rest of the Folio component tree are not
 * deleted by this pass (implementation plan §9 Pass 5) — they simply have
 * no remaining live importer after this file's own import changes below.
 */
export function AlertInvestigationPage() {
  const { alertId = "1" } = useParams<{ alertId: string }>();
  const [searchParams] = useSearchParams();
  const demoParam = searchParams.get("demo");
  const demoScenario = VALID_DEMO_SCENARIOS.includes(demoParam as DemoScenario)
    ? (demoParam as DemoScenario)
    : undefined;

  const query = useAlertInvestigation(alertId, demoScenario);

  return (
    <div className={`${mapStyles.mapRoot} ${styles.page}`}>
      <a href="#investigation-main" className={styles.skipLink}>
        Skip to investigation content
      </a>

      {query.isSuccess ? (
        <main id="investigation-main">
          <InvestigationMap data={query.data} key={query.data.alertId} />
        </main>
      ) : (
        <>
          <div className={styles.utilityBar}>
            <span className={styles.wordmark}>CNSDP</span>
            <span className={styles.wordmarkSub}>Alert Investigation</span>
          </div>

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
