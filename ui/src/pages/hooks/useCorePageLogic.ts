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

  const handleUpdateTokens = async () => {
    if (Object.keys(tokensState).length === 0) return;
    await updateSettings({ bot_instance_tokens: tokensState });
    setTokensState({});
  };

  return {
    settings,
    isLoading,
    tokensState,
    setTokensState,
    handleUpdateTokens,
  };
}
