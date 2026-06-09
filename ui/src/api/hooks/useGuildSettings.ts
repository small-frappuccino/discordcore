import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import type { ControlApiClient } from "../client";
import { getGuildSettings, updateGuildSettings, type GuildSettingsWorkspace } from "../domains/guilds";
import type { GuildRolesSettingsSection } from "../domains/guilds";

export const guildSettingsQueryKey = (baseUrl: string, guildId: string) => ["guildSettings", baseUrl, guildId];

export function useGuildSettingsQuery(client: ControlApiClient, guildId: string) {
  return useQuery({
    queryKey: guildSettingsQueryKey(client.getBaseUrl(), guildId),
    queryFn: () => getGuildSettings(client, guildId),
    enabled: !!guildId,
  });
}

export function useUpdateGuildSettingsMutation(client: ControlApiClient, guildId: string) {
  const queryClient = useQueryClient();
  
  return useMutation({
    mutationFn: (args: { originalWorkspace?: GuildSettingsWorkspace; payload: { config_version: number; bot_instance_tokens?: Record<string, string>; main_bot_instance_id?: string; feature_routing?: Record<string, string>; roles?: GuildRolesSettingsSection; } }) => 
      updateGuildSettings(client, guildId, args.originalWorkspace, args.payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: guildSettingsQueryKey(client.getBaseUrl(), guildId) });
    },
  });
}
