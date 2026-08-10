import { useQuery } from "@tanstack/react-query";
import { fetchDataSources } from "../lib/dataSourcesSource";

export function useDataSources() {
  return useQuery({
    queryKey: ["data-sources"],
    queryFn: ({ signal }) => fetchDataSources(signal),
    retry: false,
    staleTime: 30_000,
  });
}
