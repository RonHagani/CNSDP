import { useQuery } from "@tanstack/react-query";
import { fetchDetections } from "../lib/detectionsSource";

export function useDetections() {
  return useQuery({
    queryKey: ["detections"],
    queryFn: ({ signal }) => fetchDetections(signal),
    retry: false,
    staleTime: 30_000,
  });
}
