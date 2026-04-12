import { useEffect, useMemo, useState } from "react";
import { useLocation } from "react-router-dom";
import type {
  FeatureRecord,
  GuildChannelOption,
  GuildRoleOption,
} from "../api/control";
import {
  AdvancedTextInput,
  EmptyState,
  EntityMultiPickerField,
  EntityPickerField,
  FlatPageLayout,
  KeyValueList,
  LookupNotice,
  PageHeader,
  StatusBadge,
  UnsavedChangesBar,
} from "../components/ui";
import { useDashboardSession } from "../context/DashboardSessionContext";
import {
  buildMessageRouteChannelPickerOptions,
  formatGuildChannelValue,
} from "../features/features/discordEntities";
import {
  getFeatureAreaDefinition,
  getFeatureAreaRecords,
} from "../features/features/areas";
import {
  formatAllowedRoleCountValue,
  formatAllowedRoleOptionLabel,
  formatAllowedRolesValue,
  getAdminCommandsFeatureDetails,
  getCommandsFeatureDetails,
  summarizeAdminCommandsSignal,
  summarizeCommandsSignal,
  toggleAllowedRole,
} from "../features/features/commands";
import { featureSupportsField } from "../features/features/model";
import {
  formatFeatureStatusLabel,
  formatWorkspaceStateDescription,
  formatWorkspaceStateTitle,
  getFeatureStatusTone,
  summarizeFeatureArea,
} from "../features/features/presentation";
import { useFeatureMutation } from "../features/features/useFeatureMutation";
import { useFeatureWorkspace } from "../features/features/useFeatureWorkspace";
import { useGuildChannelOptions } from "../features/features/useGuildChannelOptions";
import { useGuildRoleOptions } from "../features/features/useGuildRoleOptions";

