import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import type { ControlApiClient } from "../client";
import { getPartnerBoard, setPartnerBoardTemplate, type PartnerBoardTemplateConfig } from "../domains/partners";

export const partnerBoardQueryKey = (baseUrl: string, guildId: string) => ["partnerBoard", baseUrl, guildId];

export function usePartnerBoardQuery(client: ControlApiClient, guildId: string) {
  return useQuery({
    queryKey: partnerBoardQueryKey(client.getBaseUrl(), guildId),
    queryFn: () => getPartnerBoard(client, guildId),
    enabled: !!guildId,
  });
}

export function useSetPartnerBoardTemplateMutation(client: ControlApiClient, guildId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (payload: PartnerBoardTemplateConfig) => setPartnerBoardTemplate(client, guildId, payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: partnerBoardQueryKey(client.getBaseUrl(), guildId) });
    },
  });
}
