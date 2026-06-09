import { useDashboardSession } from "../../context/DashboardSessionContext";
import { useCurrentGuild } from "../../context/GuildContext";
import { useGuildSettingsQuery, useUpdateGuildSettingsMutation } from "../../api/hooks/useGuildSettings";
import { useState, useEffect } from "react";
import { useQuery } from "@tanstack/react-query";
import { getBotProfiles } from "../../api/domains/guilds";

export function useCorePageLogic() {
  const { client } = useDashboardSession();
  const { guildId: selectedGuildID } = useCurrentGuild();
  
  const { data: settings, isLoading } = useGuildSettingsQuery(client, selectedGuildID);
  const { mutateAsync: updateSettings } = useUpdateGuildSettingsMutation(client, selectedGuildID);
  
  const { data: botProfiles, isLoading: isProfilesLoading } = useQuery({
    queryKey: ["botProfiles", client.getBaseUrl(), selectedGuildID],
    queryFn: () => getBotProfiles(client, selectedGuildID),
    enabled: !!selectedGuildID,
  });

  const [tokensState, setTokensState] = useState<Record<string, string>>({});
  const [mainBotIdState, setMainBotIdState] = useState<string | undefined>(undefined);
  const [featureRoutingState, setFeatureRoutingState] = useState<Record<string, string>>({});
  
  const [isSaving, setIsSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);

  useEffect(() => {
    if (settings) {
      setMainBotIdState(settings.workspace.sections.main_bot_instance_id);
      setFeatureRoutingState(settings.workspace.sections.feature_routing || {});
    }
  }, [settings]);

  const handleUpdateTokens = async () => {
    setIsSaving(true);
    setSaveError(null);
    try {
      const payload: {
        config_version: number;
        main_bot_instance_id?: string;
        feature_routing?: Record<string, string>;
        bot_instance_tokens?: Record<string, string>;
      } = {
        config_version: settings?.workspace.config_version ?? 0,
        main_bot_instance_id: mainBotIdState,
        feature_routing: featureRoutingState,
      };
      if (Object.keys(tokensState).length > 0) {
        payload.bot_instance_tokens = tokensState;
      }
      await updateSettings({ originalWorkspace: settings?.workspace, payload });
      setTokensState({});
    } catch (err) {
      const e = err as { status?: number; response?: { status?: number } };
      if (e?.response?.status === 412 || e?.status === 412) {
        setSaveError("Another session modified the configuration. Please refresh and try again. (Lost Update)");
      } else {
        const message = err instanceof Error ? err.message : "Failed to save settings";
        setSaveError(message);
      }
    } finally {
      setIsSaving(false);
    }
  };

  return {
    settings,
    botProfiles,
    isLoading: isLoading || isProfilesLoading,
    tokensState,
    setTokensState,
    mainBotIdState,
    setMainBotIdState,
    featureRoutingState,
    setFeatureRoutingState,
    handleUpdateTokens,
    isSaving,
    saveError,
    clearSaveError: () => setSaveError(null),
    isDirty: Object.keys(tokensState).length > 0 || 
             mainBotIdState !== settings?.workspace?.sections?.main_bot_instance_id ||
             JSON.stringify(featureRoutingState) !== JSON.stringify(settings?.workspace?.sections?.feature_routing || {}),
  };
}

