import { useQuery } from "@tanstack/react-query";
import { fetchAlertInvestigation, type DemoScenario } from "../lib/alertSource";

export function useAlertInvestigation(alertId: string, demoScenario?: DemoScenario) {
  return useQuery({
    queryKey: ["alert-investigation", alertId, demoScenario ?? null],
    queryFn: ({ signal }) => fetchAlertInvestigation(alertId, demoScenario, signal),
    retry: false,
    staleTime: 30_000,
  });
}
