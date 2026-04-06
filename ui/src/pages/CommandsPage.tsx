import { useEffect, useMemo, useState } from "react";
import { useLocation } from "react-router-dom";
import type {
  FeatureRecord,
  GuildChannelOption,
  GuildRoleOption,
} from "../api/control";
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
  canEditAdminCommands,
  canEditCommandsChannel,
  formatAllowedRoleCountValue,
  formatAllowedRoleOptionLabel,
  formatAllowedRolesValue,
  getAdminCommandsFeatureDetails,
  getCommandsFeatureDetails,
  summarizeAdminCommandsSignal,
  summarizeCommandsSignal,
  toggleAllowedRole,
} from "../features/features/commands";
import {
  formatFeatureStatusLabel,
  formatWorkspaceStateDescription,
  formatWorkspaceStateTitle,
  getFeatureStatusTone,
  summarizeFeatureArea,
} from "../features/features/presentation";
import { useFeatureMutation } from "../features/features/useFeatureMutation";
import { useFeatureWorkspace } from "../features/features/useFeatureWorkspace";
import { useGuildRoleOptions } from "../features/features/useGuildRoleOptions";
import {
  AdvancedTextInput,
  AlertBanner,
  DashboardPageSurface,
  EntityMultiPickerField,
  EntityPickerField,
  EmptyState,
  KeyValueList,
  PageHeader,
  StatusBadge,
} from "../components/ui";
import { useGuildChannelOptions } from "../features/features/useGuildChannelOptions";

