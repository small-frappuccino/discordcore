import { useQuery } from "@tanstack/react-query";
import { ControlApiClient } from "../control";

export const accessibleGuildsQueryKey = (baseUrl: string) => ["accessibleGuilds", baseUrl];
export const manageableGuildsQueryKey = (baseUrl: string) => ["manageableGuilds", baseUrl];

export function useAccessibleGuildsQuery(client: ControlApiClient, enabled: boolean = true) {
  return useQuery({
    queryKey: accessibleGuildsQueryKey(client.getBaseUrl()),
    queryFn: () => client.listAccessibleGuilds(),
    enabled,
  });
}

export function useManageableGuildsQuery(client: ControlApiClient, enabled: boolean = true) {
  return useQuery({
    queryKey: manageableGuildsQueryKey(client.getBaseUrl()),
    queryFn: () => client.listManageableGuilds(),
    enabled,
  });
}
