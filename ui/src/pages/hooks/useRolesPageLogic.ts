import { useEffect, useState } from "react";
import { useDashboardSession } from "../../context/DashboardSessionContext";
import { useCurrentGuild } from "../../context/GuildContext";
import { useGuildRoleOptionsQuery } from "../../api/hooks/useRoles";
import { useGuildSettingsQuery, useUpdateGuildSettingsMutation } from "../../api/hooks/useGuildSettings";

export function useRolesPageLogic() {
  const { client } = useDashboardSession();
  const { guildId: selectedGuildID } = useCurrentGuild();
  
  const { data: optsRes, isLoading: rolesLoading } = useGuildRoleOptionsQuery(client, selectedGuildID);
  const { data: setRes, isLoading: settingsLoading } = useGuildSettingsQuery(client, selectedGuildID);
  const updateMutation = useUpdateGuildSettingsMutation(client, selectedGuildID);

  const roles = optsRes?.roles || [];

  // Local form state
  const [dashboardRead, setDashboardRead] = useState<string[]>([]);
  const [dashboardWrite, setDashboardWrite] = useState<string[]>([]);
  const [boosterRole, setBoosterRole] = useState<string>("");
  const [muteRole, setMuteRole] = useState<string>("");
  
  const [autoAssignEnabled, setAutoAssignEnabled] = useState(false);
  const [autoAssignTarget, setAutoAssignTarget] = useState<string>("");
  const [autoAssignRequired, setAutoAssignRequired] = useState<string[]>([]);

  useEffect(() => {
    if (setRes) {
      const rs = setRes.workspace.sections.roles || {};
      setDashboardRead(rs.dashboard_read || []);
      setDashboardWrite(rs.dashboard_write || []);
      setBoosterRole(rs.booster_role || "");
      setMuteRole(rs.mute_role || "");
      
      const aa = rs.auto_assignment || {};
      setAutoAssignEnabled(aa.enabled || false);
      setAutoAssignTarget(aa.target_role || "");
      setAutoAssignRequired(aa.required_roles || []);
    }
  }, [setRes]);

  async function handleSave() {
    if (!selectedGuildID) return;
    
    updateMutation.mutate(
      {
        roles: {
          dashboard_read: dashboardRead,
          dashboard_write: dashboardWrite,
          booster_role: boosterRole,
          mute_role: muteRole,
          auto_assignment: {
            enabled: autoAssignEnabled,
            target_role: autoAssignTarget,
            required_roles: autoAssignRequired
          }
        }
      },
      {
        onSuccess: () => alert("Settings saved!"),
        onError: (e) => {
          console.error(e);
          alert("Failed to save settings");
        }
      }
    );
  }

  const isLoading = rolesLoading || settingsLoading;
  const isSaving = updateMutation.isPending;

  return {
    selectedGuildID,
    isLoading,
    isSaving,
    roles,
    dashboardRead, setDashboardRead,
    dashboardWrite, setDashboardWrite,
    boosterRole, setBoosterRole,
    muteRole, setMuteRole,
    autoAssignEnabled, setAutoAssignEnabled,
    autoAssignTarget, setAutoAssignTarget,
    autoAssignRequired, setAutoAssignRequired,
    handleSave,
  };
}