export function CommandsPage() {
  const definition = getFeatureAreaDefinition("commands");
  const location = useLocation();
  const {
    authState,
    beginLogin,
    canEditSelectedGuild,
    currentOriginLabel,
    selectedGuild,
  } = useDashboardSession();
  const workspace = useFeatureWorkspace({
    scope: "guild",
  });
  const mutation = useFeatureMutation({
    scope: "guild",
  });
  const channelOptions = useGuildChannelOptions();
  const roleOptions = useGuildRoleOptions();
  const [pendingFeatureId, setPendingFeatureId] = useState("");
  const [selectedFeatureId, setSelectedFeatureId] = useState("");
  const [channelDraft, setChannelDraft] = useState("");
  const [allowedRoleIdsDraft, setAllowedRoleIdsDraft] = useState<string[]>([]);

  const nextPath = `${location.pathname}${location.search}${location.hash}`;
  const areaFeatures = getFeatureAreaRecords(workspace.features, "commands");
  const areaSummary = summarizeFeatureArea(areaFeatures);
  const selectedServerLabel = selectedGuild?.name ?? "No server selected";
  const workspaceNotice = mutation.notice ?? workspace.notice;
  const commandsFeature =
    areaFeatures.find((feature) => feature.id === "services.commands") ?? null;
  const adminCommandsFeature =
    areaFeatures.find((feature) => feature.id === "services.admin_commands") ?? null;
  const selectedFeature =
    areaFeatures.find((feature) => feature.id === selectedFeatureId) ?? null;
  const localOverrides = areaFeatures.filter(
    (feature) => feature.override_state !== "inherit",
  ).length;
  const enabledModules = areaFeatures.filter(
    (feature) => feature.effective_enabled,
  ).length;
  const messageRouteChannelOptions = useMemo(
    () => buildMessageRouteChannelPickerOptions(channelOptions.channels),
    [channelOptions.channels],
  );

  useEffect(() => {
    if (canEditSelectedGuild) {
      return;
    }
    closeDrawer();
  }, [canEditSelectedGuild]);

  useEffect(() => {
    if (selectedFeature === null) {
      return;
    }

    switch (selectedFeature.id) {
      case "services.commands":
        setChannelDraft(getCommandsFeatureDetails(selectedFeature).channelId);
        return;
      case "services.admin_commands":
        setAllowedRoleIdsDraft(
          getAdminCommandsFeatureDetails(selectedFeature).allowedRoleIds,
        );
        return;
      default:
        return;
    }
  }, [selectedFeature]);

  if (definition === null) {
    return null;
  }

  const areaLabel = definition.label;

  function closeDrawer() {
    setSelectedFeatureId("");
    setChannelDraft("");
    setAllowedRoleIdsDraft([]);
  }

  function openDrawer(feature: FeatureRecord) {
    if (!canEditSelectedGuild) {
      return;
    }
    if (
      feature.id === "services.commands" &&
      !canEditCommandsChannel(feature)
    ) {
      return;
    }
    if (
      feature.id === "services.admin_commands" &&
      !canEditAdminCommands(feature)
    ) {
      return;
    }

    setSelectedFeatureId(feature.id);
  }

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
        await workspace.refresh();
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
        await workspace.refresh();
      }
    } finally {
      setPendingFeatureId("");
    }
  }

  async function handleSaveCommandChannel() {
    if (commandsFeature === null) {
      return;
    }

    setPendingFeatureId(commandsFeature.id);

    try {
      const updated = await mutation.patchFeature(commandsFeature.id, {
        channel_id: channelDraft.trim(),
      });
      if (updated !== null) {
        await workspace.refresh();
        closeDrawer();
      }
    } finally {
      setPendingFeatureId("");
    }
  }

  async function handleSaveAdminAccess() {
    if (adminCommandsFeature === null) {
      return;
    }

    setPendingFeatureId(adminCommandsFeature.id);

    try {
      const updated = await mutation.patchFeature(adminCommandsFeature.id, {
        allowed_role_ids: allowedRoleIdsDraft,
      });
      if (updated !== null) {
        await workspace.refresh();
        closeDrawer();
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

    const commandDetails = getCommandsFeatureDetails(commandsFeature);
    const adminDetails = getAdminCommandsFeatureDetails(adminCommandsFeature);

    return (
      <div className="workspace-view commands-workspace">
        <section className="commands-flat-section">
          <div className="commands-module-head">
            <div className="card-copy commands-module-copy">
              <p className="section-label">Commands</p>
              <div className="commands-module-title-row">
                <h2>Command routing</h2>
                <StatusBadge tone={getFeatureStatusTone(commandsFeature)}>
                  {formatFeatureStatusLabel(commandsFeature)}
                </StatusBadge>
              </div>
              <p className="section-description">
                Set the optional command destination and control whether command handling is enabled for this server.
              </p>
            </div>
          </div>

          <KeyValueList
            items={[
              {
                label: "Module state",
                value: commandsFeature.effective_enabled ? "Enabled" : "Disabled",
              },
              {
                label: "Command channel",
                value: formatGuildChannelValue(
                  commandDetails.channelId,
                  channelOptions.channels,
                  "No dedicated channel",
                ),
              },
              {
                label: "Current signal",
                value: summarizeCommandsSignal(commandsFeature),
              },
            ]}
          />

          {channelOptions.notice ? (
            <div className="flat-inline-message">
              <p className="meta-note">{channelOptions.notice.message}</p>
              <div className="inline-actions">
                <button
                  className="button-secondary"
                  type="button"
                  disabled={channelOptions.loading}
                  onClick={() => void channelOptions.refresh()}
                >
                  Retry channel lookup
                </button>
              </div>
            </div>
          ) : null}

          <p className="meta-note">
            {commandDetails.channelId === ""
              ? "Leave the command channel empty when command handling should stay available without a dedicated routing destination."
              : "The configured command channel keeps command setup and follow-up actions in one place."}
          </p>

          <div className="inline-actions commands-module-actions">
            <button
              className="button-primary"
              type="button"
              disabled={
                !canEditSelectedGuild ||
                !canEditCommandsChannel(commandsFeature) ||
                mutation.saving
              }
              onClick={() => openDrawer(commandsFeature)}
            >
              Configure command channel
            </button>
            <button
              className="button-secondary"
              type="button"
              disabled={mutation.saving || !canEditSelectedGuild}
              onClick={() =>
                void handleSetFeatureEnabled(
                  commandsFeature,
                  !commandsFeature.effective_enabled,
                )
              }
            >
              {mutation.saving && pendingFeatureId === commandsFeature.id
                ? "Saving..."
                : commandsFeature.effective_enabled
                  ? "Disable commands"
                  : "Enable commands"}
            </button>
            {commandsFeature.override_state !== "inherit" ? (
              <button
                className="button-ghost"
                type="button"
                disabled={mutation.saving || !canEditSelectedGuild}
                onClick={() => void handleUseDefault(commandsFeature)}
              >
                Use default
              </button>
            ) : null}
          </div>
        </section>

        <section className="commands-flat-section">
          <div className="commands-module-head">
            <div className="card-copy commands-module-copy">
              <p className="section-label">Admin access</p>
              <div className="commands-module-title-row">
                <h2>Admin command access</h2>
                <StatusBadge tone={getFeatureStatusTone(adminCommandsFeature)}>
                  {formatFeatureStatusLabel(adminCommandsFeature)}
                </StatusBadge>
              </div>
              <p className="section-description">
                Limit privileged command workflows to the roles configured for this server.
              </p>
            </div>
          </div>

          <KeyValueList
            items={[
              {
                label: "Module state",
                value: adminCommandsFeature.effective_enabled ? "Enabled" : "Disabled",
              },
              {
                label: "Allowed roles",
                value: formatAllowedRolesValue(adminCommandsFeature, roleOptions.roles),
              },
              {
                label: "Current signal",
                value: summarizeAdminCommandsSignal(adminCommandsFeature),
              },
            ]}
          />

          {roleOptions.notice ? (
            <div className="flat-inline-message">
              <p className="meta-note">{roleOptions.notice.message}</p>
              <div className="inline-actions">
                <button
                  className="button-secondary"
                  type="button"
                  disabled={roleOptions.loading}
                  onClick={() => void roleOptions.refresh()}
                >
                  Retry role lookup
                </button>
              </div>
            </div>
          ) : null}

          <p className="meta-note">
            {adminDetails.allowedRoleCount === 0
              ? "Choose which server roles should be allowed to use admin-only command workflows."
              : "Review the selected roles whenever command privileges need to change for this server."}
          </p>

          <div className="inline-actions commands-module-actions">
            <button
              className="button-primary"
              type="button"
              disabled={
                !canEditSelectedGuild ||
                !canEditAdminCommands(adminCommandsFeature) ||
                mutation.saving
              }
              onClick={() => openDrawer(adminCommandsFeature)}
            >
              Configure admin access
            </button>
            <button
              className="button-secondary"
              type="button"
              disabled={mutation.saving || !canEditSelectedGuild}
              onClick={() =>
                void handleSetFeatureEnabled(
                  adminCommandsFeature,
                  !adminCommandsFeature.effective_enabled,
                )
              }
            >
              {mutation.saving && pendingFeatureId === adminCommandsFeature.id
                ? "Saving..."
                : adminCommandsFeature.effective_enabled
                  ? "Disable admin commands"
                  : "Enable admin commands"}
            </button>
            {adminCommandsFeature.override_state !== "inherit" ? (
              <button
                className="button-ghost"
                type="button"
                disabled={mutation.saving || !canEditSelectedGuild}
                onClick={() => void handleUseDefault(adminCommandsFeature)}
              >
                Use default
              </button>
            ) : null}
          </div>
        </section>
      </div>
    );
  }

  return (
    <>
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
                : formatWorkspaceStateTitle(
                  areaLabel,
                  workspace.workspaceState,
                )}
            </StatusBadge>
          }
          meta={
            <>
              <span className="meta-note">Server: {selectedServerLabel}</span>
              <span className="meta-note">Origin: {currentOriginLabel}</span>
            </>
          }
          actions={renderHeaderActions()}
        />

        <DashboardPageSurface notice={workspaceNotice}>
          {workspace.workspaceState === "ready" &&
          commandsFeature !== null &&
          adminCommandsFeature !== null ? (
            <section className="commands-context-strip" aria-label="Commands summary">
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
                <strong>{formatAllowedRoleCountValue(adminCommandsFeature)}</strong>
                <p className="meta-note">
                  {formatAllowedRolesValue(adminCommandsFeature, roleOptions.roles)}
                </p>
              </div>

              <div className="commands-context-item">
                <p className="section-label">Overrides</p>
                <strong>{localOverrides}</strong>
                <p className="meta-note">
                  {enabledModules}/{areaFeatures.length} modules enabled for this server.
                </p>
              </div>
            </section>
          ) : null}

          {renderPageState()}
        </DashboardPageSurface>
      </section>

      {selectedFeature !== null ? (
        <div className="drawer-backdrop" onClick={closeDrawer} role="presentation">
          <aside
            aria-label={getDrawerLabel(selectedFeature)}
            aria-modal="true"
            className="drawer-panel commands-drawer"
            onClick={(event) => event.stopPropagation()}
            role="dialog"
          >
            <div className="card-copy">
              <p className="section-label">Commands</p>
              <div className="logging-drawer-title-row">
                <h2>{selectedFeature.label}</h2>
                <StatusBadge tone={getFeatureStatusTone(selectedFeature)}>
                  {formatFeatureStatusLabel(selectedFeature)}
                </StatusBadge>
              </div>
              <p className="section-description">{selectedFeature.description}</p>
            </div>

            {mutation.notice ? <AlertBanner notice={mutation.notice} /> : null}

            {renderDrawerBody({
              selectedFeature,
              pendingFeatureId,
              mutationSaving: mutation.saving,
              channelDraft,
              allowedRoleIdsDraft,
              availableChannels: channelOptions.channels,
              channelOptions: messageRouteChannelOptions,
              channelOptionsNotice: channelOptions.notice,
              channelOptionsLoading: channelOptions.loading,
              roleOptions: roleOptions.roles,
              roleOptionsLoading: roleOptions.loading,
              roleOptionsNotice: roleOptions.notice,
              refreshChannelOptions: channelOptions.refresh,
              setChannelDraft,
              setAllowedRoleIdsDraft,
              closeDrawer,
              refreshRoleOptions: roleOptions.refresh,
              handleSaveCommandChannel,
              handleSaveAdminAccess,
            })}
          </aside>
        </div>
      ) : null}
    </>
  );
}

