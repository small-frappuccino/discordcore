import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useDashboardSession } from "../../context/DashboardSessionContext";
import { useCurrentGuild } from "../../context/GuildContext";
import { useQOTDSettingsQuery, useUpdateQOTDSettingsMutation } from "../../api/hooks/useQOTD";
import { QOTDSchema, type QOTDFormData } from "../schemas/qotd";

export function useQOTDPageLogic() {
  const { client } = useDashboardSession();
  const { guildId: selectedGuildID } = useCurrentGuild();
  
  const { data: queryRes, isLoading: settingsLoading } = useQOTDSettingsQuery(client, selectedGuildID);
  const updateMutation = useUpdateQOTDSettingsMutation(client, selectedGuildID);
  
  const form = useForm<QOTDFormData>({
    resolver: zodResolver(QOTDSchema),
    defaultValues: {
      verified_role_id: "",
      active_deck_id: "",
      schedule: {
        hour_utc: 0,
        minute_utc: 0,
      }
    }
  });

  useEffect(() => {
    if (queryRes?.settings) {
      const s = queryRes.settings;
      form.reset({
        verified_role_id: s.verified_role_id || "",
        active_deck_id: s.active_deck_id || "",
        schedule: {
          hour_utc: s.schedule?.hour_utc || 0,
          minute_utc: s.schedule?.minute_utc || 0,
        }
      });
    }
  }, [queryRes, form]);

  const onSubmit = form.handleSubmit((data) => {
    if (!selectedGuildID) return;
    
    updateMutation.mutate(data, {
      onSuccess: () => alert("QOTD Settings saved!"),
      onError: (e) => {
        console.error(e);
        alert("Failed to save QOTD Settings");
      }
    });
  });

  const isLoading = settingsLoading;
  const isSaving = updateMutation.isPending;

  const config = queryRes?.settings;
  const activeDeck = config?.decks?.find(d => d.id === config.active_deck_id);

  return {
    selectedGuildID,
    config,
    form,
    onSubmit,
    activeDeck,
    isLoading,
    isSaving,
  };
}
