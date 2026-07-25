import { useParams, useSearchParams } from "react-router-dom";
import { useAlertInvestigation } from "./hooks/useAlertInvestigation";
import { AlertFetchError, type DemoScenario } from "./lib/alertSource";
import { InvestigationDossier } from "./InvestigationDossier";
import {
  LoadingScreen,
  NotFoundScreen,
  UnauthorizedScreen,
  UnavailableScreen,
} from "./components/StateScreens";
import dossierStyles from "./components/dossier/dossier.module.css";
import styles from "./AlertInvestigationPage.module.css";

const VALID_DEMO_SCENARIOS: DemoScenario[] = ["unauthorized", "unavailable", "slow"];

/**
 * The route component for `/alerts/:alertId`. Owns route params, the
 * demo-scenario override, and the query itself — unchanged from before
 * the shell swap. The success branch now renders exactly one thing, the
 * Causal Evidence Dossier (`InvestigationDossier`), replacing the entire
 * legacy Signal Path composition (ConduitField, ConduitMobileStrip,
 * EvidenceRibbon, ProofChain, StageContent, InvestigationHero) in one
 * coordinated change — this route never renders a mix of the two visual
 * systems. `key={query.data.alertId}` forces a fresh
 * `InvestigationDossier` mount, and therefore fresh interaction state,
 * whenever the alert identity itself changes.
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
    <div className={`${dossierStyles.dossierRoot} ${styles.page}`}>
      <a href="#investigation-main" className={styles.skipLink}>
        Skip to investigation content
      </a>

      {query.isSuccess ? (
        <main id="investigation-main">
          <InvestigationDossier data={query.data} key={query.data.alertId} />
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
