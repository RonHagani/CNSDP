import { useQuery } from "@tanstack/react-query";
import { fetchAlertInvestigation } from "../lib/alertSource";

export function useAlertInvestigation(alertId: string) {
  return useQuery({
    queryKey: ["alert-investigation", alertId],
    queryFn: ({ signal }) => fetchAlertInvestigation(alertId, signal),
    retry: false,
    staleTime: 30_000,
  });
}
