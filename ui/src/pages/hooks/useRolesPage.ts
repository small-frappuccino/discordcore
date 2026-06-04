import { useState, useEffect } from "react";
import { useDashboardSession } from "../../context/DashboardSessionContext";
import { useGuildRoleOptionsQuery } from "../../api/hooks/useRoles";
import { useGuildSettingsQuery, useUpdateGuildSettingsMutation } from "../../api/hooks/useGuildSettings";

export function useRolesPage(selectedGuildID: string | null) {
  const { client } = useDashboardSession();

  const { data: rolesData, isLoading: loadingRoles } = useGuildRoleOptionsQuery(client, selectedGuildID || "");
  const { data: settingsData, isLoading: loadingSettings } = useGuildSettingsQuery(client, selectedGuildID || "");
  const updateSettingsMutation = useUpdateGuildSettingsMutation(client, selectedGuildID || "");

  const roles = rolesData?.roles || [];

  // Local form state
  const [dashboardRead, setDashboardRead] = useState<string[]>([]);
  const [dashboardWrite, setDashboardWrite] = useState<string[]>([]);
  const [boosterRole, setBoosterRole] = useState<string>("");
  const [muteRole, setMuteRole] = useState<string>("");
  const [autoAssignEnabled, setAutoAssignEnabled] = useState(false);
  const [autoAssignTarget, setAutoAssignTarget] = useState<string>("");
  const [autoAssignRequired, setAutoAssignRequired] = useState<string[]>([]);

  // Sync server state to local state when it loads
  useEffect(() => {
    if (settingsData) {
      const rs = settingsData.workspace.sections.roles || {};
      setDashboardRead(rs.dashboard_read || []);
      setDashboardWrite(rs.dashboard_write || []);
      setBoosterRole(rs.booster_role || "");
      setMuteRole(rs.mute_role || "");
      
      const aa = rs.auto_assignment || {};
      setAutoAssignEnabled(aa.enabled || false);
      setAutoAssignTarget(aa.target_role || "");
      setAutoAssignRequired(aa.required_roles || []);
    }
  }, [settingsData]);

  const handleSave = async () => {
    if (!selectedGuildID) return;
    try {
      await updateSettingsMutation.mutateAsync({
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
      });
      alert("Settings saved!");
    } catch (e) {
      console.error(e);
      alert("Failed to save settings");
    }
  };

  return {
    roles,
    loading: loadingRoles || loadingSettings,
    saving: updateSettingsMutation.isPending,
    formControls: {
      dashboardRead, setDashboardRead,
      dashboardWrite, setDashboardWrite,
      boosterRole, setBoosterRole,
      muteRole, setMuteRole,
      autoAssignEnabled, setAutoAssignEnabled,
      autoAssignTarget, setAutoAssignTarget,
      autoAssignRequired, setAutoAssignRequired,
    },
    handleSave,
  };
}