function getDrawerLabel(feature: FeatureRecord) {
  switch (feature.id) {
    case "services.commands":
      return "Configure commands";
    case "services.admin_commands":
      return "Configure admin commands";
    default:
      return `Configure ${feature.label}`;
  }
}

interface RenderDrawerBodyProps {
  selectedFeature: FeatureRecord;
  pendingFeatureId: string;
  mutationSaving: boolean;
  channelDraft: string;
  allowedRoleIdsDraft: string[];
  availableChannels: GuildChannelOption[];
  channelOptions: Array<{ value: string; label: string; description?: string }>;
  channelOptionsLoading: boolean;
  channelOptionsNotice: { tone: "info" | "success" | "error"; message: string } | null;
  roleOptions: GuildRoleOption[];
  roleOptionsLoading: boolean;
  roleOptionsNotice: { tone: "info" | "success" | "error"; message: string } | null;
  setChannelDraft: (value: string) => void;
  setAllowedRoleIdsDraft: (
    value: string[] | ((current: string[]) => string[]),
  ) => void;
  closeDrawer: () => void;
  refreshChannelOptions: () => Promise<void>;
  refreshRoleOptions: () => Promise<void>;
  handleSaveCommandChannel: () => Promise<void>;
  handleSaveAdminAccess: () => Promise<void>;
}

