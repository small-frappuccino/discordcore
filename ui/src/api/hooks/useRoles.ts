import { useQuery } from "@tanstack/react-query";
import type { ControlApiClient } from "../client";
import { listGuildRoleOptions } from "../domains/guilds";

export const guildRolesQueryKey = (baseUrl: string, guildId: string) => ["guildRoles", baseUrl, guildId];

export function useGuildRoleOptionsQuery(client: ControlApiClient, guildId: string) {
  return useQuery({
    queryKey: guildRolesQueryKey(client.getBaseUrl(), guildId),
    queryFn: () => listGuildRoleOptions(client, guildId),
    enabled: !!guildId,
  });
}
