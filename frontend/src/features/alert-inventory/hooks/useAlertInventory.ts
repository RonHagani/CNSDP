import { useQuery } from "@tanstack/react-query";
import { fetchAlertInventory } from "../lib/alertInventorySource";

export function useAlertInventory() {
  return useQuery({
    queryKey: ["alert-inventory"],
    queryFn: ({ signal }) => fetchAlertInventory(signal),
    retry: false,
    staleTime: 30_000,
  });
}