export function CommandsPage() {
  const definition = getFeatureAreaDefinition("commands");
  const location = useLocation();
  const { authState, beginLogin, canEditSelectedGuild } = useDashboardSession();
  const workspace = useFeatureWorkspace({
    scope: "guild",
  });
  const mutation = useFeatureMutation({
    scope: "guild",
  });
  const channelOptions = useGuildChannelOptions();
  const roleOptions = useGuildRoleOptions();
  const [pendingFeatureId, setPendingFeatureId] = useState("");

  const nextPath = `${location.pathname}${location.search}${location.hash}`;
  const areaFeatures = getFeatureAreaRecords(workspace.features, "commands");
  const areaSummary = summarizeFeatureArea(areaFeatures);
  const workspaceNotice = mutation.notice ?? workspace.notice;
  const commandsFeature =
    areaFeatures.find((feature) => feature.id === "services.commands") ?? null;
  const adminCommandsFeature =
    areaFeatures.find((feature) => feature.id === "services.admin_commands") ??
    null;
  const messageRouteChannelOptions = useMemo(
    () => buildMessageRouteChannelPickerOptions(channelOptions.channels),
    [channelOptions.channels],
  );

  if (definition === null) {
    return null;
  }

  const areaLabel = definition.label;

  async function handleRefreshCommands() {
    await Promise.all([
      workspace.refresh(),
      channelOptions.refresh(),
      roleOptions.refresh(),
    ]);
  }

  async function handleSetFeatureEnabled(
    feature: FeatureRecord,
    enabled: boolean,
  ) {
    setPendingFeatureId(feature.id);

    try {
      const updated = await mutation.patchFeature(feature.id, {
        enabled,
      });
      if (updated !== null) {
        workspace.updateFeature(updated);
      }
    } finally {
      setPendingFeatureId("");
    }
  }

  async function handleUseDefault(feature: FeatureRecord) {
    setPendingFeatureId(feature.id);

    try {
      const updated = await mutation.patchFeature(feature.id, {
        enabled: null,
      });
      if (updated !== null) {
        workspace.updateFeature(updated);
      }
    } finally {
      setPendingFeatureId("");
    }
  }

  async function handleSaveCommandChannel(channelId: string) {
    if (commandsFeature === null) {
      return;
    }

    setPendingFeatureId(commandsFeature.id);

    try {
      const updated = await mutation.patchFeature(commandsFeature.id, {
        channel_id: channelId.trim(),
      });
      if (updated !== null) {
        workspace.updateFeature(updated);
      }
    } finally {
      setPendingFeatureId("");
    }
  }

  async function handleSaveAdminAccess(allowedRoleIds: string[]) {
    if (adminCommandsFeature === null) {
      return;
    }

    setPendingFeatureId(adminCommandsFeature.id);

    try {
      const updated = await mutation.patchFeature(adminCommandsFeature.id, {
        allowed_role_ids: allowedRoleIds,
      });
      if (updated !== null) {
        workspace.updateFeature(updated);
      }
    } finally {
      setPendingFeatureId("");
    }
  }

  function renderHeaderActions() {
    if (authState !== "signed_in") {
      return (
        <button
          className="button-primary"
          type="button"
          onClick={() => void beginLogin(nextPath)}
        >
          Sign in with Discord
        </button>
      );
    }

    return null;
  }

  function renderPageState() {
    if (workspace.workspaceState !== "ready") {
      return (
        <EmptyState
          title={formatWorkspaceStateTitle(areaLabel, workspace.workspaceState)}
          description={formatWorkspaceStateDescription(
            areaLabel,
            workspace.workspaceState,
          )}
          action={
            authState !== "signed_in" ? (
              <button
                className="button-primary"
                type="button"
                onClick={() => void beginLogin(nextPath)}
              >
                Sign in with Discord
              </button>
            ) : workspace.workspaceState === "unavailable" ? (
              <button
                className="button-secondary"
                type="button"
                onClick={() => void handleRefreshCommands()}
              >
                Retry loading
              </button>
            ) : undefined
          }
        />
      );
    }

    if (commandsFeature === null || adminCommandsFeature === null) {
      return (
        <div className="table-empty-state table-empty-state-compact">
          <div className="card-copy">
            <p className="section-label">Workspace</p>
            <h2>No command controls yet</h2>
            <p className="section-description">
              The selected server does not expose the command feature records
              required by this workspace yet.
            </p>
          </div>
        </div>
      );
    }

    return (
      <div className="flat-config-stack commands-workspace">
        <CommandChannelSection
          feature={commandsFeature}
          availableChannels={channelOptions.channels}
          channelOptions={messageRouteChannelOptions}
          channelOptionsLoading={channelOptions.loading}
          channelOptionsNotice={channelOptions.notice}
          canEditSelectedGuild={canEditSelectedGuild}
          mutationSaving={mutation.saving}
          pendingFeatureId={pendingFeatureId}
          onClearNotice={mutation.clearNotice}
          onRefreshChannelOptions={channelOptions.refresh}
          onSave={handleSaveCommandChannel}
          onSetFeatureEnabled={handleSetFeatureEnabled}
          onUseDefault={handleUseDefault}
        />

        <AdminCommandAccessSection
          feature={adminCommandsFeature}
          roleOptions={roleOptions.roles}
          roleOptionsLoading={roleOptions.loading}
          roleOptionsNotice={roleOptions.notice}
          canEditSelectedGuild={canEditSelectedGuild}
          mutationSaving={mutation.saving}
          pendingFeatureId={pendingFeatureId}
          onClearNotice={mutation.clearNotice}
          onRefreshRoleOptions={roleOptions.refresh}
          onSave={handleSaveAdminAccess}
          onSetFeatureEnabled={handleSetFeatureEnabled}
          onUseDefault={handleUseDefault}
        />
      </div>
    );
  }

  return (
    <section className="page-shell">
      <PageHeader
        eyebrow="Feature area"
        title={areaLabel}
        description="Configure command routing and privileged command access for the selected server without falling back to the generic feature list."
        status={
          <StatusBadge
            tone={
              workspace.workspaceState === "ready" ? areaSummary.tone : "info"
            }
          >
            {workspace.workspaceState === "ready"
              ? areaSummary.label
              : formatWorkspaceStateTitle(areaLabel, workspace.workspaceState)}
          </StatusBadge>
        }
        actions={renderHeaderActions()}
      />

      <FlatPageLayout
        notice={workspaceNotice}
        summary={
          workspace.workspaceState === "ready" &&
          commandsFeature !== null &&
          adminCommandsFeature !== null ? (
            <section
              className="commands-context-strip"
              aria-label="Commands summary"
            >
              <div className="commands-context-item">
                <p className="section-label">Command channel</p>
                <strong>
                  {formatGuildChannelValue(
                    getCommandsFeatureDetails(commandsFeature).channelId,
                    channelOptions.channels,
                    "No dedicated channel",
                  )}
                </strong>
                <p className="meta-note">
                  {summarizeCommandsSignal(commandsFeature)}
                </p>
              </div>

              <div className="commands-context-item">
                <p className="section-label">Admin access</p>
                <strong>
                  {formatAllowedRoleCountValue(adminCommandsFeature)}
                </strong>
                <p className="meta-note">
                  {formatAllowedRolesValue(
                    adminCommandsFeature,
                    roleOptions.roles,
                  )}
                </p>
              </div>
            </section>
          ) : null
        }
        workspaceTitle="Command controls"
        workspaceDescription="Keep routing and privileged access visible in the main workspace instead of moving the primary command setup into a drawer."
      >
        {renderPageState()}
      </FlatPageLayout>
    </section>
  );
}

