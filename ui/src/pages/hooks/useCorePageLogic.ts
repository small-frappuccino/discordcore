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
    refetchInterval: 5000,
  });

  const [tokensState, setTokensState] = useState<Record<string, string>>({});
  const [statusesState, setStatusesState] = useState<Record<string, string>>({});
  const [featureRoutingState, setFeatureRoutingState] = useState<Record<string, string>>({});
  
  const [isSaving, setIsSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);

  useEffect(() => {
    if (settings) {
      setFeatureRoutingState(settings.workspace.sections.feature_routing || {});
      setStatusesState(settings.workspace.sections.bot_instance_statuses || {});
    }
  }, [settings]);

  const handleUpdateTokens = async () => {
    setIsSaving(true);
    setSaveError(null);
    try {
      const payload: {
        config_version: number;
        feature_routing?: Record<string, string>;
        bot_instance_tokens?: Record<string, string>;
        bot_instance_statuses?: Record<string, string>;
      } = {
        config_version: settings?.workspace.config_version ?? 0,
        feature_routing: featureRoutingState,
        bot_instance_statuses: statusesState,
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

  const handleDeleteProfile = async (instanceId: string) => {
    setIsSaving(true);
    setSaveError(null);
    try {
      const currentRouting = settings?.workspace.sections.feature_routing || {};
      const nextRouting = { ...currentRouting };
      for (const key of Object.keys(nextRouting)) {
        if (nextRouting[key] === instanceId) {
          delete nextRouting[key];
        }
      }
      
      const payload = {
        config_version: settings?.workspace.config_version ?? 0,
        feature_routing: nextRouting,
        bot_instance_tokens: { [instanceId]: "" },
      };
      
      await updateSettings({ originalWorkspace: settings?.workspace, payload });
      
      // Clean up local states for this instance
      setTokensState(prev => { const next = {...prev}; delete next[instanceId]; return next; });
      setStatusesState(prev => { const next = {...prev}; delete next[instanceId]; return next; });
      setFeatureRoutingState(prev => {
        const next = { ...prev };
        for (const key of Object.keys(next)) {
          if (next[key] === instanceId) delete next[key];
        }
        return next;
      });
    } catch (err) {
      const e = err as { status?: number; response?: { status?: number } };
      if (e?.response?.status === 412 || e?.status === 412) {
        setSaveError("Another session modified the configuration. Please refresh and try again. (Lost Update)");
      } else {
        const message = err instanceof Error ? err.message : "Failed to delete profile";
        setSaveError(message);
      }
      throw err;
    } finally {
      setIsSaving(false);
    }
  };

  const handleResetTokens = () => {
    setTokensState({});
    if (settings) {
      setFeatureRoutingState(settings.workspace.sections.feature_routing || {});
      setStatusesState(settings.workspace.sections.bot_instance_statuses || {});
    }
    setSaveError(null);
  };

  return {
    settings,
    botProfiles,
    isLoading: isLoading || isProfilesLoading,
    tokensState,
    setTokensState,
    statusesState,
    setStatusesState,
    featureRoutingState,
    setFeatureRoutingState,
    handleUpdateTokens,
    handleDeleteProfile,
    handleResetTokens,
    isSaving,
    saveError,
    clearSaveError: () => setSaveError(null),
    isDirty: Object.keys(tokensState).length > 0 || 
             JSON.stringify(featureRoutingState) !== JSON.stringify(settings?.workspace?.sections?.feature_routing || {}) ||
             JSON.stringify(statusesState) !== JSON.stringify(settings?.workspace?.sections?.bot_instance_statuses || {}),
  };
}

