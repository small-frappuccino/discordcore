import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { ControlApiClient, type QOTDConfig } from "../control";

export const qotdSummaryQueryKey = (baseUrl: string, guildId: string) => ["qotdSummary", baseUrl, guildId];
export const qotdSettingsQueryKey = (baseUrl: string, guildId: string) => ["qotdSettings", baseUrl, guildId];

export function useQOTDSummaryQuery(client: ControlApiClient, guildId: string) {
  return useQuery({
    queryKey: qotdSummaryQueryKey(client.getBaseUrl(), guildId),
    queryFn: () => client.getQOTDSummary(guildId),
    enabled: !!guildId,
  });
}

export function useQOTDSettingsQuery(client: ControlApiClient, guildId: string) {
  return useQuery({
    queryKey: qotdSettingsQueryKey(client.getBaseUrl(), guildId),
    queryFn: () => client.getQOTDSettings(guildId),
    enabled: !!guildId,
  });
}

export function useUpdateQOTDSettingsMutation(client: ControlApiClient, guildId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (payload: QOTDConfig) => client.updateQOTDSettings(guildId, payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: qotdSettingsQueryKey(client.getBaseUrl(), guildId) });
      queryClient.invalidateQueries({ queryKey: qotdSummaryQueryKey(client.getBaseUrl(), guildId) });
    },
  });
}
