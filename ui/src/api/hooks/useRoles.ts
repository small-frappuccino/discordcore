import { useQuery } from "@tanstack/react-query";
import { ControlApiClient } from "../control";

export const guildRolesQueryKey = (baseUrl: string, guildId: string) => ["guildRoles", baseUrl, guildId];

export function useGuildRoleOptionsQuery(client: ControlApiClient, guildId: string) {
  return useQuery({
    queryKey: guildRolesQueryKey(client.getBaseUrl(), guildId),
    queryFn: () => client.listGuildRoleOptions(guildId),
    enabled: !!guildId,
  });
}
