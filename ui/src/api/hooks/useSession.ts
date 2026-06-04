import { useQuery } from "@tanstack/react-query";
import type { ControlApiClient } from "../client";
import type { ControlAuthProbe } from "../domains/auth";

export const sessionQueryKey = (baseUrl: string) => ["session", baseUrl];

export function useAuthSessionQuery(client: ControlApiClient) {
  return useQuery<ControlAuthProbe, Error>({
    queryKey: sessionQueryKey(client.getBaseUrl()),
    queryFn: () => client.getSessionStatus(),
    staleTime: 1000 * 60 * 5, // 5 minutes
    retry: false, // Don't retry auth probes, they either work or don't.
  });
}
