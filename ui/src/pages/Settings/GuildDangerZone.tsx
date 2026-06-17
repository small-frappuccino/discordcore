import { useState } from "react";
import { SettingsGroup, SettingsRow, ActionTrigger } from "../../components/ui/tahoe";
import { ConfirmationModal, PageHeader } from "../../components/ui";
import { Stack } from "../../components/layout";
import { useDashboardSession } from "../../context/DashboardSessionContext";
import toast from "react-hot-toast";

export function GuildDangerZone({ guildId }: { guildId: string }) {
  const { client, manageableGuilds } = useDashboardSession();
  const guild = manageableGuilds.find(g => g.id === guildId);

  const [confirmModal, setConfirmModal] = useState<{
    isOpen: boolean;
    title: string;
    description: string;
    actionLabel: string;
    path: string;
    requireConfirmationText?: string;
  }>({
    isOpen: false,
    title: "",
    description: "",
    actionLabel: "",
    path: "",
    requireConfirmationText: undefined
  });

  const [isExecuting, setIsExecuting] = useState(false);
  const [confirmInput, setConfirmInput] = useState("");

  if (!guild) return null;

  const handleAction = (path: string, title: string, description: string, actionLabel: string, requireConfirmationText?: string) => {
    setConfirmModal({
      isOpen: true,
      title,
      description,
      actionLabel,
      path,
      requireConfirmationText
    });
  };

  const executeAction = async () => {
    if (confirmModal.requireConfirmationText && confirmInput !== confirmModal.requireConfirmationText) {
      toast.error("Confirmation text does not match.");
      return;
    }

    setIsExecuting(true);
    try {
      await client.request("DELETE", confirmModal.path);
      toast.success("Action completed successfully.");
    } catch (error: unknown) {
      toast.error(error instanceof Error ? error.message : "Failed to execute action.");
    } finally {
      setIsExecuting(false);
      setConfirmModal(prev => ({ ...prev, isOpen: false }));
      setConfirmInput("");
    }
  };

  return (
    <Stack spacing="xl" className="settings-form w-full max-w-none">
      <PageHeader>
        <PageHeader.TitleRow>
          <PageHeader.Title>Guild Settings</PageHeader.Title>
        </PageHeader.TitleRow>
        <PageHeader.Description>Manage destructive operations and data for {guild.name}.</PageHeader.Description>
      </PageHeader>

      <Stack spacing="sm">
        <h3 className="text-lg font-semibold tracking-tight text-text-primary">Danger Zone</h3>

        <SettingsGroup>
          <SettingsRow
            title="Purge Moderation Data"
            description="Deletes all stored warnings, automated moderation logs, and history for this guild."
            isMultiline
            control={
              <ActionTrigger 
                variant="danger" 
                onClick={() => handleAction(
                  `/v1/guilds/${guildId}/data/moderation`,
                  "Purge Moderation Data",
                  "Are you sure you want to delete all moderation history? This cannot be undone.",
                  "Purge Data"
                )}
              >
                Purge Moderation
              </ActionTrigger>
            }
          />
          <SettingsRow
            title="Purge QOTD Data"
            description="Deletes all queued questions, QOTD history, and settings for this guild."
            isMultiline
            control={
              <ActionTrigger 
                variant="danger" 
                onClick={() => handleAction(
                  `/v1/guilds/${guildId}/data/qotd`,
                  "Purge QOTD Data",
                  "Are you sure you want to delete all Question of the Day data? This cannot be undone.",
                  "Purge QOTD"
                )}
              >
                Purge QOTD
              </ActionTrigger>
            }
          />
          <SettingsRow
            title="Purge Engagement Metrics"
            description="Deletes all collected analytical engagement data for this guild."
            isMultiline
            control={
              <ActionTrigger 
                variant="danger" 
                onClick={() => handleAction(
                  `/v1/guilds/${guildId}/data/engagement`,
                  "Purge Engagement Data",
                  "Are you sure you want to delete all engagement metrics? This cannot be undone.",
                  "Purge Engagement"
                )}
              >
                Purge Engagement
              </ActionTrigger>
            }
          />
          <SettingsRow
            title="Purge Global Cache"
            description="Clears all cached settings and entities. Useful if the bot is failing to sync with Discord."
            isMultiline
            control={
              <ActionTrigger 
                variant="danger" 
                onClick={() => handleAction(
                  `/v1/guilds/${guildId}/data/cache`,
                  "Purge Global Cache",
                  "This will forcefully flush all cached entities for this guild. The cache will rebuild automatically.",
                  "Flush Cache"
                )}
              >
                Flush Cache
              </ActionTrigger>
            }
          />
          <div className="border-t border-status-danger/30 my-4 mx-4" />
          <SettingsRow
            title="Wipe Guild Completely"
            description="Factory reset. Deletes all configuration, profiles, and domains associated with this guild. The bot will leave the server."
            isMultiline
            control={
              <ActionTrigger 
                variant="danger" 
                onClick={() => handleAction(
                  `/v1/guilds/${guildId}/wipe`,
                  "Wipe Guild Completely",
                  `This is a catastrophic action. All data for ${guild.name} will be permanently destroyed. Please type the guild ID to confirm.`,
                  "Wipe Everything",
                  guildId
                )}
              >
                Wipe Everything
              </ActionTrigger>
            }
          />
        </SettingsGroup>
      </Stack>

      <ConfirmationModal
        isOpen={confirmModal.isOpen}
        onClose={() => {
          setConfirmModal(prev => ({ ...prev, isOpen: false }));
          setConfirmInput("");
        }}
        onConfirm={executeAction}
        title={confirmModal.title}
        description={
          <div>
            <p>{confirmModal.description}</p>
            {confirmModal.requireConfirmationText && (
              <div className="mt-4">
                <label className="block text-sm font-medium text-text-primary mb-2">
                  Type <strong>{confirmModal.requireConfirmationText}</strong> to confirm:
                </label>
                <input
                  type="text"
                  className="w-full bg-bg-surface-active border border-border-subtle rounded-md px-3 py-2 text-text-primary"
                  value={confirmInput}
                  onChange={(e) => setConfirmInput(e.target.value)}
                  placeholder={confirmModal.requireConfirmationText}
                />
              </div>
            )}
          </div>
        }
        confirmText={confirmModal.actionLabel}
        isConfirming={isExecuting}
      />
    </Stack>
  );
}
