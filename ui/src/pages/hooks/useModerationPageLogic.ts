import { useEffect, useCallback } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import toast from "react-hot-toast";
import { formatError } from "../../app/utils";
import { useDashboardSession } from "../../context/DashboardSessionContext";
import { useCurrentGuild } from "../../context/GuildContext";
import { useGuildFeatureQuery, usePatchGuildFeatureMutation } from "../../api/hooks/useGuildFeatures";
import { useGuildSettingsQuery, useUpdateGuildSettingsMutation } from "../../api/hooks/useGuildSettings";
import { ModerationSchema, type ModerationFormData } from "../schemas/moderation";

export function useModerationPageLogic() {
  const { client } = useDashboardSession();
  const { guildId: selectedGuildID } = useCurrentGuild();
  
  const { data: automodRes, isLoading: automodLoading } = useGuildFeatureQuery(client, selectedGuildID, "automod");
  const { data: loggingRes, isLoading: loggingLoading } = useGuildFeatureQuery(client, selectedGuildID, "logging");
  const { data: settingsRes, isLoading: settingsLoading } = useGuildSettingsQuery(client, selectedGuildID);

  const automodMutation = usePatchGuildFeatureMutation(client, selectedGuildID, "automod");
  const loggingMutation = usePatchGuildFeatureMutation(client, selectedGuildID, "logging");
  const settingsMutation = useUpdateGuildSettingsMutation(client, selectedGuildID);

  const form = useForm<ModerationFormData>({
    resolver: zodResolver(ModerationSchema),
    defaultValues: {
      mute_role: ""
    }
  });

  useEffect(() => {
    if (settingsRes?.workspace?.sections?.roles) {
      form.reset({
        mute_role: settingsRes.workspace.sections.roles.mute_role || ""
      });
    }
  }, [settingsRes, form]);

  const isLoading = automodLoading || loggingLoading || settingsLoading;
  const isSaving = settingsMutation.isPending;
  const automodEnabled = automodRes?.feature?.effective_enabled || false;
  const loggingEnabled = loggingRes?.feature?.effective_enabled || false;

  const handleToggleAutomod = useCallback(() => {
    if (!selectedGuildID) return;
    automodMutation.mutate({ originalFeature: automodRes?.feature, payload: { enabled: !automodEnabled } }, {
      onSuccess: () => toast.success("Automod settings updated"),
      onError: (e) => toast.error(`Failed to update automod: ${formatError(e)}`)
    });
  }, [selectedGuildID, automodEnabled, automodMutation, automodRes]);

  const handleToggleLogging = useCallback(() => {
    if (!selectedGuildID) return;
    loggingMutation.mutate({ originalFeature: loggingRes?.feature, payload: { enabled: !loggingEnabled } }, {
      onSuccess: () => toast.success("Logging settings updated"),
      onError: (e) => toast.error(`Failed to update logging: ${formatError(e)}`)
    });
  }, [selectedGuildID, loggingEnabled, loggingMutation, loggingRes]);

  const onSubmit = form.handleSubmit((data) => {
    if (!selectedGuildID) return;
    settingsMutation.mutate({
      originalWorkspace: settingsRes?.workspace,
      payload: {
        config_version: settingsRes?.workspace?.config_version ?? 0,
        roles: {
          ...(settingsRes?.workspace?.sections?.roles || {}),
          mute_role: data.mute_role,
        },
      }
    }, {
      onSuccess: () => toast.success("Mute role saved"),
      onError: (e) => toast.error(`Failed to save mute role: ${formatError(e)}`)
    });
  });

  return {
    selectedGuildID,
    isLoading,
    isSaving,
    automodEnabled,
    loggingEnabled,
    form,
    onSubmit,
    handleToggleAutomod,
    handleToggleLogging,
  };
}