interface CommandChannelSectionProps {
  feature: FeatureRecord;
  availableChannels: GuildChannelOption[];
  channelOptions: Array<{ value: string; label: string; description?: string }>;
  channelOptionsLoading: boolean;
  channelOptionsNotice: {
    tone: "info" | "success" | "error";
    message: string;
  } | null;
  canEditSelectedGuild: boolean;
  mutationSaving: boolean;
  pendingFeatureId: string;
  onClearNotice: () => void;
  onRefreshChannelOptions: () => Promise<void>;
  onSave: (channelId: string) => Promise<void>;
  onSetFeatureEnabled: (
    feature: FeatureRecord,
    enabled: boolean,
  ) => Promise<void>;
  onUseDefault: (feature: FeatureRecord) => Promise<void>;
}

function CommandChannelSection({
  feature,
  availableChannels,
  channelOptions,
  channelOptionsLoading,
  channelOptionsNotice,
  canEditSelectedGuild,
  mutationSaving,
  pendingFeatureId,
  onClearNotice,
  onRefreshChannelOptions,
  onSave,
  onSetFeatureEnabled,
  onUseDefault,
}: CommandChannelSectionProps) {
  const details = getCommandsFeatureDetails(feature);
  const [channelDraft, setChannelDraft] = useState(details.channelId);
  const canEditChannel =
    canEditSelectedGuild && featureSupportsField(feature, "channel_id");
  const hasUnsavedChanges = details.channelId !== channelDraft.trim();

  useEffect(() => {
    setChannelDraft(details.channelId);
  }, [details.channelId]);

  function handleReset() {
    onClearNotice();
    setChannelDraft(details.channelId);
  }

  return (
    <section className="flat-config-section commands-flat-section">
      <div className="flat-config-header">
        <div className="card-copy flat-config-copy">
          <p className="section-label">Commands</p>
          <div className="flat-config-title-row commands-module-title-row">
            <h2>Command routing</h2>
            <StatusBadge tone={getFeatureStatusTone(feature)}>
              {formatFeatureStatusLabel(feature)}
            </StatusBadge>
          </div>
          <p className="section-description">
            Set the optional command destination and keep the routing controls
            in the main workspace.
          </p>
        </div>
      </div>

      <KeyValueList
        items={[
          {
            label: "Command channel",
            value: formatGuildChannelValue(
              details.channelId,
              availableChannels,
              "No dedicated channel",
            ),
          },
          {
            label: "Current signal",
            value: summarizeCommandsSignal(feature),
          },
        ]}
      />

      {channelOptionsNotice ? (
        <LookupNotice
          as="section"
          title="Channel references unavailable"
          message={channelOptionsNotice.message}
          retryLabel="Retry channel lookup"
          retryDisabled={channelOptionsLoading}
          onRetry={onRefreshChannelOptions}
        />
      ) : null}

      <div className="flat-config-fields">
        <EntityPickerField
          label="Command channel"
          value={channelDraft}
          disabled={!canEditChannel || channelOptionsLoading}
          onChange={setChannelDraft}
          options={channelOptions}
          placeholder={
            channelOptionsLoading
              ? "Loading channels..."
              : channelOptions.length === 0
                ? "No channels available"
                : "No dedicated channel"
          }
          note="Leave this empty when command handling should stay available without a dedicated routing destination."
        />

        <AdvancedTextInput
          label="Channel ID fallback"
          inputLabel="Command channel ID fallback"
          value={channelDraft}
          disabled={!canEditChannel}
          onChange={setChannelDraft}
          placeholder="Discord channel ID"
          note="Use this only when the channel picker is unavailable or when you need to paste a channel ID directly."
        />
      </div>

      <div className="inline-actions commands-module-actions flat-config-actions">
        <button
          className="button-secondary"
          type="button"
          disabled={mutationSaving || !canEditSelectedGuild}
          onClick={() =>
            void onSetFeatureEnabled(feature, !feature.effective_enabled)
          }
        >
          {mutationSaving && pendingFeatureId === feature.id
            ? "Saving..."
            : feature.effective_enabled
              ? "Disable commands"
              : "Enable commands"}
        </button>
        {feature.override_state !== "inherit" ? (
          <button
            className="button-ghost"
            type="button"
            disabled={mutationSaving || !canEditSelectedGuild}
            onClick={() => void onUseDefault(feature)}
          >
            Use default
          </button>
        ) : null}
      </div>

      <UnsavedChangesBar
        hasUnsavedChanges={hasUnsavedChanges}
        saveLabel={
          mutationSaving && pendingFeatureId === feature.id
            ? "Saving..."
            : "Save changes"
        }
        saving={mutationSaving && pendingFeatureId === feature.id}
        disabled={!canEditChannel}
        onReset={handleReset}
        onSave={() => onSave(channelDraft)}
      />
    </section>
  );
}

