import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useDashboardSession } from "../../context/DashboardSessionContext";
import {
  getTicketsConfig,
  updateTicketsConfig,
  getLiveTickets,
  getTranscriptsList,
  getTranscriptDetail,
  type TicketsFeatureConfig,
} from "../domains/tickets";

export function useTicketsConfig(guildId: string) {
  const { client } = useDashboardSession();
  return useQuery({
    queryKey: ["tickets-config", guildId],
    queryFn: () => getTicketsConfig(client, guildId),
    enabled: !!guildId,
  });
}

export function useUpdateTicketsConfig(guildId: string) {
  const { client } = useDashboardSession();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (payload: TicketsFeatureConfig) => updateTicketsConfig(client, guildId, payload),
    onSuccess: (data) => {
      queryClient.setQueryData(["tickets-config", guildId], data);
    },
  });
}

export function useLiveTickets(guildId: string) {
  const { client } = useDashboardSession();
  return useQuery({
    queryKey: ["live-tickets", guildId],
    queryFn: () => getLiveTickets(client, guildId),
    enabled: !!guildId,
  });
}

export function useTranscriptsList(guildId: string) {
  const { client } = useDashboardSession();
  return useQuery({
    queryKey: ["transcripts-list", guildId],
    queryFn: () => getTranscriptsList(client, guildId),
    enabled: !!guildId,
  });
}

export function useTranscriptDetail(guildId: string, transcriptId: string) {
  const { client } = useDashboardSession();
  return useQuery({
    queryKey: ["transcript-detail", guildId, transcriptId],
    queryFn: () => getTranscriptDetail(client, guildId, transcriptId),
    enabled: !!guildId && !!transcriptId,
  });
}
