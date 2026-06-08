import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import toast from "react-hot-toast";
import { formatError } from "../../app/utils";
import { useDashboardSession } from "../../context/DashboardSessionContext";
import { useCurrentGuild } from "../../context/GuildContext";
import { useGuildRoleOptionsQuery } from "../../api/hooks/useRoles";
import { useGuildSettingsQuery, useUpdateGuildSettingsMutation } from "../../api/hooks/useGuildSettings";
import { RolesSchema, type RolesFormData } from "../schemas/roles";

export function useRolesPageLogic() {
  const { client } = useDashboardSession();
  const { guildId: selectedGuildID } = useCurrentGuild();
  
  const { data: optsRes, isLoading: rolesLoading } = useGuildRoleOptionsQuery(client, selectedGuildID);
  const { data: setRes, isLoading: settingsLoading } = useGuildSettingsQuery(client, selectedGuildID);
  const updateMutation = useUpdateGuildSettingsMutation(client, selectedGuildID);

  const roles = optsRes?.roles || [];

  const form = useForm<RolesFormData>({
    resolver: zodResolver(RolesSchema),
    defaultValues: {
      dashboard_read: [],
      dashboard_write: [],
      booster_role: "",
      mute_role: "",
      auto_assignment: {
        enabled: false,
        target_role: "",
        required_roles: [],
      }
    }
  });

  useEffect(() => {
    if (setRes) {
      const rs = setRes.workspace.sections.roles || {};
      const aa = rs.auto_assignment || {};
      form.reset({
        dashboard_read: rs.dashboard_read || [],
        dashboard_write: rs.dashboard_write || [],
        booster_role: rs.booster_role || "",
        mute_role: rs.mute_role || "",
        auto_assignment: {
          enabled: aa.enabled || false,
          target_role: aa.target_role || "",
          required_roles: aa.required_roles || [],
        }
      });
    }
  }, [setRes, form]);

  const onSubmit = form.handleSubmit((data) => {
    if (!selectedGuildID) return;
    
    updateMutation.mutate(
      {
        config_version: setRes?.workspace.config_version ?? 0,
        roles: data
      },
      {
        onSuccess: () => toast.success("Settings saved!"),
        onError: (e) => toast.error(`Failed to save settings: ${formatError(e)}`)
      }
    );
  });

  const isLoading = rolesLoading || settingsLoading;
  const isSaving = updateMutation.isPending;

  return {
    selectedGuildID,
    isLoading,
    isSaving,
    roles,
    form,
    onSubmit,
  };
}
