import { useDashboardSession } from "../../context/DashboardSessionContext";
import { useCurrentGuild } from "../../context/GuildContext";
import { useGuildSettingsQuery, useUpdateGuildSettingsMutation } from "../../api/hooks/useGuildSettings";
import { useState } from "react";

export function useCorePageLogic() {
  const { client } = useDashboardSession();
  const { guildId: selectedGuildID } = useCurrentGuild();
  
  const { data: settings, isLoading } = useGuildSettingsQuery(client, selectedGuildID);
  const { mutateAsync: updateSettings } = useUpdateGuildSettingsMutation(client, selectedGuildID);
  
  const [tokensState, setTokensState] = useState<Record<string, string>>({});
  const [isSaving, setIsSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);

  const handleUpdateTokens = async () => {
    if (Object.keys(tokensState).length === 0) return;
    setIsSaving(true);
    setSaveError(null);
    try {
      await updateSettings({ bot_instance_tokens: tokensState });
      setTokensState({});
    } catch (err) {
      const message = err instanceof Error ? err.message : "Failed to save tokens";
      setSaveError(message);
    } finally {
      setIsSaving(false);
    }
  };

  return {
    settings,
    isLoading,
    tokensState,
    setTokensState,
    handleUpdateTokens,
    isSaving,
    saveError,
    clearSaveError: () => setSaveError(null),
  };
}
