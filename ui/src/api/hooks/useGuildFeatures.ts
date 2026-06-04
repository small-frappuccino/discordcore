import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { ControlApiClient, type FeaturePatchPayload } from "../control";

export const guildFeaturesQueryKey = (baseUrl: string, guildId: string) => ["guildFeatures", baseUrl, guildId];
export const guildFeatureQueryKey = (baseUrl: string, guildId: string, featureId: string) => ["guildFeature", baseUrl, guildId, featureId];

export function useGuildFeaturesQuery(client: ControlApiClient, guildId: string) {
  return useQuery({
    queryKey: guildFeaturesQueryKey(client.getBaseUrl(), guildId),
    queryFn: () => client.listGuildFeatures(guildId),
    enabled: !!guildId,
  });
}

export function useGuildFeatureQuery(client: ControlApiClient, guildId: string, featureId: string) {
  return useQuery({
    queryKey: guildFeatureQueryKey(client.getBaseUrl(), guildId, featureId),
    queryFn: () => client.getGuildFeature(guildId, featureId),
    enabled: !!guildId && !!featureId,
  });
}

export function usePatchGuildFeatureMutation(client: ControlApiClient, guildId: string, featureId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (payload: FeaturePatchPayload) => client.patchGuildFeature(guildId, featureId, payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: guildFeatureQueryKey(client.getBaseUrl(), guildId, featureId) });
      queryClient.invalidateQueries({ queryKey: guildFeaturesQueryKey(client.getBaseUrl(), guildId) });
    },
  });
}
