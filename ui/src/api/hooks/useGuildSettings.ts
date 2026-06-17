import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import type { ControlApiClient } from "../client";
import { getGuildSettings, updateGuildSettings, type GuildSettingsWorkspace, type GuildSettingsWorkspaceResponse } from "../domains/guilds";
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
    mutationFn: (args: { originalWorkspace?: GuildSettingsWorkspace; payload: { config_version: number; bot_instance_tokens?: Record<string, string>; bot_instance_statuses?: Record<string, string>; feature_routing?: Record<string, string>; roles?: GuildRolesSettingsSection; channels?: Record<string, string>; } }) => 
      updateGuildSettings(client, guildId, args.originalWorkspace, args.payload),
    onMutate: async (args) => {
      const qKey = guildSettingsQueryKey(client.getBaseUrl(), guildId);
      await queryClient.cancelQueries({ queryKey: qKey });
      const previousSettings = queryClient.getQueryData<GuildSettingsWorkspaceResponse>(qKey);
      
      if (previousSettings) {
        const sections = { ...previousSettings.workspace.sections };
        if (args.payload.roles !== undefined) sections.roles = args.payload.roles;
        if (args.payload.channels !== undefined) sections.channels = args.payload.channels;
        if (args.payload.feature_routing !== undefined) sections.feature_routing = args.payload.feature_routing;
        if (args.payload.bot_instance_statuses !== undefined) sections.bot_instance_statuses = args.payload.bot_instance_statuses;
        if (args.payload.bot_instance_tokens !== undefined) {
          sections.bot_instance_tokens_configured = { ...sections.bot_instance_tokens_configured };
          for (const [k, v] of Object.entries(args.payload.bot_instance_tokens)) {
            if (v === "") delete sections.bot_instance_tokens_configured[k];
            else sections.bot_instance_tokens_configured[k] = true;
          }
        }
        
        queryClient.setQueryData<GuildSettingsWorkspaceResponse>(qKey, {
          ...previousSettings,
          workspace: {
            ...previousSettings.workspace,
            sections,
          }
        });
      }
      
      return { previousSettings };
    },
    onError: (_err, _variables, context) => {
      if (context?.previousSettings) {
        queryClient.setQueryData(guildSettingsQueryKey(client.getBaseUrl(), guildId), context.previousSettings);
      }
    },
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: guildSettingsQueryKey(client.getBaseUrl(), guildId) });
      queryClient.invalidateQueries({ queryKey: ["botProfiles", client.getBaseUrl(), guildId] });
    },
  });
}
