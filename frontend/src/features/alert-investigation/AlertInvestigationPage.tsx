import { useEffect } from "react";
import { useParams, useSearchParams } from "react-router-dom";
import { useAlertInvestigation } from "./hooks/useAlertInvestigation";
import { useInvestigationCommands } from "./hooks/useInvestigationCommands";
import { AlertFetchError, type DemoScenario } from "./lib/alertSource";
import { InvestigationUIProvider, useInvestigationUI } from "./lib/InvestigationUIContext";
import { InvestigationHero } from "./components/InvestigationHero";
import { ConduitField } from "./components/ConduitField";
import { ConduitMobileStrip } from "./components/ConduitMobileStrip";
import { StageContent } from "./components/StageContent";
import { EvidenceRibbon } from "./components/EvidenceRibbon";
import { ProofChain } from "./components/ProofChain";
import {
  LoadingScreen,
  NotFoundScreen,
  UnauthorizedScreen,
  UnavailableScreen,
} from "./components/StateScreens";
import { CommandPalette } from "@/components/CommandPalette/CommandPalette";
import styles from "./AlertInvestigationPage.module.css";

const VALID_DEMO_SCENARIOS: DemoScenario[] = ["unauthorized", "unavailable", "slow"];

export function AlertInvestigationPage() {
  return (
    <InvestigationUIProvider>
      <AlertInvestigationPageInner />
    </InvestigationUIProvider>
  );
}

function AlertInvestigationPageInner() {
  const { alertId = "1" } = useParams<{ alertId: string }>();
  const [searchParams] = useSearchParams();
  const demoParam = searchParams.get("demo");
  const demoScenario = VALID_DEMO_SCENARIOS.includes(demoParam as DemoScenario)
    ? (demoParam as DemoScenario)
    : undefined;

  const query = useAlertInvestigation(alertId, demoScenario);
  const commands = useInvestigationCommands();
  const { paletteOpen, setPaletteOpen, lineage } = useInvestigationUI();

  useEffect(() => {
    function handleKeyDown(event: KeyboardEvent) {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "k") {
        event.preventDefault();
        setPaletteOpen(true);
      }
    }
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [setPaletteOpen]);

  return (
    <div className={styles.page}>
      <div className="cnsdp-grain" aria-hidden="true" />
      <a href="#investigation-main" className={styles.skipLink}>
        Skip to investigation content
      </a>

      {query.isSuccess ? (
        <div className={styles.stageFloor} data-spotlight={lineage !== null}>
          <div data-role="conduit-field">
            <ConduitField data={query.data} />
          </div>
          <ConduitMobileStrip data={query.data} />

          <div className={styles.content}>
            <div data-role="hero">
              <InvestigationHero data={query.data} onOpenPalette={() => setPaletteOpen(true)} />
            </div>

            <main id="investigation-main">
              <StageContent data={query.data} />
              <div data-role="evidence-ribbon">
                <EvidenceRibbon data={query.data} />
              </div>
              <div data-role="proof-chain">
                <ProofChain traceability={query.data.traceability} />
              </div>
            </main>
          </div>
        </div>
      ) : (
        <>
          <div className={styles.utilityBar}>
            <span className={styles.wordmark}>CNSDP</span>
            <span className={styles.wordmarkSub}>Evidence Theater</span>
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

      {query.isSuccess && (
        <CommandPalette open={paletteOpen} onOpenChange={setPaletteOpen} commands={commands} />
      )}
    </div>
  );
}
