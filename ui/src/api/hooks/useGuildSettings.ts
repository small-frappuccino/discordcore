import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { ControlApiClient, type GuildBotRoutingSettingsSection, type GuildRolesSettingsSection } from "../control";

export const guildSettingsQueryKey = (baseUrl: string, guildId: string) => ["guildSettings", baseUrl, guildId];

export function useGuildSettingsQuery(client: ControlApiClient, guildId: string) {
  return useQuery({
    queryKey: guildSettingsQueryKey(client.getBaseUrl(), guildId),
    queryFn: () => client.getGuildSettings(guildId),
    enabled: !!guildId,
  });
}

export function useUpdateGuildSettingsMutation(client: ControlApiClient, guildId: string) {
  const queryClient = useQueryClient();
  
  return useMutation({
    mutationFn: (payload: { bot_instance_id?: string; bot_routing?: GuildBotRoutingSettingsSection; roles?: GuildRolesSettingsSection; }) => 
      client.updateGuildSettings(guildId, payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: guildSettingsQueryKey(client.getBaseUrl(), guildId) });
    },
  });
}
