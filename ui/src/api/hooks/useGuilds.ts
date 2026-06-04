import { useQuery } from "@tanstack/react-query";
import type { ControlApiClient } from "../client";
import { listAccessibleGuilds, listManageableGuilds } from "../domains/guilds";

export const accessibleGuildsQueryKey = (baseUrl: string) => ["accessibleGuilds", baseUrl];
export const manageableGuildsQueryKey = (baseUrl: string) => ["manageableGuilds", baseUrl];

export function useAccessibleGuildsQuery(client: ControlApiClient, enabled: boolean = true) {
  return useQuery({
    queryKey: accessibleGuildsQueryKey(client.getBaseUrl()),
    queryFn: () => listAccessibleGuilds(client),
    enabled,
  });
}

export function useManageableGuildsQuery(client: ControlApiClient, enabled: boolean = true) {
  return useQuery({
    queryKey: manageableGuildsQueryKey(client.getBaseUrl()),
    queryFn: () => listManageableGuilds(client),
    enabled,
  });
}