function renderDrawerBody({
  selectedFeature,
  pendingFeatureId,
  mutationSaving,
  channelDraft,
  allowedRoleIdsDraft,
  availableChannels,
  channelOptions,
  channelOptionsLoading,
  channelOptionsNotice,
  roleOptions,
  roleOptionsLoading,
  roleOptionsNotice,
  setChannelDraft,
  setAllowedRoleIdsDraft,
  closeDrawer,
  refreshChannelOptions,
  refreshRoleOptions,
  handleSaveCommandChannel,
  handleSaveAdminAccess,
}: RenderDrawerBodyProps) {
  if (selectedFeature.id === "services.commands") {
    const details = getCommandsFeatureDetails(selectedFeature);

    return (
      <>
        <KeyValueList
          items={[
            {
              label: "Module state",
              value: selectedFeature.effective_enabled ? "On" : "Off",
            },
            {
              label: "Current channel",
              value: formatGuildChannelValue(
                details.channelId,
                availableChannels,
                "No dedicated channel",
              ),
            },
            {
              label: "Current signal",
              value: summarizeCommandsSignal(selectedFeature),
            },
          ]}
        />

        {channelOptionsNotice ? (
          <div className="flat-inline-message">
            <p className="meta-note">{channelOptionsNotice.message}</p>
            <div className="inline-actions">
              <button
                className="button-secondary"
                type="button"
                disabled={channelOptionsLoading}
                onClick={() => void refreshChannelOptions()}
              >
                Retry channel lookup
              </button>
            </div>
          </div>
        ) : null}

        <EntityPickerField
          label="Command channel"
          value={channelDraft}
          disabled={channelOptionsLoading}
          onChange={setChannelDraft}
          options={channelOptions}
          placeholder={
            channelOptionsLoading
              ? "Loading channels..."
              : channelOptions.length === 0
                ? "No channels available"
                : "No dedicated channel"
          }
          note="Leave this empty when you do not want a dedicated routing channel for command workflows."
        />

          <AdvancedTextInput
            label="Channel ID fallback"
            inputLabel="Command channel ID fallback"
            value={channelDraft}
            onChange={setChannelDraft}
            placeholder="Discord channel ID"
            note="Use this only when the channel picker is unavailable or when you need to paste a channel ID directly."
          />

        <p className="meta-note">
          Use one command channel when setup and follow-up actions should stay in a single place.
        </p>

        <div className="drawer-actions">
          <button
            className="button-primary"
            type="button"
            disabled={mutationSaving}
            onClick={() => void handleSaveCommandChannel()}
          >
            {mutationSaving && pendingFeatureId === selectedFeature.id
              ? "Saving..."
              : "Save command channel"}
          </button>
          <button className="button-secondary" type="button" onClick={closeDrawer}>
            Cancel
          </button>
        </div>
      </>
    );
  }

  const adminDetails = getAdminCommandsFeatureDetails(selectedFeature);

  return (
    <>
      <KeyValueList
        items={[
          {
            label: "Module state",
            value: selectedFeature.effective_enabled ? "On" : "Off",
          },
          {
            label: "Allowed roles",
            value: formatAllowedRoleCountValue(selectedFeature),
          },
          {
            label: "Current signal",
            value: summarizeAdminCommandsSignal(selectedFeature),
          },
        ]}
      />

      {roleOptionsNotice ? (
        <div className="flat-inline-message">
          <p className="meta-note">{roleOptionsNotice.message}</p>
          <div className="inline-actions">
            <button
              className="button-secondary"
              type="button"
              disabled={roleOptionsLoading}
              onClick={() => void refreshRoleOptions()}
            >
              Retry role lookup
            </button>
          </div>
        </div>
      ) : roleOptionsLoading && roleOptions.length === 0 ? (
        <div className="flat-inline-message">
          <p className="meta-note">
            Loading the current server roles before privileged access can be updated.
          </p>
        </div>
      ) : roleOptions.length === 0 ? (
        <div className="flat-inline-message">
          <p className="meta-note">
            This server did not return any selectable roles for privileged command access.
          </p>
        </div>
      ) : (
        <EntityMultiPickerField
          label="Allowed roles"
          disabled={roleOptionsLoading}
          selectedValues={allowedRoleIdsDraft}
          onToggle={(roleId) =>
            setAllowedRoleIdsDraft((current) => toggleAllowedRole(current, roleId))
          }
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
      )}

      <p className="meta-note">
        {adminDetails.allowedRoleCount > 0
          ? `The current configuration already grants access to ${formatAllowedRoleCountValue(selectedFeature).toLowerCase()}.`
          : "Choose only the roles that should be able to run privileged command workflows."}
      </p>

      <div className="drawer-actions">
        <button
          className="button-primary"
          type="button"
          disabled={mutationSaving || roleOptionsLoading || roleOptionsNotice !== null}
          onClick={() => void handleSaveAdminAccess()}
        >
          {mutationSaving && pendingFeatureId === selectedFeature.id
            ? "Saving..."
            : "Save admin access"}
        </button>
        <button className="button-secondary" type="button" onClick={closeDrawer}>
          Cancel
        </button>
      </div>
    </>
  );
}