interface AdminCommandAccessSectionProps {
  feature: FeatureRecord;
  roleOptions: GuildRoleOption[];
  roleOptionsLoading: boolean;
  roleOptionsNotice: {
    tone: "info" | "success" | "error";
    message: string;
  } | null;
  canEditSelectedGuild: boolean;
  mutationSaving: boolean;
  pendingFeatureId: string;
  onClearNotice: () => void;
  onRefreshRoleOptions: () => Promise<void>;
  onSave: (allowedRoleIds: string[]) => Promise<void>;
  onSetFeatureEnabled: (
    feature: FeatureRecord,
    enabled: boolean,
  ) => Promise<void>;
  onUseDefault: (feature: FeatureRecord) => Promise<void>;
}

function AdminCommandAccessSection({
  feature,
  roleOptions,
  roleOptionsLoading,
  roleOptionsNotice,
  canEditSelectedGuild,
  mutationSaving,
  pendingFeatureId,
  onClearNotice,
  onRefreshRoleOptions,
  onSave,
  onSetFeatureEnabled,
  onUseDefault,
}: AdminCommandAccessSectionProps) {
  const details = getAdminCommandsFeatureDetails(feature);
  const allowedRoleIdsSnapshot = JSON.stringify(details.allowedRoleIds);
  const [allowedRoleIdsDraft, setAllowedRoleIdsDraft] = useState(
    details.allowedRoleIds,
  );
  const canEditRoles =
    canEditSelectedGuild && featureSupportsField(feature, "allowed_role_ids");
  const hasUnsavedChanges = !areStringListsEqual(
    details.allowedRoleIds,
    allowedRoleIdsDraft,
  );

  useEffect(() => {
    setAllowedRoleIdsDraft(JSON.parse(allowedRoleIdsSnapshot) as string[]);
  }, [allowedRoleIdsSnapshot]);

  function handleReset() {
    onClearNotice();
    setAllowedRoleIdsDraft(details.allowedRoleIds);
  }

  return (
    <section className="flat-config-section commands-flat-section">
      <div className="flat-config-header">
        <div className="card-copy flat-config-copy">
          <p className="section-label">Admin access</p>
          <div className="flat-config-title-row commands-module-title-row">
            <h2>Admin command access</h2>
            <StatusBadge tone={getFeatureStatusTone(feature)}>
              {formatFeatureStatusLabel(feature)}
            </StatusBadge>
          </div>
          <p className="section-description">
            Keep privileged command access visible here instead of hiding the
            role selection in a separate editor.
          </p>
        </div>
      </div>

      <KeyValueList
        items={[
          {
            label: "Allowed roles",
            value: formatAllowedRolesValue(feature, roleOptions),
          },
          {
            label: "Current signal",
            value: summarizeAdminCommandsSignal(feature),
          },
        ]}
      />

      {roleOptionsNotice ? (
        <LookupNotice
          as="section"
          title="Role references unavailable"
          message={roleOptionsNotice.message}
          retryLabel="Retry role lookup"
          retryDisabled={roleOptionsLoading}
          onRetry={onRefreshRoleOptions}
        />
      ) : roleOptionsLoading && roleOptions.length === 0 ? (
        <div className="surface-subsection">
          <p className="section-label">Loading roles</p>
          <p className="meta-note">
            Loading the current server roles before privileged access can be
            updated.
          </p>
        </div>
      ) : roleOptions.length === 0 ? (
        <div className="surface-subsection">
          <p className="section-label">No roles available</p>
          <p className="meta-note">
            This server did not return any selectable roles for privileged
            command access.
          </p>
        </div>
      ) : null}

      <div className="flat-config-fields">
        <EntityMultiPickerField
          label="Allowed roles"
          disabled={
            !canEditRoles || roleOptionsLoading || roleOptionsNotice !== null
          }
          selectedValues={allowedRoleIdsDraft}
          onToggle={(roleId) =>
            setAllowedRoleIdsDraft((current) =>
              toggleAllowedRole(current, roleId),
            )
          }
          note="Choose only the roles that should be able to run privileged command workflows."
          options={roleOptions.map((role) => ({
            value: role.id,
            label: formatAllowedRoleOptionLabel(role),
            description: role.is_default
              ? "Default role for every member."
              : role.managed
                ? "Managed by an integration."
                : "Available for privileged command access.",
          }))}
        />
      </div>

      <div className="inline-actions commands-module-actions flat-config-actions">
        <button
          className="button-secondary"
          type="button"
          disabled={mutationSaving || !canEditSelectedGuild}
          onClick={() =>
            void onSetFeatureEnabled(feature, !feature.effective_enabled)
          }
        >
          {mutationSaving && pendingFeatureId === feature.id
            ? "Saving..."
            : feature.effective_enabled
              ? "Disable admin commands"
              : "Enable admin commands"}
        </button>
        {feature.override_state !== "inherit" ? (
          <button
            className="button-ghost"
            type="button"
            disabled={mutationSaving || !canEditSelectedGuild}
            onClick={() => void onUseDefault(feature)}
          >
            Use default
          </button>
        ) : null}
      </div>

      <UnsavedChangesBar
        hasUnsavedChanges={hasUnsavedChanges}
        saveLabel={
          mutationSaving && pendingFeatureId === feature.id
            ? "Saving..."
            : "Save changes"
        }
        saving={mutationSaving && pendingFeatureId === feature.id}
        disabled={
          !canEditRoles || roleOptionsLoading || roleOptionsNotice !== null
        }
        onReset={handleReset}
        onSave={() => onSave(allowedRoleIdsDraft)}
      />
    </section>
  );
}

function areStringListsEqual(currentValues: string[], nextValues: string[]) {
  return (
    currentValues.length === nextValues.length &&
    currentValues.every((value, index) => value === nextValues[index])
  );
}
