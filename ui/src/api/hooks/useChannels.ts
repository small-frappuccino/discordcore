import { useQuery } from "@tanstack/react-query";
import type { ControlApiClient } from "../client";
import { listGuildChannelOptions } from "../domains/guilds";

export const guildChannelsQueryKey = (baseUrl: string, guildId: string) => ["guildChannels", baseUrl, guildId];

export function useGuildChannelOptionsQuery(client: ControlApiClient, guildId: string) {
  return useQuery({
    queryKey: guildChannelsQueryKey(client.getBaseUrl(), guildId),
    queryFn: () => listGuildChannelOptions(client, guildId),
    enabled: !!guildId,
  });
}
