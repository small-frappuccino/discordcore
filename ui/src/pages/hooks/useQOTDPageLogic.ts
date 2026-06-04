import { useEffect, useState } from "react";
import { useDashboardSession } from "../../context/DashboardSessionContext";
import { useCurrentGuild } from "../../context/GuildContext";
import type { QOTDConfig } from "../../api/control";
import { useQOTDSettingsQuery, useUpdateQOTDSettingsMutation } from "../../api/hooks/useQOTD";

export function useQOTDPageLogic() {
  const { client } = useDashboardSession();
  const { guildId: selectedGuildID } = useCurrentGuild();
  
  const { data: queryRes, isLoading: settingsLoading } = useQOTDSettingsQuery(client, selectedGuildID);
  const updateMutation = useUpdateQOTDSettingsMutation(client, selectedGuildID);
  
  const [config, setConfig] = useState<QOTDConfig | null>(null);

  useEffect(() => {
    if (queryRes?.settings) {
      setConfig(queryRes.settings);
    }
  }, [queryRes]);

  const handleSave = async () => {
    if (!selectedGuildID || !config) return;
    updateMutation.mutate(config, {
      onSuccess: () => alert("QOTD Settings saved!"),
      onError: (e) => {
        console.error(e);
        alert("Failed to save QOTD Settings");
      }
    });
  };

  const isLoading = settingsLoading;
  const isSaving = updateMutation.isPending;

  const activeDeck = config?.decks?.find(d => d.id === config.active_deck_id);

  return {
    selectedGuildID,
    config,
    setConfig,
    activeDeck,
    isLoading,
    isSaving,
    handleSave,
  };
}
