import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { ControlApiClient, type PartnerBoardTemplateConfig } from "../control";

export const partnerBoardQueryKey = (baseUrl: string, guildId: string) => ["partnerBoard", baseUrl, guildId];

export function usePartnerBoardQuery(client: ControlApiClient, guildId: string) {
  return useQuery({
    queryKey: partnerBoardQueryKey(client.getBaseUrl(), guildId),
    queryFn: () => client.getPartnerBoard(guildId),
    enabled: !!guildId,
  });
}

export function useSetPartnerBoardTemplateMutation(client: ControlApiClient, guildId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (payload: PartnerBoardTemplateConfig) => client.setPartnerBoardTemplate(guildId, payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: partnerBoardQueryKey(client.getBaseUrl(), guildId) });
    },
  });
}
