import { useEffect, useState } from "react";
import { useDashboardSession } from "../../context/DashboardSessionContext";
import { useCurrentGuild } from "../../context/GuildContext";
import { useGuildFeatureQuery, usePatchGuildFeatureMutation } from "../../api/hooks/useGuildFeatures";
import { useGuildSettingsQuery, useUpdateGuildSettingsMutation } from "../../api/hooks/useGuildSettings";

export function useModerationPageLogic() {
  const { client } = useDashboardSession();
  const { guildId: selectedGuildID } = useCurrentGuild();
  
  const { data: automodRes, isLoading: automodLoading } = useGuildFeatureQuery(client, selectedGuildID, "automod");
  const { data: loggingRes, isLoading: loggingLoading } = useGuildFeatureQuery(client, selectedGuildID, "logging");
  const { data: settingsRes, isLoading: settingsLoading } = useGuildSettingsQuery(client, selectedGuildID);

  const automodMutation = usePatchGuildFeatureMutation(client, selectedGuildID, "automod");
  const loggingMutation = usePatchGuildFeatureMutation(client, selectedGuildID, "logging");
  const settingsMutation = useUpdateGuildSettingsMutation(client, selectedGuildID);

  const [muteRole, setMuteRole] = useState("");

  useEffect(() => {
    if (settingsRes?.workspace?.sections?.roles) {
      setMuteRole(settingsRes.workspace.sections.roles.mute_role || "");
    }
  }, [settingsRes]);

  const isLoading = automodLoading || loggingLoading || settingsLoading;
  const automodEnabled = automodRes?.feature?.effective_enabled || false;
  const loggingEnabled = loggingRes?.feature?.effective_enabled || false;

  const handleToggleAutomod = () => {
    if (!selectedGuildID) return;
    automodMutation.mutate({ enabled: !automodEnabled });
  };

  const handleToggleLogging = () => {
    if (!selectedGuildID) return;
    loggingMutation.mutate({ enabled: !loggingEnabled });
  };

  const handleSaveMuteRole = () => {
    if (!selectedGuildID) return;
    settingsMutation.mutate({
      roles: {
        mute_role: muteRole,
      },
    }, {
      onSuccess: () => alert("Mute role saved")
    });
  };

  return {
    selectedGuildID,
    isLoading,
    automodEnabled,
    loggingEnabled,
    muteRole,
    setMuteRole,
    handleToggleAutomod,
    handleToggleLogging,
    handleSaveMuteRole,
  };
}
