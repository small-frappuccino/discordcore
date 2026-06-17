import { useEffect, useMemo } from "react";
import { useForm, Controller } from "react-hook-form";
import toast from "react-hot-toast";
import { formatError } from "../app/utils";
import {
  PageHeader,
  Badge,
  PageContainer,
  SettingsGroupSkeleton,
  FormProvider,
} from "../components/ui";
import {
  SettingsGroup,
  SettingsRow,
  SaveActionBar,
  SelectMenu
} from "../components/ui/tahoe";
import { Stack } from "../components/layout";
import { useDashboardSession } from "../context/DashboardSessionContext";
import { useCurrentGuild } from "../context/GuildContext";
import { useGuildSettingsQuery, useUpdateGuildSettingsMutation } from "../api/hooks/useGuildSettings";
import { useGuildChannelOptionsQuery } from "../api/hooks/useChannels";
import type { ChannelsConfig } from "../api/config_types";

export function LoggingPage() {
  const { client } = useDashboardSession();
  const { guildId: selectedGuildID } = useCurrentGuild();
  
  const { data: settingsRes, isLoading: settingsLoading } = useGuildSettingsQuery(client, selectedGuildID);
  const { data: channelsRes, isLoading: channelsLoading } = useGuildChannelOptionsQuery(client, selectedGuildID);
  const settingsMutation = useUpdateGuildSettingsMutation(client, selectedGuildID);

  const selectOptions = useMemo(() => {
    const channels = channelsRes?.channels || [];
    return [{ value: "", label: "-- None --" }, ...channels.map(c => ({ value: c.id, label: c.name }))];
  }, [channelsRes]);

  const isLoading = settingsLoading || channelsLoading;

  const form = useForm<ChannelsConfig>({
    defaultValues: {
      avatar_logging: "",
      role_update: "",
      member_join: "",
      member_leave: "",
      message_edit: "",
      message_delete: "",
      automod_action: "",
      moderation_case: "",
      clean_action: "",
      entry_backfill: "",
    }
  });

  useEffect(() => {
    if (settingsRes?.workspace?.sections?.channels) {
      form.reset({
        avatar_logging: settingsRes.workspace.sections.channels.avatar_logging || "",
        role_update: settingsRes.workspace.sections.channels.role_update || "",
        member_join: settingsRes.workspace.sections.channels.member_join || "",
        member_leave: settingsRes.workspace.sections.channels.member_leave || "",
        message_edit: settingsRes.workspace.sections.channels.message_edit || "",
        message_delete: settingsRes.workspace.sections.channels.message_delete || "",
        automod_action: settingsRes.workspace.sections.channels.automod_action || "",
        moderation_case: settingsRes.workspace.sections.channels.moderation_case || "",
        clean_action: settingsRes.workspace.sections.channels.clean_action || "",
        entry_backfill: settingsRes.workspace.sections.channels.entry_backfill || "",
      });
    }
  }, [settingsRes, form]);

  const isSaving = settingsMutation.isPending;

  const onSubmit = form.handleSubmit((data) => {
    if (!selectedGuildID) return;
    settingsMutation.mutate({
      originalWorkspace: settingsRes?.workspace,
      payload: {
        config_version: settingsRes?.workspace?.config_version ?? 0,
        channels: {
          ...(settingsRes?.workspace?.sections?.channels || {}),
          avatar_logging: data.avatar_logging || "",
          role_update: data.role_update || "",
          member_join: data.member_join || "",
          member_leave: data.member_leave || "",
          message_edit: data.message_edit || "",
          message_delete: data.message_delete || "",
          automod_action: data.automod_action || "",
          moderation_case: data.moderation_case || "",
          clean_action: data.clean_action || "",
          entry_backfill: data.entry_backfill || "",
        },
      }
    }, {
      onSuccess: () => toast.success("Logging settings saved"),
      onError: (e) => toast.error(`Failed to save logging settings: ${formatError(e)}`)
    });
  });

  if (!selectedGuildID) {
    return <div>Select a server to manage logging.</div>;
  }

  return (
    <>
      {isLoading ? (
        <PageContainer>
          <div className="settings-form">
            <Stack spacing="xl">
              <PageHeader>
                <PageHeader.TitleRow>
                  <PageHeader.Title>Logging</PageHeader.Title>
                  <Badge variant="neutral">Loading</Badge>
                </PageHeader.TitleRow>
                <PageHeader.Description>Configure channels to receive audit logs. A log is automatically enabled when its channel is populated.</PageHeader.Description>
              </PageHeader>
              <Stack spacing="xl">
                <SettingsGroupSkeleton rows={4} />
                <SettingsGroupSkeleton rows={3} />
              </Stack>
            </Stack>
          </div>
        </PageContainer>
      ) : (
      <PageContainer>
        <form className="settings-form" onSubmit={onSubmit}>
          <Stack spacing="xl">
            <PageHeader>
              <PageHeader.TitleRow>
                <PageHeader.Title>Logging</PageHeader.Title>
                <Badge variant="success">Active</Badge>
              </PageHeader.TitleRow>
              <PageHeader.Description>Configure channels to receive audit logs. A log is automatically enabled when its channel is populated.</PageHeader.Description>
            </PageHeader>

            <fieldset disabled={isSaving} className="border-none p-0 m-0 min-w-0">
              <FormProvider {...form}>
                <Stack spacing="lg">
                  <Stack spacing="sm">
                    <h3 className="text-lg font-semibold tracking-tight text-text-primary">User Activity</h3>
                    <SettingsGroup>
                      <SettingsRow
                        title="Avatar & Username Updates"
                        description="Logs when users change their avatar, username, or discriminator."
                        control={
                          <Controller
                            name="avatar_logging"
                            control={form.control}
                            render={({ field }) => (
                              <SelectMenu
                                options={selectOptions}
                                value={field.value || ""}
                                onChange={field.onChange}
                                placeholder="Select a channel..."
                              />
                            )}
                          />
                        }
                      />
                      <SettingsRow
                        title="Role Updates"
                        description="Logs when roles are added or removed from a member."
                        control={
                          <Controller
                            name="role_update"
                            control={form.control}
                            render={({ field }) => (
                              <SelectMenu
                                options={selectOptions}
                                value={field.value || ""}
                                onChange={field.onChange}
                                placeholder="Select a channel..."
                              />
                            )}
                          />
                        }
                      />
                      <SettingsRow
                        title="Member Join"
                        description="Logs when a user joins the server."
                        control={
                          <Controller
                            name="member_join"
                            control={form.control}
                            render={({ field }) => (
                              <SelectMenu
                                options={selectOptions}
                                value={field.value || ""}
                                onChange={field.onChange}
                                placeholder="Select a channel..."
                              />
                            )}
                          />
                        }
                      />
                      <SettingsRow
                        title="Member Leave"
                        description="Logs when a user leaves or is kicked from the server."
                        control={
                          <Controller
                            name="member_leave"
                            control={form.control}
                            render={({ field }) => (
                              <SelectMenu
                                options={selectOptions}
                                value={field.value || ""}
                                onChange={field.onChange}
                                placeholder="Select a channel..."
                              />
                            )}
                          />
                        }
                      />
                    </SettingsGroup>
                  </Stack>

                  <Stack spacing="sm">
                    <h3 className="text-lg font-semibold tracking-tight text-text-primary">Message Activity</h3>
                    <SettingsGroup>
                      <SettingsRow
                        title="Message Edits"
                        description="Logs when a message is edited (requires caching)."
                        control={
                          <Controller
                            name="message_edit"
                            control={form.control}
                            render={({ field }) => (
                              <SelectMenu
                                options={selectOptions}
                                value={field.value || ""}
                                onChange={field.onChange}
                                placeholder="Select a channel..."
                              />
                            )}
                          />
                        }
                      />
                      <SettingsRow
                        title="Message Deletions"
                        description="Logs when a message is deleted."
                        control={
                          <Controller
                            name="message_delete"
                            control={form.control}
                            render={({ field }) => (
                              <SelectMenu
                                options={selectOptions}
                                value={field.value || ""}
                                onChange={field.onChange}
                                placeholder="Select a channel..."
                              />
                            )}
                          />
                        }
                      />
                    </SettingsGroup>
                  </Stack>

                  <Stack spacing="sm">
                    <h3 className="text-lg font-semibold tracking-tight text-text-primary">Moderation Activity</h3>
                    <SettingsGroup>
                      <SettingsRow
                        title="Moderation Cases"
                        description="Logs formal moderation actions like bans, timeouts, and warnings."
                        control={
                          <Controller
                            name="moderation_case"
                            control={form.control}
                            render={({ field }) => (
                              <SelectMenu
                                options={selectOptions}
                                value={field.value || ""}
                                onChange={field.onChange}
                                placeholder="Select a channel..."
                              />
                            )}
                          />
                        }
                      />
                      <SettingsRow
                        title="AutoMod Actions"
                        description="Logs when AutoMod intercepts a message or triggers an alert."
                        control={
                          <Controller
                            name="automod_action"
                            control={form.control}
                            render={({ field }) => (
                              <SelectMenu
                                options={selectOptions}
                                value={field.value || ""}
                                onChange={field.onChange}
                                placeholder="Select a channel..."
                              />
                            )}
                          />
                        }
                      />
                      <SettingsRow
                        title="Clean Actions"
                        description="Logs bulk message deletions performed via the /clean command."
                        control={
                          <Controller
                            name="clean_action"
                            control={form.control}
                            render={({ field }) => (
                              <SelectMenu
                                options={selectOptions}
                                value={field.value || ""}
                                onChange={field.onChange}
                                placeholder="Select a channel..."
                              />
                            )}
                          />
                        }
                      />
                    </SettingsGroup>
                  </Stack>
                </Stack>
              </FormProvider>
            </fieldset>
          </Stack>
        </form>
        <SaveActionBar
          isDirty={form.formState.isDirty}
          isSaving={isSaving}
          onSave={onSubmit}
          onReset={() => form.reset()}
        />
      </PageContainer>
      )}
    </>
  );
}
